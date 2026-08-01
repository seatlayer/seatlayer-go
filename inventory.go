package seatlayer

import (
	"context"
	"errors"
)

// InventoryService covers holds, booking, blocking and availability.
//
// Two complete flows, both first-class:
//
//	browser holds → RetrieveHold for authoritative pricing → charge → Book(holdId)
//	backend books labels directly — box office, phone sales, comps
//
// Never price from what the browser tells you. RetrieveHold is the
// authoritative answer, which is why it is a separate call.
type InventoryService struct{ client *Client }

func (s *InventoryService) path(eventKey, suffix string) string {
	return "/v1/events/" + escape(eventKey) + suffix
}

// HoldParams reserves specific objects by label.
type HoldParams struct {
	Labels []string
	// Selections is the alternative to Labels when you need a tier or a
	// quantity, e.g. a shared table or a GA area.
	Selections []map[string]any
	// TTLMs overrides the event's checkout window for this hold.
	TTLMs          int64
	ReplaceHoldID  string
	IdempotencyKey string
}

// Hold reserves the named objects.
func (s *InventoryService) Hold(ctx context.Context, eventKey string, p HoldParams) (map[string]any, error) {
	body := params(
		"labels", sliceOrNil(p.Labels),
		"selections", mapSliceOrNil(p.Selections),
		"ttlMs", int64OrNil(p.TTLMs),
		"replaceHoldId", stringOrNil(p.ReplaceHoldID),
	)
	return s.client.post(ctx, s.path(eventKey, "/hold"), body, p.IdempotencyKey)
}

// BestAvailableParams asks us to choose the objects.
type BestAvailableParams struct {
	// Qty is clamped to the server maximum rather than rejected.
	Qty         int
	CategoryKey string
	ZoneID      string
	// TTLMs overrides the event's checkout window. Ignored by BookBestAvailable.
	TTLMs int64
	// BookingRef is required by BookBestAvailable and ignored by HoldBestAvailable.
	BookingRef     string
	IdempotencyKey string
}

// HoldBestAvailable picks the best free objects and holds them.
//
// The picker is the one the buyer widget uses, so a phone order and a web order
// get the same answer for the same inventory.
func (s *InventoryService) HoldBestAvailable(
	ctx context.Context, eventKey string, p BestAvailableParams,
) (map[string]any, error) {
	body := params(
		"qty", p.Qty,
		"categoryKey", stringOrNil(p.CategoryKey),
		"zoneId", stringOrNil(p.ZoneID),
		"ttlMs", int64OrNil(p.TTLMs),
	)
	return s.client.post(ctx, s.path(eventKey, "/best-available"), body, p.IdempotencyKey)
}

// BookBestAvailable picks and books in one call — the box-office shape.
//
// Prefer this over hold-then-book when payment is already taken: a failure
// between two calls would strand inventory until the TTL expired.
func (s *InventoryService) BookBestAvailable(
	ctx context.Context, eventKey string, p BestAvailableParams,
) (map[string]any, error) {
	if p.BookingRef == "" {
		// Required so the sale can be reconciled against your own order — caught
		// here rather than as a 400 after a round-trip.
		return nil, errors.New("seatlayer: BookingRef is required for BookBestAvailable")
	}
	body := params(
		"qty", p.Qty,
		"bookingRef", p.BookingRef,
		"categoryKey", stringOrNil(p.CategoryKey),
		"zoneId", stringOrNil(p.ZoneID),
	)
	return s.client.post(ctx, s.path(eventKey, "/best-available-book"), body, p.IdempotencyKey)
}

// ExtendHold pushes an active hold's expiry out by a fresh window.
//
// Use this rather than release-and-re-hold when an order takes longer than the
// checkout window — invoiced sales, a phone order on hold. Releasing first hands
// the seats to whoever is racing for them in between. A hold that is gone,
// expired, or at its renewal cap answers 409 cannot_extend.
func (s *InventoryService) ExtendHold(
	ctx context.Context, eventKey, holdID string, ttlMs int64,
) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/extend"),
		params("holdId", holdID, "ttlMs", int64OrNil(ttlMs)), "")
}

// RetrieveHold returns authoritative items and prices. Charge from this, not
// from what the browser sent you.
func (s *InventoryService) RetrieveHold(ctx context.Context, eventKey, holdID string) (map[string]any, error) {
	return s.client.get(ctx, s.path(eventKey, "/holds/"+escape(holdID)), nil)
}

// Release frees held objects before the TTL expires.
func (s *InventoryService) Release(
	ctx context.Context, eventKey string, labels []string, holdID string,
) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/release"),
		params("labels", labels, "holdId", holdID), "")
}

// BookParams books either a held selection or labels outright.
type BookParams struct {
	// HoldID books a previously held selection…
	HoldID string
	// …or Labels books outright, with no prior hold.
	Labels         []string
	BookingRef     string
	IdempotencyKey string
}

// Book confirms a sale.
func (s *InventoryService) Book(ctx context.Context, eventKey string, p BookParams) (map[string]any, error) {
	body := params(
		"holdId", stringOrNil(p.HoldID),
		"labels", sliceOrNil(p.Labels),
		"bookingRef", stringOrNil(p.BookingRef),
	)
	return s.client.post(ctx, s.path(eventKey, "/book"), body, p.IdempotencyKey)
}

// BoxOfficeBook books named objects as a box-office sale.
func (s *InventoryService) BoxOfficeBook(
	ctx context.Context, eventKey string, labels []string, bookingRef string,
) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/box-book"),
		params("labels", labels, "bookingRef", bookingRef), "")
}

// Unbook reverses a booking. Requires a key with cancel authority.
func (s *InventoryService) Unbook(ctx context.Context, eventKey string, labels []string) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/unbook"), params("labels", labels), "")
}

// Block holds inventory back from sale (house seats, production holds).
func (s *InventoryService) Block(ctx context.Context, eventKey string, labels []string) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/block"), params("labels", labels), "")
}

// Unblock returns blocked objects to sale.
func (s *InventoryService) Unblock(ctx context.Context, eventKey string, labels []string) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/unblock"), params("labels", labels), "")
}

// UnblockAll returns every blocked object in an event to sale.
func (s *InventoryService) UnblockAll(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/unblock-all"), nil, "")
}

// RetrieveAvailability reads per-object availability rules.
func (s *InventoryService) RetrieveAvailability(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, s.path(eventKey, "/availability"), nil)
}

// UpdateAvailability replaces per-object availability rules.
func (s *InventoryService) UpdateAvailability(
	ctx context.Context, eventKey string, fields map[string]any,
) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/availability"), fields, "")
}

func mapSliceOrNil(value []map[string]any) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
