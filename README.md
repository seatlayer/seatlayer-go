# SeatLayer Go SDK

Official Go server SDK for the [SeatLayer](https://seatlayer.io) reserved-seating API.

> **Server-side only.** This package authenticates with your secret key. Never embed it in
> anything a ticket buyer can reach — browser surfaces get short-lived, origin-bound tokens that
> you mint here.

## Install

```bash
go get github.com/seatlayer/seatlayer-go
```

Requires Go 1.23 or newer (for range-over-func iterators). **No dependencies** — standard library
only.

## Quick start

```go
import (
    "context"
    "os"

    "github.com/seatlayer/seatlayer-go"
)

client, err := seatlayer.New(os.Getenv("SEATLAYER_SECRET_KEY"))
if err != nil {
    return err
}
ctx := context.Background()

// 1. Provision a venue for a new organiser from one of your templates.
chart, err := client.Charts.Copy(ctx, "c_template_arena")
if err != nil {
    return err
}
chartID := chart["meta"].(map[string]any)["id"].(string)
if _, err := client.Charts.Publish(ctx, chartID); err != nil {
    return err
}

// 2. Create an event on it.
event, err := client.Events.Create(ctx, seatlayer.EventCreateParams{
    ChartID: chartID,
    Name:    "Spring Gala",
})
if err != nil {
    return err
}
eventKey := event["meta"].(map[string]any)["key"].(string)

// 3. Sell four seats over the phone.
held, err := client.Inventory.HoldBestAvailable(ctx, eventKey, seatlayer.BestAvailableParams{Qty: 4})
if err != nil {
    return err
}
// … take payment against held["items"], which carry authoritative prices …
_, err = client.Inventory.Book(ctx, eventKey, seatlayer.BookParams{
    HoldID:     held["holdId"].(string),
    BookingRef: "order-8842",
})
```

Every method takes a `context.Context`. Cancelling it stops retries immediately rather than being
treated as a transient fault to back off through.

## Test vs live

Keys carry their own mode. `sk_test_…` keys can only touch test-mode events and `sk_live_…` only
live ones; crossing them returns `403 mode_mismatch`.

```go
client, err := seatlayer.New(os.Getenv("SEATLAYER_SECRET_KEY"))
if err != nil {
    return err
}
if os.Getenv("ENV") == "production" && client.Mode() != "live" {
    return errors.New("refusing to boot production against test-mode seating data")
}
```

A publishable `pk_` key is rejected by `New` with a message naming the mistake, rather than
failing as a `401` three round-trips later.

## The two selling flows

**Buyer picks seats in the browser.** Your frontend holds them; your backend confirms the price and
books. Never price from what the browser sent you — `RetrieveHold` is authoritative.

```go
hold, err := client.Inventory.RetrieveHold(ctx, eventKey, holdID)
// … charge the total of hold["items"] in hold["currency"] …
_, err = client.Inventory.Book(ctx, eventKey, seatlayer.BookParams{
    HoldID: holdID, BookingRef: charge.ID,
})
```

**Your backend picks the seats.** Phone orders, box office, comps.

```go
// Payment already taken — book outright, so nothing is stranded if a second call fails.
_, err := client.Inventory.BookBestAvailable(ctx, eventKey, seatlayer.BestAvailableParams{
    Qty: 2, BookingRef: "phone-1183",
})

// Or name the seats yourself.
_, err = client.Inventory.BoxOfficeBook(ctx, eventKey, []string{"A-1", "A-2"}, "comp-14")
```

## Listing and pagination

`List` returns one `Page` plus a cursor. `All` is a range-over-func iterator that pages as you
consume it — deliberately not a slice, because the point of paginating is to *not* hold an
unbounded result set in memory.

```go
// One page, your own paging.
page, err := client.Events.List(ctx, &seatlayer.EventListParams{Limit: 50})
page.Items
page.NextCursor   // "" once exhausted

// Or let the SDK walk it.
for event, err := range client.Events.All(ctx, nil) {
    if err != nil {
        return err
    }
    sync(event)
}
```

The error rides alongside each item so a failed page reaches you — an iterator that silently ended
on error would look identical to a list that finished.

Listing events includes live availability counts by default, which costs the server one round-trip
**per event**. `All` drops them automatically — walking a whole catalogue is exactly when you don't
want that — and you can control it explicitly:

```go
client.Events.List(ctx, &seatlayer.EventListParams{Limit: 50, NoCounts: true})
```

## Keeping a hold alive

When an order takes longer than the checkout window — an invoice, a phone sale — extend rather than
release and re-hold. Releasing first hands the seats to whoever is racing for them in between.

```go
_, err := client.Inventory.ExtendHold(ctx, eventKey, holdID, 10*60*1000)

var conflict *seatlayer.ConflictError
if errors.As(err, &conflict) {
    // Gone, expired, or at its renewal cap — the buyer has to re-pick.
}
```

## Embedding the control room

Your secret key never reaches a browser. Mint a scoped token instead.

```go
session, err := client.Sessions.CreateManageSession(ctx, eventKey, seatlayer.ManageSessionParams{
    AllowedOrigin:    "https://box-office.yourplatform.com",
    Capabilities:     []string{seatlayer.CapabilityView, seatlayer.CapabilityBlock},
    ExpiresInSeconds: 3600,
})
```

`Capabilities` is **required** by this SDK even though the API defaults it. That default grants all
four including `event:cancel`, which reverses paid bookings — not something that should arrive by
forgetting a field. Grant the smallest set the page needs.

## Webhooks

Verify every delivery against the **raw** body. Decoding and re-encoding changes the bytes — in Go
specifically, `encoding/json` marshals map keys in sorted order while a real delivery arrives in
the order we serialised it, so a round trip reorders it and verification fails.

```go
func handleWebhook(w http.ResponseWriter, r *http.Request) {
    payload, err := io.ReadAll(r.Body)   // raw bytes, before any decoding
    if err != nil {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    event, err := seatlayer.VerifyWebhook(
        payload,
        r.Header.Get("X-SeatLayer-Signature"),
        os.Getenv("SEATLAYER_WEBHOOK_SECRET"),
    )
    if errors.Is(err, seatlayer.ErrWebhookVerification) {
        w.WriteHeader(http.StatusBadRequest)
        return
    }

    // The signed body carries "at", but nothing enforces a freshness window, so a
    // captured delivery stays valid indefinitely. Deduplicate on occurrenceId —
    // this is your replay protection, not an optimisation.
    if alreadyProcessed(event["occurrenceId"].(string)) {
        w.WriteHeader(http.StatusOK)
        return
    }

    process(event)
    w.WriteHeader(http.StatusOK)
}
```

## Errors

Errors are values here, not exceptions — reach for `errors.As`:

```go
_, err := client.Inventory.HoldBestAvailable(ctx, eventKey, seatlayer.BestAvailableParams{Qty: 6})

var conflict *seatlayer.ConflictError
var rateLimit *seatlayer.RateLimitError
var auth *seatlayer.AuthError

switch {
case errors.As(err, &conflict) && conflict.SoldOut():
    return offerAlternativeDates()          // a business outcome, not a bug
case errors.As(err, &rateLimit):
    return retryAfter(rateLimit.RetryAfter)
case errors.As(err, &auth) && auth.ModeMismatch():
    return errors.New("test key pointed at a live event, or the reverse")
case err != nil:
    return err
}
```

| Type | Status | Means |
|---|---|---|
| `AuthError` | 401, 403 | Bad, revoked, or wrong-mode key |
| `NotFoundError` | 404 | No such resource *for this organisation* |
| `ConflictError` | 409 | Inventory moved, or a guard rejected the change |
| `ValidationError` | 422 | Understood and rejected |
| `RateLimitError` | 429 | Over budget; carries `RetryAfter` |
| `ConnectionError` | — | No answer: DNS, TLS, socket, context deadline (unwraps) |

Every API error carries `Status`, `Code`, `Body`, and `RequestID` — quote the request id in support
requests.

## Reliability

**Retries.** 429, 408 and 5xx are retried with exponential backoff and full jitter; `Retry-After`
wins when the server sends it. 4xx is never retried — it will not start succeeding.

**Idempotency.** Every mutating request carries an `Idempotency-Key`, generated if you do not supply
one, and **reused across retries** so a retried booking cannot become two bookings. Pass your own
order id for end-to-end deduplication:

```go
client.Inventory.Book(ctx, eventKey, seatlayer.BookParams{
    HoldID: holdID, IdempotencyKey: "order-" + orderID,
})
```

```go
client, err := seatlayer.New(
    os.Getenv("SEATLAYER_SECRET_KEY"),
    seatlayer.WithMaxRetries(3),
    seatlayer.WithHTTPClient(&http.Client{Timeout: 30 * time.Second}),
)
```

`Client` is safe for concurrent use.

## Escape hatch

For surface this SDK does not wrap yet — same auth, retries, idempotency and error mapping:

```go
client.Do(ctx, http.MethodPost, "/v1/events/ev_1/some-new-route", nil, map[string]any{"qty": 2}, "")
```

## API surface

| Service | Methods |
| --- | --- |
| `Charts` | `List` `All` `Create` `Retrieve` `Update` `Delete` `Copy` `Archive` `Unarchive` `Publish` |
| `Events` | `List` `All` `Create` `Retrieve` `Update` `Delete` `UpdateChart` `Close` `Reopen` `Archive` `RetrieveHoldTTL` `UpdateHoldTTL` `RetrieveReport` `RetrieveLog` |
| `Inventory` | `Hold` `HoldBestAvailable` `BookBestAvailable` `ExtendHold` `RetrieveHold` `Release` `Book` `BoxOfficeBook` `Unbook` `Block` `Unblock` `UnblockAll` `RetrieveAvailability` `UpdateAvailability` |
| `Sessions` | `CreateManageSession` `RevokeManageSession` `CreateDesignerSession` `RevokeDesignerSession` |
| `Webhooks` | `List` `Create` `Update` `Delete` `ListDeliveries` |
| `Workspaces` | `List` `Create` `Retrieve` `Update` |

Full reference: [docs.seatlayer.io/server-sdk](https://docs.seatlayer.io/server-sdk/install/)

## Related resources

- [Server SDK guide](https://docs.seatlayer.io/server-sdk/install/)
- [Errors, retries and idempotency](https://docs.seatlayer.io/server-sdk/reliability/)
- [Webhook verification](https://docs.seatlayer.io/server-sdk/webhooks/)
- [Server API reference](https://docs.seatlayer.io/server-api/events/)
- [OpenAPI description](https://docs.seatlayer.io/openapi.json)
- [Agent-readable documentation](https://docs.seatlayer.io/llms.txt)
- [SeatLayer GitHub organization](https://github.com/seatlayer)

### Other SeatLayer SDKs

| Surface | Package |
|---|---|
| Browser (vanilla) | [`@seatlayer/js`](https://github.com/seatlayer/seatlayer-sdk) |
| React | [`@seatlayer/react`](https://github.com/seatlayer/seatlayer-sdk) |
| React Native | [`@seatlayer/react-native`](https://github.com/seatlayer/seatlayer-react-native) |
| iOS | [`seatlayer-ios`](https://github.com/seatlayer/seatlayer-ios) |
| Android | [`seatlayer-android`](https://github.com/seatlayer/seatlayer-android) |
| Flutter | [`seatlayer_flutter`](https://github.com/seatlayer/seatlayer-flutter) |
| Node.js (server) | [`@seatlayer/server`](https://github.com/seatlayer/seatlayer-node) |
| Python (server) | [`seatlayer`](https://github.com/seatlayer/seatlayer-python) |
| PHP (server) | [`seatlayer/seatlayer-php`](https://github.com/seatlayer/seatlayer-php) |
| Java (server) | [`io.seatlayer:seatlayer-java`](https://github.com/seatlayer/seatlayer-java) |
| Ruby (server) | [`seatlayer`](https://github.com/seatlayer/seatlayer-ruby) |
| .NET (server) | [`SeatLayer`](https://github.com/seatlayer/seatlayer-dotnet) |

## Development

```bash
gofmt -l .          # must be empty
go vet ./...
go test -race ./...
```

## License

MIT
