package seatlayer

import (
	"context"
	"errors"
)

// SessionsService mints short-lived, origin-bound browser tokens.
//
// The governing rule: the SDK mints tokens, widgets consume them. Your secret
// key never reaches a browser.
type SessionsService struct{ client *Client }

// Capabilities a manage-session token can carry.
const (
	CapabilityView    = "event:view"
	CapabilityBlock   = "event:block"
	CapabilityCancel  = "event:cancel"
	CapabilityReports = "event:reports"
)

// ManageSessionParams scopes a control-room token.
type ManageSessionParams struct {
	// AllowedOrigin is the https origin the token is bound to.
	AllowedOrigin string
	// Capabilities is required. See CreateManageSession for why.
	Capabilities []string
	// ExpiresInSeconds is 300–14400. Defaults to 3600 server-side.
	ExpiresInSeconds int
}

// CreateManageSession mints a manage-session token for the control room.
//
// Capabilities is required here even though the API defaults it. That default
// grants all four — including event:cancel, which un-books paid inventory.
// Granting the ability to reverse sales by forgetting a field is not a default
// worth inheriting.
func (s *SessionsService) CreateManageSession(
	ctx context.Context, eventKey string, p ManageSessionParams,
) (map[string]any, error) {
	if len(p.Capabilities) == 0 {
		return nil, errors.New(
			"seatlayer: Capabilities is required: pass the smallest set the page needs, " +
				"e.g. []string{seatlayer.CapabilityView}; omitting it server-side grants " +
				"event:cancel, which can reverse paid bookings")
	}
	body := params(
		"allowedOrigin", p.AllowedOrigin,
		"capabilities", p.Capabilities,
		"expiresInSeconds", intOrNil(p.ExpiresInSeconds),
	)
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/manage-sessions", body, "")
}

// RevokeManageSession invalidates a manage token before it expires.
func (s *SessionsService) RevokeManageSession(ctx context.Context, eventKey, sessionID string) error {
	_, err := s.client.delete(ctx, "/v1/events/"+escape(eventKey)+"/manage-sessions/"+escape(sessionID))
	return err
}

// DesignerSessionParams scopes an embedded-Designer token.
type DesignerSessionParams struct {
	WorkspaceID string
	// ChartID must already exist — create or copy a chart first.
	ChartID       string
	AllowedOrigin string
	// Authority is "read-only", "edit", or "publish".
	Authority string
	// Mode is "normal" or "safe".
	Mode             string
	ExpiresInSeconds int
}

// CreateDesignerSession mints a token so an organiser can edit a chart inside
// your own UI.
func (s *SessionsService) CreateDesignerSession(
	ctx context.Context, p DesignerSessionParams,
) (map[string]any, error) {
	body := params(
		"workspaceId", p.WorkspaceID,
		"chartId", p.ChartID,
		"allowedOrigin", p.AllowedOrigin,
		"authority", stringOrNil(p.Authority),
		"mode", stringOrNil(p.Mode),
		"expiresInSeconds", intOrNil(p.ExpiresInSeconds),
	)
	return s.client.post(ctx, "/v1/designer/sessions", body, "")
}

// RevokeDesignerSession invalidates a designer token before it expires.
func (s *SessionsService) RevokeDesignerSession(ctx context.Context, sessionID string) error {
	_, err := s.client.delete(ctx, "/v1/designer/sessions/"+escape(sessionID))
	return err
}

func intOrNil(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
