package seatlayer

import "context"

// WorkspacesService manages workspaces, which isolate one tenant's charts and
// events from another's.
type WorkspacesService struct{ client *Client }

// List returns the organisation's workspaces.
func (s *WorkspacesService) List(ctx context.Context) (map[string]any, error) {
	return s.client.get(ctx, "/v1/workspaces", nil)
}

// Create provisions a workspace, typically one per tenant.
func (s *WorkspacesService) Create(
	ctx context.Context, name, externalRef, idempotencyKey string,
) (map[string]any, error) {
	return s.client.post(ctx, "/v1/workspaces",
		params("name", name, "externalRef", stringOrNil(externalRef)), idempotencyKey)
}

// Retrieve fetches one workspace.
func (s *WorkspacesService) Retrieve(ctx context.Context, workspaceID string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/workspaces/"+escape(workspaceID), nil)
}

// Update renames, re-references, or disables a workspace.
//
// The organisation's default workspace cannot be disabled — the API answers 409
// default_workspace_required. Promote another one first.
func (s *WorkspacesService) Update(
	ctx context.Context, workspaceID string, fields map[string]any,
) (map[string]any, error) {
	return s.client.patch(ctx, "/v1/workspaces/"+escape(workspaceID), fields)
}
