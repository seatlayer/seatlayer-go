package seatlayer

import (
	"context"
	"net/url"
	"strconv"
)

// OrdersService reads orders across the organization and runs refunds,
// cancellations and ticket resends on event orders. Season order actions
// live on SeasonsService.
type OrdersService struct{ client *Client }

// OrderRefundParams refunds an order in full or in part.
type OrderRefundParams struct {
	// AmountMinor refunds this much, in minor units. Zero leaves it out, which
	// refunds what is left.
	AmountMinor    int64
	IdempotencyKey string
}

func (p *OrderRefundParams) request() (any, string) {
	if p == nil {
		return nil, ""
	}
	return params("amountMinor", int64OrNil(p.AmountMinor)), p.IdempotencyKey
}

// OrderCancelParams cancels an order without moving money.
type OrderCancelParams struct {
	// Reason is one of lost_dispute, refunded_elsewhere or other.
	Reason string
	Note   string
	// RefundConfirmed and UsedTicketsAcknowledged are sent only when true.
	RefundConfirmed         bool
	UsedTicketsAcknowledged bool
	// NotifyBuyer is sent only when set, so an explicit false is possible.
	NotifyBuyer    *bool
	IdempotencyKey string
}

func (p OrderCancelParams) body() map[string]any {
	return params(
		"reason", p.Reason,
		"note", stringOrNil(p.Note),
		"refundConfirmed", boolOrNil(p.RefundConfirmed),
		"usedTicketsAcknowledged", boolOrNil(p.UsedTicketsAcknowledged),
		"notifyBuyer", pointerOrNil(p.NotifyBuyer),
	)
}

// OrderResendParams sends an order's tickets again, optionally to another email.
type OrderResendParams struct {
	Email string
}

func (p *OrderResendParams) body() any {
	if p == nil {
		return nil
	}
	return params("email", stringOrNil(p.Email))
}

// OrderListParams filters and pages the organization's orders.
type OrderListParams struct {
	// EventKey limits to one event's orders (not Season orders).
	EventKey string
	// SeasonKey limits to one Season's orders, renewals included.
	SeasonKey string
	// HolderRef limits to one holder's orders: the Season order with this id
	// and renewals offered to this reference.
	HolderRef string
	Status    string
	// From and To bound the creation time, in epoch milliseconds.
	From   int64
	To     int64
	Cursor string
	Limit  int
}

// EventOrderListParams filters and pages one event's orders.
type EventOrderListParams struct {
	// Q searches buyer name, email, order number or seat.
	Q      string
	Status string
	PaidBy string
	// From and To bound the payment time, in epoch milliseconds.
	From   int64
	To     int64
	Cursor string
	Limit  int
}

// EventOrderExportParams filters the event orders CSV export.
type EventOrderExportParams struct {
	// Q searches buyer name, email, order number or seat.
	Q      string
	Status string
	PaidBy string
	// From and To bound the payment time, in epoch milliseconds.
	From int64
	To   int64
}

func setInt64IfNotZero(values url.Values, key string, value int64) {
	if value != 0 {
		values.Set(key, strconv.FormatInt(value, 10))
	}
}

func setIntIfPositive(values url.Values, key string, value int) {
	if value > 0 {
		values.Set(key, strconv.Itoa(value))
	}
}

// List lists orders across events and Seasons, one cursor page at a time.
func (s *OrdersService) List(ctx context.Context, p *OrderListParams) (map[string]any, error) {
	query := url.Values{}
	if p != nil {
		setIfNotEmpty(query, "eventKey", p.EventKey)
		setIfNotEmpty(query, "seasonKey", p.SeasonKey)
		setIfNotEmpty(query, "holderRef", p.HolderRef)
		setIfNotEmpty(query, "status", p.Status)
		setInt64IfNotZero(query, "from", p.From)
		setInt64IfNotZero(query, "to", p.To)
		setIfNotEmpty(query, "cursor", p.Cursor)
		setIntIfPositive(query, "limit", p.Limit)
	}
	return s.client.get(ctx, "/v1/orders", query)
}

// Retrieve fetches an order and its lines.
func (s *OrdersService) Retrieve(ctx context.Context, orderID string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/orders/"+escape(orderID), nil)
}

// ListTickets lists an order's tickets with scan links.
func (s *OrdersService) ListTickets(ctx context.Context, orderID string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/orders/"+escape(orderID)+"/tickets", nil)
}

// ListEventOrders lists one event's orders, one cursor page at a time.
func (s *OrdersService) ListEventOrders(
	ctx context.Context, eventKey string, p *EventOrderListParams,
) (map[string]any, error) {
	query := url.Values{}
	if p != nil {
		setIfNotEmpty(query, "q", p.Q)
		setIfNotEmpty(query, "status", p.Status)
		setIfNotEmpty(query, "paidBy", p.PaidBy)
		setInt64IfNotZero(query, "from", p.From)
		setInt64IfNotZero(query, "to", p.To)
		setIfNotEmpty(query, "cursor", p.Cursor)
		setIntIfPositive(query, "limit", p.Limit)
	}
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/orders", query)
}

// RetrieveEventOrder fetches one event order with its guests and tickets.
func (s *OrdersService) RetrieveEventOrder(
	ctx context.Context, eventKey, orderID string,
) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/orders/"+escape(orderID), nil)
}

// ExportEventOrdersCSV downloads one event's orders as CSV text with a header
// row and one price line per row, returned unchanged.
func (s *OrdersService) ExportEventOrdersCSV(
	ctx context.Context, eventKey string, p *EventOrderExportParams,
) (string, error) {
	query := url.Values{}
	if p != nil {
		setIfNotEmpty(query, "q", p.Q)
		setIfNotEmpty(query, "status", p.Status)
		setIfNotEmpty(query, "paidBy", p.PaidBy)
		setInt64IfNotZero(query, "from", p.From)
		setInt64IfNotZero(query, "to", p.To)
	}
	return s.client.getText(ctx, "/v1/events/"+escape(eventKey)+"/orders.csv", query, "text/csv")
}

// RefundEventOrder refunds an event order through its payment provider. Pass
// nil to refund what is left.
func (s *OrdersService) RefundEventOrder(
	ctx context.Context, eventKey, orderID string, p *OrderRefundParams,
) (map[string]any, error) {
	body, idempotencyKey := p.request()
	return s.client.postHeaderReplay(ctx,
		"/v1/events/"+escape(eventKey)+"/orders/"+escape(orderID)+"/refund", body, idempotencyKey)
}

// CancelEventOrder cancels an event order without moving money.
func (s *OrdersService) CancelEventOrder(
	ctx context.Context, eventKey, orderID string, p OrderCancelParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx,
		"/v1/events/"+escape(eventKey)+"/orders/"+escape(orderID)+"/cancel", p.body(), p.IdempotencyKey)
}

// ResendEventOrderTickets emails an event order's tickets again. It has no
// replay contract, so it is single-attempt. Pass nil to send no body.
func (s *OrdersService) ResendEventOrderTickets(
	ctx context.Context, eventKey, orderID string, p *OrderResendParams,
) (map[string]any, error) {
	return s.client.post(ctx,
		"/v1/events/"+escape(eventKey)+"/orders/"+escape(orderID)+"/resend", p.body(), "")
}
