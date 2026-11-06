# Changelog

## Unreleased

- Cancelling a date voids its tickets as a `date_cancel_void` job, named by
  `SeasonDateCancelVoidJobID(amendmentID)` (`sbj_void_…`); follow it with
  `Seasons.RetrieveBulkJob`. `season.ticket.voided` says `date_cancelled` for
  each Season ticket it voids.
- A Season package holds 2 to 50 dates. `SeasonHolderImportRowsPerCall(dates)`
  gives the most rows one `Seasons.CreateHolderImport` call takes (100 up to
  20 dates, 43 at 50); a call with more answers `422 invalid_rows`, a
  `*ValidationError` with `maxRows` in its `Body`.
- `Seasons.Update` answers may carry `singleTicketDates.jobRefused`: why a
  `season_singles` job could not start. The change stands, and sending
  `SingleTickets` true again opens any date still not open. An `Update`
  retried with its first `IdempotencyKey` and `ExpectedRevision` is not
  applied twice.
- An `EventSeries.Create` given up for its key answers `409 create_abandoned`
  (a `*ConflictError`) with a message to send it again with a new idempotency
  key.
- Added the 39 remaining Season operations to `Client.Seasons` (87 in all):
  performances, package updates and prices, `Publish`, checkout tests,
  occurrence inventory returns and reclaims, the holder list, Season order
  refunds, cancellations and resends, hosted renewals and contacts, renewal
  notices, renewal quotes, intents and checkouts, sales and seat reports, seat counts, two CSV exports, bulk
  refunds, and the release of lapsed offers, with their jobs.
- Added `Client.Holders` (holders, their tickets and transfers, and
  `ListCustomers`), `Client.Orders` (organization and event orders, refunds,
  cancellations, resends and the event orders CSV export) and
  `Client.EventSeries` (create, list, retrieve, add dates, update).
- Added `Holders.CreateReturn`: give back a holder's seat on one date, one
  per ticket.
- Added event sales settings to `Client.Events`: buyer checkout, payment
  gateway, sales readiness, preflight and transitions, booking fee, and
  discount codes.
- CSV exports return the CSV text unchanged as a `string`. 204 deletes
  (`Seasons.DeletePlan`, `Events.DeleteDiscountCode`) return only an error.
- 27 of the new mutations use exact header replay, bringing the total to 41.
  Domain-exact and unsupported operations stay single-attempt.
- Added ten `WebhookEvent…` constants: `series.created`,
  `series.dates_added`, `order.paid`, `order.refunded`, `order.cancelled`,
  `customer.created`, `customer.email_changed`,
  `season.ticket.transfer_sent`, `season.ticket.transferred` and
  `season.ticket.transfer_cancelled`.
- Added `Seasons.RefundPerformance` (refund every order of a cancelled date as
  a job, with header replay; `SeasonPerformanceRefundParams`),
  `Seasons.ListBulkJobs`, and `Seasons.RetrieveBulkJob`.
- `SeasonPackagePrice` takes `PriceTypeKey`, with the `SeasonPriceTypeAdult`,
  `…Child`, `…Concession`, `…Senior` and `…Student` constants.
- `SeasonAmendmentParams` takes `Refund` and `Reason`, for the new `postpone`
  and `cancel` kinds.
- `SeasonUpdateParams` takes `CheckoutMode`, `TransferCutoffHours` and
  `ReturnCutoffHours`.
- `SeasonHolderImportRow` takes `HolderEmail` and `HolderName`.
- Webhook create and update accept all 56 public event names, with a
  `WebhookEvent…` constant for each; they used to refuse every Season event
  except the three ticket transfer ones.
- `EventListParams` takes `SeriesKey` to list one event series' dates.
- Added `Seasons.RetrieveRenewalQuote`: what renewing an offer costs, and whether SeatLayer checkout can take it now.
- Added `Seasons.CreateRenewalIntent`: record a holder's intent for a renewal taken by phone or at the box office, so your own checkout can commit.
- Added `Seasons.CreateRenewalCheckout`: a SeatLayer checkout that renews an offer with no hosted page, with header replay (`SeasonRenewalCheckoutParams`).
- Added `Holders.CreateRenewalCheckout`: the same checkout for one of a holder's offers, with header replay.
- Added `Holders.DeclineRenewalOffer`: decline one of a holder's offers, with an optional reason (`SeasonRenewalDeclineParams`; nil sends no body).
- `Seasons.DeclineRenewalOffer` takes an optional `SeasonRenewalDeclineParams` with the holder's `Reason`; existing calls compile unchanged.
- `SeasonUpdateParams` takes `Audience` (`embed_only` or `public`).
- `Seasons.OpenSales` answers `warnings` (`checkout_test_missing` until a checkout test or rehearsal has passed) and can refuse with `409 plan_not_published` or `409 checkout_mode_not_chosen`.
- A cancelled date refunds per date only: `SeasonAmendmentParams.Refund` is `none` or `per_date`, and `SeasonPerformanceRefundParams.Amount` is `per_date` (`full` answers `422 date_refund_per_date_only`; refund a whole order with `Orders.Refund`).
- `EventSeriesDatesParams.RetryFailed` makes the failed dates again (`409 series_busy`, `422 nothing_to_retry`).
- `OrderListParams.HolderRef` lists one holder's orders; orders carry `holderRef` and `customerId`, and `buyer.email` can be null (a box office sale with no email).
- Season holds, give-backs and date cancels can refuse with `performance_cancelled`, `refund_not_started` (503), `missing_manage_capability` (403) or `invalid_refund`; a holder's status can be `reserved`; hosted renewal readiness carries `missingPrices`.
- `EventSeries.Create` takes `FromEventKey`: an event you already set up becomes the first date under its own key, and every other date is a copy of it. Leave `Name` and `ChartKey` empty with it. It refuses with `404 event_not_found`, or `422 event_has_no_date`, `event_in_season`, `event_in_performance_group`, `event_in_series` or `invalid_request`. An empty `Name` or `ChartKey` is now left out of the request.
- Every date of an event series carries the series' plain name (`Hamlet`, not `Hamlet · 3 Nov`); tell dates apart by `startsAt`. `EventSeries.Update` renames each date that carries the series name, and a series name cannot hold `{n}`, `{date}` or `{time}`.
- Added `Seasons.ReleaseLapsedOffers`: release the seats of every lapsed renewal offer as a job (`202 {"job": ...}`), with header replay. An offer extended before its turn is skipped and keeps its seat. Follow it with `RetrieveBulkJob`.
- `Seasons.RetrieveHostedRenewals` and `Seasons.ListRenewalOffers` read one page at a time, newest first: pass one `SeasonOfferPageParams` with `Cursor` (the `nextCursor` of the page before) and `Limit` (1-500 and 1-100). Both answers carry `nextCursor`, null on the last page. Existing calls compile unchanged and read the first page. A bad cursor or limit answers `400 invalid_cursor` or `invalid_limit`.
- `Seasons.CreateRenewalOffers` answers `201 {"offers": [...]}` for 100 contracts or fewer, and `202 {"job": ..., "batchId": ...}` above that: a `renewal_run` job makes the offers. Follow it with `RetrieveBulkJob`, then read the offers with `ListRenewalOffers`. It can refuse with `409 job_mismatch` or `503 season_not_configured`.
- `Seasons.CreateHolderImportFromSeason` answers `202 {"job": ...}` when a commit brings across more than 100 holders: a `holder_import` job, whose `summary` in `RetrieveBulkJob` has the same counts when it completes. It can refuse with `409 job_mismatch`, or `503 job_busy` or `season_not_configured`.
- `Seasons.CreateRenewalNotices` takes up to 20,000 offer ids; more answers `422 too_many_offer_ids`.
- `SeasonRenewalNoticesParams` takes `BatchID` instead of `OfferIDs`: `CreateRenewalNotices` emails every open offer one `CreateRenewalOffers` batch made, such as the batch a `renewal_run` job made.
- Single tickets on a season: `SeasonCreateParams` and `SeasonUpdateParams` take `SingleTickets` (`*bool`; nil leaves it out, and the default is Season only; anything else answers `422 invalid_single_tickets`). Every season read carries `singleTickets`, and a season on sale may carry `singleTicketsNotOpen`. `Seasons.Update`, `OpenSales` and `ResumeSales` answer `singleTicketDates` (`direction`, `opened`, `closed`, `notOpened`, `jobId`) when they open or stop the dates' own sales; a `season_singles` job does it for more than 12 upcoming dates.

## 0.8.0 (2026-09-10)

- Added Event hosting regions: `EventCreateParams.Region` with the
  `EventHostingRegion` constants, `WorkspaceCreateParams.DefaultRegion`, and
  `Workspaces.UpdateWithParams` for changing the default used by future Events.

## 0.7.0 (2026-08-30)

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

## 0.6.0 (2026-08-23)

- Added exact immutable Event configuration binding reads and compare-and-set
  attach/detach through `Events.RetrieveConfigurationBinding` and
  `Events.UpdateConfigurationBinding`. Updates remain deliberately single-attempt.

## v0.5.0 (2026-08-21)

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

## v0.2.0 (2026-08-12)

- Added channel allocation management and origin-bound buyer access sessions.
- Added channel-aware hold and booking controls, including explicit privileged override reasons.
- Added paginated booking lifecycle reads and encoded booking retrieval.
- Booking and cancellation calls now reject missing or blank stable booking references.
- Expanded the README with private-sale guidance and direct links across the SeatLayer SDK family.

## v0.1.0 (unreleased)

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
- `VerifyWebhook`: raw-body HMAC-SHA256 verification via `hmac.Equal`; errors wrap
  `ErrWebhookVerification` for `errors.Is`.
- `CreateManageSession` requires explicit capabilities; the API's default grants
  `event:cancel`, which reverses paid bookings.
- `BookBestAvailable` requires a `BookingRef` so the sale can be reconciled.
- `New` returns an error on a `pk_` key rather than failing as a 401 later.
- `All()` returns a range-over-func iterator (`iter.Seq2`), paging as you consume it,
  with page errors delivered alongside each item so they cannot be silently dropped.
- Standard library only, no dependencies.

Requires Go 1.23 (for range-over-func iterators).
