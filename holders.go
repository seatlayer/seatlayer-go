package seatlayer

import (
	"context"
	"net/http"
	"net/url"
)

// HoldersService reads and changes ticket holders, their tickets and ticket
// transfers, and lists the customers behind them.
type HoldersService struct{ client *Client }

// CustomerListParams filters and pages the organization's customers.
type CustomerListParams struct {
	Email     string
	HolderRef string
	Cursor    string
	Limit     int
}

// HolderUpdateParams changes a holder's contact details. An empty Email is
// left out; use FieldNull to clear Name or Phone.
type HolderUpdateParams struct {
	Email          string
	Name           NullableField[string]
	Phone          NullableField[string]
	IdempotencyKey string
}

// HolderSessionRevokeParams signs out one session instead of all of them.
type HolderSessionRevokeParams struct {
	SessionID string
}

// HolderTicketListParams filters and pages a holder's tickets.
type HolderTicketListParams struct {
	SeasonKey string
	// From and To bound the date start time, in epoch milliseconds.
	From   int64
	To     int64
	Cursor string
	Limit  int
}

// HolderTransferCreateParams sends some of a holder's Season tickets to an email.
type HolderTransferCreateParams struct {
	TicketIDs      []string
	ToEmail        string
	IdempotencyKey string
}

// ListCustomers lists or finds customers by email or holder reference.
func (s *HoldersService) ListCustomers(ctx context.Context, p *CustomerListParams) (map[string]any, error) {
	query := url.Values{}
	if p != nil {
		setIfNotEmpty(query, "email", p.Email)
		setIfNotEmpty(query, "holderRef", p.HolderRef)
		setIfNotEmpty(query, "cursor", p.Cursor)
		setIntIfPositive(query, "limit", p.Limit)
	}
	return s.client.get(ctx, "/v1/customers", query)
}

// Retrieve fetches the customer behind a holder reference.
func (s *HoldersService) Retrieve(ctx context.Context, holderRef string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/holders/"+escape(holderRef), nil)
}

// Update sets a holder's email, name or phone.
func (s *HoldersService) Update(
	ctx context.Context, holderRef string, p HolderUpdateParams,
) (map[string]any, error) {
	body := nullableFields(params("email", stringOrNil(p.Email)),
		fieldRequestValue("name", p.Name),
		fieldRequestValue("phone", p.Phone),
	)
	return s.client.mutationHeaderReplay(ctx, http.MethodPatch,
		"/v1/holders/"+escape(holderRef), body, p.IdempotencyKey)
}

// RevokeSessions signs a holder's customer out of the ticket site. Pass nil to
// sign out every session and send no body.
func (s *HoldersService) RevokeSessions(
	ctx context.Context, holderRef string, p *HolderSessionRevokeParams,
) (map[string]any, error) {
	var body any
	if p != nil {
		body = params("sessionId", stringOrNil(p.SessionID))
	}
	return s.client.post(ctx, "/v1/holders/"+escape(holderRef)+"/sessions/revoke", body, "")
}

// ListSeasons lists the Seasons a holder holds seats in.
func (s *HoldersService) ListSeasons(ctx context.Context, holderRef string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/holders/"+escape(holderRef)+"/seasons", nil)
}

// ListTickets lists a holder's tickets, date by date.
func (s *HoldersService) ListTickets(
	ctx context.Context, holderRef string, p *HolderTicketListParams,
) (map[string]any, error) {
	query := url.Values{}
	if p != nil {
		setIfNotEmpty(query, "seasonKey", p.SeasonKey)
		setInt64IfNotZero(query, "from", p.From)
		setInt64IfNotZero(query, "to", p.To)
		setIfNotEmpty(query, "cursor", p.Cursor)
		setIntIfPositive(query, "limit", p.Limit)
	}
	return s.client.get(ctx, "/v1/holders/"+escape(holderRef)+"/tickets", query)
}

// RetrieveTicket fetches one of a holder's tickets with its QR.
func (s *HoldersService) RetrieveTicket(
	ctx context.Context, holderRef, ticketID string,
) (map[string]any, error) {
	return s.client.get(ctx, "/v1/holders/"+escape(holderRef)+"/tickets/"+escape(ticketID), nil)
}

// CreateTransfer sends some of a holder's Season tickets to an email.
func (s *HoldersService) CreateTransfer(
	ctx context.Context, holderRef string, p HolderTransferCreateParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx, "/v1/holders/"+escape(holderRef)+"/transfers",
		params("ticketIds", p.TicketIDs, "toEmail", p.ToEmail), p.IdempotencyKey)
}

// ListTransfers lists transfers a holder sent or was sent.
func (s *HoldersService) ListTransfers(ctx context.Context, holderRef string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/holders/"+escape(holderRef)+"/transfers", nil)
}

// CancelTransfer cancels a transfer that has not been accepted. It is
// single-attempt.
func (s *HoldersService) CancelTransfer(
	ctx context.Context, holderRef, transferID string,
) (map[string]any, error) {
	return s.client.post(ctx,
		"/v1/holders/"+escape(holderRef)+"/transfers/"+escape(transferID)+"/cancel", nil, "")
}

// AcceptTransfer accepts a transfer for the recipient holder. It is
// single-attempt.
func (s *HoldersService) AcceptTransfer(
	ctx context.Context, holderRef, transferID string,
) (map[string]any, error) {
	return s.client.post(ctx,
		"/v1/holders/"+escape(holderRef)+"/transfers/"+escape(transferID)+"/accept", nil, "")
}
