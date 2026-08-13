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

// ManageSessionParams scopes a control-room token.
type ManageSessionParams struct {
	// AllowedOrigin is the https origin the token is bound to.
	AllowedOrigin string
	// Capabilities is required. See CreateManageSession for why.
	Capabilities []ManageCapability
	// ExpiresInSeconds is 300–14400. Defaults to 3600 server-side.
	ExpiresInSeconds int
	// WorkspaceID optionally confirms the event belongs to this workspace.
	WorkspaceID string
}

// CreateManageSession mints a manage-session token for the control room.
//
// The raw API defaults an omitted list to view-only (event:view). This SDK
// still requires an explicit set so browser authority remains visible at every
// call site.
func (s *SessionsService) CreateManageSession(
	ctx context.Context, eventKey string, p ManageSessionParams,
) (ManageSession, error) {
	if len(p.Capabilities) == 0 {
		return ManageSession{}, errors.New(
			"seatlayer: Capabilities is required: pass the smallest set the page needs, " +
				"e.g. []seatlayer.ManageCapability{seatlayer.CapabilityView}")
	}
	body := params(
		"allowedOrigin", p.AllowedOrigin,
		"capabilities", p.Capabilities,
		"expiresInSeconds", intOrNil(p.ExpiresInSeconds),
		"workspaceId", stringOrNil(p.WorkspaceID),
	)
	response, err := s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/manage-sessions", body, "")
	if err != nil {
		return ManageSession{}, err
	}
	return decodeResponse[ManageSession](response)
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
	// CanPublish is the legacy authority flag; when supplied it must agree with
	// Authority. A pointer preserves an explicit false value.
	CanPublish *bool
	// Mode is "normal" or "safe".
	Mode string
	// SafeModeOptions is accepted only when Mode is "safe".
	SafeModeOptions *DesignerSafeModeOptionsParams
	// Features carries the Designer feature-policy object.
	Features         map[string]any
	ExpiresInSeconds int
}

// CreateDesignerSession mints a token so an organiser can edit a chart inside
// your own UI.
func (s *SessionsService) CreateDesignerSession(
	ctx context.Context, p DesignerSessionParams,
) (DesignerSessionEnvelope, error) {
	body := params(
		"workspaceId", p.WorkspaceID,
		"chartId", p.ChartID,
		"allowedOrigin", p.AllowedOrigin,
		"authority", stringOrNil(p.Authority),
		"canPublish", p.CanPublish,
		"mode", stringOrNil(p.Mode),
		"safeModeOptions", p.SafeModeOptions,
		"features", p.Features,
		"expiresInSeconds", intOrNil(p.ExpiresInSeconds),
	)
	response, err := s.client.post(ctx, "/v1/designer/sessions", body, "")
	if err != nil {
		return DesignerSessionEnvelope{}, err
	}
	return decodeResponse[DesignerSessionEnvelope](response)
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
