package seatlayer

import (
	"context"
	"net/url"
	"strconv"
)

// PerformanceGroupsService manages fixed multi-performance runs. It is a
// secret-key surface: create a browser session here, then pass the revealed
// token to PerformanceGroupPicker in the browser SDK.
type PerformanceGroupsService struct{ client *Client }

// PerformanceGroupListParams filters one page of fixed runs.
type PerformanceGroupListParams struct {
	WorkspaceID string
	ExternalRef string
	State       string
	Limit       int
	Cursor      string
}

func (p *PerformanceGroupListParams) query() url.Values {
	values := url.Values{}
	if p == nil {
		return values
	}
	setIfNotEmpty(values, "workspaceId", p.WorkspaceID)
	setIfNotEmpty(values, "externalRef", p.ExternalRef)
	setIfNotEmpty(values, "state", p.State)
	setIfNotEmpty(values, "cursor", p.Cursor)
	if p.Limit > 0 {
		values.Set("limit", strconv.Itoa(p.Limit))
	}
	return values
}

// PerformanceGroupCreateParams describes a draft run. It must have two to
// eight compatible assigned-seat events; the API enforces that compatibility.
type PerformanceGroupCreateParams struct {
	Name           string
	EventKeys      []string
	ExternalRef    NullableField[string]
	IdempotencyKey string
}

// PerformanceGroupBuyerAccessSessionParams scopes one browser bearer.
type PerformanceGroupBuyerAccessSessionParams struct {
	AllowedOrigin     string
	IncludePublic     bool
	ChannelIDsByEvent map[string][]string
	ExpiresInSeconds  int
	MaxQuantity       NullableField[int]
	BuyerRef          NullableField[string]
	PartnerRef        NullableField[string]
}

// List returns one page of fixed performance runs.
func (s *PerformanceGroupsService) List(
	ctx context.Context, p *PerformanceGroupListParams,
) (map[string]any, error) {
	return s.client.get(ctx, "/v1/performance-groups", p.query())
}

// Create makes a draft run with exact header replay.
func (s *PerformanceGroupsService) Create(
	ctx context.Context, p PerformanceGroupCreateParams,
) (map[string]any, error) {
	body := params("name", p.Name, "eventKeys", p.EventKeys)
	if value, present := p.ExternalRef.requestValue(); present {
		body["externalRef"] = value
	}
	return s.client.postHeaderReplay(ctx, "/v1/performance-groups", body, p.IdempotencyKey)
}

// Retrieve returns one run and its ordered performances.
func (s *PerformanceGroupsService) Retrieve(
	ctx context.Context, performanceGroupKey string,
) (map[string]any, error) {
	return s.client.get(ctx, performanceGroupPath(performanceGroupKey, ""), nil)
}

// Delete removes a draft run only. Activated runs retain their audit identity.
func (s *PerformanceGroupsService) Delete(ctx context.Context, performanceGroupKey string) error {
	_, err := s.client.delete(ctx, performanceGroupPath(performanceGroupKey, ""))
	return err
}

// Activate starts lifecycle coordination. Poll RetrieveLifecycle when the
// returned lifecycleOperation is not terminal.
func (s *PerformanceGroupsService) Activate(
	ctx context.Context, performanceGroupKey string, expectedRevision int,
) (map[string]any, error) {
	return s.client.post(
		ctx, performanceGroupPath(performanceGroupKey, "/activate"),
		params("expectedRevision", expectedRevision), "")
}

// Close stops new group sales. Poll RetrieveLifecycle until the close is terminal.
func (s *PerformanceGroupsService) Close(
	ctx context.Context, performanceGroupKey string, expectedRevision int,
) (map[string]any, error) {
	return s.client.post(
		ctx, performanceGroupPath(performanceGroupKey, "/close"),
		params("expectedRevision", expectedRevision), "")
}

// RetrieveLifecycle reads the lifecycle operation returned by Activate or Close.
func (s *PerformanceGroupsService) RetrieveLifecycle(
	ctx context.Context, performanceGroupKey, operationID string,
) (map[string]any, error) {
	return s.client.get(ctx, performanceGroupPath(
		performanceGroupKey, "/lifecycle/"+escape(operationID)), nil)
}

// CreateBuyerAccessSession reveals one origin-bound browser bearer. This
// one-time-secret operation is deliberately single-attempt.
func (s *PerformanceGroupsService) CreateBuyerAccessSession(
	ctx context.Context, performanceGroupKey string, p PerformanceGroupBuyerAccessSessionParams,
) (map[string]any, error) {
	body := params(
		"allowedOrigin", p.AllowedOrigin,
		"includePublic", p.IncludePublic,
		"channelIdsByEvent", p.ChannelIDsByEvent,
		"expiresInSeconds", intOrNil(p.ExpiresInSeconds),
	)
	if value, present := p.MaxQuantity.requestValue(); present {
		body["maxQuantity"] = value
	}
	if value, present := p.BuyerRef.requestValue(); present {
		body["buyerRef"] = value
	}
	if value, present := p.PartnerRef.requestValue(); present {
		body["partnerRef"] = value
	}
	return s.client.post(ctx, performanceGroupPath(performanceGroupKey, "/buyer-access-sessions"), body, "")
}

// ListBuyerAccessSessions returns the current token records without their bearer values.
func (s *PerformanceGroupsService) ListBuyerAccessSessions(
	ctx context.Context, performanceGroupKey string, limit int,
) (map[string]any, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	return s.client.get(ctx, performanceGroupPath(performanceGroupKey, "/buyer-access-sessions"), query)
}

// RevokeBuyerAccessSession prevents a browser bearer from starting another hold.
func (s *PerformanceGroupsService) RevokeBuyerAccessSession(
	ctx context.Context, performanceGroupKey, sessionID string,
) (map[string]any, error) {
	return s.client.delete(ctx, performanceGroupPath(
		performanceGroupKey, "/buyer-access-sessions/"+escape(sessionID)))
}

// RetrieveHold is the trusted server projection of one group hold.
func (s *PerformanceGroupsService) RetrieveHold(
	ctx context.Context, performanceGroupKey, operationID string,
) (map[string]any, error) {
	return s.client.get(ctx, performanceGroupPath(performanceGroupKey, "/holds/"+escape(operationID)), nil)
}

// BookHold confirms payment on a committed group hold. Keep both IDs stable
// and poll RetrieveBooking while the returned booking state is book_pending.
func (s *PerformanceGroupsService) BookHold(
	ctx context.Context, performanceGroupKey, operationID, bookActionID, bookingRef string,
) (map[string]any, error) {
	return s.client.post(ctx, performanceGroupPath(
		performanceGroupKey, "/holds/"+escape(operationID)+"/book"),
		params("bookActionId", bookActionID, "bookingRef", bookingRef), "")
}

// RetrieveBooking returns a group booking operation, including its terminal outcome.
func (s *PerformanceGroupsService) RetrieveBooking(
	ctx context.Context, performanceGroupKey, actionID string,
) (map[string]any, error) {
	return s.client.get(ctx, performanceGroupPath(performanceGroupKey, "/bookings/"+escape(actionID)), nil)
}

func performanceGroupPath(performanceGroupKey, suffix string) string {
	return "/v1/performance-groups/" + escape(performanceGroupKey) + suffix
}
