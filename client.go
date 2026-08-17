// Package seatlayer is the official Go server SDK for the SeatLayer
// reserved-seating API.
//
// Server-side only: this package authenticates with your secret key. Never
// embed it in anything a ticket buyer can reach — browser surfaces get
// short-lived, origin-bound tokens that you mint with Sessions.
//
//	client, err := seatlayer.New(os.Getenv("SEATLAYER_SECRET_KEY"))
//	if err != nil {
//		return err
//	}
//	held, err := client.Inventory.HoldBestAvailable(ctx, "summer-gala",
//		seatlayer.BestAvailableParams{Qty: 4})
package seatlayer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the public API.
	DefaultBaseURL = "https://api.seatlayer.io"
	// DefaultMaxRetries counts total attempts, not extra ones.
	DefaultMaxRetries = 3
	// DefaultTimeout applies per attempt.
	DefaultTimeout = 30 * time.Second

	userAgent = "seatlayer-go"
)

// idempotencyKeyPattern is the API's own charset for Idempotency-Key.
var idempotencyKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// Client talks to the SeatLayer server API.
//
// It is safe for concurrent use: the underlying http.Client is, and Client
// holds no mutable state of its own.
type Client struct {
	Charts     *ChartsService
	Channels   *ChannelsService
	Events     *EventsService
	Inventory  *InventoryService
	Sessions   *SessionsService
	Templates  *TemplatesService
	Webhooks   *WebhooksService
	Workspaces *WorkspacesService

	secretKey  string
	baseURL    string
	maxRetries int
	mode       string
	httpClient *http.Client
}

type rawRequestBody struct {
	data        []byte
	contentType string
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL points the client at a different API host.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) { c.baseURL = strings.TrimRight(baseURL, "/") }
}

// WithMaxRetries sets total attempts for retryable failures.
func WithMaxRetries(attempts int) Option {
	return func(c *Client) { c.maxRetries = attempts }
}

// WithHTTPClient supplies your own http.Client — for a custom transport, a
// proxy, or a test server. Its Timeout applies per attempt.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) { c.httpClient = httpClient }
}

// New builds a Client from a secret key.
//
// It returns an error rather than panicking on a bad key, so a misconfigured
// deployment fails at startup with a message that names the problem.
func New(secretKey string, options ...Option) (*Client, error) {
	if secretKey == "" {
		return nil, errors.New("seatlayer: a secret key is required")
	}
	// Caught here rather than as a 401 three round-trips later. The pk_ case
	// gets its own message: it is the one people paste by mistake.
	if strings.HasPrefix(secretKey, "pk_") {
		return nil, errors.New(
			"seatlayer: that is a publishable key; the server SDK needs a secret key (sk_live_… or sk_test_…)")
	}
	if !strings.HasPrefix(secretKey, "sk_") {
		return nil, errors.New("seatlayer: a secret key starts with sk_live_ or sk_test_")
	}

	client := &Client{
		secretKey:  secretKey,
		baseURL:    DefaultBaseURL,
		maxRetries: DefaultMaxRetries,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}
	switch {
	case strings.HasPrefix(secretKey, "sk_test_"):
		client.mode = "test"
	case strings.HasPrefix(secretKey, "sk_live_"):
		client.mode = "live"
	default:
		client.mode = "unknown"
	}

	for _, option := range options {
		option(client)
	}

	client.Charts = &ChartsService{client: client}
	client.Channels = &ChannelsService{client: client}
	client.Events = &EventsService{client: client}
	client.Inventory = &InventoryService{client: client}
	client.Sessions = &SessionsService{client: client}
	client.Templates = &TemplatesService{client: client}
	client.Webhooks = &WebhooksService{client: client}
	client.Workspaces = &WorkspacesService{client: client}

	return client, nil
}

// Mode reports "live" or "test", derived from the key prefix.
func (c *Client) Mode() string { return c.mode }

// Ready runs the dependency-aware readiness probe.
func (c *Client) Ready(ctx context.Context) (map[string]any, error) {
	return c.Do(ctx, http.MethodGet, "/health/ready", nil, nil, "")
}

// Do is the escape hatch for surface this SDK does not wrap yet. Reads retain
// retries; raw mutations are single-attempt because their replay contract is
// unknown.
func (c *Client) Do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	idempotencyKey string,
) (map[string]any, error) {
	return c.do(ctx, method, path, query, body, idempotencyKey, false)
}

// do keeps mutation retry policy out of the raw escape hatch. Only resource
// methods backed by the API's exact response replay opt in to headerReplay.
func (c *Client) do(
	ctx context.Context,
	method, path string,
	query url.Values,
	body any,
	idempotencyKey string,
	headerReplay bool,
) (map[string]any, error) {
	endpoint := c.baseURL + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var payload []byte
	contentType := "application/json"
	if body != nil {
		if raw, ok := body.(rawRequestBody); ok {
			payload = append([]byte(nil), raw.data...)
			contentType = raw.contentType
			if contentType == "" {
				contentType = "application/octet-stream"
			}
		} else {
			encoded, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("seatlayer: encoding request body: %w", err)
			}
			payload = encoded
		}
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+c.secretKey)
	header.Set("Accept", "application/json")
	header.Set("User-Agent", userAgent)
	if payload != nil {
		header.Set("Content-Type", contentType)
	}

	// Only operations with exact server-side response replay get an automatic
	// key. A caller key on any other mutation is forwarded, but cannot opt that
	// operation into automatic retries.
	if method != http.MethodGet && method != http.MethodHead {
		key := idempotencyKey
		if key == "" && headerReplay {
			key = newIdempotencyKey()
		}
		if key != "" && !idempotencyKeyPattern.MatchString(key) {
			return nil, fmt.Errorf(
				"seatlayer: invalid Idempotency-Key %q: allowed characters are A-Z a-z 0-9 . _ : - and the length must be 1-128",
				key)
		}
		if key != "" {
			header.Set("Idempotency-Key", key)
		}
	}
	retryAllowed := method == http.MethodGet || method == http.MethodHead || headerReplay

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}

		request, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
		if err != nil {
			return nil, fmt.Errorf("seatlayer: building request: %w", err)
		}
		request.Header = header.Clone()

		response, err := c.httpClient.Do(request)
		if err != nil {
			// A cancelled context is the caller's decision, not a transient fault.
			if ctx.Err() != nil {
				return nil, &ConnectionError{Op: method + " " + path, Err: ctx.Err()}
			}
			lastErr = &ConnectionError{Op: method + " " + path, Err: err}
			if retryAllowed && attempt < c.maxRetries-1 {
				if err := sleepCtx(ctx, backoff(attempt, -1)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		responseBody, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		if readErr != nil {
			lastErr = &ConnectionError{Op: method + " " + path, Err: readErr}
			if retryAllowed && attempt < c.maxRetries-1 {
				if err := sleepCtx(ctx, backoff(attempt, -1)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, lastErr
		}

		if response.StatusCode >= 200 && response.StatusCode < 300 {
			if response.StatusCode == http.StatusNoContent || len(responseBody) == 0 {
				return map[string]any{}, nil
			}
			var decoded map[string]any
			if err := json.Unmarshal(responseBody, &decoded); err != nil {
				return nil, fmt.Errorf("seatlayer: decoding response: %w", err)
			}
			return decoded, nil
		}

		// A proxy or WAF can answer with HTML; that must not become a decode
		// failure that hides the real status from the caller.
		errorBody := map[string]any{}
		_ = json.Unmarshal(responseBody, &errorBody)

		retryAfter := parseRetryAfter(response.Header, errorBody)

		if retryAllowed && isRetryable(response.StatusCode) && attempt < c.maxRetries-1 {
			wait := backoff(attempt, -1)
			if response.StatusCode == http.StatusTooManyRequests {
				wait = time.Duration(retryAfter * float64(time.Second))
			}
			if err := sleepCtx(ctx, wait); err != nil {
				return nil, err
			}
			continue
		}

		return nil, errorFromResponse(
			response.StatusCode, errorBody, response.Header.Get("X-Request-ID"), retryAfter)
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, &ConnectionError{Op: method + " " + path, Err: errors.New("no attempts made")}
}

func (c *Client) get(ctx context.Context, path string, query url.Values) (map[string]any, error) {
	return c.Do(ctx, http.MethodGet, path, query, nil, "")
}

func (c *Client) post(ctx context.Context, path string, body any, idempotencyKey string) (map[string]any, error) {
	return c.Do(ctx, http.MethodPost, path, nil, body, idempotencyKey)
}

func (c *Client) postHeaderReplay(
	ctx context.Context, path string, body any, idempotencyKey string,
) (map[string]any, error) {
	return c.do(ctx, http.MethodPost, path, nil, body, idempotencyKey, true)
}

func (c *Client) put(ctx context.Context, path string, body any) (map[string]any, error) {
	return c.Do(ctx, http.MethodPut, path, nil, body, "")
}

func (c *Client) putRaw(
	ctx context.Context, path string, body []byte, contentType string,
) (map[string]any, error) {
	return c.Do(ctx, http.MethodPut, path, nil,
		rawRequestBody{data: body, contentType: contentType}, "")
}

func (c *Client) patch(ctx context.Context, path string, body any) (map[string]any, error) {
	return c.Do(ctx, http.MethodPatch, path, nil, body, "")
}

func (c *Client) delete(ctx context.Context, path string) (map[string]any, error) {
	return c.Do(ctx, http.MethodDelete, path, nil, nil, "")
}

func (c *Client) deleteQuery(
	ctx context.Context, path string, query url.Values,
) (map[string]any, error) {
	return c.Do(ctx, http.MethodDelete, path, query, nil, "")
}

// isRetryable reports whether a status is worth another attempt.
//
// 429 and 5xx are transient by definition. A 4xx is the API saying the request
// itself is wrong; retrying only burns rate-limit budget and delays the error
// the caller needs to see.
func isRetryable(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == http.StatusRequestTimeout ||
		(status >= 500 && status < 600)
}

// backoff returns exponential delay with full jitter, so a fleet of workers
// limited at the same moment does not retry in lockstep and re-limit itself.
func backoff(attempt int, retryAfterSeconds float64) time.Duration {
	if retryAfterSeconds >= 0 {
		return time.Duration(retryAfterSeconds * float64(time.Second))
	}
	ceiling := math.Min(8, 0.25*math.Pow(2, float64(attempt)))
	return time.Duration(rand.Float64() * ceiling * float64(time.Second))
}

// sleepCtx waits, but gives up immediately if the caller cancels.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return &ConnectionError{Op: "waiting to retry", Err: ctx.Err()}
	}
}

func parseRetryAfter(header http.Header, body map[string]any) float64 {
	if raw := header.Get("Retry-After"); raw != "" {
		if seconds, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil && seconds >= 0 {
			return seconds
		}
	}
	// Fall back to the JSON field for routes that predate the headers.
	if seconds, ok := body["retryAfterSeconds"].(float64); ok {
		return seconds
	}
	return 1
}

func newIdempotencyKey() string {
	// A UUID-shaped random string; the exact shape does not matter, only that it
	// is unique per logical call and stable across that call's retries.
	const hex = "0123456789abcdef"
	b := make([]byte, 36)
	for i := range b {
		switch i {
		case 8, 13, 18, 23:
			b[i] = '-'
		default:
			b[i] = hex[rand.Intn(16)]
		}
	}
	return string(b)
}

// escape percent-encodes a path segment, including slashes.
func escape(segment string) string {
	return url.PathEscape(segment)
}

// params builds a request body, dropping nil values so optional fields stay
// optional rather than being sent as JSON null.
func params(pairs ...any) map[string]any {
	if len(pairs)%2 != 0 {
		panic("seatlayer: params takes alternating keys and values")
	}
	out := make(map[string]any, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		key, _ := pairs[i].(string)
		if pairs[i+1] != nil {
			out[key] = pairs[i+1]
		}
	}
	return out
}

func boolOrNil(value bool) any {
	if !value {
		return nil
	}
	return value
}

// decodeResponse projects the transport's forward-compatible JSON object into
// a named public wire type. Unknown additive fields remain harmless.
func decodeResponse[T any](response map[string]any) (T, error) {
	var decoded T
	encoded, err := json.Marshal(response)
	if err != nil {
		return decoded, fmt.Errorf("seatlayer: encoding response projection: %w", err)
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return decoded, fmt.Errorf("seatlayer: decoding response projection: %w", err)
	}
	return decoded, nil
}
