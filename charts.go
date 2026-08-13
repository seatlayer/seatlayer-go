package seatlayer

import (
	"context"
	"errors"
	"iter"
	"net/url"
	"strconv"
)

// ChartsService covers seat-map definitions that events are created from.
//
// Even when organisers draw their own venues in the embedded Designer you need
// this: CreateDesignerSession requires a chart id that already exists, so the
// usual platform flow is copy a template here, then hand over a session.
type ChartsService struct{ client *Client }

// ChartListParams filters and pages a chart listing.
type ChartListParams struct {
	WorkspaceID string
	ExternalRef string
	Archived    bool
	// Limit is the page size. Clamped server-side; asking for more is not an error.
	Limit int
	// Cursor continues a previous page. Leave empty to start.
	Cursor string
}

func (p *ChartListParams) query() url.Values {
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
	if p.Archived {
		values.Set("archived", "1")
	}
	return values
}

// List returns one page of charts.
func (s *ChartsService) List(ctx context.Context, p *ChartListParams) (Page, error) {
	response, err := s.client.get(ctx, "/v1/charts", p.query())
	if err != nil {
		return Page{}, err
	}
	return pageFrom(response, "charts"), nil
}

// All walks every chart, paging transparently.
//
//	for chart, err := range client.Charts.All(ctx, nil) { ... }
func (s *ChartsService) All(ctx context.Context, p *ChartListParams) iter.Seq2[map[string]any, error] {
	return paginate(ctx, func(ctx context.Context, cursor string) (Page, error) {
		next := ChartListParams{}
		if p != nil {
			next = *p
		}
		next.Cursor = cursor
		return s.List(ctx, &next)
	})
}

// ChartCreateParams describes a new chart.
type ChartCreateParams struct {
	Name        string
	Doc         map[string]any
	ExternalRef string
	WorkspaceID string
	// IdempotencyKey makes a retried create collapse into the original.
	IdempotencyKey string
}

// Create makes a chart. Pass Doc to import an existing document.
func (s *ChartsService) Create(ctx context.Context, p ChartCreateParams) (map[string]any, error) {
	body := params(
		"name", p.Name,
		"doc", mapOrNil(p.Doc),
		"externalRef", stringOrNil(p.ExternalRef),
		"workspaceId", stringOrNil(p.WorkspaceID),
	)
	return s.client.postHeaderReplay(ctx, "/v1/charts", body, p.IdempotencyKey)
}

// Retrieve fetches a chart and its document.
func (s *ChartsService) Retrieve(ctx context.Context, chartID string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/charts/"+escape(chartID), nil)
}

// ChartUpdateParams supports both the optimistic document-replacement branch
// and the metadata-only branch of the chart update contract.
type ChartUpdateParams struct {
	Doc               map[string]any
	ExpectedUpdatedAt int64
	Name              string
	Issues            *float64
	ExternalRef       NullableField[string]
}

// Update changes a document and/or chart metadata. ExpectedUpdatedAt is
// required whenever Doc is supplied so concurrent writers cannot silently
// overwrite each other. ExternalRef can be FieldNull[string]() to clear it.
func (s *ChartsService) Update(
	ctx context.Context, chartID string, p ChartUpdateParams,
) (map[string]any, error) {
	body := params(
		"name", stringOrNil(p.Name),
		"issues", p.Issues,
	)
	if p.Doc != nil {
		body["doc"] = p.Doc
		body["expectedUpdatedAt"] = p.ExpectedUpdatedAt
	}
	if value, present := p.ExternalRef.requestValue(); present {
		body["externalRef"] = value
	}
	return s.client.put(ctx, "/v1/charts/"+escape(chartID), body)
}

// Delete removes a chart.
func (s *ChartsService) Delete(ctx context.Context, chartID string) error {
	_, err := s.client.delete(ctx, "/v1/charts/"+escape(chartID))
	return err
}

// ChartCopyParams overrides fields inherited from the source chart.
type ChartCopyParams struct {
	Name           string
	ExternalRef    NullableField[string]
	WorkspaceID    string
	IdempotencyKey string
}

// Copy duplicates a chart — the usual way to provision a venue from a template.
func (s *ChartsService) Copy(
	ctx context.Context, chartID string, options ...ChartCopyParams,
) (map[string]any, error) {
	if len(options) > 1 {
		return nil, errors.New("seatlayer: Copy accepts at most one params value")
	}
	var body map[string]any
	var idempotencyKey string
	if len(options) == 1 {
		p := options[0]
		body = params(
			"name", stringOrNil(p.Name),
			"workspaceId", stringOrNil(p.WorkspaceID),
		)
		if value, present := p.ExternalRef.requestValue(); present {
			body["externalRef"] = value
		}
		if len(body) == 0 {
			body = nil
		}
		idempotencyKey = p.IdempotencyKey
	}
	return s.client.postHeaderReplay(ctx, "/v1/charts/"+escape(chartID)+"/duplicate",
		body, idempotencyKey)
}

// Archive moves a chart to the archive.
func (s *ChartsService) Archive(ctx context.Context, chartID string) (map[string]any, error) {
	return s.client.post(ctx, "/v1/charts/"+escape(chartID)+"/archive", nil, "")
}

// Unarchive restores a chart from the archive.
func (s *ChartsService) Unarchive(ctx context.Context, chartID string) (map[string]any, error) {
	return s.client.post(ctx, "/v1/charts/"+escape(chartID)+"/unarchive", nil, "")
}

// Publish publishes the draft. Events can only be created from a published chart.
func (s *ChartsService) Publish(ctx context.Context, chartID string) (map[string]any, error) {
	return s.client.post(ctx, "/v1/charts/"+escape(chartID)+"/publish", nil, "")
}

func setIfNotEmpty(values url.Values, key, value string) {
	if value != "" {
		values.Set(key, value)
	}
}

// stringOrNil keeps an empty optional string out of the request body entirely,
// rather than sending "".
func stringOrNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func mapOrNil(value map[string]any) any {
	if len(value) == 0 {
		return nil
	}
	return value
}

func sliceOrNil(value []string) any {
	if len(value) == 0 {
		return nil
	}
	return value
}
