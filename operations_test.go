package seatlayer

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// operationCase is one public operation added for Season platform slice 2a:
// the call, the exact request it must send, and its replay class.
type operationCase struct {
	group string
	name  string
	call  func(ctx context.Context, c *Client) (any, error)
	// want is "METHOD /escaped/path" plus "?query" when there is one.
	want string
	// body is the expected JSON body; empty means no body at all.
	body string
	// replay marks header-replay operations, which must carry an Idempotency-Key.
	replay bool
	// key, when set, is the caller key the request must carry unchanged.
	key      string
	status   int
	response string
}

func (tc operationCase) stub() stub {
	status, body := tc.status, tc.response
	if status == 0 {
		status = http.StatusOK
	}
	if body == "" && status != http.StatusNoContent {
		body = `{}`
	}
	return stub{status: status, body: body}
}

func newOperationCases() []operationCase {
	yes, no := true, false
	const season = "sea/a"
	const seasonBase = "/v1/seasons/sea%2Fa"
	const holderBase = "/v1/holders/hol%2F1"
	const eventBase = "/v1/events/ev%2F1"

	return []operationCase{
		// ---------- seasons (39) ----------
		{group: "seasons", name: "CreateHolderImportFromSeason", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.CreateHolderImportFromSeason(ctx, season, SeasonHolderImportFromSeasonParams{
				SourceSeasonID: "sea_2026", SuccessorPlanActivationID: "pa_1", DryRun: &yes, IdempotencyKey: "import-season-1",
			})
		}, want: "POST " + seasonBase + "/imports/from-season",
			body:   `{"sourceSeasonId":"sea_2026","successorPlanActivationId":"pa_1","dryRun":true}`,
			replay: true, key: "import-season-1", status: 201},
		{group: "seasons", name: "ListPerformances", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ListPerformances(ctx, season)
		}, want: "GET " + seasonBase + "/performances"},
		{group: "seasons", name: "CreatePerformances", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.CreatePerformances(ctx, season, SeasonPerformancesCreateParams{
				TitlePattern: "Match {n}", FirstStartsAt: 1700000000000, Timezone: "Europe/London",
				Repeat: "weekly", Count: 3, Venue: FieldValue("Main hall"), DurationMinutes: FieldNull[int](),
				Region: RegionWesternEurope, AttachToPlanKeys: []string{"plan_1"},
			})
		}, want: "POST " + seasonBase + "/performances",
			body: `{"titlePattern":"Match {n}","firstStartsAt":1700000000000,"timezone":"Europe/London",` +
				`"repeat":"weekly","count":3,"venue":"Main hall","durationMinutes":null,` +
				`"region":"western-europe","attachToPlanKeys":["plan_1"]}`,
			replay: true, status: 201},
		{group: "seasons", name: "AttachPerformances", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.AttachPerformances(ctx, season, SeasonPerformancesAttachParams{
				EventKeys: []string{"ev_1", "ev_2"}, AttachToPlanKeys: []string{"plan_1"},
			})
		}, want: "POST " + seasonBase + "/performances/attach",
			body: `{"eventKeys":["ev_1","ev_2"],"attachToPlanKeys":["plan_1"]}`, replay: true},
		{group: "seasons", name: "DeletePerformance", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.DeletePerformance(ctx, season, "ev/1", "delete-performance-1")
		}, want: "DELETE " + seasonBase + "/performances/ev%2F1", replay: true, key: "delete-performance-1"},
		{group: "seasons", name: "UpdatePlan", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.UpdatePlan(ctx, season, "plan/1", SeasonPlanUpdateParams{
				Name: "Full season", EventKeys: []string{"ev_1", "ev_2"},
			})
		}, want: "PATCH " + seasonBase + "/plans/plan%2F1",
			body: `{"name":"Full season","eventKeys":["ev_1","ev_2"]}`, replay: true},
		{group: "seasons", name: "DeletePlan", call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Seasons.DeletePlan(ctx, season, "plan/1", "")
		}, want: "DELETE " + seasonBase + "/plans/plan%2F1", replay: true, status: 204},
		{group: "seasons", name: "Publish", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.Publish(ctx, season, "publish-1")
		}, want: "POST " + seasonBase + "/publish", replay: true, key: "publish-1"},
		{group: "seasons", name: "CreateCheckoutTest", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.CreateCheckoutTest(ctx, season, &SeasonCheckoutTestParams{PlanKey: "plan_1", Quantity: 2})
		}, want: "POST " + seasonBase + "/checkout-tests", body: `{"planKey":"plan_1","quantity":2}`, replay: true},
		{group: "seasons", name: "ListCheckoutTests", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ListCheckoutTests(ctx, season)
		}, want: "GET " + seasonBase + "/checkout-tests"},
		{group: "seasons", name: "ReplacePackagePrices", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ReplacePackagePrices(ctx, season, "plan/1", SeasonPackagePricesParams{
				Prices: []SeasonPackagePrice{{CategoryKey: "cat_a", UnitPrice: 12000}}, Currency: "EUR",
			})
		}, want: "PUT " + seasonBase + "/plans/plan%2F1/prices",
			body: `{"prices":[{"categoryKey":"cat_a","unitPrice":12000}],"currency":"EUR"}`, replay: true},
		{group: "seasons", name: "ListPrices", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ListPrices(ctx, season)
		}, want: "GET " + seasonBase + "/prices"},
		{group: "seasons", name: "AttachPlanPerformance", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.AttachPlanPerformance(ctx, season, "plan/1", "ev_3", "")
		}, want: "POST " + seasonBase + "/plans/plan%2F1/performances", body: `{"eventKey":"ev_3"}`, replay: true},
		{group: "seasons", name: "DeletePlanPerformance", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.DeletePlanPerformance(ctx, season, "plan/1", "ev/3", "")
		}, want: "DELETE " + seasonBase + "/plans/plan%2F1/performances/ev%2F3", replay: true},
		{group: "seasons", name: "MarkPerformancePast", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.MarkPerformancePast(ctx, season, "ev/1", SeasonMarkPastParams{Force: true})
		}, want: "POST " + seasonBase + "/performances/ev%2F1/past", body: `{"force":true}`, replay: true},
		{group: "seasons", name: "ReturnOccurrenceInventory", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ReturnOccurrenceInventory(ctx, season, "ev/1", SeasonOccurrenceReturnParams{
				ActionID: "ret_1", BookingRef: "order_1", PlanActivationID: "pa_1", Labels: []string{"A-1"},
				Source: SeasonInventorySource{BookingActionID: "book_1"},
			})
		}, want: "POST " + seasonBase + "/occurrences/ev%2F1/returns",
			body: `{"actionId":"ret_1","bookingRef":"order_1","planActivationId":"pa_1","labels":["A-1"],` +
				`"source":{"bookingActionId":"book_1"}}`},
		{group: "seasons", name: "ReclaimOccurrenceInventory", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ReclaimOccurrenceInventory(ctx, season, "ev/1", SeasonOccurrenceReclaimParams{
				ActionID: "rec_1", ReturnActionID: "ret_1", BookingRef: "order_1", PlanActivationID: "pa_1",
				Labels: []string{"A-1"}, Source: SeasonInventorySource{RenewalOfferID: "offer_1"},
			})
		}, want: "POST " + seasonBase + "/occurrences/ev%2F1/reclaims",
			body: `{"actionId":"rec_1","returnActionId":"ret_1","bookingRef":"order_1","planActivationId":"pa_1",` +
				`"labels":["A-1"],"source":{"renewalOfferId":"offer_1"}}`},
		{group: "seasons", name: "ListHolders", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ListHolders(ctx, season, &SeasonHolderListParams{
				Q: "ann lee", Status: "active", Cursor: "c/1", Limit: 20,
			})
		}, want: "GET " + seasonBase + "/holders?cursor=c%2F1&limit=20&q=ann+lee&status=active"},
		{group: "seasons", name: "RefundOrder", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RefundOrder(ctx, season, "ord/1", &OrderRefundParams{AmountMinor: 500, IdempotencyKey: "refund-1"})
		}, want: "POST " + seasonBase + "/orders/ord%2F1/refund", body: `{"amountMinor":500}`,
			replay: true, key: "refund-1"},
		{group: "seasons", name: "CancelOrder", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.CancelOrder(ctx, season, "ord/1", OrderCancelParams{
				Reason: "other", Note: "duplicate", RefundConfirmed: true, NotifyBuyer: &no,
			})
		}, want: "POST " + seasonBase + "/orders/ord%2F1/cancel",
			body: `{"reason":"other","note":"duplicate","refundConfirmed":true,"notifyBuyer":false}`, replay: true},
		{group: "seasons", name: "CancelOrderDate", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.CancelOrderDate(ctx, season, "ord/1", "ev/2", &SeasonOrderDateCancelParams{ActionID: "date_1"})
		}, want: "POST " + seasonBase + "/orders/ord%2F1/dates/ev%2F2/cancel", body: `{"actionId":"date_1"}`, replay: true},
		{group: "seasons", name: "ResendOrderTickets", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ResendOrderTickets(ctx, season, "ord/1", &OrderResendParams{Email: "a@example.com"})
		}, want: "POST " + seasonBase + "/orders/ord%2F1/resend", body: `{"email":"a@example.com"}`},
		{group: "seasons", name: "RetrieveHostedRenewals", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RetrieveHostedRenewals(ctx, season)
		}, want: "GET " + seasonBase + "/renewal-hosted"},
		{group: "seasons", name: "RotateRenewalOfferLink", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RotateRenewalOfferLink(ctx, season, "offer/1")
		}, want: "POST " + seasonBase + "/renewal-offers/offer%2F1/link", status: 201},
		{group: "seasons", name: "UpdateRenewalOfferContact", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.UpdateRenewalOfferContact(ctx, season, "offer/1", SeasonRenewalContactParams{
				Email: FieldValue("b@example.com"), Name: FieldNull[string](),
			})
		}, want: "POST " + seasonBase + "/renewal-offers/offer%2F1/contact", body: `{"email":"b@example.com","name":null}`},
		{group: "seasons", name: "CreateRenewalNotices", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.CreateRenewalNotices(ctx, season, &SeasonRenewalNoticesParams{OfferIDs: []string{"offer_1"}})
		}, want: "POST " + seasonBase + "/renewal-notices", body: `{"offerIds":["offer_1"]}`},
		{group: "seasons", name: "ResendRenewalOffer", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ResendRenewalOffer(ctx, season, "offer/1")
		}, want: "POST " + seasonBase + "/renewal-offers/offer%2F1/resend", status: 202},
		{group: "seasons", name: "RetrieveSalesReport", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RetrieveSalesReport(ctx, season)
		}, want: "GET " + seasonBase + "/reports/sales"},
		{group: "seasons", name: "RetrieveSeatsReport", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RetrieveSeatsReport(ctx, season)
		}, want: "GET " + seasonBase + "/reports/seats"},
		{group: "seasons", name: "ExportHoldersCSV", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ExportHoldersCSV(ctx, season, &SeasonHolderExportParams{Q: "ann", Status: "active"})
		}, want: "GET " + seasonBase + "/reports/holders.csv?q=ann&status=active", response: "name\nAnn\n"},
		{group: "seasons", name: "ExportOrdersCSV", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ExportOrdersCSV(ctx, season)
		}, want: "GET " + seasonBase + "/reports/orders.csv", response: "order\nord_1\n"},
		{group: "seasons", name: "RetrieveSeatCounts", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RetrieveSeatCounts(ctx, season)
		}, want: "GET " + seasonBase + "/seat-counts"},
		{group: "seasons", name: "RefundPerformance", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RefundPerformance(ctx, season, "ev/1", SeasonPerformanceRefundParams{
				Amount: "per_date", Reason: "Storm", IdempotencyKey: "refund-date-1",
			})
		}, want: "POST " + seasonBase + "/performances/ev%2F1/refunds",
			body:   `{"amount":"per_date","reason":"Storm"}`,
			replay: true, key: "refund-date-1", status: 202,
			response: `{"job":{"jobId":"sbj_1","kind":"bulk_refund","state":"planning"}}`},
		{group: "seasons", name: "ReleaseLapsedOffers", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ReleaseLapsedOffers(ctx, season, "release-lapsed-1")
		}, want: "POST " + seasonBase + "/renewal-offers/release-lapsed",
			replay: true, key: "release-lapsed-1", status: 202,
			response: `{"job":{"jobId":"sbj_2","kind":"lapsed_release","state":"planning"}}`},
		{group: "seasons", name: "ListBulkJobs", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ListBulkJobs(ctx, season)
		}, want: "GET " + seasonBase + "/jobs", response: `{"jobs":[]}`},
		{group: "seasons", name: "RetrieveBulkJob", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RetrieveBulkJob(ctx, season, "job/1")
		}, want: "GET " + seasonBase + "/jobs/job%2F1"},
		// Season seats (phase 4).
		{group: "seasons", name: "SetAsideBlock", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.SetAsideBlock(ctx, season, SeasonBlockSetAsideParams{
				ExpectedRevision: 7, KeepForSingleTickets: &SeasonSeatSelection{Labels: []string{"A-6"}},
				IdempotencyKey: "block-1",
			})
		}, want: "POST " + seasonBase + "/block",
			body:   `{"expectedRevision":7,"keepForSingleTickets":{"labels":["A-6"]}}`,
			replay: true, key: "block-1", status: 202,
			response: `{"job":{"jobId":"sbsa_sbp_1","kind":"season_block_set_aside","state":"running"},"block":{"mode":"per_hold"}}`},
		{group: "seasons", name: "RetrieveBlock", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RetrieveBlock(ctx, season)
		}, want: "GET " + seasonBase + "/block", response: `{"block":{"mode":"block"}}`},
		{group: "seasons", name: "ReleaseBlock", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.ReleaseBlock(ctx, season, SeasonBlockReleaseParams{
				ExpectedRevision: 7, Scope: SeasonBlockReleaseScope{All: true}, EventKeys: []string{"ev/1"},
				At: "2026-12-01T10:00:00Z", Reason: "Unsold",
			})
		}, want: "POST " + seasonBase + "/block/release",
			body:   `{"expectedRevision":7,"scope":{"all":true},"eventKeys":["ev/1"],"at":"2026-12-01T10:00:00Z","reason":"Unsold"}`,
			replay: true, status: 202,
			response: `{"job":{"jobId":"sbr_1","kind":"season_block_release","state":"planning"},"block":{"mode":"block"}}`},
		{group: "seasons", name: "RetrieveRenewalQuote", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.RetrieveRenewalQuote(ctx, season, "offer/1")
		}, want: "GET " + seasonBase + "/renewal-offers/offer%2F1/quote",
			response: `{"offerId":"offer/1","state":"offered","totalMinor":13500,"currency":"GBP","lines":[],"performances":[],"problem":null}`},
		{group: "seasons", name: "CreateRenewalIntent", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.CreateRenewalIntent(ctx, season, "offer/1")
		}, want: "POST " + seasonBase + "/renewal-offers/offer%2F1/intent",
			response: `{"offer":{"offerId":"offer/1","state":"intent_received"}}`},
		{group: "seasons", name: "CreateRenewalCheckout", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Seasons.CreateRenewalCheckout(ctx, season, "offer/1", SeasonRenewalCheckoutParams{
				ReturnURL: "https://tickets.example/renewed", BuyerEmail: "ana@example.com",
				BuyerName: "Ana Silva", Provider: "stripe", IdempotencyKey: "renew-1",
			})
		}, want: "POST " + seasonBase + "/renewal-offers/offer%2F1/checkout",
			body: `{"returnUrl":"https://tickets.example/renewed","buyerEmail":"ana@example.com",` +
				`"buyerName":"Ana Silva","provider":"stripe"}`,
			replay: true, key: "renew-1", status: 201,
			response: `{"orderId":"ord_1","totalMinor":13500,"currency":"GBP","expiresAt":1,"payment":{"url":"https://pay.example/1"}}`},

		// ---------- holders (14) ----------
		{group: "holders", name: "ListCustomers", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.ListCustomers(ctx, &CustomerListParams{
				Email: "a@example.com", HolderRef: "hol/1", Cursor: "c/1", Limit: 10,
			})
		}, want: "GET /v1/customers?cursor=c%2F1&email=a%40example.com&holderRef=hol%2F1&limit=10"},
		{group: "holders", name: "Retrieve", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.Retrieve(ctx, "hol/1")
		}, want: "GET " + holderBase},
		{group: "holders", name: "Update", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.Update(ctx, "hol/1", HolderUpdateParams{
				Email: "c@example.com", Name: FieldValue("Ann"), Phone: FieldNull[string](), IdempotencyKey: "holder-1",
			})
		}, want: "PATCH " + holderBase, body: `{"email":"c@example.com","name":"Ann","phone":null}`,
			replay: true, key: "holder-1"},
		{group: "holders", name: "RevokeSessions", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.RevokeSessions(ctx, "hol/1", &HolderSessionRevokeParams{SessionID: "csn_0123456789abcdef01234567"})
		}, want: "POST " + holderBase + "/sessions/revoke", body: `{"sessionId":"csn_0123456789abcdef01234567"}`},
		{group: "holders", name: "ListSeasons", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.ListSeasons(ctx, "hol/1")
		}, want: "GET " + holderBase + "/seasons"},
		{group: "holders", name: "ListTickets", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.ListTickets(ctx, "hol/1", &HolderTicketListParams{
				SeasonKey: "sea/a", From: 1, To: 2, Cursor: "c", Limit: 5,
			})
		}, want: "GET " + holderBase + "/tickets?cursor=c&from=1&limit=5&seasonKey=sea%2Fa&to=2"},
		{group: "holders", name: "RetrieveTicket", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.RetrieveTicket(ctx, "hol/1", "tkt/1")
		}, want: "GET " + holderBase + "/tickets/tkt%2F1"},
		{group: "holders", name: "CreateTransfer", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.CreateTransfer(ctx, "hol/1", HolderTransferCreateParams{
				TicketIDs: []string{"tkt_0123456789abcdef01234567"}, ToEmail: "d@example.com",
			})
		}, want: "POST " + holderBase + "/transfers",
			body: `{"ticketIds":["tkt_0123456789abcdef01234567"],"toEmail":"d@example.com"}`, replay: true, status: 201},
		{group: "holders", name: "ListTransfers", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.ListTransfers(ctx, "hol/1")
		}, want: "GET " + holderBase + "/transfers"},
		{group: "holders", name: "CancelTransfer", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.CancelTransfer(ctx, "hol/1", "tr/1")
		}, want: "POST " + holderBase + "/transfers/tr%2F1/cancel"},
		{group: "holders", name: "AcceptTransfer", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.AcceptTransfer(ctx, "hol/1", "tr/1")
		}, want: "POST " + holderBase + "/transfers/tr%2F1/accept"},
		{group: "holders", name: "CreateReturn", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.CreateReturn(ctx, "hol/1", HolderReturnCreateParams{
				TicketIDs: []string{"tkt_0123456789abcdef01234567"},
			})
		}, want: "POST " + holderBase + "/returns",
			body: `{"ticketIds":["tkt_0123456789abcdef01234567"]}`, replay: true, status: 201,
			response: `{"return":{"returnId":"ret_1","state":"pending","seasonKey":"sea_1","tickets":[],"createdAt":1}}`},
		{group: "holders", name: "CreateRenewalCheckout", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.CreateRenewalCheckout(ctx, "hol/1", "offer/1", SeasonRenewalCheckoutParams{
				ReturnURL: "https://tickets.example/renewed", BuyerEmail: "cara@example.com",
				BuyerName: "Cara", Provider: "stripe",
			})
		}, want: "POST " + holderBase + "/renewal-offers/offer%2F1/checkout",
			body: `{"returnUrl":"https://tickets.example/renewed","buyerEmail":"cara@example.com",` +
				`"buyerName":"Cara","provider":"stripe"}`,
			replay: true, status: 201, response: `{"orderId":"ord_1","payment":{"url":"https://pay.example/1"}}`},
		{group: "holders", name: "DeclineRenewalOffer", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Holders.DeclineRenewalOffer(ctx, "hol/1", "offer/1", &SeasonRenewalDeclineParams{Reason: "Moving away"})
		}, want: "POST " + holderBase + "/renewal-offers/offer%2F1/decline", body: `{"reason":"Moving away"}`,
			response: `{"offer":{"offerId":"offer/1","state":"declined","declineReason":"Moving away"}}`},

		// ---------- orders (9) ----------
		{group: "orders", name: "List", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.List(ctx, &OrderListParams{
				EventKey: "ev/1", SeasonKey: "sea/a", HolderRef: "ord/1", Status: "paid", From: 10, To: 20, Cursor: "c", Limit: 25,
			})
		}, want: "GET /v1/orders?cursor=c&eventKey=ev%2F1&from=10&holderRef=ord%2F1&limit=25&seasonKey=sea%2Fa&status=paid&to=20"},
		{group: "orders", name: "Retrieve", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.Retrieve(ctx, "ord/1")
		}, want: "GET /v1/orders/ord%2F1"},
		{group: "orders", name: "ListTickets", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.ListTickets(ctx, "ord/1")
		}, want: "GET /v1/orders/ord%2F1/tickets"},
		{group: "orders", name: "ListEventOrders", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.ListEventOrders(ctx, "ev/1", &EventOrderListParams{
				Q: "ann lee", Status: "paid", PaidBy: "card", From: 10, To: 20, Cursor: "c", Limit: 25,
			})
		}, want: "GET " + eventBase + "/orders?cursor=c&from=10&limit=25&paidBy=card&q=ann+lee&status=paid&to=20"},
		{group: "orders", name: "RetrieveEventOrder", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.RetrieveEventOrder(ctx, "ev/1", "ord/1")
		}, want: "GET " + eventBase + "/orders/ord%2F1"},
		{group: "orders", name: "ExportEventOrdersCSV", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.ExportEventOrdersCSV(ctx, "ev/1", &EventOrderExportParams{
				Q: "ann", Status: "paid", PaidBy: "card", From: 10, To: 20,
			})
		}, want: "GET " + eventBase + "/orders.csv?from=10&paidBy=card&q=ann&status=paid&to=20", response: "order\nord_1\n"},
		{group: "orders", name: "RefundEventOrder", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.RefundEventOrder(ctx, "ev/1", "ord/1", &OrderRefundParams{AmountMinor: 250})
		}, want: "POST " + eventBase + "/orders/ord%2F1/refund", body: `{"amountMinor":250}`, replay: true},
		{group: "orders", name: "CancelEventOrder", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.CancelEventOrder(ctx, "ev/1", "ord/1", OrderCancelParams{
				Reason: "lost_dispute", UsedTicketsAcknowledged: true, NotifyBuyer: &yes, IdempotencyKey: "cancel-event-1",
			})
		}, want: "POST " + eventBase + "/orders/ord%2F1/cancel",
			body:   `{"reason":"lost_dispute","usedTicketsAcknowledged":true,"notifyBuyer":true}`,
			replay: true, key: "cancel-event-1"},
		{group: "orders", name: "ResendEventOrderTickets", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Orders.ResendEventOrderTickets(ctx, "ev/1", "ord/1", nil)
		}, want: "POST " + eventBase + "/orders/ord%2F1/resend"},

		// ---------- event sales (14), on Events ----------
		{group: "eventSales", name: "RetrieveBuyerCheckout", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.RetrieveBuyerCheckout(ctx, "ev/1")
		}, want: "GET " + eventBase + "/buyer-checkout"},
		{group: "eventSales", name: "ReplaceBuyerCheckout", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.ReplaceBuyerCheckout(ctx, "ev/1", EventBuyerCheckoutParams{BuyerCheckout: "seatlayer"})
		}, want: "PUT " + eventBase + "/buyer-checkout", body: `{"buyerCheckout":"seatlayer","expectedRevision":0}`},
		{group: "eventSales", name: "RetrievePaymentGateway", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.RetrievePaymentGateway(ctx, "ev/1", "EUR")
		}, want: "GET " + eventBase + "/payment-gateway?currency=EUR"},
		{group: "eventSales", name: "ReplacePaymentGateway", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.ReplacePaymentGateway(ctx, "ev/1", &EventPaymentGatewayParams{
				ConnectionID: FieldValue("conn_1"), Provider: FieldNull[string](),
			})
		}, want: "PUT " + eventBase + "/payment-gateway", body: `{"connectionId":"conn_1","provider":null}`},
		{group: "eventSales", name: "RetrieveSalesReadiness", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.RetrieveSalesReadiness(ctx, "ev/1")
		}, want: "GET " + eventBase + "/sales/readiness"},
		{group: "eventSales", name: "PreflightSales", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.PreflightSales(ctx, "ev/1", "ord_1")
		}, want: "POST " + eventBase + "/sales/preflight", body: `{"orderId":"ord_1"}`},
		{group: "eventSales", name: "TransitionSales", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.TransitionSales(ctx, "ev/1", EventSalesTransitionParams{To: "live", ExpectedRevision: 3, Reason: "go"})
		}, want: "POST " + eventBase + "/sales/transition", body: `{"to":"live","expectedRevision":3,"reason":"go"}`},
		{group: "eventSales", name: "RetrieveBookingFee", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.RetrieveBookingFee(ctx, "ev/1")
		}, want: "GET " + eventBase + "/booking-fee"},
		{group: "eventSales", name: "ReplaceBookingFee", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.ReplaceBookingFee(ctx, "ev/1", 150)
		}, want: "PUT " + eventBase + "/booking-fee", body: `{"bookingFeeMinor":150}`},
		{group: "eventSales", name: "ListDiscountCodes", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.ListDiscountCodes(ctx, "ev/1")
		}, want: "GET " + eventBase + "/discount-codes"},
		{group: "eventSales", name: "CreateDiscountCode", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.CreateDiscountCode(ctx, "ev/1", EventDiscountCodeCreateParams{
				Code: "EARLY", Kind: "percent", Percent: 10, MaxUses: 100, Active: &no,
			})
		}, want: "POST " + eventBase + "/discount-codes",
			body: `{"code":"EARLY","kind":"percent","percent":10,"maxUses":100,"active":false}`, status: 201},
		{group: "eventSales", name: "UpdateDiscountCode", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.UpdateDiscountCode(ctx, "ev/1", "code/1", EventDiscountCodeUpdateParams{
				State: "paused", EndsAt: FieldNull[int64](), MaxUses: FieldValue(5),
			})
		}, want: "PATCH " + eventBase + "/discount-codes/code%2F1", body: `{"state":"paused","endsAt":null,"maxUses":5}`},
		{group: "eventSales", name: "EndDiscountCode", call: func(ctx context.Context, c *Client) (any, error) {
			return c.Events.EndDiscountCode(ctx, "ev/1", "code/1")
		}, want: "POST " + eventBase + "/discount-codes/code%2F1/end"},
		{group: "eventSales", name: "DeleteDiscountCode", call: func(ctx context.Context, c *Client) (any, error) {
			return nil, c.Events.DeleteDiscountCode(ctx, "ev/1", "code/1")
		}, want: "DELETE " + eventBase + "/discount-codes/code%2F1", status: 204},

		// ---------- event series (5) ----------
		{group: "eventSeries", name: "Create", call: func(ctx context.Context, c *Client) (any, error) {
			return c.EventSeries.Create(ctx, EventSeriesCreateParams{
				Name: "Hamlet", ChartKey: "chart_1",
				Schedule: EventSeriesSchedule{Rule: "weekly", From: "2027-01-04", Count: 4, Times: []string{"19:30"}},
				Timezone: "Europe/London", DurationMinutes: 150, DryRun: true,
			})
		}, want: "POST /v1/event-series",
			body: `{"name":"Hamlet","chartKey":"chart_1","schedule":{"rule":"weekly","from":"2027-01-04","count":4,` +
				`"times":["19:30"]},"timezone":"Europe/London","durationMinutes":150,"dryRun":true}`,
			replay: true, status: 202},
		{group: "eventSeries", name: "List", call: func(ctx context.Context, c *Client) (any, error) {
			return c.EventSeries.List(ctx, &EventSeriesListParams{Limit: 10, Cursor: "c/1"})
		}, want: "GET /v1/event-series?cursor=c%2F1&limit=10"},
		{group: "eventSeries", name: "Retrieve", call: func(ctx context.Context, c *Client) (any, error) {
			return c.EventSeries.Retrieve(ctx, "ev/1")
		}, want: "GET /v1/event-series/ev%2F1"},
		{group: "eventSeries", name: "AddDates", call: func(ctx context.Context, c *Client) (any, error) {
			return c.EventSeries.AddDates(ctx, "ev/1", EventSeriesDatesParams{
				Dates: []string{"2027-02-01T19:30"}, IdempotencyKey: "dates-1",
			})
		}, want: "POST /v1/event-series/ev%2F1/dates", body: `{"dates":["2027-02-01T19:30"]}`,
			replay: true, key: "dates-1", status: 202},
		{group: "eventSeries", name: "Update", call: func(ctx context.Context, c *Client) (any, error) {
			return c.EventSeries.Update(ctx, "ev/1", EventSeriesUpdateParams{
				Name: "Hamlet (revival)", Sales: "off",
				Prices: &EventSeriesPrices{Categories: []EventSeriesPriceCategory{
					{Key: "cat_a", Price: FieldValue(45.5), Tiers: []EventSeriesPriceTier{{ID: "t1", Price: 40}}},
					{Key: "cat_b", Price: FieldNull[float64]()},
				}},
			})
		}, want: "PATCH /v1/event-series/ev%2F1",
			body: `{"name":"Hamlet (revival)","sales":"off","prices":{"categories":[` +
				`{"key":"cat_a","price":45.5,"tiers":[{"id":"t1","price":40}]},{"key":"cat_b","price":null}]}}`,
			replay: true},
	}
}

func requestLine(r recorded) string {
	line := r.method + " " + r.escapedPath
	if r.query != "" {
		line += "?" + r.query
	}
	return line
}

func assertJSONBody(t *testing.T, got, want string) {
	t.Helper()
	if want == "" {
		if got != "" {
			t.Fatalf("body = %s, want none", got)
		}
		return
	}
	var gotValue, wantValue any
	if err := json.Unmarshal([]byte(got), &gotValue); err != nil {
		t.Fatalf("body %q is not JSON: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &wantValue); err != nil {
		t.Fatalf("bad expected body %q: %v", want, err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestSeason2aOperationsCoverTheSpec(t *testing.T) {
	counts := map[string]int{}
	for _, tc := range newOperationCases() {
		counts[tc.group]++
	}
	want := map[string]int{"seasons": 42, "holders": 14, "orders": 9, "eventSales": 14, "eventSeries": 5}
	if !reflect.DeepEqual(counts, want) {
		t.Fatalf("operation cases per group = %v, want %v (84 in all)", counts, want)
	}
}

func TestSeason2aOperationsSendExactRequests(t *testing.T) {
	for _, tc := range newOperationCases() {
		t.Run(tc.group+"/"+tc.name, func(t *testing.T) {
			client, calls := newTestClient(t, []stub{tc.stub()})
			if _, err := tc.call(context.Background(), client); err != nil {
				t.Fatalf("call failed: %v", err)
			}
			if len(*calls) != 1 {
				t.Fatalf("requests = %d, want 1", len(*calls))
			}
			got := call(t, calls, 0)
			if line := requestLine(got); line != tc.want {
				t.Fatalf("request = %q, want %q", line, tc.want)
			}
			assertJSONBody(t, got.body, tc.body)
			if tc.body != "" && got.header.Get("Content-Type") != "application/json" {
				t.Fatalf("Content-Type = %q", got.header.Get("Content-Type"))
			}
			key := got.header.Get("Idempotency-Key")
			switch {
			case tc.replay && key == "":
				t.Fatal("header-replay operation sent no Idempotency-Key")
			case tc.replay && tc.key != "" && key != tc.key:
				t.Fatalf("Idempotency-Key = %q, want caller key %q", key, tc.key)
			case !tc.replay && key != "":
				t.Fatalf("operation unexpectedly sent Idempotency-Key %q", key)
			}
		})
	}
}

// Header-replay operations retry with the same key; reads retry; every other
// mutation (domain-exact and unsupported alike) is single-attempt.
func TestSeason2aOperationsRetryOnlyWhenReplaySafe(t *testing.T) {
	for _, tc := range newOperationCases() {
		t.Run(tc.group+"/"+tc.name, func(t *testing.T) {
			limited := stub{status: 429, body: `{"error":"rate_limited"}`, headers: map[string]string{"Retry-After": "0"}}
			client, calls := newTestClient(t, []stub{limited, tc.stub()})
			_, err := tc.call(context.Background(), client)
			read := strings.HasPrefix(tc.want, "GET ")
			if tc.replay || read {
				if err != nil {
					t.Fatalf("retried call failed: %v", err)
				}
				if len(*calls) != 2 {
					t.Fatalf("attempts = %d, want 2", len(*calls))
				}
				first := call(t, calls, 0).header.Get("Idempotency-Key")
				if second := call(t, calls, 1).header.Get("Idempotency-Key"); first != second {
					t.Fatalf("retry changed Idempotency-Key: %q then %q", first, second)
				}
				return
			}
			var limitedErr *RateLimitError
			if !errors.As(err, &limitedErr) {
				t.Fatalf("want RateLimitError, got %v", err)
			}
			if len(*calls) != 1 {
				t.Fatalf("single-attempt operation made %d attempts", len(*calls))
			}
		})
	}
}

// A holder return keeps the caller's key across a retry, escapes the holder
// reference, and hands back the return envelope.
func TestHolderCreateReturnKeepsCallerKeyAcrossRetry(t *testing.T) {
	created := `{"return":{"returnId":"ret_1","state":"returned","seasonKey":"sea_1",` +
		`"tickets":[{"ticketId":"tkt_0123456789abcdef01234567","eventKey":"ev_1","label":"A-1",` +
		`"state":"returned","error":null}],"createdAt":1}}`
	client, calls := newTestClient(t, []stub{
		{status: 503, body: `{"error":"unavailable"}`, headers: map[string]string{"Retry-After": "0"}},
		{status: 201, body: created},
	})
	got, err := client.Holders.CreateReturn(context.Background(), "a/b", HolderReturnCreateParams{
		TicketIDs: []string{"tkt_0123456789abcdef01234567"}, IdempotencyKey: "return-1",
	})
	if err != nil {
		t.Fatalf("CreateReturn: %v", err)
	}
	if ret, ok := got["return"].(map[string]any); !ok || ret["returnId"] != "ret_1" {
		t.Fatalf("result = %v, want the return envelope", got)
	}
	if len(*calls) != 2 {
		t.Fatalf("attempts = %d, want 2", len(*calls))
	}
	for i := range 2 {
		sent := call(t, calls, i)
		if line := requestLine(sent); line != "POST /v1/holders/a%2Fb/returns" {
			t.Fatalf("attempt %d request = %q", i, line)
		}
		assertJSONBody(t, sent.body, `{"ticketIds":["tkt_0123456789abcdef01234567"]}`)
		if key := sent.header.Get("Idempotency-Key"); key != "return-1" {
			t.Fatalf("attempt %d Idempotency-Key = %q, want return-1", i, key)
		}
	}
}

// A renewal checkout keeps one key across a retry, generated or the caller's,
// for a season offer and for a holder's offer alike.
func TestRenewalCheckoutKeepsOneKeyAcrossRetry(t *testing.T) {
	unavailable := stub{status: 503, body: `{"error":"unavailable"}`, headers: map[string]string{"Retry-After": "0"}}
	started := stub{status: 201, body: `{"orderId":"ord_1","payment":{"url":"https://pay.example/1"}}`}
	client, calls := newTestClient(t, []stub{unavailable, started, unavailable, started})
	ctx := context.Background()
	p := SeasonRenewalCheckoutParams{ReturnURL: "https://tickets.example/r"}

	if _, err := client.Seasons.CreateRenewalCheckout(ctx, "sea/a", "offer/1", p); err != nil {
		t.Fatalf("Seasons.CreateRenewalCheckout: %v", err)
	}
	p.IdempotencyKey = "renew-1"
	got, err := client.Holders.CreateRenewalCheckout(ctx, "a/b", "offer/1", p)
	if err != nil {
		t.Fatalf("Holders.CreateRenewalCheckout: %v", err)
	}
	if got["orderId"] != "ord_1" {
		t.Fatalf("result = %v, want the started order", got)
	}
	if len(*calls) != 4 {
		t.Fatalf("attempts = %d, want 4", len(*calls))
	}
	lines := []string{
		"POST /v1/seasons/sea%2Fa/renewal-offers/offer%2F1/checkout",
		"POST /v1/seasons/sea%2Fa/renewal-offers/offer%2F1/checkout",
		"POST /v1/holders/a%2Fb/renewal-offers/offer%2F1/checkout",
		"POST /v1/holders/a%2Fb/renewal-offers/offer%2F1/checkout",
	}
	for i, line := range lines {
		sent := call(t, calls, i)
		if got := requestLine(sent); got != line {
			t.Fatalf("attempt %d request = %q, want %q", i, got, line)
		}
		assertJSONBody(t, sent.body, `{"returnUrl":"https://tickets.example/r"}`)
	}
	first := call(t, calls, 0).header.Get("Idempotency-Key")
	if first == "" || call(t, calls, 1).header.Get("Idempotency-Key") != first {
		t.Fatalf("generated key not reused: %q then %q", first, call(t, calls, 1).header.Get("Idempotency-Key"))
	}
	for i := 2; i < 4; i++ {
		if key := call(t, calls, i).header.Get("Idempotency-Key"); key != "renew-1" {
			t.Fatalf("attempt %d Idempotency-Key = %q, want renew-1", i, key)
		}
	}
}

// The decline reason reaches the wire on both declines; without one the season
// decline still sends an empty object and the holder decline sends no body.
func TestRenewalDeclineReason(t *testing.T) {
	ok := stub{status: 200, body: `{"offer":{"state":"declined"}}`}
	client, calls := newTestClient(t, []stub{ok, ok, ok, ok})
	ctx := context.Background()

	_, _ = client.Seasons.DeclineRenewalOffer(ctx, "sea/a", "offer/1", SeasonRenewalDeclineParams{Reason: "Moving away"})
	_, _ = client.Seasons.DeclineRenewalOffer(ctx, "sea_1", "offer_1")
	_, _ = client.Holders.DeclineRenewalOffer(ctx, "a/b", "offer/1", &SeasonRenewalDeclineParams{Reason: "Moving away"})
	_, _ = client.Holders.DeclineRenewalOffer(ctx, "hol_1", "offer_1", nil)

	want := []struct{ line, body string }{
		{"POST /v1/seasons/sea%2Fa/renewal-offers/offer%2F1/decline", `{"reason":"Moving away"}`},
		{"POST /v1/seasons/sea_1/renewal-offers/offer_1/decline", `{}`},
		{"POST /v1/holders/a%2Fb/renewal-offers/offer%2F1/decline", `{"reason":"Moving away"}`},
		{"POST /v1/holders/hol_1/renewal-offers/offer_1/decline", ""},
	}
	if len(*calls) != len(want) {
		t.Fatalf("requests = %d, want %d", len(*calls), len(want))
	}
	for i, w := range want {
		sent := call(t, calls, i)
		if line := requestLine(sent); line != w.line {
			t.Fatalf("call %d = %q, want %q", i, line, w.line)
		}
		assertJSONBody(t, sent.body, w.body)
		if key := sent.header.Get("Idempotency-Key"); key != "" {
			t.Fatalf("call %d sent Idempotency-Key %q", i, key)
		}
	}
	if _, err := client.Seasons.DeclineRenewalOffer(ctx, "sea_1", "offer_1",
		SeasonRenewalDeclineParams{}, SeasonRenewalDeclineParams{}); err == nil {
		t.Fatal("two params values: want an error")
	}
}

func TestCSVExportsReturnRawText(t *testing.T) {
	csv := "name,email\r\n\"Lee, Ann\",ann@example.com\r\n"
	text := stub{status: 200, body: csv, headers: map[string]string{"Content-Type": "text/csv; charset=utf-8"}}
	client, calls := newTestClient(t, []stub{text, text, text})
	ctx := context.Background()

	holders, err := client.Seasons.ExportHoldersCSV(ctx, "sea/a", nil)
	if err != nil || holders != csv {
		t.Fatalf("ExportHoldersCSV = %q, %v", holders, err)
	}
	orders, err := client.Seasons.ExportOrdersCSV(ctx, "sea/a")
	if err != nil || orders != csv {
		t.Fatalf("ExportOrdersCSV = %q, %v", orders, err)
	}
	eventOrders, err := client.Orders.ExportEventOrdersCSV(ctx, "ev/1", nil)
	if err != nil || eventOrders != csv {
		t.Fatalf("ExportEventOrdersCSV = %q, %v", eventOrders, err)
	}
	for i := range 3 {
		got := call(t, calls, i)
		if got.query != "" {
			t.Fatalf("call %d sent query %q with nil params", i, got.query)
		}
		if accept := got.header.Get("Accept"); accept != "text/csv" {
			t.Fatalf("call %d Accept = %q, want text/csv", i, accept)
		}
	}
}

func TestCSVExportErrorsStayTyped(t *testing.T) {
	client, _ := newTestClient(t, []stub{{status: 404, body: `{"error":"not_found","message":"no season"}`}})
	text, err := client.Seasons.ExportOrdersCSV(context.Background(), "missing")
	var notFound *NotFoundError
	if text != "" || !errors.As(err, &notFound) {
		t.Fatalf("want NotFoundError and no text, got %q, %v", text, err)
	}
}

func TestNoContentDeletesReturnOnlyNil(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 204}, {status: 204}})
	ctx := context.Background()
	if err := client.Seasons.DeletePlan(ctx, "sea/a", "plan/1", "delete-plan-1"); err != nil {
		t.Fatalf("DeletePlan: %v", err)
	}
	if err := client.Events.DeleteDiscountCode(ctx, "ev/1", "code/1"); err != nil {
		t.Fatalf("DeleteDiscountCode: %v", err)
	}
	if got := call(t, calls, 0).header.Get("Idempotency-Key"); got != "delete-plan-1" {
		t.Fatalf("DeletePlan Idempotency-Key = %q", got)
	}
	if got := call(t, calls, 1).header.Get("Idempotency-Key"); got != "" {
		t.Fatalf("DeleteDiscountCode sent Idempotency-Key %q", got)
	}
}

func TestOptionalBodiesAreOmittedWhenNil(t *testing.T) {
	responses := make([]stub, 9)
	for i := range responses {
		responses[i] = stub{status: 200, body: `{}`}
	}
	client, calls := newTestClient(t, responses)
	ctx := context.Background()

	_, _ = client.Seasons.CreateCheckoutTest(ctx, "sea/a", nil)
	_, _ = client.Seasons.RefundOrder(ctx, "sea/a", "ord/1", nil)
	_, _ = client.Seasons.CancelOrderDate(ctx, "sea/a", "ord/1", "ev/1", nil)
	_, _ = client.Seasons.CreateRenewalNotices(ctx, "sea/a", nil)
	_, _ = client.Holders.RevokeSessions(ctx, "hol/1", nil)
	_, _ = client.Events.ReplacePaymentGateway(ctx, "ev/1", nil)
	_, _ = client.Events.ReplaceBookingFee(ctx, "ev/1")
	_, _ = client.EventSeries.AddDates(ctx, "ev/1", EventSeriesDatesParams{
		Schedule: &EventSeriesSchedule{Rule: "dates", Dates: []string{"2027-03-01"}, Times: []string{"20:00"}},
	})
	_, _ = client.EventSeries.AddDates(ctx, "ev/1", EventSeriesDatesParams{RetryFailed: true})

	for i := range 6 {
		if body := call(t, calls, i).body; body != "" {
			t.Fatalf("call %d body = %q, want none", i, body)
		}
	}
	assertJSONBody(t, call(t, calls, 6).body, `{"bookingFeeMinor":null}`)
	assertJSONBody(t, call(t, calls, 7).body,
		`{"schedule":{"rule":"dates","dates":["2027-03-01"],"times":["20:00"]}}`)
	// Make again the dates that failed to be made.
	assertJSONBody(t, call(t, calls, 8).body, `{"retryFailed":true}`)
	if _, err := client.Events.ReplaceBookingFee(ctx, "ev/1", 1, 2); err == nil {
		t.Fatal("ReplaceBookingFee accepted two values")
	}
}

func TestWebhookAcceptsSeasonPlatformEventNames(t *testing.T) {
	names := []WebhookEventName{
		WebhookEventSeriesCreated, WebhookEventSeriesDatesAdded,
		WebhookEventOrderPaid, WebhookEventOrderRefunded, WebhookEventOrderCancelled,
		WebhookEventCustomerCreated, WebhookEventCustomerEmailChanged,
		WebhookEventSeasonTicketTransferSent, WebhookEventSeasonTicketTransferred,
		WebhookEventSeasonTicketTransferCancelled,
	}
	client, calls := newTestClient(t, []stub{{status: 201, body: `{"sub":{"id":"wh_1"},"secret":"whsec_once"}`}})
	if _, err := client.Webhooks.Create(context.Background(), "https://hooks.example", names); err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertJSONBody(t, call(t, calls, 0).body, `{"url":"https://hooks.example","events":[`+
		`"series.created","series.dates_added","order.paid","order.refunded","order.cancelled",`+
		`"customer.created","customer.email_changed","season.ticket.transfer_sent",`+
		`"season.ticket.transferred","season.ticket.transfer_cancelled"]}`)
}

func TestWebhookAcceptsAllPublicEventNames(t *testing.T) {
	names := make([]WebhookEventName, 0, len(supportedWebhookEvents))
	for name := range supportedWebhookEvents {
		names = append(names, name)
	}
	want := []string{
		"seat.booked", "seat.released", "seat.blocked", "hold.expired",
		"hold.created", "hold.extended", "event.created", "event.soldout",
		"object.resale.listed", "object.resale.unlisted", "object.resale.sold", "customer.created",
		"customer.email_changed", "order.paid", "order.refunded", "order.cancelled",
		"season.ticket.transfer_sent", "season.ticket.transferred", "season.ticket.transfer_cancelled", "season.ticket.issued",
		"season.ticket.voided", "season.structure.activated", "season.structure.closed", "season.structure.archived",
		"season.plan.published", "season.plan.superseded", "season.sales.opened", "season.sales.paused",
		"season.sales.ended", "season.hold.committed", "season.hold.released", "season.hold.expired",
		"season.booking.confirmed", "season.booking.cancelled", "season.booking.emergency_requested", "season.occurrence.returned",
		"season.occurrence.reclaimed", "season.renewal.offered", "season.renewal.intent_received", "season.renewal.renewed",
		"season.renewal.declined", "season.renewal.lapsed", "season.renewal.released", "season.operation.partial_terminal",
		"season.amendment.applied", "season.performance.added", "season.performance.removed", "season.performance.past",
		"season.performance.postponed", "season.performance.cancelled", "season.performance.rescheduled", "season.bulk_action.completed",
		"season.block.ready", "season.block.released",
		"season.order.paid", "season.order.refunded", "series.created", "series.dates_added",
	}
	if len(names) != 58 || len(want) != 58 {
		t.Fatalf("supported names = %d, listed = %d, want 58", len(names), len(want))
	}
	for _, name := range want {
		if _, ok := supportedWebhookEvents[WebhookEventName(name)]; !ok {
			t.Errorf("%s is not accepted", name)
		}
	}
	added := []WebhookEventName{
		WebhookEventSeasonPerformancePostponed, WebhookEventSeasonPerformanceCancelled,
		WebhookEventSeasonPerformanceRescheduled, WebhookEventSeasonBulkActionCompleted,
		WebhookEventSeasonOrderPaid, WebhookEventSeasonStructureActivated,
	}
	client, calls := newTestClient(t, []stub{{status: 201, body: `{"sub":{"id":"wh_1"},"secret":"whsec_once"}`}})
	if _, err := client.Webhooks.Create(context.Background(), "https://hooks.example", added); err != nil {
		t.Fatalf("Create: %v", err)
	}
	assertJSONBody(t, call(t, calls, 0).body, `{"url":"https://hooks.example","events":[`+
		`"season.performance.postponed","season.performance.cancelled","season.performance.rescheduled",`+
		`"season.bulk_action.completed","season.order.paid","season.structure.activated"]}`)
	if _, err := client.Webhooks.Create(context.Background(), "https://hooks.example",
		[]WebhookEventName{"season.unknown"}); err == nil {
		t.Fatal("Create accepted an unknown event name")
	}
}

func TestSeasonRequestsSendPriceTypesSaleSettingsContactsAndDateCancels(t *testing.T) {
	responses := make([]stub, 6)
	for i := range responses {
		responses[i] = stub{status: 200, body: `{}`}
	}
	client, calls := newTestClient(t, responses)
	ctx := context.Background()
	email, name, dryRun := "ana@example.com", "Ana Silva", false

	_, _ = client.Seasons.ReplacePackagePrices(ctx, "sea_1", "plan_1", SeasonPackagePricesParams{
		Prices: []SeasonPackagePrice{
			{CategoryKey: "stalls", UnitPrice: 18000},
			{CategoryKey: "stalls", PriceTypeKey: SeasonPriceTypeChild, UnitPrice: 9000},
		},
	})
	_, _ = client.Seasons.Update(ctx, "sea_1", SeasonUpdateParams{
		ExpectedRevision: 3, CheckoutMode: "seatlayer", Audience: "public",
		TransferCutoffHours: FieldValue(24), ReturnCutoffHours: FieldNull[int](),
	})
	_, _ = client.Seasons.Update(ctx, "sea_1", SeasonUpdateParams{ExpectedRevision: 4, Name: "Renamed"})
	_, _ = client.Seasons.CreateHolderImport(ctx, "sea_1", SeasonHolderImportParams{
		SuccessorPlanActivationID: "pa_1", DryRun: &dryRun,
		Rows: []SeasonHolderImportRow{{
			RowID: "r1", HolderRef: "h1", PriorPlanActivationID: "pa_0", PriorContractRef: "c1",
			Labels: []string{"A-1"}, HolderEmail: &email, HolderName: &name,
		}},
	})
	_, _ = client.Seasons.CreateAmendment(ctx, "sea_1", SeasonAmendmentParams{
		EventKey: "ev_night_3", Kind: "cancel", Refund: "per_date", Reason: "Storm", IdempotencyKey: "night-3-storm",
	})
	_, _ = client.Seasons.CreateAmendment(ctx, "sea_1", SeasonAmendmentParams{EventKey: "ev_1", Kind: "postpone"})

	assertJSONBody(t, call(t, calls, 0).body, `{"prices":[{"categoryKey":"stalls","unitPrice":18000},`+
		`{"categoryKey":"stalls","priceTypeKey":"child","unitPrice":9000}]}`)
	assertJSONBody(t, call(t, calls, 1).body,
		`{"expectedRevision":3,"checkoutMode":"seatlayer","audience":"public","transferCutoffHours":24,"returnCutoffHours":null}`)
	assertJSONBody(t, call(t, calls, 2).body, `{"expectedRevision":4,"name":"Renamed"}`)
	assertJSONBody(t, call(t, calls, 3).body, `{"successorPlanActivationId":"pa_1","dryRun":false,"rows":[{"rowId":"r1",`+
		`"holderRef":"h1","priorPlanActivationId":"pa_0","priorContractRef":"c1","labels":["A-1"],`+
		`"holderEmail":"ana@example.com","holderName":"Ana Silva"}]}`)
	assertJSONBody(t, call(t, calls, 4).body,
		`{"eventKey":"ev_night_3","kind":"cancel","refund":"per_date","reason":"Storm"}`)
	if key := call(t, calls, 4).header.Get("Idempotency-Key"); key != "night-3-storm" {
		t.Fatalf("amendment Idempotency-Key = %q", key)
	}
	assertJSONBody(t, call(t, calls, 5).body, `{"eventKey":"ev_1","kind":"postpone"}`)
}

// The renewal offer list and the hosted renewals page with a cursor and a
// limit; with no params they read the first page and send no query.
func TestRenewalOfferPagesSendCursorAndLimit(t *testing.T) {
	page := stub{status: 200, body: `{"offers":[],"truncated":false,"nextCursor":"c/2"}`}
	client, calls := newTestClient(t, []stub{page, page, page, page})
	ctx := context.Background()

	got, err := client.Seasons.ListRenewalOffers(ctx, "sea/a", SeasonOfferPageParams{Cursor: "c/1", Limit: 50})
	if err != nil {
		t.Fatalf("ListRenewalOffers: %v", err)
	}
	if got["nextCursor"] != "c/2" {
		t.Fatalf("nextCursor = %v, want c/2", got["nextCursor"])
	}
	_, _ = client.Seasons.ListRenewalOffers(ctx, "sea/a")
	_, _ = client.Seasons.RetrieveHostedRenewals(ctx, "sea/a", SeasonOfferPageParams{Cursor: "c/1", Limit: 500})
	_, _ = client.Seasons.RetrieveHostedRenewals(ctx, "sea/a", SeasonOfferPageParams{Cursor: "c/1"})

	lines := []string{
		"GET /v1/seasons/sea%2Fa/renewal-offers?cursor=c%2F1&limit=50",
		"GET /v1/seasons/sea%2Fa/renewal-offers",
		"GET /v1/seasons/sea%2Fa/renewal-hosted?cursor=c%2F1&limit=500",
		"GET /v1/seasons/sea%2Fa/renewal-hosted?cursor=c%2F1",
	}
	for i, line := range lines {
		if got := requestLine(call(t, calls, i)); got != line {
			t.Fatalf("call %d = %q, want %q", i, got, line)
		}
	}
	two := []SeasonOfferPageParams{{Limit: 1}, {Limit: 2}}
	if _, err := client.Seasons.ListRenewalOffers(ctx, "sea/a", two...); err == nil {
		t.Fatal("ListRenewalOffers accepted two params values")
	}
	if _, err := client.Seasons.RetrieveHostedRenewals(ctx, "sea/a", two...); err == nil {
		t.Fatal("RetrieveHostedRenewals accepted two params values")
	}
	if len(*calls) != 4 {
		t.Fatalf("requests = %d, want 4", len(*calls))
	}
}

// Above 100 contracts or holders the renewal run and the bring-across answer
// 202 with the job that does the work; the renewal run also names its batch.
func TestRenewalRunAndBringAcrossAnswerAJobAbove100(t *testing.T) {
	client, calls := newTestClient(t, []stub{
		{status: 202, body: `{"job":{"jobId":"sbj_1","kind":"renewal_run","state":"planning"},"batchId":"srb_1"}`},
		{status: 202, body: `{"job":{"jobId":"sbj_2","kind":"holder_import","state":"planning"}}`},
	})
	ctx := context.Background()

	run, err := client.Seasons.CreateRenewalOffers(ctx, "sea/a", SeasonRenewalOffersParams{DeadlineAt: 123})
	if err != nil {
		t.Fatalf("CreateRenewalOffers: %v", err)
	}
	if job, ok := run["job"].(map[string]any); !ok || job["kind"] != "renewal_run" || run["batchId"] != "srb_1" {
		t.Fatalf("renewal run = %v, want the job and its batchId", run)
	}
	imported, err := client.Seasons.CreateHolderImportFromSeason(ctx, "sea/a", SeasonHolderImportFromSeasonParams{
		SourceSeasonID: "sea_2026", SuccessorPlanActivationID: "pa_1",
	})
	if err != nil {
		t.Fatalf("CreateHolderImportFromSeason: %v", err)
	}
	if job, ok := imported["job"].(map[string]any); !ok || job["kind"] != "holder_import" {
		t.Fatalf("bring-across = %v, want the holder_import job", imported)
	}
	for i := range 2 {
		if call(t, calls, i).header.Get("Idempotency-Key") == "" {
			t.Fatalf("call %d sent no Idempotency-Key", i)
		}
	}
}

// Releasing lapsed offers sends no body and keeps one key across a retry,
// generated or the caller's.
func TestReleaseLapsedOffersKeepsOneKeyAcrossRetry(t *testing.T) {
	unavailable := stub{status: 503, body: `{"error":"job_busy"}`, headers: map[string]string{"Retry-After": "0"}}
	accepted := stub{status: 202, body: `{"job":{"jobId":"sbj_1","kind":"lapsed_release","state":"planning"}}`}
	client, calls := newTestClient(t, []stub{unavailable, accepted, unavailable, accepted})
	ctx := context.Background()

	got, err := client.Seasons.ReleaseLapsedOffers(ctx, "sea/a", "")
	if err != nil {
		t.Fatalf("ReleaseLapsedOffers: %v", err)
	}
	if job, ok := got["job"].(map[string]any); !ok || job["jobId"] != "sbj_1" {
		t.Fatalf("result = %v, want the job", got)
	}
	if _, err := client.Seasons.ReleaseLapsedOffers(ctx, "sea_1", "release-1"); err != nil {
		t.Fatalf("ReleaseLapsedOffers with a key: %v", err)
	}
	if len(*calls) != 4 {
		t.Fatalf("attempts = %d, want 4", len(*calls))
	}
	for i, line := range []string{
		"POST /v1/seasons/sea%2Fa/renewal-offers/release-lapsed",
		"POST /v1/seasons/sea%2Fa/renewal-offers/release-lapsed",
		"POST /v1/seasons/sea_1/renewal-offers/release-lapsed",
		"POST /v1/seasons/sea_1/renewal-offers/release-lapsed",
	} {
		sent := call(t, calls, i)
		if got := requestLine(sent); got != line {
			t.Fatalf("attempt %d request = %q, want %q", i, got, line)
		}
		if sent.body != "" {
			t.Fatalf("attempt %d body = %q, want none", i, sent.body)
		}
	}
	first := call(t, calls, 0).header.Get("Idempotency-Key")
	if first == "" || call(t, calls, 1).header.Get("Idempotency-Key") != first {
		t.Fatalf("generated key not reused: %q then %q", first, call(t, calls, 1).header.Get("Idempotency-Key"))
	}
	for i := 2; i < 4; i++ {
		if key := call(t, calls, i).header.Get("Idempotency-Key"); key != "release-1" {
			t.Fatalf("attempt %d Idempotency-Key = %q, want release-1", i, key)
		}
	}
}

// Renewal notices for one offer batch send its batchId and no offer ids.
func TestRenewalNoticesForOneOfferBatchSendTheBatchID(t *testing.T) {
	client, calls := newTestClient(t, []stub{{status: 200, body: `{"scheduled":240,"withoutEmail":3}`}})

	got, err := client.Seasons.CreateRenewalNotices(context.Background(), "sea/a",
		&SeasonRenewalNoticesParams{BatchID: "srb/1"})
	if err != nil {
		t.Fatalf("CreateRenewalNotices: %v", err)
	}
	if got["scheduled"] != float64(240) {
		t.Fatalf("scheduled = %v, want 240", got["scheduled"])
	}
	sent := call(t, calls, 0)
	if line := requestLine(sent); line != "POST /v1/seasons/sea%2Fa/renewal-notices" {
		t.Fatalf("request = %q", line)
	}
	assertJSONBody(t, sent.body, `{"batchId":"srb/1"}`)
	if key := sent.header.Get("Idempotency-Key"); key != "" {
		t.Fatalf("Idempotency-Key = %q, want none", key)
	}
}

// Single tickets reach the wire on create and update, an explicit false
// included, and are left out when nil.
func TestSeasonSingleTicketsReachTheWireOnCreateAndUpdate(t *testing.T) {
	answer := stub{status: 200, body: `{"season":{"key":"sea_1","singleTickets":true},` +
		`"singleTicketDates":{"direction":"open","opened":["ev_1"],"closed":[],"notOpened":[],"jobId":null}}`}
	client, calls := newTestClient(t, []stub{answer, answer, answer})
	ctx := context.Background()
	on, off := true, false

	if _, err := client.Seasons.Create(ctx, SeasonCreateParams{
		Name: "Series", EventKeys: []string{"ev_1", "ev_2"}, SingleTickets: &on,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	updated, err := client.Seasons.Update(ctx, "sea/a", SeasonUpdateParams{ExpectedRevision: 5, SingleTickets: &off})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := client.Seasons.Update(ctx, "sea/a", SeasonUpdateParams{ExpectedRevision: 6, Name: "Series 2"}); err != nil {
		t.Fatalf("Update without single tickets: %v", err)
	}

	assertJSONBody(t, call(t, calls, 0).body, `{"name":"Series","eventKeys":["ev_1","ev_2"],"singleTickets":true}`)
	assertJSONBody(t, call(t, calls, 1).body, `{"expectedRevision":5,"singleTickets":false}`)
	assertJSONBody(t, call(t, calls, 2).body, `{"expectedRevision":6,"name":"Series 2"}`)
	if got := requestLine(call(t, calls, 1)); got != "PATCH /v1/seasons/sea%2Fa" {
		t.Fatalf("update request = %q", got)
	}
	dates, ok := updated["singleTicketDates"].(map[string]any)
	if !ok || dates["direction"] != "open" {
		t.Fatalf("singleTicketDates = %v, want the dates it opened", updated["singleTicketDates"])
	}
}

// InventoryMode reaches the wire on create and update only when set, and every
// Season answer's inventoryMode reads back.
func TestSeasonInventoryModeReachesTheWireAndReadsBack(t *testing.T) {
	block := stub{status: 200, body: `{"season":{"key":"sea_1","revision":5,"inventoryMode":"block"}}`}
	perHold := stub{status: 200, body: `{"season":{"key":"sea_2","revision":1,"inventoryMode":"per_hold"}}`}
	list := stub{status: 200, body: `{"seasons":[{"key":"sea_1","inventoryMode":"block"},` +
		`{"key":"sea_2","inventoryMode":"per_hold"}],"nextCursor":null}`}
	client, calls := newTestClient(t, []stub{block, perHold, block, perHold, perHold, block, list})
	ctx := context.Background()

	created, err := client.Seasons.Create(ctx, SeasonCreateParams{
		Name: "Series", EventKeys: []string{"ev_1", "ev_2"}, InventoryMode: SeasonInventoryModeBlock,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := client.Seasons.Create(ctx, SeasonCreateParams{Name: "Series", EventKeys: []string{"ev_1", "ev_2"}}); err != nil {
		t.Fatalf("Create without inventory mode: %v", err)
	}
	turnedOn, err := client.Seasons.Update(ctx, "sea_1", SeasonUpdateParams{
		ExpectedRevision: 5, InventoryMode: SeasonInventoryModeBlock,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := client.Seasons.Update(ctx, "sea_1", SeasonUpdateParams{
		ExpectedRevision: 5, InventoryMode: SeasonInventoryModePerHold,
	}); err != nil {
		t.Fatalf("Update to per_hold: %v", err)
	}
	if _, err := client.Seasons.Update(ctx, "sea_1", SeasonUpdateParams{ExpectedRevision: 5, Name: "Series 2"}); err != nil {
		t.Fatalf("Update without inventory mode: %v", err)
	}
	retrieved, err := client.Seasons.Retrieve(ctx, "sea_1")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	listed, err := client.Seasons.List(ctx, nil)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	assertJSONBody(t, call(t, calls, 0).body, `{"name":"Series","eventKeys":["ev_1","ev_2"],"inventoryMode":"block"}`)
	assertJSONBody(t, call(t, calls, 1).body, `{"name":"Series","eventKeys":["ev_1","ev_2"]}`)
	assertJSONBody(t, call(t, calls, 2).body, `{"expectedRevision":5,"inventoryMode":"block"}`)
	assertJSONBody(t, call(t, calls, 3).body, `{"expectedRevision":5,"inventoryMode":"per_hold"}`)
	assertJSONBody(t, call(t, calls, 4).body, `{"expectedRevision":5,"name":"Series 2"}`)
	wantLines := []string{
		"POST /v1/seasons", "POST /v1/seasons", "PATCH /v1/seasons/sea_1", "PATCH /v1/seasons/sea_1",
		"PATCH /v1/seasons/sea_1", "GET /v1/seasons/sea_1", "GET /v1/seasons",
	}
	for index, want := range wantLines {
		if got := requestLine(call(t, calls, index)); got != want {
			t.Fatalf("request %d = %q, want %q", index, got, want)
		}
	}
	for name, answer := range map[string]map[string]any{"Create": created, "Update": turnedOn, "Retrieve": retrieved} {
		season, _ := answer["season"].(map[string]any)
		if season["inventoryMode"] != string(SeasonInventoryModeBlock) {
			t.Fatalf("%s inventoryMode = %v, want block", name, season["inventoryMode"])
		}
	}
	items, _ := listed["seasons"].([]any)
	var modes []string
	for _, item := range items {
		mode, _ := item.(map[string]any)["inventoryMode"].(string)
		modes = append(modes, mode)
	}
	if !reflect.DeepEqual(modes, []string{"block", "per_hold"}) {
		t.Fatalf("List inventoryMode = %v, want [block per_hold]", modes)
	}
}

// The inventory mode refusals, and opening sales before the seats are set
// aside, come back as the typed errors with their code.
func TestSeasonInventoryModeRefusalsCarryTheirCode(t *testing.T) {
	refusals := []struct {
		status int
		code   string
	}{
		{409, "season_block_packages"}, {409, "season_block_on_sale"}, {409, "season_not_draft_or_active"},
		{409, "season_block_exists"}, {422, "invalid_inventory_mode"},
	}
	var responses []stub
	for _, refusal := range refusals {
		responses = append(responses, stub{
			status: refusal.status,
			body:   `{"error":"` + refusal.code + `","code":"` + refusal.code + `","message":"refused"}`,
		})
	}
	responses = append(responses, stub{status: 409, body: `{"error":"block_not_ready","code":"block_not_ready"}`})
	client, calls := newTestClient(t, responses)
	ctx := context.Background()

	for _, refusal := range refusals {
		_, err := client.Seasons.Update(ctx, "sea_1", SeasonUpdateParams{
			ExpectedRevision: 5, InventoryMode: SeasonInventoryModeBlock,
		})
		var apiErr *APIError
		var conflict *ConflictError
		var validation *ValidationError
		switch {
		case errors.As(err, &conflict):
			apiErr = &conflict.APIError
		case errors.As(err, &validation):
			apiErr = &validation.APIError
		}
		if apiErr == nil || apiErr.Status != refusal.status || apiErr.Code != refusal.code {
			t.Fatalf("Update error = %v, want %d %s", err, refusal.status, refusal.code)
		}
	}
	_, err := client.Seasons.OpenSales(ctx, "sea_1", 5)
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Code != "block_not_ready" {
		t.Fatalf("OpenSales error = %v, want ConflictError block_not_ready", err)
	}
	if len(*calls) != len(responses) {
		t.Fatalf("sent %d requests, want %d", len(*calls), len(responses))
	}
}
