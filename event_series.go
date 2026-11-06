package seatlayer

import (
	"context"
	"net/http"
	"net/url"
)

// EventSeriesService plans and manages event series: runs of dated events made
// from one floor plan and a schedule.
type EventSeriesService struct{ client *Client }

// EventSeriesSchedule describes the dates of a series. Rule is one of none,
// daily, weekly, biweekly or dates. Days are YYYY-MM-DD, times are HH:MM and
// skipped moments are YYYY-MM-DDTHH:MM, all in the series' zone.
type EventSeriesSchedule struct {
	Rule string `json:"rule"`
	// From is the first day. Required unless Rule is "dates".
	From string `json:"from,omitempty"`
	// Until is the last day, inclusive. Give Until or Count, not both.
	Until string `json:"until,omitempty"`
	Count int    `json:"count,omitempty"`
	// Times are the start times on each day: one date per day and time.
	Times []string `json:"times"`
	Skip  []string `json:"skip,omitempty"`
	// Dates are the days for Rule "dates".
	Dates []string `json:"dates,omitempty"`
}

// EventSeriesCreateParams plans an event series from a schedule.
type EventSeriesCreateParams struct {
	// Name is the series name; every date carries it as its name. Leave it
	// empty with FromEventKey, which takes the event's own name.
	Name string
	// ChartKey is a published floor plan of this organization. Leave it empty
	// with FromEventKey.
	ChartKey string
	Schedule EventSeriesSchedule
	// Timezone is the IANA zone the schedule is read in. Empty leaves it out.
	Timezone string
	Venue    string
	// DurationMinutes sets each date's end. Zero leaves it out.
	DurationMinutes int
	// DryRun answers the dates the schedule makes and creates nothing.
	DryRun         bool
	IdempotencyKey string
	// FromEventKey makes the series from an event you already set up: it
	// becomes the first date under its own key, and every other date is a copy
	// of it (floor plan, prices, checkout, poster and its own seat blocks, none
	// of its orders, holds or sales). The series takes the event's name, zone
	// and floor plan, so leave Name, ChartKey, Venue and DurationMinutes empty.
	FromEventKey string
}

// EventSeriesListParams pages the event series listing.
type EventSeriesListParams struct {
	Limit  int
	Cursor string
}

// EventSeriesDatesParams adds dates to a series: set Schedule, or Dates as
// exact moments (YYYY-MM-DDTHH:MM in the series' zone), not both.
type EventSeriesDatesParams struct {
	Schedule *EventSeriesSchedule
	Dates    []string
	// RetryFailed makes again every date that failed to be made (up to 100, as
	// in job.failedDates): 422 nothing_to_retry when none did, 409 series_busy
	// while another change runs. Give it alone, without Schedule or Dates.
	RetryFailed    bool
	IdempotencyKey string
}

// EventSeriesPriceTier is one tier price inside a category.
type EventSeriesPriceTier struct {
	ID    string  `json:"id"`
	Price float64 `json:"price"`
}

// EventSeriesPriceCategory is one floor plan category's price. Use FieldNull
// to send an explicit null price.
type EventSeriesPriceCategory struct {
	Key   string
	Price NullableField[float64]
	Tiers []EventSeriesPriceTier
}

// EventSeriesPrices lists every category of the floor plan, as the event
// pricing route takes them. New orders pay these.
type EventSeriesPrices struct {
	Categories []EventSeriesPriceCategory
}

func (p EventSeriesPrices) requestValue() map[string]any {
	categories := make([]map[string]any, len(p.Categories))
	for i, category := range p.Categories {
		categories[i] = nullableFields(
			params("key", category.Key, "tiers", seasonSliceOrNil(category.Tiers)),
			fieldRequestValue("price", category.Price),
		)
	}
	return map[string]any{"categories": categories}
}

// EventSeriesUpdateParams changes every upcoming date of a series. Empty
// fields are left out.
type EventSeriesUpdateParams struct {
	Name   string
	Prices *EventSeriesPrices
	// Sales is "on" or "off".
	Sales          string
	IdempotencyKey string
}

// Create plans an event series from a schedule, or makes it as a job. Retried
// safely with one IdempotencyKey. A series from an event that never started
// and was given up answers 409 create_abandoned (a *ConflictError) for that key
// every time: send the request again with a new IdempotencyKey.
func (s *EventSeriesService) Create(
	ctx context.Context, p EventSeriesCreateParams,
) (map[string]any, error) {
	return s.client.postHeaderReplay(ctx, "/v1/event-series", params(
		"fromEventKey", stringOrNil(p.FromEventKey),
		"name", stringOrNil(p.Name),
		"chartKey", stringOrNil(p.ChartKey),
		"schedule", p.Schedule,
		"timezone", stringOrNil(p.Timezone),
		"venue", stringOrNil(p.Venue),
		"durationMinutes", intOrNil(p.DurationMinutes),
		"dryRun", boolOrNil(p.DryRun),
	), p.IdempotencyKey)
}

// List lists event series, one cursor page at a time.
func (s *EventSeriesService) List(ctx context.Context, p *EventSeriesListParams) (map[string]any, error) {
	query := url.Values{}
	if p != nil {
		setIntIfPositive(query, "limit", p.Limit)
		setIfNotEmpty(query, "cursor", p.Cursor)
	}
	return s.client.get(ctx, "/v1/event-series", query)
}

// Retrieve fetches an event series, its dates and its job.
func (s *EventSeriesService) Retrieve(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/event-series/"+escape(eventKey), nil)
}

// AddDates adds dates to an event series, copied from its first date.
func (s *EventSeriesService) AddDates(
	ctx context.Context, eventKey string, p EventSeriesDatesParams,
) (map[string]any, error) {
	body := params("dates", sliceOrNil(p.Dates), "retryFailed", boolOrNil(p.RetryFailed))
	if p.Schedule != nil {
		body["schedule"] = *p.Schedule
	}
	return s.client.postHeaderReplay(ctx, "/v1/event-series/"+escape(eventKey)+"/dates", body, p.IdempotencyKey)
}

// Update changes the name, prices or sales of every upcoming date.
func (s *EventSeriesService) Update(
	ctx context.Context, eventKey string, p EventSeriesUpdateParams,
) (map[string]any, error) {
	body := params("name", stringOrNil(p.Name), "sales", stringOrNil(p.Sales))
	if p.Prices != nil {
		body["prices"] = p.Prices.requestValue()
	}
	return s.client.mutationHeaderReplay(ctx, http.MethodPatch,
		"/v1/event-series/"+escape(eventKey), body, p.IdempotencyKey)
}
