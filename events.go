package seatlayer

import (
	"context"
	"errors"
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
	Currency    string
	Description string
	// EndsAt is epoch milliseconds.
	EndsAt        int64
	Timezone      string
	Locale        string
	PosterAssetID string
	// Mode is normally inferred from the secret key; when supplied it must match.
	Mode           string
	IdempotencyKey string
	// Nullable overrides convenience scalar fields when explicit JSON null is
	// semantically different from omission.
	Nullable EventCreateNullableFields
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
		"description", stringOrNil(p.Description),
		"endsAt", int64OrNil(p.EndsAt),
		"timezone", stringOrNil(p.Timezone),
		"locale", stringOrNil(p.Locale),
		"posterAssetId", stringOrNil(p.PosterAssetID),
		"mode", stringOrNil(p.Mode),
	)
	p.Nullable.apply(body)
	return s.client.postHeaderReplay(ctx, "/v1/events", body, p.IdempotencyKey)
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

// UpdatePoster uploads raw PNG, JPEG, or WebP bytes. Content type defaults to
// application/octet-stream; pass one explicit media type when it is known.
func (s *EventsService) UpdatePoster(
	ctx context.Context, eventKey string, image []byte, contentType ...string,
) (map[string]any, error) {
	if len(contentType) > 1 {
		return nil, errors.New("seatlayer: UpdatePoster accepts at most one content type")
	}
	mediaType := "application/octet-stream"
	if len(contentType) == 1 && contentType[0] != "" {
		mediaType = contentType[0]
	}
	return s.client.putRaw(ctx, "/v1/events/"+escape(eventKey)+"/poster", image, mediaType)
}

// DeletePoster removes the event poster used by share cards.
func (s *EventsService) DeletePoster(
	ctx context.Context, eventKey string,
) (map[string]any, error) {
	return s.client.delete(ctx, "/v1/events/"+escape(eventKey)+"/poster")
}

// EventChartUpdateParams acknowledges assignment changes caused by a new chart.
type EventChartUpdateParams struct {
	AcknowledgeDroppedAssignments *bool
	Reason                        string
}

// UpdateChart moves a live event onto the latest published version of its chart.
// The optional params value preserves the original no-argument call shape.
func (s *EventsService) UpdateChart(
	ctx context.Context, eventKey string, options ...EventChartUpdateParams,
) (map[string]any, error) {
	if len(options) > 1 {
		return nil, errors.New("seatlayer: UpdateChart accepts at most one params value")
	}
	p := EventChartUpdateParams{}
	if len(options) == 1 {
		p = options[0]
	}
	body := params(
		"acknowledgeDroppedAssignments", p.AcknowledgeDroppedAssignments,
		"reason", stringOrNil(p.Reason),
	)
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/update-chart", body, "")
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

// ListTicketReleases returns releases with server-computed quota consumption.
func (s *EventsService) ListTicketReleases(ctx context.Context, eventKey string) (TicketReleaseList, error) {
	response, err := s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/releases", nil)
	if err != nil {
		return TicketReleaseList{}, err
	}
	return decodeResponse[TicketReleaseList](response)
}

// UpdateTicketReleases replaces the complete ordered release list. The public
// route has no replay contract, so this is deliberately single-attempt.
func (s *EventsService) UpdateTicketReleases(
	ctx context.Context, eventKey string, releases []TicketReleaseReplaceInput,
) (TicketReleaseList, error) {
	body := make([]map[string]any, len(releases))
	for i, release := range releases {
		body[i] = release.requestValue()
	}
	response, err := s.client.put(ctx, "/v1/events/"+escape(eventKey)+"/releases", map[string]any{
		"releases": body,
	})
	if err != nil {
		return TicketReleaseList{}, err
	}
	return decodeResponse[TicketReleaseList](response)
}

// CloseTicketRelease closes one release immediately while preserving it for
// reporting. It remains single-attempt because it has no replay contract.
func (s *EventsService) CloseTicketRelease(
	ctx context.Context, eventKey, releaseID string,
) (TicketReleaseList, error) {
	response, err := s.client.post(ctx,
		"/v1/events/"+escape(eventKey)+"/releases/"+escape(releaseID)+"/close", nil, "")
	if err != nil {
		return TicketReleaseList{}, err
	}
	return decodeResponse[TicketReleaseList](response)
}

// RetrieveHoldTTL reads the checkout window buyers get for this event.
func (s *EventsService) RetrieveHoldTTL(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/hold-ttl", nil)
}

// UpdateHoldTTL sets the checkout window in milliseconds. Omit holdTTLMs to
// send JSON null and restore the event default.
func (s *EventsService) UpdateHoldTTL(
	ctx context.Context, eventKey string, holdTTLMs ...int64,
) (map[string]any, error) {
	if len(holdTTLMs) > 1 {
		return nil, errors.New("seatlayer: UpdateHoldTTL accepts zero or one value")
	}
	var value any
	if len(holdTTLMs) == 1 {
		value = holdTTLMs[0]
	}
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/hold-ttl",
		map[string]any{"holdTtlMs": value}, "")
}

// RetrieveReport fetches the event report.
func (s *EventsService) RetrieveReport(ctx context.Context, eventKey string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/report", nil)
}

// EventLogListParams pages an event audit log newest first.
type EventLogListParams struct {
	Limit  int
	Before int64
}

// RetrieveLog fetches one page of the event audit log.
func (s *EventsService) RetrieveLog(
	ctx context.Context, eventKey string, options ...EventLogListParams,
) (EventLogPage, error) {
	if len(options) > 1 {
		return EventLogPage{}, errors.New("seatlayer: RetrieveLog accepts at most one params value")
	}
	query := url.Values{}
	if len(options) == 1 {
		if options[0].Limit != 0 {
			query.Set("limit", strconv.Itoa(options[0].Limit))
		}
		if options[0].Before != 0 {
			query.Set("before", strconv.FormatInt(options[0].Before, 10))
		}
	}
	response, err := s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/log", query)
	if err != nil {
		return EventLogPage{}, err
	}
	return decodeResponse[EventLogPage](response)
}

func int64OrNil(value int64) any {
	if value == 0 {
		return nil
	}
	return value
}
