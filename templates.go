package seatlayer

import (
	"context"
	"errors"
)

// TemplatesService materializes published catalog templates as draft charts.
type TemplatesService struct{ client *Client }

// TemplateInstantiateParams optionally pins a catalog snapshot or overrides
// the draft that the API creates. EditedDoc is sent whenever it is non-nil,
// including an empty object.
type TemplateInstantiateParams struct {
	Name           string
	WorkspaceID    string
	EditedDoc      map[string]any
	Version        int
	SHA256         string
	IdempotencyKey string
}

// InstantiateTemplate materializes a published catalog template as a draft
// chart. The body is always a JSON object, even when no optional parameters are
// supplied. The API supports exact header replay for this operation.
func (s *TemplatesService) InstantiateTemplate(
	ctx context.Context, templateID string, options ...TemplateInstantiateParams,
) (map[string]any, error) {
	if len(options) > 1 {
		return nil, errors.New("seatlayer: InstantiateTemplate accepts at most one params value")
	}
	p := TemplateInstantiateParams{}
	if len(options) == 1 {
		p = options[0]
	}
	body := params(
		"name", stringOrNil(p.Name),
		"workspaceId", stringOrNil(p.WorkspaceID),
		"version", intOrNil(p.Version),
		"sha256", stringOrNil(p.SHA256),
	)
	if p.EditedDoc != nil {
		body["editedDoc"] = p.EditedDoc
	}
	return s.client.postHeaderReplay(
		ctx, "/v1/templates/"+escape(templateID)+"/instantiate", body, p.IdempotencyKey,
	)
}
