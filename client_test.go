package seatlayer

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// recorded captures one request the SDK made.
type recorded struct {
	method      string
	path        string
	escapedPath string
	query       string
	header      http.Header
	body        string
}

type stub struct {
	status  int
	body    string
	headers map[string]string
}

// newTestClient serves a queue of responses from a real httptest server, so the
// transport, retry loop and header handling are all exercised rather than mocked.
func newTestClient(t *testing.T, responses []stub, options ...Option) (*Client, *[]recorded) {
	t.Helper()
	calls := &[]recorded{}
	index := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		*calls = append(*calls, recorded{
			method: r.Method, path: r.URL.Path, escapedPath: r.URL.EscapedPath(), query: r.URL.RawQuery,
			header: r.Header.Clone(), body: string(body),
		})
		if index >= len(responses) {
			t.Errorf("more requests than queued responses (request %d)", index+1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		next := responses[index]
		index++
		for k, v := range next.headers {
			w.Header().Set(k, v)
		}
		w.WriteHeader(next.status)
		_, _ = io.WriteString(w, next.body)
	}))
	t.Cleanup(server.Close)

	options = append([]Option{WithBaseURL(server.URL)}, options...)
	client, err := New("sk_test_abc", options...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return client, calls
}

func call(t *testing.T, calls *[]recorded, index int) recorded {
	t.Helper()
	if index >= len(*calls) {
		t.Fatalf("no request recorded at index %d (got %d)", index, len(*calls))
	}
	return (*calls)[index]
}

// ---------- construction ----------

func TestNewRejectsPublishableKey(t *testing.T) {
	// The pk_/sk_ mix-up is the most common first-run failure; a 401 three
	// round-trips later teaches nothing.
	_, err := New("pk_test_abc")
	if err == nil || !strings.Contains(err.Error(), "publishable key") {
		t.Fatalf("want a publishable-key error, got %v", err)
	}
}

func TestNewRejectsNonSecretKey(t *testing.T) {
	for _, key := range []string{"", "nonsense"} {
		if _, err := New(key); err == nil {
			t.Fatalf("want an error for key %q", key)
		}
	}
}

func TestNewReportsMode(t *testing.T) {
	live, _ := New("sk_live_abc")
	test, _ := New("sk_test_abc")
	if live.Mode() != "live" || test.Mode() != "test" {
		t.Fatalf("modes wrong: %s %s", live.Mode(), test.Mode())
	}
}

// ---------- requests ----------

func TestSendsBearerAuthAndParsesBody(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"meta":{"key":"ev_1"}}`}})

	result, err := client.Events.Retrieve(context.Background(), "ev_1")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}

	meta := result["meta"].(map[string]any)
	if meta["key"] != "ev_1" {
		t.Fatalf("body not parsed: %v", result)
	}
	if got := call(t, calls, 0).header.Get("Authorization"); got != "Bearer sk_test_abc" {
		t.Fatalf("auth header = %q", got)
	}
	if got := call(t, calls, 0).path; got != "/v1/events/ev_1" {
		t.Fatalf("path = %q", got)
	}
}

func TestEscapesPathParameters(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{}`}})
	_, _ = client.Events.Retrieve(context.Background(), "ev/../admin")

	// The server must see one segment, not a traversal.
	if got := call(t, calls, 0).escapedPath; got != "/v1/events/ev%2F..%2Fadmin" {
		t.Fatalf("escaped path = %q", got)
	}
}

func TestIdempotencyKeyOnlyGeneratedForHeaderReplayMutations(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 200, body: `{"events":[]}`},
		{status: 201, body: `{}`},
		{status: 201, body: `{}`},
		{status: 201, body: `{}`},
	})

	_, _ = client.Events.List(context.Background(), nil)
	_, _ = client.Events.Create(context.Background(), EventCreateParams{ChartID: "c_1"})
	_, _ = client.Templates.InstantiateTemplate(context.Background(), "arena")
	_, _ = client.Inventory.Hold(context.Background(), "ev_1", HoldParams{Labels: []string{"A-1"}})

	if got := call(t, calls, 0).header.Get("Idempotency-Key"); got != "" {
		t.Fatalf("GET should carry no idempotency key, got %q", got)
	}
	if got := call(t, calls, 1).header.Get("Idempotency-Key"); !idempotencyKeyPattern.MatchString(got) {
		t.Fatalf("replay-backed POST idempotency key invalid: %q", got)
	}
	if got := call(t, calls, 2).header.Get("Idempotency-Key"); !idempotencyKeyPattern.MatchString(got) {
		t.Fatalf("template instantiation idempotency key invalid: %q", got)
	}
	if got := call(t, calls, 3).header.Get("Idempotency-Key"); got != "" {
		t.Fatalf("ordinary mutation should not get an automatic idempotency key, got %q", got)
	}
}

func TestHonoursCallerIdempotencyKey(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 201, body: `{}`}})
	_, _ = client.Events.Create(context.Background(),
		EventCreateParams{ChartID: "c_1", IdempotencyKey: "order-42"})

	if got := call(t, calls, 0).header.Get("Idempotency-Key"); got != "order-42" {
		t.Fatalf("idempotency key = %q", got)
	}
}

func TestRejectsInvalidIdempotencyKey(t *testing.T) {
	client, _ := newTestClient(t, nil)
	_, err := client.Events.Create(context.Background(),
		EventCreateParams{ChartID: "c_1", IdempotencyKey: "has spaces"})
	if err == nil || !strings.Contains(err.Error(), "invalid Idempotency-Key") {
		t.Fatalf("want an idempotency-key error, got %v", err)
	}
}

func TestDropsEmptyQueryParameters(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"charts":[]}`}})
	_, _ = client.Charts.List(context.Background(), &ChartListParams{WorkspaceID: "ws_1"})

	if got := call(t, calls, 0).query; got != "workspaceId=ws_1" {
		t.Fatalf("query = %q", got)
	}
}

// ---------- errors ----------

func TestStableErrorCodeFallbackAndResponseEvidence(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		expectedCode string
	}{
		{name: "code wins", body: `{"code":"stable_code","error":"legacy_error","details":{"field":"slug"}}`, expectedCode: "stable_code"},
		{name: "error fallback", body: `{"error":"legacy_error","details":{"field":"slug"}}`, expectedCode: "legacy_error"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, _ := newTestClient(t, []stub{{
				status: 400, body: test.body,
				headers: map[string]string{"X-Request-ID": "req_contract_1"},
			}})
			_, err := client.Do(context.Background(), http.MethodPost,
				"/v1/contract-fixture", nil, map[string]any{"value": 1}, "")

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("want *APIError, got %T", err)
			}
			if apiErr.Status != 400 || apiErr.Code != test.expectedCode ||
				apiErr.RequestID != "req_contract_1" {
				t.Fatalf("response evidence = %#v", apiErr)
			}
			details, ok := apiErr.Body["details"].(map[string]any)
			if !ok || details["field"] != "slug" {
				t.Fatalf("error body = %#v", apiErr.Body)
			}
		})
	}
}

func TestModeMismatchIsTyped(t *testing.T) {
	client, _ := newTestClient(t, []stub{{status: 403, body: `{"error":"mode_mismatch"}`}})
	_, err := client.Events.Retrieve(context.Background(), "ev_1")

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("want *AuthError, got %T", err)
	}
	if !authErr.ModeMismatch() {
		t.Fatal("want ModeMismatch() true")
	}
}

func TestConflictsExposedPerSeat(t *testing.T) {
	client, _ := newTestClient(t, []stub{{
		status: 409,
		body:   `{"error":"conflict","conflicts":[{"label":"A-1","status":"booked"}]}`,
	}})
	_, err := client.Inventory.Hold(context.Background(), "ev_1", HoldParams{Labels: []string{"A-1"}})

	var conflict *ConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("want *ConflictError, got %T", err)
	}
	if got := conflict.Conflicts(); len(got) != 1 || got[0]["label"] != "A-1" {
		t.Fatalf("conflicts = %v", got)
	}
}

func TestSoldOutIsABusinessOutcome(t *testing.T) {
	client, _ := newTestClient(t, []stub{{status: 409, body: `{"error":"conflict","reason":"sold_out"}`}})
	_, err := client.Inventory.HoldBestAvailable(context.Background(), "ev_1", BestAvailableParams{Qty: 4})

	var conflict *ConflictError
	if !errors.As(err, &conflict) || !conflict.SoldOut() {
		t.Fatalf("want a sold-out conflict, got %v", err)
	}
}

func TestNotFoundIsTyped(t *testing.T) {
	client, _ := newTestClient(t, []stub{{status: 404, body: `{"error":"not_found"}`}})
	_, err := client.Events.Retrieve(context.Background(), "ev_1")

	var notFound *NotFoundError
	if !errors.As(err, &notFound) {
		t.Fatalf("want *NotFoundError, got %T", err)
	}
}

func TestSurfacesRequestID(t *testing.T) {
	client, _ := newTestClient(t, []stub{{
		status: 500, body: `{"error":"internal"}`, headers: map[string]string{"X-Request-ID": "req_9"},
	}}, WithMaxRetries(1))
	_, err := client.Events.Retrieve(context.Background(), "ev_1")

	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.RequestID != "req_9" {
		t.Fatalf("want request id req_9, got %v", err)
	}
}

func TestSurvivesNonJSONErrorBody(t *testing.T) {
	// A proxy or WAF can answer with HTML; that must not become a decode failure
	// that hides the real status from the caller.
	client, _ := newTestClient(t, []stub{{
		status: 502, body: `<html>bad gateway</html>`,
		headers: map[string]string{"X-Request-ID": "req_proxy_1"},
	}}, WithMaxRetries(1))
	_, err := client.Events.Retrieve(context.Background(), "ev_1")

	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 502 || apiErr.Code != "unknown_error" ||
		apiErr.RequestID != "req_proxy_1" || len(apiErr.Body) != 0 {
		t.Fatalf("want a base 502 APIError with response evidence, got %#v", apiErr)
	}
}

// ---------- retry ----------

func TestHeaderReplayMutationsRetry429AndReuseIdempotencyKey(t *testing.T) {
	tests := []struct {
		name string
		call func(*Client) error
	}{
		{"create chart", func(client *Client) error {
			_, err := client.Charts.Create(context.Background(), ChartCreateParams{Name: "Main"})
			return err
		}},
		{"copy chart", func(client *Client) error {
			_, err := client.Charts.Copy(context.Background(), "c_1")
			return err
		}},
		{"instantiate template", func(client *Client) error {
			_, err := client.Templates.InstantiateTemplate(context.Background(), "arena")
			return err
		}},
		{"create event", func(client *Client) error {
			_, err := client.Events.Create(context.Background(), EventCreateParams{ChartID: "c_1"})
			return err
		}},
		{"create workspace", func(client *Client) error {
			_, err := client.Workspaces.Create(context.Background(), WorkspaceCreateParams{
				Name: "Tenant", ExternalRef: FieldValue("tenant-1"),
			})
			return err
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client, calls := newTestClient(t, []stub{
				{status: 429, body: `{"error":"rate_limited"}`, headers: map[string]string{"Retry-After": "0"}},
				{status: 201, body: `{"ok":true}`},
			})

			if err := test.call(client); err != nil {
				t.Fatalf("request: %v", err)
			}
			if len(*calls) != 2 {
				t.Fatalf("want 2 attempts, got %d", len(*calls))
			}
			first := call(t, calls, 0).header.Get("Idempotency-Key")
			if first == "" || first != call(t, calls, 1).header.Get("Idempotency-Key") {
				t.Fatal("idempotency key missing or changed between attempts")
			}
		})
	}
}

func TestBookingWithCallerKeyRemainsSingleAttempt(t *testing.T) {
	client, calls := newTestClient(t, []stub{{
		status: 429, body: `{"error":"rate_limited"}`, headers: map[string]string{"Retry-After": "0"},
	}})

	_, err := client.Inventory.Book(context.Background(), "ev_1", BookParams{
		Labels: []string{"A-1"}, BookingRef: "order-42", IdempotencyKey: "request-42",
	})
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("want *RateLimitError, got %T", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("booking must remain single-attempt, got %d attempts", len(*calls))
	}
	if got := call(t, calls, 0).header.Get("Idempotency-Key"); got != "request-42" {
		t.Fatalf("caller key = %q", got)
	}
}

func TestRawMutationIsFailClosedForRetries(t *testing.T) {
	client, calls := newTestClient(t, []stub{{
		status: 429, body: `{"error":"rate_limited"}`, headers: map[string]string{"Retry-After": "0"},
	}})

	_, err := client.Do(context.Background(), http.MethodPost, "/v1/events", nil,
		map[string]any{"chartId": "c_1"}, "raw-42")
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("want *RateLimitError, got %T", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("raw mutation must be single-attempt, got %d attempts", len(*calls))
	}
}

func TestReadRetriesTransientFailure(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 429, body: `{}`, headers: map[string]string{"Retry-After": "0"}},
		{status: 200, body: `{"meta":{"key":"ev_1"}}`},
	})

	if _, err := client.Events.Retrieve(context.Background(), "ev_1"); err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("read should retry transient failure, got %d attempts", len(*calls))
	}
}

func TestDoesNotRetry4xx(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 422, body: `{"error":"invalid_slug"}`}})
	_, err := client.Events.Create(context.Background(), EventCreateParams{ChartID: "c_1"})

	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("a 4xx must not be retried, got %d attempts", len(*calls))
	}
}

func TestGivesUpAfterMaxRetries(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 429, body: `{}`, headers: map[string]string{"Retry-After": "0"}},
		{status: 429, body: `{}`, headers: map[string]string{"Retry-After": "0"}},
	}, WithMaxRetries(2))

	_, err := client.Events.Create(context.Background(), EventCreateParams{ChartID: "c_1"})
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) {
		t.Fatalf("want *RateLimitError, got %T", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("want 2 attempts, got %d", len(*calls))
	}
}

func TestRetryAfterHeaderWins(t *testing.T) {
	client, _ := newTestClient(t, []stub{{
		status:  429,
		body:    `{"code":"rate_budget_exhausted","error":"rate_limited","retryAfterSeconds":99}`,
		headers: map[string]string{"Retry-After": "7", "X-Request-ID": "req_rate_1"},
	}}, WithMaxRetries(1))

	_, err := client.Events.Retrieve(context.Background(), "ev_1")
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) || rateLimit.RetryAfter != 7 ||
		rateLimit.Status != 429 || rateLimit.Code != "rate_budget_exhausted" ||
		rateLimit.RequestID != "req_rate_1" || rateLimit.Body["retryAfterSeconds"] != float64(99) {
		t.Fatalf("want typed 429 evidence with RetryAfter 7 from the header, got %#v", rateLimit)
	}
}

func TestCancelledContextStopsImmediately(t *testing.T) {
	// A cancelled context is the caller's decision, not a transient fault to
	// retry through.
	client, calls := newTestClient(t, []stub{
		{status: 500, body: `{}`},
		{status: 500, body: `{}`},
		{status: 500, body: `{}`},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	_, err := client.Events.Create(ctx, EventCreateParams{ChartID: "c_1"})
	if err == nil {
		t.Fatal("want an error once the context expires")
	}
	if len(*calls) > 3 {
		t.Fatalf("kept retrying past cancellation: %d attempts", len(*calls))
	}
}

// ---------- pagination ----------

func TestAllWalksPagesAndStops(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 200, body: `{"charts":[{"id":"c_1"},{"id":"c_2"}],"nextCursor":"cur_1"}`},
		{status: 200, body: `{"charts":[{"id":"c_3"}]}`},
	})

	var seen []string
	for chart, err := range client.Charts.All(context.Background(), nil) {
		if err != nil {
			t.Fatalf("All: %v", err)
		}
		seen = append(seen, chart["id"].(string))
	}

	if strings.Join(seen, ",") != "c_1,c_2,c_3" {
		t.Fatalf("seen = %v", seen)
	}
	if len(*calls) != 2 {
		t.Fatalf("want 2 pages, got %d", len(*calls))
	}
	// Absent nextCursor terminates — a caller looping cannot spin forever.
	if !strings.Contains(call(t, calls, 1).query, "cursor=cur_1") {
		t.Fatalf("second page did not carry the cursor: %q", call(t, calls, 1).query)
	}
}

func TestAllEventsSkipsCountsFanout(t *testing.T) {
	// Counts cost a server round-trip PER EVENT, which is exactly the cost
	// pagination was added to avoid.
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"events":[]}`}})
	for range client.Events.All(context.Background(), nil) {
	}
	if !strings.Contains(call(t, calls, 0).query, "counts=0") {
		t.Fatalf("All should drop counts, query = %q", call(t, calls, 0).query)
	}
}

func TestSinglePageKeepsCounts(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"events":[]}`}})
	_, _ = client.Events.List(context.Background(), &EventListParams{Limit: 10})
	if strings.Contains(call(t, calls, 0).query, "counts=0") {
		t.Fatalf("a single page should keep counts, query = %q", call(t, calls, 0).query)
	}
}

func TestAllSurfacesPageErrors(t *testing.T) {
	// A failed page must reach the caller rather than silently ending the loop.
	client, _ := newTestClient(t, []stub{{status: 500, body: `{"error":"internal"}`}}, WithMaxRetries(1))

	var got error
	for _, err := range client.Charts.All(context.Background(), nil) {
		got = err
	}
	if got == nil {
		t.Fatal("want the page error to surface through the iterator")
	}
}

func TestAllStopsWhenCallerBreaks(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 200, body: `{"charts":[{"id":"c_1"}],"nextCursor":"cur_1"}`},
	})

	for range client.Charts.All(context.Background(), nil) {
		break
	}
	if len(*calls) != 1 {
		t.Fatalf("breaking out must stop fetching, got %d pages", len(*calls))
	}
}

// ---------- guards ----------

func TestManageSessionRequiresCapabilities(t *testing.T) {
	client, _ := newTestClient(t, nil)
	// The API safely defaults to view-only, but the SDK keeps authority explicit
	// at every browser-token call site.
	_, err := client.Sessions.CreateManageSession(context.Background(), "ev_1",
		ManageSessionParams{AllowedOrigin: "https://box.example"})
	if err == nil || !strings.Contains(err.Error(), "Capabilities is required") {
		t.Fatalf("want a capabilities error, got %v", err)
	}
}

func TestManageSessionSendsCapabilities(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 201, body: `{"token":"mse_x"}`}})
	_, _ = client.Sessions.CreateManageSession(context.Background(), "ev_1", ManageSessionParams{
		AllowedOrigin: "https://box.example",
		Capabilities:  []ManageCapability{CapabilityView},
	})

	var body map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 0).body), &body)
	caps := body["capabilities"].([]any)
	if len(caps) != 1 || caps[0] != "event:view" {
		t.Fatalf("capabilities = %v", caps)
	}
}

func TestBookBestAvailableRequiresBookingRef(t *testing.T) {
	client, _ := newTestClient(t, nil)
	_, err := client.Inventory.BookBestAvailable(context.Background(), "ev_1", BestAvailableParams{Qty: 2})
	if err == nil || !strings.Contains(err.Error(), "BookingRef is required") {
		t.Fatalf("want a booking-ref error, got %v", err)
	}
}

func TestChartUpdateSendsExpectedUpdatedAt(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"meta":{}}`}})
	_, _ = client.Charts.Update(context.Background(), "c_1", ChartUpdateParams{
		Doc: map[string]any{"version": 1}, ExpectedUpdatedAt: 1234,
	})

	var body map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 0).body), &body)
	if body["expectedUpdatedAt"] != float64(1234) {
		t.Fatalf("expectedUpdatedAt = %v", body["expectedUpdatedAt"])
	}
}

func TestExtendHold(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"ok":true,"expiresAt":123}`}})
	_, _ = client.Inventory.ExtendHold(context.Background(), "ev_1", "h_9", 600000)

	if got := call(t, calls, 0).path; got != "/v1/events/ev_1/extend" {
		t.Fatalf("path = %q", got)
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 0).body), &body)
	if body["holdId"] != "h_9" || body["ttlMs"] != float64(600000) {
		t.Fatalf("body = %v", body)
	}
}

func TestHoldCarriesChannelAuthority(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"holdId":"h_1"}`}})
	_, _ = client.Inventory.Hold(context.Background(), "ev_1", HoldParams{
		Labels:                    []string{"A-1"},
		ChannelIDs:                []string{"ch_partner"},
		IgnoreChannelRestrictions: true,
		Reason:                    "partner checkout",
		IdempotencyKey:            "partner-order-42",
	})

	var body map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 0).body), &body)
	channels := body["channelIds"].([]any)
	if len(channels) != 1 || channels[0] != "ch_partner" || body["reason"] != "partner checkout" {
		t.Fatalf("body = %v", body)
	}
	if body["ignoreChannelRestrictions"] != true {
		t.Fatalf("override = %v", body["ignoreChannelRestrictions"])
	}
}

func TestCreatesOriginBoundBuyerAccessSession(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 201, body: `{"token":"bas_x"}`}})
	_, _ = client.Channels.CreateBuyerAccessSession(context.Background(), "ev/1", BuyerAccessSessionParams{
		ChannelIDs:     []string{"ch_1"},
		IncludePublic:  false,
		AllowedOrigin:  "https://partner.example",
		MaxQuantity:    4,
		IdempotencyKey: "partner-order-42",
	})

	request := call(t, calls, 0)
	if request.escapedPath != "/v1/events/ev%2F1/buyer-access-sessions" {
		t.Fatalf("escaped path = %q", request.escapedPath)
	}
	if request.header.Get("Idempotency-Key") != "partner-order-42" {
		t.Fatalf("idempotency key = %q", request.header.Get("Idempotency-Key"))
	}
	var body map[string]any
	_ = json.Unmarshal([]byte(request.body), &body)
	if body["includePublic"] != false || body["allowedOrigin"] != "https://partner.example" {
		t.Fatalf("body = %v", body)
	}
}

func TestRetrieveBookingTrimsAndEncodesReference(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"bookingRef":"order / 42"}`}})
	_, _ = client.Inventory.RetrieveBooking(context.Background(), "ev_1", "  order / 42  ")

	if got := call(t, calls, 0).escapedPath; got != "/v1/events/ev_1/bookings/order%20%2F%2042" {
		t.Fatalf("path = %q", got)
	}
}

func TestUnbookRejectsBlankBookingReference(t *testing.T) {
	client, _ := newTestClient(t, nil)
	_, err := client.Inventory.Unbook(context.Background(), "ev_1", []string{"A-1"}, "   ")
	if err == nil || !strings.Contains(err.Error(), "BookingRef is required") {
		t.Fatalf("want a booking-ref error, got %v", err)
	}
}

func TestSpentHoldIsAConflict(t *testing.T) {
	client, _ := newTestClient(t, []stub{{status: 409, body: `{"error":"cannot_extend","reason":"expired"}`}})
	_, err := client.Inventory.ExtendHold(context.Background(), "ev_1", "h_9", 0)

	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Code != "cannot_extend" {
		t.Fatalf("want a cannot_extend conflict, got %v", err)
	}
}

func TestAPI02WireContractFields(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 201, body: `{"meta":{"key":"ev_1"}}`},
		{status: 200, body: `{"ok":true,"updated":true,"meta":{}}`},
		{status: 200, body: `{"ok":true,"holdTtlMs":null}`},
		{status: 200, body: `{"ok":true,"blocked":["A-1"]}`},
		{status: 200, body: `{"ok":true,"holdId":"h_1","expiresAt":123,"extends":1}`},
	})

	_, _ = client.Events.Create(context.Background(), EventCreateParams{
		ChartID: "c_1", Description: "Matinee", EndsAt: 1800,
		Timezone: "Asia/Kolkata", Locale: "en-IN", PosterAssetID: "ast_1",
		Nullable: EventCreateNullableFields{Venue: FieldNull[string]()},
	})
	acknowledge := true
	_, _ = client.Events.UpdateChart(context.Background(), "ev_1", EventChartUpdateParams{
		AcknowledgeDroppedAssignments: &acknowledge,
		Reason:                        "approved migration",
	})
	_, _ = client.Events.UpdateHoldTTL(context.Background(), "ev_1")
	_, _ = client.Inventory.Block(context.Background(), "ev_1", []string{"A-1"}, 2000)
	_, _ = client.Inventory.ExtendHold(
		context.Background(), "ev_1", "h_1", 600000,
		TrustedInventoryAccess{
			ChannelIDs: []string{"ch_partner"}, IgnoreChannelRestrictions: true,
			Reason: "staff override",
		},
	)

	var createBody map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 0).body), &createBody)
	if createBody["venue"] != nil || createBody["description"] != "Matinee" ||
		createBody["posterAssetId"] != "ast_1" ||
		createBody["endsAt"] != float64(1800) {
		t.Fatalf("event create body = %v", createBody)
	}
	var chartBody map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 1).body), &chartBody)
	if chartBody["acknowledgeDroppedAssignments"] != true || chartBody["reason"] != "approved migration" {
		t.Fatalf("event chart body = %v", chartBody)
	}
	if call(t, calls, 2).body != `{"holdTtlMs":null}` {
		t.Fatalf("hold TTL reset body = %q", call(t, calls, 2).body)
	}
	var blockBody map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 3).body), &blockBody)
	if blockBody["releaseAt"] != float64(2000) {
		t.Fatalf("block body = %v", blockBody)
	}
	var extendBody map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 4).body), &extendBody)
	if extendBody["ignoreChannelRestrictions"] != true || extendBody["reason"] != "staff override" {
		t.Fatalf("extend body = %v", extendBody)
	}
}

func TestPosterAndEventLogContracts(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 200, body: `{"meta":{"key":"ev_1"}}`},
		{status: 200, body: `{"meta":{"key":"ev_1"}}`},
		{status: 200, body: `{"entries":[],"nextBefore":null}`},
	})
	image := []byte("\x89PNG\r\n\x1a\nposter")
	_, _ = client.Events.UpdatePoster(context.Background(), "ev/1", image, "image/png")
	_, _ = client.Events.DeletePoster(context.Background(), "ev/1")
	page, err := client.Events.RetrieveLog(context.Background(), "ev/1", EventLogListParams{
		Limit: 50, Before: 123,
	})
	if err != nil || page.NextBefore != nil || len(page.Entries) != 0 {
		t.Fatalf("event log = %#v, %v", page, err)
	}
	if got := call(t, calls, 0); got.escapedPath != "/v1/events/ev%2F1/poster" ||
		got.body != string(image) || got.header.Get("Content-Type") != "image/png" {
		t.Fatalf("poster request = %#v", got)
	}
	if got := call(t, calls, 1).method; got != http.MethodDelete {
		t.Fatalf("poster delete method = %q", got)
	}
	if got := call(t, calls, 2).query; got != "before=123&limit=50" {
		t.Fatalf("event log query = %q", got)
	}
}

func TestAccessLinkLifecycleContract(t *testing.T) {
	link := `{"id":"alk_1","channelId":"chn/1","label":null,"includePublic":false,` +
		`"expiresAt":2000,"maxRedemptions":10,"redemptions":0,"maxQuantity":4,` +
		`"sessionTtlSeconds":1800,"state":"active","status":"active","createdAt":1000,` +
		`"createdBy":null,"revokedAt":null,"lastRedeemedAt":null,"rotatedFrom":null,"rotatedTo":null}`
	client, calls := newTestClient(t, []stub{
		{status: 201, body: `{"link":` + link + `,"url":"https://app.seatlayer.io/a#once",` +
			`"capability":"alc_once","revealedOnce":true}`},
		{status: 200, body: `{"links":[` + strings.TrimSuffix(link, "}") + `,"activeSessions":0}]}`},
		{status: 201, body: `{"link":` + link + `,"url":"https://app.seatlayer.io/a#next",` +
			`"capability":"alc_next","revealedOnce":true,"previous":` + link + `,"endedSessions":2}`},
		{status: 200, body: `{"ok":true,"link":` + link + `,"endedSessions":2}`},
	})
	includePublic := false
	created, err := client.Channels.CreateAccessLink(context.Background(), "ev/1", "chn/1",
		AccessLinkCreateParams{
			Label: FieldNull[string](), ExpiresAt: 2000, IncludePublic: &includePublic,
			IdempotencyKey: "access-1",
		})
	if err != nil || created.Capability != "alc_once" || !created.RevealedOnce {
		t.Fatalf("access-link create = %#v, %v", created, err)
	}
	listed, err := client.Channels.ListAccessLinks(context.Background(), "ev/1", "chn/1")
	if err != nil || len(listed.Links) != 1 || listed.Links[0].ActiveSessions != 0 {
		t.Fatalf("access-link list = %#v, %v", listed, err)
	}
	rotated, err := client.Channels.RotateAccessLink(context.Background(), "ev/1", "chn/1", "alk/1",
		AccessLinkRotateParams{EndActiveSessions: false, Reason: "misplaced"})
	if err != nil || rotated.EndedSessions == nil || *rotated.EndedSessions != 2 {
		t.Fatalf("access-link rotate = %#v, %v", rotated, err)
	}
	revoked, err := client.Channels.RevokeAccessLink(context.Background(), "ev/1", "chn/1", "alk/1",
		AccessLinkRevokeParams{EndActiveSessions: true, Reason: "leaked URL"})
	if err != nil || !revoked.OK || revoked.EndedSessions != 2 {
		t.Fatalf("access-link revoke = %#v, %v", revoked, err)
	}
	var createBody map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 0).body), &createBody)
	if createBody["label"] != nil || createBody["expiresAt"] != float64(2000) ||
		createBody["includePublic"] != false {
		t.Fatalf("access-link create body = %v", createBody)
	}
	if call(t, calls, 0).header.Get("Idempotency-Key") != "access-1" {
		t.Fatalf("access-link idempotency header = %q", call(t, calls, 0).header.Get("Idempotency-Key"))
	}
	if got := call(t, calls, 2).body; got != `{"endActiveSessions":false,"reason":"misplaced"}` {
		t.Fatalf("access-link rotate body = %q", got)
	}
	if got := call(t, calls, 3); got.escapedPath !=
		"/v1/events/ev%2F1/channels/chn%2F1/access-links/alk%2F1" ||
		got.query != "endActiveSessions=1&reason=leaked+URL" {
		t.Fatalf("access-link revoke request = %#v", got)
	}
}

func TestAccessLinkCreateIsSingleAttempt(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 500, body: `{"code":"internal"}`},
		{status: 201, body: `{}`},
	}, WithMaxRetries(2))
	_, err := client.Channels.CreateAccessLink(context.Background(), "ev_1", "ch_1",
		AccessLinkCreateParams{})
	if err == nil || len(*calls) != 1 {
		t.Fatalf("one-time reveal must make one attempt; calls=%d err=%v", len(*calls), err)
	}
}

func TestBoundedNullableAndChartContracts(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 201, body: `{}`},
		{status: 200, body: `{}`},
		{status: 201, body: `{}`},
		{status: 201, body: `{}`},
		{status: 200, body: `{}`},
	})
	_, _ = client.Charts.Copy(context.Background(), "c/1", ChartCopyParams{
		Name: "Balcony", ExternalRef: FieldNull[string](), WorkspaceID: "ws_2",
	})
	issues := 2.0
	_, _ = client.Charts.Update(context.Background(), "c/1", ChartUpdateParams{
		Name: "Arena", Issues: &issues, ExternalRef: FieldNull[string](),
	})
	_, _ = client.Channels.CreateBuyerAccessSession(context.Background(), "ev_1",
		BuyerAccessSessionParams{
			IncludePublic: false, AllowedOrigin: "https://tickets.example",
			Nullable: BuyerAccessSessionNullableFields{
				MaxQuantity: FieldNull[int](), BuyerRef: FieldNull[string](),
				PartnerRef: FieldNull[string](), ClientRequestID: FieldNull[string](),
			},
		})
	_, _ = client.Workspaces.Create(context.Background(), WorkspaceCreateParams{
		Name: "Promoter", ExternalRef: FieldNull[string](),
	})
	_, _ = client.Channels.Archive(context.Background(), "ev_1", "ch_1",
		ChannelArchiveParams{Destination: FieldNull[string](), Reason: "retired"})

	wants := []map[string]any{
		{"name": "Balcony", "externalRef": nil, "workspaceId": "ws_2"},
		{"name": "Arena", "issues": float64(2), "externalRef": nil},
		{"includePublic": false, "allowedOrigin": "https://tickets.example",
			"maxQuantity": nil, "buyerRef": nil, "partnerRef": nil, "clientRequestId": nil},
		{"name": "Promoter", "externalRef": nil},
		{"destination": nil, "reason": "retired"},
	}
	for i, want := range wants {
		var got map[string]any
		_ = json.Unmarshal([]byte(call(t, calls, i).body), &got)
		if !mapsEqual(got, want) {
			t.Fatalf("request %d body = %#v, want %#v", i, got, want)
		}
	}
}

func TestTemplateAndTicketReleaseTransportContracts(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 201, body: `{"meta":{"id":"c_draft"}}`},
		{status: 200, body: `{"releases":[{"id":"rel_0123456789ab","position":1,"name":"Early","categoryKey":null,"price":2500,"previousPrice":null,"quota":10,"startsAt":null,"endsAt":null,"action":"buy","actionUrl":null,"soldOutAt":null,"consumed":2,"remaining":8}]}`},
		{status: 200, body: `{"releases":[]}`},
		{status: 200, body: `{"releases":[]}`},
	})

	chart, err := client.Templates.InstantiateTemplate(context.Background(), "arena/2026",
		TemplateInstantiateParams{IdempotencyKey: "template-arena-2026"})
	if err != nil || chart["meta"].(map[string]any)["id"] != "c_draft" {
		t.Fatalf("instantiate template = %#v, %v", chart, err)
	}
	listed, err := client.Events.ListTicketReleases(context.Background(), "ev/1")
	if err != nil || len(listed.Releases) != 1 || listed.Releases[0].Consumed == nil ||
		*listed.Releases[0].Consumed != 2 || listed.Releases[0].Remaining == nil ||
		*listed.Releases[0].Remaining != 8 {
		t.Fatalf("list releases = %#v, %v", listed, err)
	}
	_, err = client.Events.UpdateTicketReleases(context.Background(), "ev/1", []TicketReleaseReplaceInput{{
		Name: "Early", Price: 2500, Quota: FieldValue(10),
	}})
	if err != nil {
		t.Fatalf("replace releases: %v", err)
	}
	_, err = client.Events.CloseTicketRelease(context.Background(), "ev/1", "rel/0123456789ab")
	if err != nil {
		t.Fatalf("close release: %v", err)
	}

	if got := call(t, calls, 0); got.escapedPath != "/v1/templates/arena%2F2026/instantiate" ||
		got.header.Get("Idempotency-Key") != "template-arena-2026" || got.body != "{}" {
		t.Fatalf("template request = %#v", got)
	}
	if got := call(t, calls, 1); got.escapedPath != "/v1/events/ev%2F1/releases" {
		t.Fatalf("list release request = %#v", got)
	}
	if got := call(t, calls, 2); got.method != "PUT" || got.escapedPath != "/v1/events/ev%2F1/releases" {
		t.Fatalf("replace release request = %#v", got)
	}
	var replaceBody map[string]any
	if err := json.Unmarshal([]byte(call(t, calls, 2).body), &replaceBody); err != nil ||
		!mapsEqual(replaceBody, map[string]any{"releases": []any{map[string]any{
			"name": "Early", "price": float64(2500), "quota": float64(10),
		}}}) {
		t.Fatalf("replace body = %#v, decode err = %v", replaceBody, err)
	}
	if got := call(t, calls, 3); got.escapedPath !=
		"/v1/events/ev%2F1/releases/rel%2F0123456789ab/close" {
		t.Fatalf("close release request = %#v", got)
	}
}

func TestEventConfigurationBindingContract(t *testing.T) {
	binding := `{"configuration":{"id":"ec_touring","version":3},"revision":7,` +
		`"changedBy":"api-key:key_1","changedAt":123,"audit":[` +
		`{"id":"eca_1","from":null,"to":{"id":"ec_touring","version":3},` +
		`"revision":7,"actor":"api-key:key_1","createdAt":123}]}`
	client, calls := newTestClient(t, []stub{
		{status: 200, body: binding},
		{status: 200, body: binding},
		{status: 200, body: `{"configuration":null,"revision":8,"changedBy":null,"changedAt":null,"audit":[]}`},
	})

	retrieved, err := client.Events.RetrieveConfigurationBinding(context.Background(), "ev / main")
	if err != nil || retrieved.Configuration == nil || retrieved.Configuration.ID != "ec_touring" ||
		retrieved.Configuration.Version != 3 || len(retrieved.Audit) != 1 ||
		retrieved.Audit[0].From != nil || retrieved.Audit[0].To == nil {
		t.Fatalf("retrieve configuration binding = %#v, %v", retrieved, err)
	}
	configuration := &EventConfigurationRef{ID: "ec_touring", Version: 3}
	_, err = client.Events.UpdateConfigurationBinding(context.Background(), "ev / main",
		EventConfigurationBindingUpdateParams{ExpectedRevision: 6, Configuration: configuration})
	if err != nil {
		t.Fatalf("attach configuration binding: %v", err)
	}
	detached, err := client.Events.UpdateConfigurationBinding(context.Background(), "ev / main",
		EventConfigurationBindingUpdateParams{ExpectedRevision: 7, Configuration: nil})
	if err != nil || detached.Configuration != nil || detached.ChangedBy != nil || detached.ChangedAt != nil {
		t.Fatalf("detach configuration binding = %#v, %v", detached, err)
	}

	for i := 0; i < 3; i++ {
		got := call(t, calls, i)
		if got.escapedPath != "/v1/events/ev%20%2F%20main/event-configuration" {
			t.Fatalf("configuration request %d path = %q", i, got.escapedPath)
		}
	}
	if got := call(t, calls, 0); got.method != "GET" {
		t.Fatalf("configuration read method = %q", got.method)
	}
	wants := []map[string]any{
		{"expectedRevision": float64(6), "configuration": map[string]any{"id": "ec_touring", "version": float64(3)}},
		{"expectedRevision": float64(7), "configuration": nil},
	}
	for i, want := range wants {
		got := call(t, calls, i+1)
		var body map[string]any
		if err := json.Unmarshal([]byte(got.body), &body); err != nil || !mapsEqual(body, want) {
			t.Fatalf("configuration request %d body = %#v, decode err = %v", i+1, body, err)
		}
		if got.method != "PUT" || got.header.Get("Idempotency-Key") != "" {
			t.Fatalf("configuration request %d transport = %#v", i+1, got)
		}
	}
}

func TestEventConfigurationMutationRemainsSingleAttempt(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 429, body: `{"error":"rate_limited"}`, headers: map[string]string{"Retry-After": "0"}},
	}, WithMaxRetries(2))
	_, err := client.Events.UpdateConfigurationBinding(context.Background(), "ev_1",
		EventConfigurationBindingUpdateParams{ExpectedRevision: 1})
	if err == nil || len(*calls) != 1 {
		t.Fatalf("configuration binding must be single-attempt; calls=%d err=%v", len(*calls), err)
	}
}

func TestTicketReleaseMutationsRemainSingleAttempt(t *testing.T) {
	update, updateCalls := newTestClient(t, []stub{
		{status: 429, body: `{"error":"rate_limited"}`, headers: map[string]string{"Retry-After": "0"}},
	}, WithMaxRetries(2))
	_, updateErr := update.Events.UpdateTicketReleases(context.Background(), "ev_1", nil)
	if updateErr == nil || len(*updateCalls) != 1 {
		t.Fatalf("release replacement must be single-attempt; calls=%d err=%v", len(*updateCalls), updateErr)
	}

	close, closeCalls := newTestClient(t, []stub{
		{status: 429, body: `{"error":"rate_limited"}`, headers: map[string]string{"Retry-After": "0"}},
	}, WithMaxRetries(2))
	_, closeErr := close.Events.CloseTicketRelease(context.Background(), "ev_1", "rel_0123456789ab")
	if closeErr == nil || len(*closeCalls) != 1 {
		t.Fatalf("release close must be single-attempt; calls=%d err=%v", len(*closeCalls), closeErr)
	}
}

func mapsEqual(left, right map[string]any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return string(leftJSON) == string(rightJSON)
}

func TestWebhookEnvelopesAndDeliveryFilters(t *testing.T) {
	sub := `{"id":"wh_1","url":"https://hooks.example/seatlayer","events":["seat.booked"],` +
		`"disabled":false,"lastStatus":null,"lastAt":null,"createdAt":1,` +
		`"mode":"test","environment":null,"uptime7d":null}`
	client, calls := newTestClient(t, []stub{
		{status: 200, body: `{"subs":[` + sub + `]}`},
		{status: 201, body: `{"sub":` + sub + `,"secret":"whsec_once"}`},
		{status: 200, body: `{"sub":` + strings.Replace(sub, `"disabled":false`, `"disabled":true`, 1) + `}`},
		{status: 200, body: `{"deliveries":[],"nextBefore":100}`},
	})

	listed, err := client.Webhooks.List(context.Background())
	if err != nil || len(listed.Subs) != 1 || listed.Subs[0].Events[0] != WebhookEventSeatBooked {
		t.Fatalf("webhook list = %#v, %v", listed, err)
	}
	created, err := client.Webhooks.Create(
		context.Background(), "https://hooks.example/seatlayer",
		[]WebhookEventName{WebhookEventSeatBooked},
	)
	if err != nil || created.Secret != "whsec_once" {
		t.Fatalf("webhook create = %#v, %v", created, err)
	}
	disabled := true
	updated, err := client.Webhooks.Update(context.Background(), "wh_1", WebhookUpdateParams{
		Disabled: &disabled,
	})
	if err != nil || !updated.Sub.Disabled {
		t.Fatalf("webhook update = %#v, %v", updated, err)
	}
	page, err := client.Webhooks.ListDeliveries(context.Background(), "wh_1", WebhookDeliveryListParams{
		Limit: 10, Status: WebhookDeliveryFailed, Before: 200,
	})
	if err != nil || page.NextBefore == nil || *page.NextBefore != 100 {
		t.Fatalf("delivery page = %#v, %v", page, err)
	}
	if got := call(t, calls, 3).query; got != "before=200&limit=10&status=failed" {
		t.Fatalf("delivery query = %q", got)
	}
}

func TestWebhookEnumsRejectUnknownValuesBeforeTransport(t *testing.T) {
	client, calls := newTestClient(t, nil)
	_, err := client.Webhooks.Create(context.Background(), "https://hooks.example",
		[]WebhookEventName{"booking.created"})
	if err == nil || !strings.Contains(err.Error(), "supported webhook event names") {
		t.Fatalf("want event-name error, got %v", err)
	}
	_, err = client.Webhooks.ListDeliveries(context.Background(), "wh_1",
		WebhookDeliveryListParams{Status: "pending"})
	if err == nil || !strings.Contains(err.Error(), "status must be ok or failed") {
		t.Fatalf("want delivery-status error, got %v", err)
	}
	if len(*calls) != 0 {
		t.Fatalf("invalid enum made %d transport calls", len(*calls))
	}
}

func TestTypedHoldAndDesignerEnvelopes(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 200, body: `{"holdId":"h_1","status":"active","expiresAt":123,` +
			`"bookingRef":null,"eventKey":"ev_1","mode":"test","externalRef":null,` +
			`"workspaceId":"ws_1","items":[]}`},
		{status: 201, body: `{"session":{"id":"dsess_1","token":"dse_x",` +
			`"workspaceId":"ws_1","chartId":"c_1","allowedOrigin":"https://app.example",` +
			`"authority":"edit","canEdit":true,"canPublish":false,"mode":"safe",` +
			`"safeModeOptions":{"allowDeletingObjects":false,"allowEditingAreaCapacity":true},` +
			`"featurePolicy":{"images":false},"expiresAt":123,"designerUrl":"https://app.example/embed"}}`},
		{status: 200, body: `{"sessions":[]}`},
	})

	hold, err := client.Inventory.RetrieveHold(context.Background(), "ev_1", "h_1")
	if err != nil || hold.EventKey == nil || *hold.EventKey != "ev_1" || hold.Status != "active" {
		t.Fatalf("hold = %#v, %v", hold, err)
	}
	allowCapacity := true
	designer, err := client.Sessions.CreateDesignerSession(context.Background(), DesignerSessionParams{
		WorkspaceID: "ws_1", ChartID: "c_1", AllowedOrigin: "https://app.example",
		Mode: "safe", SafeModeOptions: &DesignerSafeModeOptionsParams{AllowEditingAreaCapacity: &allowCapacity},
		Features: map[string]any{"images": false},
	})
	if err != nil || designer.Session.ID != "dsess_1" || designer.Session.Mode != "safe" {
		t.Fatalf("designer = %#v, %v", designer, err)
	}
	_, _ = client.Channels.ListBuyerAccessSessions(
		context.Background(), "ev_1", BuyerAccessSessionListParams{Limit: 25},
	)
	if got := call(t, calls, 2).query; got != "limit=25" {
		t.Fatalf("buyer session query = %q", got)
	}
}

func TestOptionalFieldsAreOmittedNotNulled(t *testing.T) {
	// Sending "name": null is not the same as omitting it; some fields treat an
	// explicit null as "clear this".
	client, calls := newTestClient(t, []stub{{status: 201, body: `{}`}})
	_, _ = client.Events.Create(context.Background(), EventCreateParams{ChartID: "c_1"})

	var body map[string]any
	_ = json.Unmarshal([]byte(call(t, calls, 0).body), &body)
	for _, key := range []string{"name", "slug", "venue", "currency", "startsAt"} {
		if _, present := body[key]; present {
			t.Fatalf("empty optional %q should be omitted, body = %v", key, body)
		}
	}
}
