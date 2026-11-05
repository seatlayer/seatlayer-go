package seatlayer

import (
	"context"
	"errors"
	"net/url"
)

// The event sales operations below live on EventsService because they are
// /v1/events/{eventKey}/… routes, like the rest of that service.

// EventBuyerCheckoutParams chooses who takes payment for an event.
type EventBuyerCheckoutParams struct {
	// BuyerCheckout is "seatlayer" or "integrator".
	BuyerCheckout    string
	ExpectedRevision int
}

// EventPaymentGatewayParams assigns a connected gateway to an event. Use
// FieldNull to send an explicit null.
type EventPaymentGatewayParams struct {
	ConnectionID NullableField[string]
	Provider     NullableField[string]
}

// EventSalesTransitionParams moves an event's sales to another state.
type EventSalesTransitionParams struct {
	// To is one of setup, testable, live, paused or ended.
	To               string
	ExpectedRevision int
	// Reason is optional. Empty leaves it out.
	Reason string
}

// EventDiscountCodeCreateParams creates a discount code. Zero values are left
// out, except Code and Kind, which are always sent.
type EventDiscountCodeCreateParams struct {
	Code string
	// Kind is "percent" or "amount".
	Kind        string
	Percent     int
	AmountMinor int64
	// StartsAt and EndsAt are epoch milliseconds.
	StartsAt int64
	EndsAt   int64
	MaxUses  int
	// Active is sent only when set, so an explicit false is possible.
	Active *bool
}

// EventDiscountCodeUpdateParams changes or pauses a discount code. Zero values
// are left out; use FieldNull to clear StartsAt, EndsAt or MaxUses.
type EventDiscountCodeUpdateParams struct {
	Code string
	// Kind is "percent" or "amount".
	Kind        string
	Percent     int
	AmountMinor int64
	// StartsAt and EndsAt are epoch milliseconds.
	StartsAt NullableField[int64]
	EndsAt   NullableField[int64]
	MaxUses  NullableField[int]
	// Active is sent only when set, so an explicit false is possible.
	Active *bool
	// State is "active" or "paused".
	State string
}

// RetrieveBuyerCheckout reads who takes payment for an event, and what blocks
// a change.
func (s *EventsService) RetrieveBuyerCheckout(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/buyer-checkout", nil)
}

// ReplaceBuyerCheckout chooses SeatLayer checkout or your own for an event. It
// is a compare-and-set on ExpectedRevision and is single-attempt.
func (s *EventsService) ReplaceBuyerCheckout(
	ctx context.Context, eventKey string, p EventBuyerCheckoutParams,
) (map[string]any, error) {
	return s.client.put(ctx, "/v1/events/"+escape(eventKey)+"/buyer-checkout", map[string]any{
		"buyerCheckout":    p.BuyerCheckout,
		"expectedRevision": p.ExpectedRevision,
	})
}

// RetrievePaymentGateway reads an event's payment gateway and the ones it could
// use. A non-empty currency checks the options against it instead of the
// event's own.
func (s *EventsService) RetrievePaymentGateway(
	ctx context.Context, eventKey, currency string,
) (map[string]any, error) {
	query := url.Values{}
	setIfNotEmpty(query, "currency", currency)
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/payment-gateway", query)
}

// ReplacePaymentGateway assigns one of the organization's connected gateways
// to an event. It has no replay contract, so it is single-attempt. Pass nil to
// send no body.
func (s *EventsService) ReplacePaymentGateway(
	ctx context.Context, eventKey string, p *EventPaymentGatewayParams,
) (map[string]any, error) {
	var body any
	if p != nil {
		body = nullableFields(map[string]any{},
			fieldRequestValue("connectionId", p.ConnectionID),
			fieldRequestValue("provider", p.Provider),
		)
	}
	return s.client.put(ctx, "/v1/events/"+escape(eventKey)+"/payment-gateway", body)
}

// RetrieveSalesReadiness reads what an event still needs before its next sales
// step.
func (s *EventsService) RetrieveSalesReadiness(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/sales/readiness", nil)
}

// PreflightSales records a test order as the event's sales preflight. It has no
// replay contract, so it is single-attempt.
func (s *EventsService) PreflightSales(
	ctx context.Context, eventKey, orderID string,
) (map[string]any, error) {
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/sales/preflight",
		map[string]any{"orderId": orderID}, "")
}

// TransitionSales moves an event's sales to testable, live, paused or ended. It
// is a compare-and-set on ExpectedRevision and is single-attempt.
func (s *EventsService) TransitionSales(
	ctx context.Context, eventKey string, p EventSalesTransitionParams,
) (map[string]any, error) {
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/sales/transition", params(
		"to", p.To,
		"expectedRevision", p.ExpectedRevision,
		"reason", stringOrNil(p.Reason),
	), "")
}

// RetrieveBookingFee reads an event's booking fee.
func (s *EventsService) RetrieveBookingFee(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/booking-fee", nil)
}

// ReplaceBookingFee sets or clears an event's booking fee, in minor units. Omit
// bookingFeeMinor to send JSON null and clear the fee. It has no replay
// contract, so it is single-attempt.
func (s *EventsService) ReplaceBookingFee(
	ctx context.Context, eventKey string, bookingFeeMinor ...int64,
) (map[string]any, error) {
	if len(bookingFeeMinor) > 1 {
		return nil, errors.New("seatlayer: ReplaceBookingFee accepts zero or one value")
	}
	var value any
	if len(bookingFeeMinor) == 1 {
		value = bookingFeeMinor[0]
	}
	return s.client.put(ctx, "/v1/events/"+escape(eventKey)+"/booking-fee",
		map[string]any{"bookingFeeMinor": value})
}

// ListDiscountCodes lists an event's discount codes.
func (s *EventsService) ListDiscountCodes(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/discount-codes", nil)
}

// CreateDiscountCode creates a discount code for an event. It has no replay
// contract, so it is single-attempt.
func (s *EventsService) CreateDiscountCode(
	ctx context.Context, eventKey string, p EventDiscountCodeCreateParams,
) (map[string]any, error) {
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/discount-codes", params(
		"code", p.Code,
		"kind", p.Kind,
		"percent", intOrNil(p.Percent),
		"amountMinor", int64OrNil(p.AmountMinor),
		"startsAt", int64OrNil(p.StartsAt),
		"endsAt", int64OrNil(p.EndsAt),
		"maxUses", intOrNil(p.MaxUses),
		"active", pointerOrNil(p.Active),
	), "")
}

// UpdateDiscountCode changes or pauses a discount code. It has no replay
// contract, so it is single-attempt.
func (s *EventsService) UpdateDiscountCode(
	ctx context.Context, eventKey, codeID string, p EventDiscountCodeUpdateParams,
) (map[string]any, error) {
	body := nullableFields(params(
		"code", stringOrNil(p.Code),
		"kind", stringOrNil(p.Kind),
		"percent", intOrNil(p.Percent),
		"amountMinor", int64OrNil(p.AmountMinor),
		"active", pointerOrNil(p.Active),
		"state", stringOrNil(p.State),
	),
		fieldRequestValue("startsAt", p.StartsAt),
		fieldRequestValue("endsAt", p.EndsAt),
		fieldRequestValue("maxUses", p.MaxUses),
	)
	return s.client.patch(ctx, "/v1/events/"+escape(eventKey)+"/discount-codes/"+escape(codeID), body)
}

// EndDiscountCode ends a discount code for good. It is single-attempt.
func (s *EventsService) EndDiscountCode(
	ctx context.Context, eventKey, codeID string,
) (map[string]any, error) {
	return s.client.post(ctx,
		"/v1/events/"+escape(eventKey)+"/discount-codes/"+escape(codeID)+"/end", nil, "")
}

// DeleteDiscountCode deletes a discount code nobody has used. The API answers
// 204 with no body, so only an error comes back. It is single-attempt.
func (s *EventsService) DeleteDiscountCode(ctx context.Context, eventKey, codeID string) error {
	_, err := s.client.delete(ctx, "/v1/events/"+escape(eventKey)+"/discount-codes/"+escape(codeID))
	return err
}
