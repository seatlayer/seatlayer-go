# Changelog

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
