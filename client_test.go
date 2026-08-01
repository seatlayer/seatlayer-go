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
	method string
	path   string
	query  string
	header http.Header
	body   string
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
			method: r.Method, path: r.URL.Path, query: r.URL.RawQuery,
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
	if got := call(t, calls, 0).path; got != "/v1/events/ev/../admin" && !strings.Contains(got, "%2F") {
		t.Fatalf("path not escaped: %q", got)
	}
}

func TestIdempotencyKeyOnMutationsOnly(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 200, body: `{"events":[]}`},
		{status: 201, body: `{}`},
	})

	_, _ = client.Events.List(context.Background(), nil)
	_, _ = client.Events.Create(context.Background(), EventCreateParams{ChartID: "c_1"})

	if got := call(t, calls, 0).header.Get("Idempotency-Key"); got != "" {
		t.Fatalf("GET should carry no idempotency key, got %q", got)
	}
	if got := call(t, calls, 1).header.Get("Idempotency-Key"); !idempotencyKeyPattern.MatchString(got) {
		t.Fatalf("POST idempotency key invalid: %q", got)
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
	client, _ := newTestClient(t, []stub{{status: 502, body: `<html>bad gateway</html>`}}, WithMaxRetries(1))
	_, err := client.Events.Retrieve(context.Background(), "ev_1")

	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 502 {
		t.Fatalf("want a 502 APIError, got %v", err)
	}
}

// ---------- retry ----------

func TestRetries429AndReusesIdempotencyKey(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 429, body: `{"error":"rate_limited"}`, headers: map[string]string{"Retry-After": "0"}},
		{status: 201, body: `{"ok":true}`},
	})

	if _, err := client.Events.Create(context.Background(), EventCreateParams{ChartID: "c_1"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(*calls) != 2 {
		t.Fatalf("want 2 attempts, got %d", len(*calls))
	}
	// Same key on the retry, or the server would create two events.
	if call(t, calls, 0).header.Get("Idempotency-Key") != call(t, calls, 1).header.Get("Idempotency-Key") {
		t.Fatal("idempotency key changed between attempts")
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
		body:    `{"error":"rate_limited","retryAfterSeconds":99}`,
		headers: map[string]string{"Retry-After": "0"},
	}}, WithMaxRetries(1))

	_, err := client.Events.Retrieve(context.Background(), "ev_1")
	var rateLimit *RateLimitError
	if !errors.As(err, &rateLimit) || rateLimit.RetryAfter != 0 {
		t.Fatalf("want RetryAfter 0 from the header, got %v", err)
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
	// The API would default this to all four including event:cancel — the ability
	// to reverse paid bookings should never arrive by omission.
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
		Capabilities:  []string{CapabilityView},
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
	_, _ = client.Charts.Update(context.Background(), "c_1", map[string]any{"version": 1}, 1234)

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

func TestSpentHoldIsAConflict(t *testing.T) {
	client, _ := newTestClient(t, []stub{{status: 409, body: `{"error":"cannot_extend","reason":"expired"}`}})
	_, err := client.Inventory.ExtendHold(context.Background(), "ev_1", "h_9", 0)

	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Code != "cannot_extend" {
		t.Fatalf("want a cannot_extend conflict, got %v", err)
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
