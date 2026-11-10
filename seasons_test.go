package seatlayer

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// The void job of a cancelled date is named from the amendment, and a holder
// import call is sized by its package's dates; a call over it is a typed 422.
func TestSeasonDateCancelVoidJobIDAndHolderImportRowsPerCall(t *testing.T) {
	if got := SeasonDateCancelVoidJobID("amd_1"); got != "sbj_void_amd_1" {
		t.Fatalf("void job id = %q, want sbj_void_amd_1", got)
	}
	for _, tc := range []struct{ dates, rows int }{{1, 100}, {20, 100}, {21, 95}, {50, 43}, {0, 100}, {5000, 1}} {
		if got := SeasonHolderImportRowsPerCall(tc.dates); got != tc.rows {
			t.Errorf("rows per call at %d dates = %d, want %d", tc.dates, got, tc.rows)
		}
	}

	client, calls := newTestClient(t, []stub{{status: 422, body: `{"error":"invalid_rows","code":"invalid_rows",` +
		`"message":"At most 43 rows a call for this package.","maxRows":43}`}})
	_, err := client.Seasons.CreateHolderImport(context.Background(), "sea_1",
		SeasonHolderImportParams{SuccessorPlanActivationID: "spa_1", Rows: []SeasonHolderImportRow{}})
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("want *ValidationError, got %T", err)
	}
	if validation.Code != "invalid_rows" {
		t.Fatalf("code = %q, want invalid_rows", validation.Code)
	}
	if got := validation.Body["maxRows"]; got != float64(43) {
		t.Fatalf("maxRows = %v, want 43", got)
	}
	if len(*calls) != 1 {
		t.Fatalf("attempts = %d, want 1", len(*calls))
	}
}

func TestSeasonsMapAll90OperationsAndReplayClasses(t *testing.T) {
	const seasonOperations = 90
	responses := make([]stub, seasonOperations)
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

	// The 42 operations added in Season platform slices 2a, 2.11, 2.20 and 2.8 and season seats, from the shared table.
	var added []operationCase
	for _, tc := range newOperationCases() {
		if tc.group == "seasons" {
			added = append(added, tc)
			_, _ = tc.call(ctx, client)
		}
	}

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
	replay := map[int]bool{2: true, 4: true, 5: true, 10: true, 18: true, 27: true, 29: true, 38: true}
	for _, tc := range added {
		want = append(want, tc.want)
		replay[len(want)-1] = tc.replay
	}
	if len(want) != seasonOperations || len(*calls) != seasonOperations {
		t.Fatalf("calls = %d, want list = %d, want %d", len(*calls), len(want), seasonOperations)
	}
	if got := call(t, calls, 26).body; got != "" {
		t.Fatalf("rehearsal body = %q, want empty", got)
	}
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

// The phase 4 answers, shaped as the API sends them (the app repo's contract §2, §6 and §8).
func TestSeasonSeatsAnswersReachTheCaller(t *testing.T) {
	pending := `{"state":"finishing","code":"block_date_leave_pending","retryable":true,` +
		`"eventKey":"ev_1","shardsLeft":[1],"shardCount":2,"message":"Still leaving one shard"}`
	codes := []string{"season_block_not_enabled", "block_exists", "season_block_set_aside",
		"season_inventory_locked", "date_leave_in_progress"}
	responses := []stub{
		{status: 202, body: `{"job":{"jobId":"sbsa_sbp_1","kind":"season_block_set_aside","state":"running"},"block":{"mode":"per_hold"}}`},
		{status: 200, body: `{"job":{"jobId":"sbr_1","kind":"season_block_release","state":"planning"}}`},
		{status: 202, body: pending, headers: map[string]string{"Retry-After": "2"}},
		{status: 202, body: pending},
	}
	for _, code := range codes {
		responses = append(responses, stub{status: 409, body: `{"error":"` + code + `","code":"` + code + `"}`})
	}
	client, calls := newTestClient(t, responses)
	ctx := context.Background()

	set, err := client.Seasons.SetAsideBlock(ctx, "sea_1", SeasonBlockSetAsideParams{ExpectedRevision: 7})
	if err != nil || set["job"].(map[string]any)["kind"] != "season_block_set_aside" {
		t.Fatalf("SetAsideBlock = %v, %v", set, err)
	}
	// No KeepForSingleTickets: nothing kept back, and the key is generated.
	if body := call(t, calls, 0).body; body != `{"expectedRevision":7}` {
		t.Fatalf("set-aside body = %s", body)
	}
	if call(t, calls, 0).header.Get("Idempotency-Key") == "" {
		t.Fatal("set-aside sent no Idempotency-Key")
	}
	// A job of a kind this SDK did not know before reads as any other.
	job, err := client.Seasons.RetrieveBulkJob(ctx, "sea_1", "sbr_1")
	if err != nil || job["job"].(map[string]any)["kind"] != "season_block_release" {
		t.Fatalf("RetrieveBulkJob = %v, %v", job, err)
	}
	cancel, err := client.Seasons.CreateAmendment(ctx, "sea_1", SeasonAmendmentParams{EventKey: "ev_1", Kind: "cancel"})
	if err != nil || !IsSeasonDateLeavePending(cancel) {
		t.Fatalf("CreateAmendment = %v, %v", cancel, err)
	}
	past, err := client.Seasons.MarkPerformancePast(ctx, "sea_1", "ev_1", SeasonMarkPastParams{})
	if err != nil || !IsSeasonDateLeavePending(past) {
		t.Fatalf("MarkPerformancePast = %v, %v", past, err)
	}
	if IsSeasonDateLeavePending(map[string]any{"amendment": map[string]any{"amendmentId": "am_1"}}) ||
		IsSeasonDateLeavePending(map[string]any{"state": "finishing", "code": "season_busy"}) ||
		IsSeasonDateLeavePending(map[string]any{"state": "done", "code": "block_date_leave_pending"}) ||
		IsSeasonDateLeavePending(nil) {
		t.Fatal("IsSeasonDateLeavePending accepted another answer")
	}
	for _, code := range codes {
		_, err := client.Seasons.ReleaseBlock(ctx, "sea_1", SeasonBlockReleaseParams{
			ExpectedRevision: 7, Scope: SeasonBlockReleaseScope{Labels: []string{"A-1"}},
		})
		var conflict *ConflictError
		if !errors.As(err, &conflict) || conflict.Code != code {
			t.Fatalf("ReleaseBlock error = %v, want ConflictError %s", err, code)
		}
	}
}
