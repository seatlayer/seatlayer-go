# SeatLayer Go Server SDK for Reserved Seating

[![CI](https://github.com/seatlayer/seatlayer-go/actions/workflows/ci.yml/badge.svg)](https://github.com/seatlayer/seatlayer-go/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/seatlayer/seatlayer-go.svg)](https://pkg.go.dev/github.com/seatlayer/seatlayer-go)
[![License: MIT](https://img.shields.io/badge/license-MIT-111827.svg)](LICENSE)

The official SeatLayer Go server SDK is the **trusted side** of a reserved-seating
integration: inspect the holds a buyer created, price from server data, and book with a
stable `BookingRef`. From Go you manage seating charts, events, sales channels, and live
seat inventory through one typed ticketing API client.

[SeatLayer module on pkg.go.dev](https://pkg.go.dev/github.com/seatlayer/seatlayer-go) ·
[SeatLayer server SDK documentation](https://docs.seatlayer.io/server-sdk/install/) ·
[SeatLayer developer platform](https://seatlayer.io/developers/) ·
[SeatLayer JavaScript seat map SDK](https://www.npmjs.com/package/@seatlayer/js) ·
[SeatLayer AI Toolkit](https://github.com/seatlayer/seatlayer-ai-toolkit)

> **Server-side only.** This package authenticates with your secret key. Never embed it in
> anything a ticket buyer can reach — browser surfaces get short-lived, origin-bound tokens that
> you mint here.

## Install

```bash
go get github.com/seatlayer/seatlayer-go@v0.6.0
```

The module resolves straight from this repository through the Go module proxy, so there is no
registry account to create; `v0.6.0` is the current release and the API reference is published on
[pkg.go.dev](https://pkg.go.dev/github.com/seatlayer/seatlayer-go). Requires Go 1.23 or newer (for range-over-func iterators). **No dependencies** — standard library
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

// 1. Materialize a published catalog template as the organiser's draft chart.
chart, err := client.Templates.InstantiateTemplate(ctx, "arena")
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

For nullable event-create fields, ordinary scalar fields cover the common value-or-omit case. Use
the `Nullable` overlay when the wire call must contain an explicit JSON null, for example
`Nullable: seatlayer.EventCreateNullableFields{Venue: seatlayer.FieldNull[string]()}`.

Every method takes a `context.Context`. Cancelling it stops retries immediately rather than being
treated as a transient fault to back off through.

## Test vs live

## Fixed Renewable Seasons

Version `v0.7.0` exposes all 48 trusted organizer operations through
`client.Seasons`.

After the test hold/book/cancel journey and matching webhook deliveries,
`client.Seasons.ValidateBuyerRehearsal(ctx, seasonKey)` sends no evidence body;
SeatLayer discovers the retained chain automatically. Retrieved Season holds
contain inventory identity, not an authoritative amount—your platform owns
package price, payment, order, tax, refunds, benefits, and ticket or pass delivery.

```go
checked, err := client.Seasons.Validate(ctx, seatlayer.SeasonSelectionParams{
    SourcePerformanceGroupKeys: []string{"pg_subscription_run"},
})
created, err := client.Seasons.Create(ctx, seatlayer.SeasonCreateParams{
    Name: "2027 subscription",
    SourcePerformanceGroupKeys: []string{"pg_subscription_run"},
    IdempotencyKey: "season-create-2027",
})
```

Treat `202` as accepted work and poll `RetrieveLifecycle` with the returned
operation identity. Buyer-session minting and domain-exact booking,
cancellation, and renewal actions remain single-attempt; only declared
header-replay catalogue mutations retry automatically.


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
// … charge from hold.Items, whose UnitPrice and Currency are authoritative …
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

## Private and partner sales

Channels reserve inventory for a partner, member group, presale, or other private allocation. A
buyer access session is short-lived and origin-bound, so the browser receives only the allocation
it is allowed to sell; your secret key remains on your server.

```go
_, err := client.Channels.CreateChannel(ctx, eventKey, seatlayer.ChannelCreateParams{
	Name:         "Venue members",
	AccessIntent: "private",
})

_, err = client.Channels.UpdateAssignments(ctx, eventKey, seatlayer.ChannelAssignmentParams{
	Labels:            []string{"A-1", "A-2"},
	AssignmentVersion: 1,
	TargetChannelID:   "ch_members",
})

access, err := client.Channels.CreateBuyerAccessSession(ctx, eventKey,
	seatlayer.BuyerAccessSessionParams{
		ChannelIDs:    []string{"ch_members"},
		IncludePublic: false,
		AllowedOrigin: "https://members.example",
		MaxQuantity:   2,
	})
```

Pass the returned token to the buyer SDK. Trusted backend sale params accept `ChannelIDs`, an
explicit privileged `IgnoreChannelRestrictions` flag, and an audit `Reason`.

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
    Capabilities:     []seatlayer.ManageCapability{
        seatlayer.CapabilityView,
        seatlayer.CapabilityBlock,
    },
    ExpiresInSeconds: 3600,
})
```

`Capabilities` is **required** by this SDK even though the raw API safely defaults an omitted list
to view-only (`event:view`). Keeping the field required makes browser authority visible at every
call site. Grant the smallest set the page needs. The constants also cover channel management and
SeatLayer-managed orders, refunds, ticket delivery, door, and box-office capabilities.

Designer minting returns a `DesignerSessionEnvelope`; the token and the effective safe-mode and
feature policy live under `result.Session`. Pass `SafeModeOptions` only with `Mode: "safe"`.

## Webhooks

Webhook methods expose the wire envelopes directly: `List` returns `WebhookList.Subs`, `Create`
returns `WebhookCreateEnvelope` with the show-once `Secret`, and `Update` returns
`WebhookEnvelope.Sub`. Use the `WebhookEvent…` constants for the eight accepted event names and
`WebhookDeliveryListParams` for `limit`, `status`, and `before` filters.

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

**Retries.** Reads (`GET`/`HEAD`) retry 429, 408 and 5xx with exponential backoff and full jitter;
`Retry-After` wins when the server sends it. Automatic mutation retries are limited to the five
operations backed by exact response replay: `Charts.Create`, `Charts.Copy`,
`Templates.InstantiateTemplate`, `Events.Create`, and `Workspaces.Create`. Other mutations,
including ticket-release changes, stay single-attempt. Other 4xx responses are never retried.

**Idempotency.** Those five replay-backed operations carry an `Idempotency-Key`, generated when you
do not supply one and reused across attempts. Other mutations are single-attempt and receive no
automatic key. A caller-supplied key is forwarded but does not enable retries. This includes
inventory holds and bookings, show-once credential or secret creation, unsupported operations, and
raw `Do` mutations. Keep `BookingRef` in the booking body for reconciliation, but handle an unknown
network outcome explicitly instead of automatically repeating the sale.

```go
client.Events.Create(ctx, seatlayer.EventCreateParams{
    ChartID: chartID, IdempotencyKey: "provision-event-" + eventID,
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

For surface this SDK does not wrap yet, `Do` keeps auth and error mapping. Raw reads retain the
read retry policy; raw mutations are always single-attempt because their replay contract is unknown:

```go
client.Do(ctx, http.MethodPost, "/v1/events/ev_1/some-new-route", nil, map[string]any{"qty": 2}, "")
```

## API surface

| Service | Methods |
| --- | --- |
| `Charts` | `List` `All` `Create` `Retrieve` `Update` `Delete` `Copy` `Archive` `Unarchive` `Publish` |
| `Events` | `List` `All` `Create` `Retrieve` `RetrieveConfigurationBinding` `UpdateConfigurationBinding` `Update` `Delete` `UpdatePoster` `DeletePoster` `UpdateChart` `Close` `Reopen` `Archive` `RetrieveHoldTTL` `UpdateHoldTTL` `RetrieveReport` `RetrieveLog` |
| `Channels` | `ListChannels` `CreateChannel` `UpdateChannel` `UpdateAssignments` `ListAllocation` `RetrieveAccessPreview` `RetrieveReport` `Pause` `Unpause` `Archive` `CreateBuyerAccessSession` `ListBuyerAccessSessions` `RevokeBuyerAccessSession` `CreateAccessLink` `ListAccessLinks` `RotateAccessLink` `RevokeAccessLink` |
| `Inventory` | `Hold` `HoldBestAvailable` `BookBestAvailable` `ExtendHold` `RetrieveHold` `Release` `Book` `BoxOfficeBook` `Unbook` `Block` `Unblock` `UnblockAll` `RetrieveAvailability` `UpdateAvailability` `ListBookings` `RetrieveBooking` |
| `Sessions` | `CreateManageSession` `RevokeManageSession` `CreateDesignerSession` `RevokeDesignerSession` |
| `Webhooks` | `List` `Create` `Update` `Delete` `ListDeliveries` |
| `Workspaces` | `List` `Create` `Retrieve` `Update` |

Full reference: [docs.seatlayer.io/server-sdk](https://docs.seatlayer.io/server-sdk/install/)

## Frequently asked questions

### How do I book seats from Go?

Add the [`github.com/seatlayer/seatlayer-go` module](https://pkg.go.dev/github.com/seatlayer/seatlayer-go),
construct a client with `seatlayer.New` and your secret key, and call `client.Inventory.Book` with
the hold id and a stable `BookingRef`. When your own backend picks the seats — phone orders, box
office, comps — `Inventory.BookBestAvailable` and `Inventory.BoxOfficeBook` book outright with no
prior hold. A booking reference is required on every booking call, so each sale is tied to an
immutable order id you can reconcile against later.

### What does the server SDK do that the buyer SDK does not?

The buyer SDK runs in the browser or mobile app and only **selects and holds** seats. This Go SDK
runs on your trusted server and **inspects and books** them. Your secret key never reaches a buyer
surface: browsers receive short-lived, origin-bound tokens minted here through
`Sessions.CreateManageSession` or `Channels.CreateBuyerAccessSession`. Always price a sale from
`Inventory.RetrieveHold`, never from values the browser sent you.

### How do temporary seat holds work server-side?

A hold reserves seats against concurrent buyers for a limited checkout window. From Go you
retrieve it with `Inventory.RetrieveHold`, whose items and currency are authoritative for pricing,
and confirm it with `Inventory.Book`. Use `Inventory.ExtendHold` for a long checkout instead of
releasing and re-holding, which would hand the seats to whoever is racing for them. Booking is a
single automatic attempt: after an unknown network outcome you may reconcile and repeat the exact
same event, hold, and `BookingRef` — seats already booked under that reference are not sold again.

### Can I use my own payment provider?

Yes. SeatLayer never processes payment. Charge through Stripe, Adyen, Braintree, or any provider
you already use, calculating the total from the server-inspected hold items rather than from
client input, then call `Inventory.Book` with your charge or order id as the `BookingRef`. The
[holds and checkout guide](https://docs.seatlayer.io/buyer-sdk/holds-and-checkout/) walks through
the full handoff.

## Continue your Go integration

- [Follow the SeatLayer server SDK guide](https://docs.seatlayer.io/server-sdk/install/)
  for installation, authentication, and the full hold-to-booking flow.
- [Handle errors, retries, and safe booking repeats](https://docs.seatlayer.io/server-sdk/reliability/)
  before connecting a production order flow.
- [Verify SeatLayer webhooks](https://docs.seatlayer.io/server-sdk/webhooks/)
  to react to holds, expiry, and bookings on your server.
- [Browse the SeatLayer server API reference](https://docs.seatlayer.io/server-api/events/)
  for every endpoint behind this SDK.
- [Generate clients from the SeatLayer OpenAPI description](https://docs.seatlayer.io/openapi.json)
  or explore the raw API surface.
- [Point AI coding agents at the SeatLayer docs index](https://docs.seatlayer.io/llms.txt)
  (`llms.txt`) for an agent-readable map of the documentation.
- [Explore every SeatLayer SDK on GitHub](https://github.com/seatlayer)
  across web, mobile, and server.

## SeatLayer SDK ecosystem

| Surface | Package or source |
|---|---|
| JavaScript | [`@seatlayer/js`](https://www.npmjs.com/package/@seatlayer/js) |
| React | [`@seatlayer/react`](https://www.npmjs.com/package/@seatlayer/react) |
| React Native | [`@seatlayer/react-native`](https://www.npmjs.com/package/@seatlayer/react-native) |
| iOS | [`seatlayer-ios`](https://github.com/seatlayer/seatlayer-ios) |
| Flutter | [`seatlayer`](https://pub.dev/packages/seatlayer) |
| Android | [`seatlayer-android`](https://github.com/seatlayer/seatlayer-android) |
| Server SDKs | [Node.js, Python, PHP, Ruby, .NET, Java, and Go](https://docs.seatlayer.io/server-sdk/install/) |
| Node.js (server) | [`@seatlayer/server`](https://www.npmjs.com/package/@seatlayer/server) |
| Python (server) | [`seatlayer`](https://pypi.org/project/seatlayer/) |
| PHP (server) | [`seatlayer/seatlayer-php`](https://packagist.org/packages/seatlayer/seatlayer-php) |
| Ruby (server) | [`seatlayer`](https://rubygems.org/gems/seatlayer) |
| .NET (server) | [`SeatLayer`](https://www.nuget.org/packages/SeatLayer) |
| Java (server) | [`io.seatlayer:seatlayer-java`](https://central.sonatype.com/artifact/io.seatlayer/seatlayer-java) |
| Go (server) | [`github.com/seatlayer/seatlayer-go`](https://pkg.go.dev/github.com/seatlayer/seatlayer-go) (this module) |

## Development

```bash
gofmt -l .          # must be empty
go vet ./...
go test -race ./...
```

## License

MIT
