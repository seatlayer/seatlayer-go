package seatlayer

import (
	"context"
	"iter"
	"net/url"
	"strconv"
)

// EventsService covers event lifecycle, metadata and reports.
type EventsService struct{ client *Client }

// EventListParams filters and pages an event listing.
type EventListParams struct {
	WorkspaceID string
	ExternalRef string
	// Limit is the page size. Clamped server-side; asking for more is not an error.
	Limit int
	// Cursor continues a previous page. Leave empty to start.
	Cursor string
	// NoCounts drops live availability counts, which cost the server one
	// round-trip per event. All sets this automatically.
	NoCounts bool
}

func (p *EventListParams) query() url.Values {
	values := url.Values{}
	if p == nil {
		return values
	}
	setIfNotEmpty(values, "workspaceId", p.WorkspaceID)
	setIfNotEmpty(values, "externalRef", p.ExternalRef)
	setIfNotEmpty(values, "cursor", p.Cursor)
	if p.Limit > 0 {
		values.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.NoCounts {
		values.Set("counts", "0")
	}
	return values
}

// List returns one page of events, including live availability counts unless
// NoCounts is set.
func (s *EventsService) List(ctx context.Context, p *EventListParams) (Page, error) {
	response, err := s.client.get(ctx, "/v1/events", p.query())
	if err != nil {
		return Page{}, err
	}
	return pageFrom(response, "events"), nil
}

// All walks every event, paging transparently.
//
// Counts are dropped by default here — you are walking the whole list, so
// per-event availability is rarely what you want and always what it costs.
func (s *EventsService) All(ctx context.Context, p *EventListParams) iter.Seq2[map[string]any, error] {
	return paginate(ctx, func(ctx context.Context, cursor string) (Page, error) {
		next := EventListParams{NoCounts: true}
		if p != nil {
			next = *p
			next.NoCounts = true
		}
		next.Cursor = cursor
		return s.List(ctx, &next)
	})
}

// EventCreateParams describes a new event.
type EventCreateParams struct {
	// ChartID must reference a published chart.
	ChartID string
	Name    string
	Slug    string
	// StartsAt is epoch milliseconds.
	StartsAt    int64
	Venue       string
	ExternalRef string
	// Currency overrides the organisation currency for this event.
	Currency       string
	IdempotencyKey string
}

// Create makes an event from a published chart.
func (s *EventsService) Create(ctx context.Context, p EventCreateParams) (map[string]any, error) {
	body := params(
		"chartId", p.ChartID,
		"name", stringOrNil(p.Name),
		"slug", stringOrNil(p.Slug),
		"startsAt", int64OrNil(p.StartsAt),
		"venue", stringOrNil(p.Venue),
		"externalRef", stringOrNil(p.ExternalRef),
		"currency", stringOrNil(p.Currency),
	)
	return s.client.post(ctx, "/v1/events", body, p.IdempotencyKey)
}

// Retrieve fetches an event with live counts.
func (s *EventsService) Retrieve(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey), nil)
}

// Update changes event metadata.
func (s *EventsService) Update(ctx context.Context, eventKey string, fields map[string]any) (map[string]any, error) {
	return s.client.patch(ctx, "/v1/events/"+escape(eventKey), fields)
}

// Delete soft-deletes an event.
func (s *EventsService) Delete(ctx context.Context, eventKey string) error {
	_, err := s.client.delete(ctx, "/v1/events/"+escape(eventKey))
	return err
}

// UpdateChart moves a live event onto the latest published version of its chart.
func (s *EventsService) UpdateChart(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/update-chart", nil, "")
}

// Close stops buyer sales. Existing holds keep their TTL.
func (s *EventsService) Close(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/close", nil, "")
}

// Reopen resumes buyer sales.
func (s *EventsService) Reopen(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/reopen", nil, "")
}

// Archive moves an event to the archive, preserving reporting.
func (s *EventsService) Archive(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/archive", nil, "")
}

// RetrieveHoldTTL reads the checkout window buyers get for this event.
func (s *EventsService) RetrieveHoldTTL(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/hold-ttl", nil)
}

// UpdateHoldTTL sets the checkout window, in milliseconds.
func (s *EventsService) UpdateHoldTTL(ctx context.Context, eventKey string, holdTTLMs int64) (map[string]any, error) {
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/hold-ttl",
		params("holdTtlMs", holdTTLMs), "")
}

// RetrieveReport fetches the event report.
func (s *EventsService) RetrieveReport(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/report", nil)
}

// RetrieveLog fetches the event audit log.
func (s *EventsService) RetrieveLog(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/log", nil)
}

func int64OrNil(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
