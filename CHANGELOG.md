# Changelog

## 0.7.0 — 2026-08-30

- Added coverage for all 48 Fixed Renewable Season server
  operations under `Client.Seasons`, with exact path encoding and
  operation-specific retry/idempotency behavior.
- Season allocations are identity-only and the API response declares host
  pricing authority. Buyer rehearsal validation sends no evidence body because
  SeatLayer discovers the retained hold, booking, cancellation, and delivered
  webhook chain automatically.

## 0.6.1

- Documentation only. Refreshes the README, adds frequently asked
  questions, and aligns package metadata. No API or behaviour changes.

## 0.6.0 — 2026-08-23

- Added exact immutable Event configuration binding reads and compare-and-set
  attach/detach through `Events.RetrieveConfigurationBinding` and
  `Events.UpdateConfigurationBinding`. Updates remain deliberately single-attempt.

## v0.5.0 — 2026-08-21

- Added `PerformanceGroups`, the trusted server service for fixed two-to-eight
  performance runs. It creates and activates groups, mints one-time browser
  access, retrieves authoritative group holds, and confirms bookings with
  stable action and order references. Browser-only group routes remain outside
  this secret-key SDK.

- Added template instantiation and ticket-release management
  (`Templates.InstantiateTemplate`, `Events.ListTicketReleases`,
  `Events.UpdateTicketReleases`, and `Events.CloseTicketRelease`). Template
  instantiation uses exact header replay; release replacement and close remain
  deliberately single-attempt.

- **Security/reliability:** Mutations now default to a single attempt. Automatic header-replay
  retries are limited to chart create/copy, template instantiation, event create, and workspace create, preventing
  transient failures from duplicating holds or best-available results and from issuing extra
  show-once credentials.
- Aligned event, inventory, webhook, manage-session, and Designer requests with the generated
  public contract. Named Go response types now preserve the webhook `subs`/`sub`, Designer
  `session`, and hold-inspection shapes; event chart acknowledgement, hold-TTL reset, scheduled
  blocks, trusted hold extension, and delivery filters are available.
- Removed unsupported buyer-access-session `State` and `Cursor` fields; that listing accepts only
  `Limit`.
- Added deterministic transport-contract coverage for stable error-code fallback, HTTP status,
  decoded body and `X-Request-ID` exposure, typed 429 `Retry-After` precedence, non-JSON gateway
  failures, and single-attempt unsafe mutations.
- Reached all 71 public operation wrappers with raw event-poster upload/removal and typed hosted
  access-link create/list/rotate/revoke results. One-time capability reveals stay single-attempt.
- Added exact chart copy/metadata params, typed event-log pagination, and explicit-null helpers for
  buyer sessions, channel archive destinations, and workspace creation.

## v0.2.0 — 2026-08-12

- Added channel allocation management and origin-bound buyer access sessions.
- Added channel-aware hold and booking controls, including explicit privileged override reasons.
- Added paginated booking lifecycle reads and encoded booking retrieval.
- Booking and cancellation calls now reject missing or blank stable booking references.
- Expanded the README with private-sale guidance and direct links across the SeatLayer SDK family.

## v0.1.0 — unreleased

First release of the SeatLayer Go server SDK.

- `Client` with secret-key auth, per-attempt timeouts, and a `Do` escape hatch.
- Services: `Charts`, `Events`, `Inventory`, `Sessions`, `Webhooks`, `Workspaces`.
- Every method takes a `context.Context`; cancellation stops retries immediately
  rather than being treated as a transient fault.
- Automatic `Idempotency-Key` on every mutation, reused across retries so a retried
  booking cannot become two bookings.
- Retries on 429/408/5xx with exponential backoff and full jitter; honours `Retry-After`.
  4xx is never retried.
- Typed errors reachable with `errors.As`: `AuthError` (with `ModeMismatch()`),
  `ConflictError` (with `Conflicts()` and `SoldOut()`), `RateLimitError`,
  `ValidationError`, `NotFoundError`, `ConnectionError` (which unwraps).
- `VerifyWebhook` — raw-body HMAC-SHA256 verification via `hmac.Equal`; errors wrap
  `ErrWebhookVerification` for `errors.Is`.
- `CreateManageSession` requires explicit capabilities; the API's default grants
  `event:cancel`, which reverses paid bookings.
- `BookBestAvailable` requires a `BookingRef` so the sale can be reconciled.
- `New` returns an error on a `pk_` key rather than failing as a 401 later.
- `All()` returns a range-over-func iterator (`iter.Seq2`), paging as you consume it,
  with page errors delivered alongside each item so they cannot be silently dropped.
- Standard library only — no dependencies.

Requires Go 1.23 (for range-over-func iterators).
