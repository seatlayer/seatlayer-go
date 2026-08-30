package seatlayer

import (
	"context"
	"net/http"
	"testing"
)

func TestSeasonsMapAll48OperationsAndReplayClasses(t *testing.T) {
	responses := make([]stub, 48)
	for i := range responses {
		responses[i] = stub{status: 200, body: `{}`}
	}
	client, calls := newTestClient(t, responses)
	ctx := context.Background()
	key := "sea/a"

	_, _ = client.Seasons.List(ctx, &SeasonListParams{WorkspaceID: "ws 1", StructureState: "draft", Limit: 20, Cursor: "c/1"})
	_, _ = client.Seasons.Validate(ctx, SeasonSelectionParams{SourcePerformanceGroupKeys: []string{"pg_1"}})
	_, _ = client.Seasons.Create(ctx, SeasonCreateParams{Name: "Series", EventKeys: []string{"ev_1", "ev_2"}, IdempotencyKey: "create-1"})
	_, _ = client.Seasons.Retrieve(ctx, key)
	_, _ = client.Seasons.Update(ctx, key, SeasonUpdateParams{ExpectedRevision: 1, Name: "Series 2", IdempotencyKey: "update-1"})
	_ = client.Seasons.Delete(ctx, key, "delete-1")
	_, _ = client.Seasons.Activate(ctx, key, 1)
	_, _ = client.Seasons.Close(ctx, key, 2)
	_, _ = client.Seasons.Archive(ctx, key, 3)
	_, _ = client.Seasons.RetrieveLifecycle(ctx, key, "life/1")
	_, _ = client.Seasons.CreatePlan(ctx, key, SeasonPlanCreateParams{Name: "Plan", EventKeys: []string{"ev_1", "ev_2"}, IdempotencyKey: "plan-1"})
	_, _ = client.Seasons.RetrievePlan(ctx, key, "plan/1")
	_, _ = client.Seasons.PublishPlan(ctx, key, "plan/1", 2)
	_, _ = client.Seasons.SupersedePlan(ctx, key, "plan/1", 3)
	_, _ = client.Seasons.OpenSales(ctx, key, 3)
	_, _ = client.Seasons.PauseSales(ctx, key, 4)
	_, _ = client.Seasons.ResumeSales(ctx, key, 5)
	_, _ = client.Seasons.EndSales(ctx, key, 6)
	_, _ = client.Seasons.DuplicateToLive(ctx, key, SeasonDuplicateToLiveParams{EventKeys: []string{"live_1", "live_2"}, IdempotencyKey: "live-1"})
	_, _ = client.Seasons.CreateBuyerAccessSession(ctx, key, SeasonBuyerAccessSessionParams{AllowedOrigin: "https://tickets.example", IncludePublic: true})
	_, _ = client.Seasons.ListBuyerAccessSessions(ctx, key, 10)
	_, _ = client.Seasons.RevokeBuyerAccessSession(ctx, key, "session/1")
	_, _ = client.Seasons.RetrieveHold(ctx, key, "hold/1")
	_, _ = client.Seasons.BookHold(ctx, key, "hold/1", "book_1", "order_1")
	_, _ = client.Seasons.RetrieveBooking(ctx, key, "book/1")
	_, _ = client.Seasons.CancelBooking(ctx, key, "book/1", SeasonCancelBookingParams{CancelActionID: "cancel_1", BookingRef: "order_1", PlanActivationID: "pa_1", RightDisposition: "release"})
	_, _ = client.Seasons.ValidateBuyerRehearsal(ctx, key)
	_, _ = client.Seasons.CreateHolderImport(ctx, key, SeasonHolderImportParams{SuccessorPlanActivationID: "pa_1", Rows: []SeasonHolderImportRow{}, IdempotencyKey: "import-1"})
	_, _ = client.Seasons.RetrieveHolderImport(ctx, key, "import/1")
	_, _ = client.Seasons.CreateRenewalOffers(ctx, key, SeasonRenewalOffersParams{DeadlineAt: 123, IdempotencyKey: "offers-1"})
	_, _ = client.Seasons.ListRenewalOffers(ctx, key)
	_, _ = client.Seasons.RetrieveRenewalOffer(ctx, key, "offer/1")
	_, _ = client.Seasons.ExtendRenewalOffer(ctx, key, "offer/1", 456)
	_, _ = client.Seasons.InspectRenewalOffer(ctx, key, "offer/1")
	_, _ = client.Seasons.CommitRenewalOffer(ctx, key, "offer/1", "commit_1", "order_1", "book_1", "pa_1")
	_, _ = client.Seasons.DeclineRenewalOffer(ctx, key, "offer/1")
	_, _ = client.Seasons.ReleaseRenewalOffer(ctx, key, "offer/1")
	_, _ = client.Seasons.ListOccurrences(ctx, key)
	_, _ = client.Seasons.CreateAmendment(ctx, key, SeasonAmendmentParams{EventKey: "ev_1", Kind: "reschedule", IdempotencyKey: "amend-1"})
	_, _ = client.Seasons.ListAmendments(ctx, key)
	_, _ = client.Seasons.RetrieveAmendment(ctx, key, "amend/1")
	_, _ = client.Seasons.RetrieveReport(ctx, key)
	_, _ = client.Seasons.ListOperations(ctx, key)
	_, _ = client.Seasons.RetrieveSupportLookup(ctx, key, &SeasonSupportLookupParams{HolderRef: "holder a/b"})
	_, _ = client.Seasons.ListOutbox(ctx, key)
	_, _ = client.Seasons.ReplayOutbox(ctx, key, "occurrence/1")
	_, _ = client.Seasons.ListAudit(ctx, key)
	_, _ = client.Seasons.ExportSupportSnapshot(ctx, key)

	want := []string{
		"GET /v1/seasons?cursor=c%2F1&limit=20&structureState=draft&workspaceId=ws+1",
		"POST /v1/seasons/validate", "POST /v1/seasons", "GET /v1/seasons/sea%2Fa",
		"PATCH /v1/seasons/sea%2Fa", "DELETE /v1/seasons/sea%2Fa",
		"POST /v1/seasons/sea%2Fa/activate", "POST /v1/seasons/sea%2Fa/close",
		"POST /v1/seasons/sea%2Fa/archive", "GET /v1/seasons/sea%2Fa/lifecycle/life%2F1",
		"POST /v1/seasons/sea%2Fa/plans", "GET /v1/seasons/sea%2Fa/plans/plan%2F1",
		"POST /v1/seasons/sea%2Fa/plans/plan%2F1/publish", "POST /v1/seasons/sea%2Fa/plans/plan%2F1/supersede",
		"POST /v1/seasons/sea%2Fa/sales/open", "POST /v1/seasons/sea%2Fa/sales/pause",
		"POST /v1/seasons/sea%2Fa/sales/resume", "POST /v1/seasons/sea%2Fa/sales/end",
		"POST /v1/seasons/sea%2Fa/duplicate-to-live", "POST /v1/seasons/sea%2Fa/buyer-access-sessions",
		"GET /v1/seasons/sea%2Fa/buyer-access-sessions?limit=10",
		"DELETE /v1/seasons/sea%2Fa/buyer-access-sessions/session%2F1",
		"GET /v1/seasons/sea%2Fa/holds/hold%2F1", "POST /v1/seasons/sea%2Fa/holds/hold%2F1/book",
		"GET /v1/seasons/sea%2Fa/bookings/book%2F1", "POST /v1/seasons/sea%2Fa/bookings/book%2F1/cancel",
		"POST /v1/seasons/sea%2Fa/buyer-rehearsals/validate", "POST /v1/seasons/sea%2Fa/imports",
		"GET /v1/seasons/sea%2Fa/imports/import%2F1", "POST /v1/seasons/sea%2Fa/renewal-offers",
		"GET /v1/seasons/sea%2Fa/renewal-offers", "GET /v1/seasons/sea%2Fa/renewal-offers/offer%2F1",
		"POST /v1/seasons/sea%2Fa/renewal-offers/offer%2F1/extend",
		"GET /v1/seasons/sea%2Fa/renewal-offers/offer%2F1/inspect",
		"POST /v1/seasons/sea%2Fa/renewal-offers/offer%2F1/commit",
		"POST /v1/seasons/sea%2Fa/renewal-offers/offer%2F1/decline",
		"POST /v1/seasons/sea%2Fa/renewal-offers/offer%2F1/release",
		"GET /v1/seasons/sea%2Fa/occurrences", "POST /v1/seasons/sea%2Fa/amendments",
		"GET /v1/seasons/sea%2Fa/amendments", "GET /v1/seasons/sea%2Fa/amendments/amend%2F1",
		"GET /v1/seasons/sea%2Fa/reports", "GET /v1/seasons/sea%2Fa/operations",
		"GET /v1/seasons/sea%2Fa/support-lookups?holderRef=holder+a%2Fb", "GET /v1/seasons/sea%2Fa/outbox",
		"POST /v1/seasons/sea%2Fa/outbox/occurrence%2F1/replay", "GET /v1/seasons/sea%2Fa/audit",
		"GET /v1/seasons/sea%2Fa/export",
	}
	if len(*calls) != len(want) {
		t.Fatalf("calls = %d, want %d", len(*calls), len(want))
	}
	if got := call(t, calls, 26).body; got != "" {
		t.Fatalf("rehearsal body = %q, want empty", got)
	}
	replay := map[int]bool{2: true, 4: true, 5: true, 10: true, 18: true, 27: true, 29: true, 38: true}
	for i, expected := range want {
		got := call(t, calls, i)
		actual := got.method + " " + got.escapedPath
		if got.query != "" {
			actual += "?" + got.query
		}
		if actual != expected {
			t.Errorf("call %d = %q, want %q", i, actual, expected)
		}
		if replay[i] && got.header.Get("Idempotency-Key") == "" {
			t.Errorf("call %d missing Idempotency-Key", i)
		}
		if !replay[i] && got.header.Get("Idempotency-Key") != "" {
			t.Errorf("call %d unexpectedly has Idempotency-Key", i)
		}
	}
	if call(t, calls, 4).method != http.MethodPatch || call(t, calls, 5).method != http.MethodDelete {
		t.Fatal("update/delete methods drifted")
	}
}
