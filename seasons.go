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
	// CheckoutMode is who takes the money: "seatlayer" or "server". Empty
	// leaves it unchanged; it cannot go back to undecided, and it is fixed once
	// a live booking exists.
	CheckoutMode string
	// TransferCutoffHours is how many hours (0 to 720) before a date starts its
	// tickets stop being sent to others. FieldNull means until it starts.
	TransferCutoffHours NullableField[int]
	// ReturnCutoffHours is how many hours (0 to 720) before a date starts a
	// holder can no longer give a seat back. FieldNull means until it starts.
	ReturnCutoffHours NullableField[int]
	IdempotencyKey    string
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
	// HolderEmail and HolderName are where the holder's renewal emails go.
	HolderEmail *string `json:"holderEmail,omitempty"`
	HolderName  *string `json:"holderName,omitempty"`
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
	EventKey string
	// Kind is "reschedule", "replace", "cancel_exception", "postpone" (the
	// date stays with no new time yet), or "cancel" (it will not happen).
	Kind     string
	StartsAt int64
	Name     string
	// Refund, with Kind "cancel" only, refunds the date's SeatLayer orders as a
	// job: "none" (the default), "per_date", or "full". Empty leaves it out.
	Refund string
	// Reason is kept on the refund job. Empty leaves it out.
	Reason         string
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
	body := params("expectedRevision", p.ExpectedRevision, "name", stringOrNil(p.Name),
		"checkoutMode", stringOrNil(p.CheckoutMode))
	if value, present := p.Edition.requestValue(); present {
		body["edition"] = value
	}
	if value, present := p.TransferCutoffHours.requestValue(); present {
		body["transferCutoffHours"] = value
	}
	if value, present := p.ReturnCutoffHours.requestValue(); present {
		body["returnCutoffHours"] = value
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
			"refund", stringOrNil(p.Refund),
			"reason", stringOrNil(p.Reason),
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

// SeasonHolderImportFromSeasonParams derives renewal rights from a previous
// Season's holders instead of uploaded rows.
type SeasonHolderImportFromSeasonParams struct {
	SourceSeasonID            string
	SuccessorPlanActivationID string
	// DryRun checks the import without committing it. Nil leaves it out.
	DryRun         *bool
	IdempotencyKey string
}

// SeasonPerformancesCreateParams creates performances inside a draft Season
// from a repeat rule.
type SeasonPerformancesCreateParams struct {
	TitlePattern string
	// FirstStartsAt is epoch milliseconds. Zero leaves it out.
	FirstStartsAt int64
	Timezone      string
	// Repeat is one of none, daily, weekly, biweekly or dates.
	Repeat string
	// Count is how many performances to create. Zero leaves it out.
	Count int
	// Dates are epoch milliseconds, used with Repeat "dates".
	Dates           []int64
	Venue           NullableField[string]
	DurationMinutes NullableField[int]
	// Region places live inventory near the venue. Empty leaves it out.
	Region           EventHostingRegion
	AttachToPlanKeys []string
	IdempotencyKey   string
}

// SeasonPerformancesAttachParams attaches existing events to a draft Season.
type SeasonPerformancesAttachParams struct {
	EventKeys        []string
	AttachToPlanKeys []string
	IdempotencyKey   string
}

// SeasonPlanUpdateParams renames or re-scopes a draft package. Empty fields
// are left out.
type SeasonPlanUpdateParams struct {
	Name           string
	EventKeys      []string
	IdempotencyKey string
}

// SeasonCheckoutTestParams scopes a Season checkout test.
type SeasonCheckoutTestParams struct {
	PlanKey string
	// Quantity is the number of seats, 1 to 4. Zero leaves it out.
	Quantity       int
	IdempotencyKey string
}

// The price types a package can price a category for.
const (
	SeasonPriceTypeAdult      = "adult"
	SeasonPriceTypeChild      = "child"
	SeasonPriceTypeConcession = "concession"
	SeasonPriceTypeSenior     = "senior"
	SeasonPriceTypeStudent    = "student"
)

// SeasonPackagePrice is one category price, in minor units, for a package.
type SeasonPackagePrice struct {
	CategoryKey string `json:"categoryKey"`
	// PriceTypeKey is one of the SeasonPriceType constants. Empty leaves it
	// out, which prices the category for adults. Each category takes each type once.
	PriceTypeKey string `json:"priceTypeKey,omitempty"`
	UnitPrice    int64  `json:"unitPrice"`
}

// SeasonPackagePricesParams replaces every category price of a package.
type SeasonPackagePricesParams struct {
	Prices []SeasonPackagePrice
	// Currency is a three-letter code. Empty leaves it out.
	Currency       string
	IdempotencyKey string
}

// SeasonMarkPastParams marks a Season performance as past.
type SeasonMarkPastParams struct {
	Force          bool
	IdempotencyKey string
}

// SeasonInventorySource names where returned seats came from: set exactly one
// of BookingActionID or RenewalOfferID.
type SeasonInventorySource struct {
	BookingActionID string `json:"bookingActionId,omitempty"`
	RenewalOfferID  string `json:"renewalOfferId,omitempty"`
}

// SeasonOccurrenceReturnParams returns one occurrence allocation to Event inventory.
type SeasonOccurrenceReturnParams struct {
	ActionID         string
	BookingRef       string
	PlanActivationID string
	Labels           []string
	Source           SeasonInventorySource
}

// SeasonOccurrenceReclaimParams reclaims returned occurrence inventory.
type SeasonOccurrenceReclaimParams struct {
	ActionID         string
	ReturnActionID   string
	BookingRef       string
	PlanActivationID string
	Labels           []string
	Source           SeasonInventorySource
}

// SeasonHolderListParams filters and pages a Season's holders.
type SeasonHolderListParams struct {
	// Q searches holder name, email, reference, order number or seat.
	Q      string
	Status string
	Cursor string
	Limit  int
}

// SeasonHolderExportParams filters the holders CSV export.
type SeasonHolderExportParams struct {
	// Q searches holder name, email, reference, order number or seat.
	Q      string
	Status string
}

// SeasonOrderDateCancelParams takes back one date of a Season order.
type SeasonOrderDateCancelParams struct {
	ActionID       string
	IdempotencyKey string
}

// SeasonRenewalContactParams sets who a renewal offer is sent to. Use
// FieldNull to clear a value.
type SeasonRenewalContactParams struct {
	Email NullableField[string]
	Name  NullableField[string]
}

// SeasonRenewalNoticesParams limits renewal notices to some offers. Empty
// OfferIDs leaves the field out.
type SeasonRenewalNoticesParams struct {
	OfferIDs []string
}

// SeasonPerformanceRefundParams refunds every SeatLayer order holding a
// cancelled date.
type SeasonPerformanceRefundParams struct {
	// Amount is "per_date" (the date's share of each order, rounded down) or
	// "full" (what is left of each order).
	Amount string
	// Reason is kept on the job. Empty leaves it out.
	Reason         string
	IdempotencyKey string
}

// CreateHolderImportFromSeason dry-runs or commits renewal rights derived from
// a previous Season's holders.
func (s *SeasonsService) CreateHolderImportFromSeason(
	ctx context.Context, seasonKey string, p SeasonHolderImportFromSeasonParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/imports/from-season"),
		params(
			"sourceSeasonId", p.SourceSeasonID,
			"successorPlanActivationId", p.SuccessorPlanActivationID,
			"dryRun", pointerOrNil(p.DryRun),
		), p.IdempotencyKey)
}

// ListPerformances lists the Season's performances and package membership in
// every lifecycle state.
func (s *SeasonsService) ListPerformances(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/performances"), nil)
}

// CreatePerformances creates performances inside a draft Season from a repeat rule.
func (s *SeasonsService) CreatePerformances(
	ctx context.Context, seasonKey string, p SeasonPerformancesCreateParams,
) (map[string]any, error) {
	body := params(
		"titlePattern", p.TitlePattern,
		"firstStartsAt", int64OrNil(p.FirstStartsAt),
		"timezone", p.Timezone,
		"repeat", p.Repeat,
		"count", intOrNil(p.Count),
		"dates", seasonSliceOrNil(p.Dates),
		"region", stringOrNil(string(p.Region)),
		"attachToPlanKeys", seasonSliceOrNil(p.AttachToPlanKeys),
	)
	nullableFields(body,
		fieldRequestValue("venue", p.Venue),
		fieldRequestValue("durationMinutes", p.DurationMinutes),
	)
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/performances"), body, p.IdempotencyKey)
}

// AttachPerformances attaches existing events to a draft Season and its packages.
func (s *SeasonsService) AttachPerformances(
	ctx context.Context, seasonKey string, p SeasonPerformancesAttachParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/performances/attach"),
		params(
			"eventKeys", p.EventKeys,
			"attachToPlanKeys", seasonSliceOrNil(p.AttachToPlanKeys),
		), p.IdempotencyKey)
}

// DeletePerformance removes a performance from a draft Season and every package.
func (s *SeasonsService) DeletePerformance(
	ctx context.Context, seasonKey, eventKey, idempotencyKey string,
) (map[string]any, error) {
	return s.client.mutationHeaderReplay(ctx, http.MethodDelete,
		seasonPath(seasonKey, "/performances/"+escape(eventKey)), nil, idempotencyKey)
}

// UpdatePlan renames a draft package or changes the performances it covers.
func (s *SeasonsService) UpdatePlan(
	ctx context.Context, seasonKey, planKey string, p SeasonPlanUpdateParams,
) (map[string]any, error) {
	return s.client.mutationHeaderReplay(ctx, http.MethodPatch,
		seasonPath(seasonKey, "/plans/"+escape(planKey)),
		params("name", stringOrNil(p.Name), "eventKeys", seasonSliceOrNil(p.EventKeys)),
		p.IdempotencyKey)
}

// DeletePlan deletes a draft package that is not the last one. The API answers
// 204 with no body, so only an error comes back.
func (s *SeasonsService) DeletePlan(
	ctx context.Context, seasonKey, planKey, idempotencyKey string,
) error {
	_, err := s.client.mutationHeaderReplay(ctx, http.MethodDelete,
		seasonPath(seasonKey, "/plans/"+escape(planKey)), nil, idempotencyKey)
	return err
}

// Publish activates the Season structure and publishes every draft package in
// one retryable call.
func (s *SeasonsService) Publish(
	ctx context.Context, seasonKey, idempotencyKey string,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/publish"), nil, idempotencyKey)
}

// CreateCheckoutTest runs a server-side hold and release to prove the Season
// buyer path. Pass nil to send no body.
func (s *SeasonsService) CreateCheckoutTest(
	ctx context.Context, seasonKey string, p *SeasonCheckoutTestParams,
) (map[string]any, error) {
	var body any
	idempotencyKey := ""
	if p != nil {
		body = params("planKey", stringOrNil(p.PlanKey), "quantity", intOrNil(p.Quantity))
		idempotencyKey = p.IdempotencyKey
	}
	return s.client.postHeaderReplay(ctx, seasonPath(seasonKey, "/checkout-tests"), body, idempotencyKey)
}

// ListCheckoutTests lists recent Season checkout tests, newest first.
func (s *SeasonsService) ListCheckoutTests(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/checkout-tests"), nil)
}

// ReplacePackagePrices replaces a package price table, per chart category, in
// minor units.
func (s *SeasonsService) ReplacePackagePrices(
	ctx context.Context, seasonKey, planKey string, p SeasonPackagePricesParams,
) (map[string]any, error) {
	prices := p.Prices
	if prices == nil {
		// prices is required on the wire; an empty list clears them.
		prices = []SeasonPackagePrice{}
	}
	return s.client.mutationHeaderReplay(ctx, http.MethodPut,
		seasonPath(seasonKey, "/plans/"+escape(planKey)+"/prices"),
		params("prices", prices, "currency", stringOrNil(p.Currency)), p.IdempotencyKey)
}

// ListPrices lists every package price table for the Season.
func (s *SeasonsService) ListPrices(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/prices"), nil)
}

// AttachPlanPerformance adds a performance to a published package and extends
// its allocations.
func (s *SeasonsService) AttachPlanPerformance(
	ctx context.Context, seasonKey, planKey, eventKey, idempotencyKey string,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx,
		seasonPath(seasonKey, "/plans/"+escape(planKey)+"/performances"),
		params("eventKey", eventKey), idempotencyKey)
}

// DeletePlanPerformance removes a performance from a published package and
// releases its allocations.
func (s *SeasonsService) DeletePlanPerformance(
	ctx context.Context, seasonKey, planKey, eventKey, idempotencyKey string,
) (map[string]any, error) {
	return s.client.mutationHeaderReplay(ctx, http.MethodDelete,
		seasonPath(seasonKey, "/plans/"+escape(planKey)+"/performances/"+escape(eventKey)),
		nil, idempotencyKey)
}

// MarkPerformancePast marks a performance past so it stops blocking new Season sales.
func (s *SeasonsService) MarkPerformancePast(
	ctx context.Context, seasonKey, eventKey string, p SeasonMarkPastParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx,
		seasonPath(seasonKey, "/performances/"+escape(eventKey)+"/past"),
		params("force", boolOrNil(p.Force)), p.IdempotencyKey)
}

// ReturnOccurrenceInventory returns one Season occurrence allocation to
// ordinary Event inventory. It is single-attempt; ActionID makes a repeat safe.
func (s *SeasonsService) ReturnOccurrenceInventory(
	ctx context.Context, seasonKey, eventKey string, p SeasonOccurrenceReturnParams,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/occurrences/"+escape(eventKey)+"/returns"),
		params(
			"actionId", p.ActionID,
			"bookingRef", p.BookingRef,
			"planActivationId", p.PlanActivationID,
			"labels", p.Labels,
			"source", p.Source,
		), "")
}

// ReclaimOccurrenceInventory reclaims returned occurrence inventory while it is
// still free. It is single-attempt; ActionID makes a repeat safe.
func (s *SeasonsService) ReclaimOccurrenceInventory(
	ctx context.Context, seasonKey, eventKey string, p SeasonOccurrenceReclaimParams,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/occurrences/"+escape(eventKey)+"/reclaims"),
		params(
			"actionId", p.ActionID,
			"returnActionId", p.ReturnActionID,
			"bookingRef", p.BookingRef,
			"planActivationId", p.PlanActivationID,
			"labels", p.Labels,
			"source", p.Source,
		), "")
}

// ListHolders lists Season ticket holders, one cursor page at a time.
func (s *SeasonsService) ListHolders(
	ctx context.Context, seasonKey string, p *SeasonHolderListParams,
) (map[string]any, error) {
	query := url.Values{}
	if p != nil {
		setIfNotEmpty(query, "q", p.Q)
		setIfNotEmpty(query, "status", p.Status)
		setIfNotEmpty(query, "cursor", p.Cursor)
		if p.Limit > 0 {
			query.Set("limit", strconv.Itoa(p.Limit))
		}
	}
	return s.client.get(ctx, seasonPath(seasonKey, "/holders"), query)
}

// RefundOrder refunds a Season order in full or in part through its payment
// provider. Pass nil to refund what is left.
func (s *SeasonsService) RefundOrder(
	ctx context.Context, seasonKey, orderID string, p *OrderRefundParams,
) (map[string]any, error) {
	body, idempotencyKey := p.request()
	return s.client.postHeaderReplay(ctx,
		seasonPath(seasonKey, "/orders/"+escape(orderID)+"/refund"), body, idempotencyKey)
}

// CancelOrder cancels a Season order without moving money and releases every date.
func (s *SeasonsService) CancelOrder(
	ctx context.Context, seasonKey, orderID string, p OrderCancelParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx,
		seasonPath(seasonKey, "/orders/"+escape(orderID)+"/cancel"), p.body(), p.IdempotencyKey)
}

// CancelOrderDate takes back one date of a Season order and puts its seats back
// on sale. Pass nil to send no body.
func (s *SeasonsService) CancelOrderDate(
	ctx context.Context, seasonKey, orderID, eventKey string, p *SeasonOrderDateCancelParams,
) (map[string]any, error) {
	var body any
	idempotencyKey := ""
	if p != nil {
		body = params("actionId", stringOrNil(p.ActionID))
		idempotencyKey = p.IdempotencyKey
	}
	return s.client.postHeaderReplay(ctx,
		seasonPath(seasonKey, "/orders/"+escape(orderID)+"/dates/"+escape(eventKey)+"/cancel"),
		body, idempotencyKey)
}

// ResendOrderTickets emails a Season order's tickets again. It has no replay
// contract, so it is single-attempt. Pass nil to send no body.
func (s *SeasonsService) ResendOrderTickets(
	ctx context.Context, seasonKey, orderID string, p *OrderResendParams,
) (map[string]any, error) {
	return s.client.post(ctx,
		seasonPath(seasonKey, "/orders/"+escape(orderID)+"/resend"), p.body(), "")
}

// RetrieveHostedRenewals reads renewal links, payments and emails per offer.
func (s *SeasonsService) RetrieveHostedRenewals(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/renewal-hosted"), nil)
}

// RotateRenewalOfferLink makes a new renewal link for one holder; the old link
// stops working. The link is shown once, so this is deliberately single-attempt.
func (s *SeasonsService) RotateRenewalOfferLink(
	ctx context.Context, seasonKey, offerID string,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)+"/link"), nil, "")
}

// UpdateRenewalOfferContact sets the name and email a renewal offer is sent to.
// It is single-attempt.
func (s *SeasonsService) UpdateRenewalOfferContact(
	ctx context.Context, seasonKey, offerID string, p SeasonRenewalContactParams,
) (map[string]any, error) {
	body := nullableFields(map[string]any{},
		fieldRequestValue("email", p.Email),
		fieldRequestValue("name", p.Name),
	)
	return s.client.post(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)+"/contact"), body, "")
}

// CreateRenewalNotices emails renewal invitations now and reminders before the
// deadline. It is single-attempt. Pass nil to send no body.
func (s *SeasonsService) CreateRenewalNotices(
	ctx context.Context, seasonKey string, p *SeasonRenewalNoticesParams,
) (map[string]any, error) {
	var body any
	if p != nil {
		body = params("offerIds", seasonSliceOrNil(p.OfferIDs))
	}
	return s.client.post(ctx, seasonPath(seasonKey, "/renewal-notices"), body, "")
}

// ResendRenewalOffer sends one holder a new renewal invitation. It has no
// replay contract, so it is single-attempt.
func (s *SeasonsService) ResendRenewalOffer(
	ctx context.Context, seasonKey, offerID string,
) (map[string]any, error) {
	return s.client.post(ctx, seasonPath(seasonKey, "/renewal-offers/"+escape(offerID)+"/resend"), nil, "")
}

// RetrieveSalesReport reads Season money, seats, holders and renewals as totals.
func (s *SeasonsService) RetrieveSalesReport(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/reports/sales"), nil)
}

// RetrieveSeatsReport reads the seats still free per package and per date.
func (s *SeasonsService) RetrieveSeatsReport(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/reports/seats"), nil)
}

// ExportHoldersCSV downloads every Season ticket holder as CSV text with a
// header row, returned unchanged.
func (s *SeasonsService) ExportHoldersCSV(
	ctx context.Context, seasonKey string, p *SeasonHolderExportParams,
) (string, error) {
	query := url.Values{}
	if p != nil {
		setIfNotEmpty(query, "q", p.Q)
		setIfNotEmpty(query, "status", p.Status)
	}
	return s.client.getText(ctx, seasonPath(seasonKey, "/reports/holders.csv"), query, "text/csv")
}

// ExportOrdersCSV downloads every Season order SeatLayer took as CSV text with
// a header row, returned unchanged.
func (s *SeasonsService) ExportOrdersCSV(ctx context.Context, seasonKey string) (string, error) {
	return s.client.getText(ctx, seasonPath(seasonKey, "/reports/orders.csv"), nil, "text/csv")
}

// RetrieveSeatCounts reads Season seats sold per package and per date.
func (s *SeasonsService) RetrieveSeatCounts(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/seat-counts"), nil)
}

// RefundPerformance refunds every SeatLayer order holding a cancelled date, as
// a job; the answer is {"job": ...}. Follow it with RetrieveBulkJob. It is
// retried safely with one Idempotency-Key, which replays the same job.
func (s *SeasonsService) RefundPerformance(
	ctx context.Context, seasonKey, eventKey string, p SeasonPerformanceRefundParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx,
		seasonPath(seasonKey, "/performances/"+escape(eventKey)+"/refunds"),
		params("amount", p.Amount, "reason", stringOrNil(p.Reason)), p.IdempotencyKey)
}

// ListBulkJobs lists the Season's bulk jobs, newest first (100 at most).
func (s *SeasonsService) ListBulkJobs(ctx context.Context, seasonKey string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/jobs"), nil)
}

// RetrieveBulkJob reads one bulk job with its counts and up to 500 failed items.
func (s *SeasonsService) RetrieveBulkJob(ctx context.Context, seasonKey, jobID string) (map[string]any, error) {
	return s.client.get(ctx, seasonPath(seasonKey, "/jobs/"+escape(jobID)), nil)
}
