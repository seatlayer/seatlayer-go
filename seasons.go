package seatlayer

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
)

// SeasonsService manages Fixed Renewable Seasons from trusted server code.
// Browser selection belongs in the distinct SeasonPicker and receives only a
// scoped buyer token minted through this service.
type SeasonsService struct{ client *Client }

type SeasonListParams struct {
	WorkspaceID    string
	StructureState string
	Limit          int
	Cursor         string
}

func (p *SeasonListParams) query() url.Values {
	values := url.Values{}
	if p == nil {
		return values
	}
	setIfNotEmpty(values, "workspaceId", p.WorkspaceID)
	setIfNotEmpty(values, "structureState", p.StructureState)
	setIfNotEmpty(values, "cursor", p.Cursor)
	if p.Limit > 0 {
		values.Set("limit", strconv.Itoa(p.Limit))
	}
	return values
}

type SeasonSelectionParams struct {
	EventKeys                  []string
	SourcePerformanceGroupKeys []string
}

func (p SeasonSelectionParams) body() map[string]any {
	return params(
		"eventKeys", seasonSliceOrNil(p.EventKeys),
		"sourcePerformanceGroupKeys", seasonSliceOrNil(p.SourcePerformanceGroupKeys),
	)
}

type SeasonCreateParams struct {
	Name                       string
	Edition                    NullableField[string]
	EventKeys                  []string
	SourcePerformanceGroupKeys []string
	IdempotencyKey             string
}

type SeasonUpdateParams struct {
	ExpectedRevision int
	Name             string
	Edition          NullableField[string]
	IdempotencyKey   string
}

type SeasonPlanCreateParams struct {
	Name                       string
	EventKeys                  []string
	SourcePerformanceGroupKeys []string
	IdempotencyKey             string
}

type SeasonDuplicateToLiveParams struct {
	EventKeys      []string
	Name           string
	IdempotencyKey string
}

type SeasonBuyerAccessSessionParams struct {
	AllowedOrigin    string
	IncludePublic    bool
	ExpiresInSeconds int
	MaxQuantity      NullableField[int]
	BuyerRef         NullableField[string]
}

type SeasonCancelBookingParams struct {
	CancelActionID   string
	BookingRef       string
	PlanActivationID string
	RightDisposition string
}

type SeasonHolderImportRow struct {
	RowID                 string   `json:"rowId"`
	HolderRef             string   `json:"holderRef"`
	PriorPlanActivationID string   `json:"priorPlanActivationId"`
	PriorContractRef      string   `json:"priorContractRef"`
	Labels                []string `json:"labels"`
	ExistingBookingRef    *string  `json:"existingBookingRef,omitempty"`
}

type SeasonHolderImportParams struct {
	SuccessorPlanActivationID string
	DryRun                    *bool
	Rows                      []SeasonHolderImportRow
	IdempotencyKey            string
}

type SeasonRenewalOffersParams struct {
	SuccessorPlanActivationID string
	DeadlineAt                int64
	ContractIDs               []string
	IdempotencyKey            string
}

type SeasonAmendmentParams struct {
	EventKey       string
	Kind           string
	StartsAt       int64
	Name           string
	IdempotencyKey string
}

type SeasonSupportLookupParams struct {
	BookingRef string
	HolderRef  string
}

func seasonPath(seasonKey, suffix string) string {
	return "/v1/seasons/" + escape(seasonKey) + suffix
}

func seasonSliceOrNil[T any](values []T) any {
	if values == nil {
		return nil
	}
	return values
}

// List returns one cursor page of Seasons.
func (s *SeasonsService) List(ctx context.Context, p *SeasonListParams) (map[string]any, error) {
	return s.client.get(ctx, "/v1/seasons", p.query())
}

// Validate is a read-only compatibility preflight.
func (s *SeasonsService) Validate(ctx context.Context, p SeasonSelectionParams) (map[string]any, error) {
	return s.client.post(ctx, "/v1/seasons/validate", p.body(), "")
}

func (s *SeasonsService) Create(ctx context.Context, p SeasonCreateParams) (map[string]any, error) {
	body := p.SeasonSelectionParams().body()
	body["name"] = p.Name
	if value, present := p.Edition.requestValue(); present {
		body["edition"] = value
	}
	return s.client.postHeaderReplay(ctx, "/v1/seasons", body, p.IdempotencyKey)
}

func (p SeasonCreateParams) SeasonSelectionParams() SeasonSelectionParams {
	return SeasonSelectionParams{
		EventKeys: p.EventKeys, SourcePerformanceGroupKeys: p.SourcePerformanceGroupKeys,
	}
}

func (s *SeasonsService) Retrieve(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, ""), nil)
}

func (s *SeasonsService) Update(
	ctx context.Context, seasonKey string, p SeasonUpdateParams,
) (map[string]any, error) {
	body := params("expectedRevision", p.ExpectedRevision, "name", stringOrNil(p.Name))
	if value, present := p.Edition.requestValue(); present {
		body["edition"] = value
	}
	return s.client.mutationHeaderReplay(
		ctx, http.MethodPatch, seasonPath(seasonKey, ""), body, p.IdempotencyKey)
}

func (s *SeasonsService) Delete(
	ctx context.Context, seasonKey, idempotencyKey string,
) error {
	_, err := s.client.mutationHeaderReplay(
		ctx, http.MethodDelete, seasonPath(seasonKey, ""), nil, idempotencyKey)
	return err
}

func (s *SeasonsService) Activate(
	ctx context.Context, seasonKey string, expectedRevision int,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/activate"),
		params("expectedRevision", expectedRevision), "")
}

func (s *SeasonsService) Close(
	ctx context.Context, seasonKey string, expectedRevision int,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/close"),
		params("expectedRevision", expectedRevision), "")
}

func (s *SeasonsService) Archive(
	ctx context.Context, seasonKey string, expectedRevision int,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/archive"),
		params("expectedRevision", expectedRevision), "")
}

func (s *SeasonsService) RetrieveLifecycle(
	ctx context.Context, seasonKey, operationID string,
) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/lifecycle/"+escape(operationID)), nil)
}

func (s *SeasonsService) CreatePlan(
	ctx context.Context, seasonKey string, p SeasonPlanCreateParams,
) (map[string]any, error) {
	body := SeasonSelectionParams{
		EventKeys: p.EventKeys, SourcePerformanceGroupKeys: p.SourcePerformanceGroupKeys,
	}.body()
	body["name"] = p.Name
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/plans"), body, p.IdempotencyKey)
}

func (s *SeasonsService) RetrievePlan(
	ctx context.Context, seasonKey, planKey string,
) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/plans/"+escape(planKey)), nil)
}

func (s *SeasonsService) PublishPlan(
	ctx context.Context, seasonKey, planKey string, expectedRevision int,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/plans/"+escape(planKey)+"/publish"),
		params("expectedRevision", expectedRevision), "")
}

func (s *SeasonsService) SupersedePlan(
	ctx context.Context, seasonKey, planKey string, expectedRevision int,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/plans/"+escape(planKey)+"/supersede"),
		params("expectedRevision", expectedRevision), "")
}

func (s *SeasonsService) sales(
	ctx context.Context, seasonKey, action string, expectedRevision int,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/sales/"+action),
		params("expectedRevision", expectedRevision), "")
}

func (s *SeasonsService) OpenSales(ctx context.Context, seasonKey string, expectedRevision int) (map[string]any, error) {
	return s.sales(ctx, seasonKey, "open", expectedRevision)
}

func (s *SeasonsService) PauseSales(ctx context.Context, seasonKey string, expectedRevision int) (map[string]any, error) {
	return s.sales(ctx, seasonKey, "pause", expectedRevision)
}

func (s *SeasonsService) ResumeSales(ctx context.Context, seasonKey string, expectedRevision int) (map[string]any, error) {
	return s.sales(ctx, seasonKey, "resume", expectedRevision)
}

func (s *SeasonsService) EndSales(ctx context.Context, seasonKey string, expectedRevision int) (map[string]any, error) {
	return s.sales(ctx, seasonKey, "end", expectedRevision)
}

func (s *SeasonsService) DuplicateToLive(
	ctx context.Context, seasonKey string, p SeasonDuplicateToLiveParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/duplicate-to-live"),
		params("eventKeys", p.EventKeys, "name", stringOrNil(p.Name)), p.IdempotencyKey)
}

// CreateBuyerAccessSession reveals a show-once bearer and is deliberately single-attempt.
func (s *SeasonsService) CreateBuyerAccessSession(
	ctx context.Context, seasonKey string, p SeasonBuyerAccessSessionParams,
) (map[string]any, error) {
	body := params(
		"allowedOrigin", p.AllowedOrigin,
		"includePublic", p.IncludePublic,
		"expiresInSeconds", intOrNil(p.ExpiresInSeconds),
	)
	if value, present := p.MaxQuantity.requestValue(); present {
		body["maxQuantity"] = value
	}
	if value, present := p.BuyerRef.requestValue(); present {
		body["buyerRef"] = value
	}
	return s.client.post(ctx, seasonPath(seasonKey, "/buyer-access-sessions"), body, "")
}

func (s *SeasonsService) ListBuyerAccessSessions(
	ctx context.Context, seasonKey string, limit int,
) (map[string]any, error) {
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	return s.client.get(ctx, seasonPath(seasonKey, "/buyer-access-sessions"), query)
}

func (s *SeasonsService) RevokeBuyerAccessSession(
	ctx context.Context, seasonKey, sessionID string,
) (map[string]any, error) {
	return s.client.delete(ctx, seasonPath(seasonKey, "/buyer-access-sessions/"+escape(sessionID)))
}

func (s *SeasonsService) RetrieveHold(
	ctx context.Context, seasonKey, operationID string,
) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/holds/"+escape(operationID)), nil)
}

func (s *SeasonsService) BookHold(
	ctx context.Context, seasonKey, operationID, bookActionID, bookingRef string,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/holds/"+escape(operationID)+"/book"),
		params("bookActionId", bookActionID, "bookingRef", bookingRef), "")
}

func (s *SeasonsService) RetrieveBooking(
	ctx context.Context, seasonKey, actionID string,
) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/bookings/"+escape(actionID)), nil)
}

func (s *SeasonsService) CancelBooking(
	ctx context.Context, seasonKey, actionID string, p SeasonCancelBookingParams,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/bookings/"+escape(actionID)+"/cancel"),
		params(
			"cancelActionId", p.CancelActionID,
			"bookingRef", p.BookingRef,
			"planActivationId", p.PlanActivationID,
			"rightDisposition", p.RightDisposition,
		), "")
}

func (s *SeasonsService) ValidateBuyerRehearsal(
	ctx context.Context, seasonKey string,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/buyer-rehearsals/validate"), nil, "")
}

func (s *SeasonsService) CreateHolderImport(
	ctx context.Context, seasonKey string, p SeasonHolderImportParams,
) (map[string]any, error) {
	body := params(
		"successorPlanActivationId", p.SuccessorPlanActivationID,
		"dryRun", p.DryRun,
		"rows", p.Rows,
	)
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/imports"), body, p.IdempotencyKey)
}

func (s *SeasonsService) RetrieveHolderImport(
	ctx context.Context, seasonKey, importID string,
) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/imports/"+escape(importID)), nil)
}

func (s *SeasonsService) CreateRenewalOffers(
	ctx context.Context, seasonKey string, p SeasonRenewalOffersParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/renewal-offers"),
		params(
			"successorPlanActivationId", stringOrNil(p.SuccessorPlanActivationID),
			"deadlineAt", p.DeadlineAt,
			"contractIds", seasonSliceOrNil(p.ContractIDs),
		), p.IdempotencyKey)
}

func (s *SeasonsService) ListRenewalOffers(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/renewal-offers"), nil)
}

func (s *SeasonsService) RetrieveRenewalOffer(
	ctx context.Context, seasonKey, offerID string,
) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)), nil)
}

func (s *SeasonsService) ExtendRenewalOffer(
	ctx context.Context, seasonKey, offerID string, deadlineAt int64,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)+"/extend"),
		params("deadlineAt", deadlineAt), "")
}

func (s *SeasonsService) InspectRenewalOffer(
	ctx context.Context, seasonKey, offerID string,
) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)+"/inspect"), nil)
}

func (s *SeasonsService) CommitRenewalOffer(
	ctx context.Context, seasonKey, offerID, commitActionID, orderRef, bookingRef, planActivationID string,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)+"/commit"),
		params(
			"commitActionId", commitActionID,
			"orderRef", orderRef,
			"bookingRef", bookingRef,
			"planActivationId", planActivationID,
		), "")
}

func (s *SeasonsService) DeclineRenewalOffer(
	ctx context.Context, seasonKey, offerID string,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)+"/decline"),
		map[string]any{}, "")
}

func (s *SeasonsService) ReleaseRenewalOffer(
	ctx context.Context, seasonKey, offerID string,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)+"/release"),
		map[string]any{}, "")
}

func (s *SeasonsService) ListOccurrences(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/occurrences"), nil)
}

func (s *SeasonsService) CreateAmendment(
	ctx context.Context, seasonKey string, p SeasonAmendmentParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/amendments"),
		params(
			"eventKey", p.EventKey,
			"kind", p.Kind,
			"startsAt", int64OrNil(p.StartsAt),
			"name", stringOrNil(p.Name),
		), p.IdempotencyKey)
}

func (s *SeasonsService) ListAmendments(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/amendments"), nil)
}

func (s *SeasonsService) RetrieveAmendment(
	ctx context.Context, seasonKey, amendmentID string,
) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/amendments/"+escape(amendmentID)), nil)
}

func (s *SeasonsService) RetrieveReport(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/reports"), nil)
}

func (s *SeasonsService) ListOperations(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/operations"), nil)
}

func (s *SeasonsService) RetrieveSupportLookup(
	ctx context.Context, seasonKey string, p *SeasonSupportLookupParams,
) (map[string]any, error) {
	query := url.Values{}
	if p != nil {
		setIfNotEmpty(query, "bookingRef", p.BookingRef)
		setIfNotEmpty(query, "holderRef", p.HolderRef)
	}
	return s.client.get(ctx, seasonPath(seasonKey, "/support-lookups"), query)
}

func (s *SeasonsService) ListOutbox(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/outbox"), nil)
}

func (s *SeasonsService) ReplayOutbox(
	ctx context.Context, seasonKey, occurrenceID string,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/outbox/"+escape(occurrenceID)+"/replay"),
		map[string]any{}, "")
}

func (s *SeasonsService) ListAudit(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/audit"), nil)
}

func (s *SeasonsService) ExportSupportSnapshot(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/export"), nil)
}
