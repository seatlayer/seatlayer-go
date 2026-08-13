package seatlayer

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// ChannelsService manages private allocations, reporting, and buyer access.
type ChannelsService struct{ client *Client }

func (s *ChannelsService) path(eventKey, suffix string) string {
	return "/v1/events/" + escape(eventKey) + "/channels" + suffix
}

// ListChannels lists an event's allocation channels.
func (s *ChannelsService) ListChannels(
	ctx context.Context, eventKey string, includeArchived bool,
) (map[string]any, error) {
	query := url.Values{}
	if includeArchived {
		query.Set("includeArchived", "1")
	}
	return s.client.get(ctx, s.path(eventKey, ""), query)
}

// ChannelCreateParams defines a private allocation channel.
type ChannelCreateParams struct {
	Name           string
	Color          string
	Marker         string
	ExternalRef    string
	AccessIntent   string
	Reason         string
	IdempotencyKey string
}

// CreateChannel creates a private allocation channel.
func (s *ChannelsService) CreateChannel(
	ctx context.Context, eventKey string, p ChannelCreateParams,
) (map[string]any, error) {
	body := params(
		"name", p.Name,
		"color", stringOrNil(p.Color),
		"marker", stringOrNil(p.Marker),
		"externalRef", stringOrNil(p.ExternalRef),
		"accessIntent", stringOrNil(p.AccessIntent),
		"reason", stringOrNil(p.Reason),
	)
	return s.client.post(ctx, s.path(eventKey, ""), body, p.IdempotencyKey)
}

// ChannelUpdateParams defines mutable channel fields.
type ChannelUpdateParams struct {
	Name                  string
	AccessIntent          string
	AcknowledgeLiveAccess *bool
	Reason                string
}

// UpdateChannel updates a private allocation channel.
func (s *ChannelsService) UpdateChannel(
	ctx context.Context, eventKey, channelID string, p ChannelUpdateParams,
) (map[string]any, error) {
	body := params(
		"name", stringOrNil(p.Name),
		"accessIntent", stringOrNil(p.AccessIntent),
		"acknowledgeLiveAccess", p.AcknowledgeLiveAccess,
		"reason", stringOrNil(p.Reason),
	)
	return s.client.patch(ctx, s.path(eventKey, "/"+escape(channelID)), body)
}

// ChannelAssignmentParams moves inventory between public and private allocation.
type ChannelAssignmentParams struct {
	Labels            []string
	AssignmentVersion int64
	TargetChannelID   string
	Reason            string
	IdempotencyKey    string
}

// UpdateAssignments changes the allocation owner of inventory labels.
func (s *ChannelsService) UpdateAssignments(
	ctx context.Context, eventKey string, p ChannelAssignmentParams,
) (map[string]any, error) {
	body := params(
		"labels", p.Labels,
		"assignmentVersion", p.AssignmentVersion,
		"reason", stringOrNil(p.Reason),
	)
	body["targetChannelId"] = stringOrNil(p.TargetChannelID)
	return s.client.post(ctx, s.path(eventKey, "/assignments"), body, p.IdempotencyKey)
}

// ListAllocation returns the current allocation ledger.
func (s *ChannelsService) ListAllocation(
	ctx context.Context, eventKey, afterLabel string, limit int,
) (map[string]any, error) {
	query := url.Values{}
	if afterLabel != "" {
		query.Set("afterLabel", afterLabel)
	}
	if limit != 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	return s.client.get(ctx, s.path(eventKey, "/allocation"), query)
}

// RetrieveAccessPreview shows the inventory visible to a buyer access scope.
func (s *ChannelsService) RetrieveAccessPreview(
	ctx context.Context, eventKey string, channelIDs []string, includePublic *bool,
) (map[string]any, error) {
	query := url.Values{}
	if len(channelIDs) > 0 {
		query.Set("channelIds", strings.Join(channelIDs, ","))
	}
	if includePublic != nil {
		value := "0"
		if *includePublic {
			value = "1"
		}
		query.Set("includePublic", value)
	}
	return s.client.get(ctx, s.path(eventKey, "/preview"), query)
}

// RetrieveReport returns channel allocation totals.
func (s *ChannelsService) RetrieveReport(
	ctx context.Context, eventKey string,
) (map[string]any, error) {
	return s.client.get(ctx, s.path(eventKey, "/report"), nil)
}

// Pause temporarily disables a channel.
func (s *ChannelsService) Pause(
	ctx context.Context, eventKey, channelID, reason string,
) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/"+escape(channelID)+"/pause"),
		params("reason", stringOrNil(reason)), "")
}

// Unpause restores a paused channel.
func (s *ChannelsService) Unpause(
	ctx context.Context, eventKey, channelID, reason string,
) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/"+escape(channelID)+"/unpause"),
		params("reason", stringOrNil(reason)), "")
}

// ChannelArchiveParams permits the contract's explicit null destination.
type ChannelArchiveParams struct {
	Destination NullableField[string]
	Reason      string
}

// Archive retires a channel with an explicit destination value.
func (s *ChannelsService) Archive(
	ctx context.Context, eventKey, channelID string, p ChannelArchiveParams,
) (map[string]any, error) {
	destination, present := p.Destination.requestValue()
	if !present {
		return nil, errors.New("seatlayer: archive destination is required; use FieldNull[string]() for null")
	}
	body := params("reason", stringOrNil(p.Reason))
	body["destination"] = destination
	return s.client.post(ctx, s.path(eventKey, "/"+escape(channelID)+"/archive"), body, "")
}

// BuyerAccessSessionParams defines the security boundary of a buyer token.
type BuyerAccessSessionParams struct {
	ChannelIDs       []string
	IncludePublic    bool
	AllowedOrigin    string
	ExpiresInSeconds int
	MaxQuantity      int
	BuyerRef         string
	PartnerRef       string
	ClientRequestID  string
	IdempotencyKey   string
	// Nullable overrides fields when explicit JSON null is different from omission.
	Nullable BuyerAccessSessionNullableFields
}

// CreateBuyerAccessSession mints a short-lived, origin-bound buyer token.
func (s *ChannelsService) CreateBuyerAccessSession(
	ctx context.Context, eventKey string, p BuyerAccessSessionParams,
) (map[string]any, error) {
	body := params(
		"channelIds", sliceOrNil(p.ChannelIDs),
		"includePublic", p.IncludePublic,
		"allowedOrigin", p.AllowedOrigin,
		"expiresInSeconds", intOrNil(p.ExpiresInSeconds),
		"maxQuantity", intOrNil(p.MaxQuantity),
		"buyerRef", stringOrNil(p.BuyerRef),
		"partnerRef", stringOrNil(p.PartnerRef),
		"clientRequestId", stringOrNil(p.ClientRequestID),
	)
	p.Nullable.apply(body)
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/buyer-access-sessions",
		body, p.IdempotencyKey)
}

// BuyerAccessSessionListParams limits the buyer access-session projection.
type BuyerAccessSessionListParams struct {
	Limit int
}

// ListBuyerAccessSessions returns one page of buyer access sessions.
func (s *ChannelsService) ListBuyerAccessSessions(
	ctx context.Context, eventKey string, p BuyerAccessSessionListParams,
) (map[string]any, error) {
	query := url.Values{}
	if p.Limit != 0 {
		query.Set("limit", strconv.Itoa(p.Limit))
	}
	return s.client.get(ctx, "/v1/events/"+escape(eventKey)+"/buyer-access-sessions", query)
}

// RevokeBuyerAccessSession revokes a buyer token before it expires.
func (s *ChannelsService) RevokeBuyerAccessSession(
	ctx context.Context, eventKey, sessionID string,
) (map[string]any, error) {
	return s.client.delete(ctx, "/v1/events/"+escape(eventKey)+
		"/buyer-access-sessions/"+escape(sessionID))
}

// AccessLinkCreateParams configures a hosted link. The response capability is
// revealed once and the operation is never automatically retried.
type AccessLinkCreateParams struct {
	Label             NullableField[string]
	ExpiresAt         int64
	MaxRedemptions    int
	MaxQuantity       int
	SessionTTLSeconds int
	IncludePublic     *bool
	Reason            string
	IdempotencyKey    string
}

// CreateAccessLink mints a hosted link and its one-time capability.
func (s *ChannelsService) CreateAccessLink(
	ctx context.Context, eventKey, channelID string, p AccessLinkCreateParams,
) (AccessLinkReveal, error) {
	body := params(
		"expiresAt", int64OrNil(p.ExpiresAt),
		"maxRedemptions", intOrNil(p.MaxRedemptions),
		"maxQuantity", intOrNil(p.MaxQuantity),
		"sessionTtlSeconds", intOrNil(p.SessionTTLSeconds),
		"includePublic", p.IncludePublic,
		"reason", stringOrNil(p.Reason),
	)
	if value, present := p.Label.requestValue(); present {
		body["label"] = value
	}
	response, err := s.client.post(ctx,
		s.path(eventKey, "/"+escape(channelID)+"/access-links"), body, p.IdempotencyKey)
	if err != nil {
		return AccessLinkReveal{}, err
	}
	return decodeResponse[AccessLinkReveal](response)
}

// ListAccessLinks returns status only, never a stored capability.
func (s *ChannelsService) ListAccessLinks(
	ctx context.Context, eventKey, channelID string,
) (AccessLinkList, error) {
	response, err := s.client.get(ctx,
		s.path(eventKey, "/"+escape(channelID)+"/access-links"), nil)
	if err != nil {
		return AccessLinkList{}, err
	}
	return decodeResponse[AccessLinkList](response)
}

// AccessLinkRotateParams requires the caller to choose whether old sessions end.
type AccessLinkRotateParams struct {
	EndActiveSessions bool
	Reason            string
}

// RotateAccessLink replaces a hosted link and reveals the successor once.
func (s *ChannelsService) RotateAccessLink(
	ctx context.Context, eventKey, channelID, linkID string, p AccessLinkRotateParams,
) (AccessLinkReveal, error) {
	body := params(
		"endActiveSessions", p.EndActiveSessions,
		"reason", stringOrNil(p.Reason),
	)
	response, err := s.client.post(ctx, s.path(eventKey, "/"+escape(channelID)+
		"/access-links/"+escape(linkID)+"/rotate"), body, "")
	if err != nil {
		return AccessLinkReveal{}, err
	}
	return decodeResponse[AccessLinkReveal](response)
}

// AccessLinkRevokeParams controls optional session cascade and audit reason.
type AccessLinkRevokeParams struct {
	EndActiveSessions bool
	Reason            string
}

// RevokeAccessLink stops a hosted link from admitting new buyers.
func (s *ChannelsService) RevokeAccessLink(
	ctx context.Context, eventKey, channelID, linkID string,
	options ...AccessLinkRevokeParams,
) (AccessLinkRevokeResult, error) {
	if len(options) > 1 {
		return AccessLinkRevokeResult{}, errors.New(
			"seatlayer: RevokeAccessLink accepts at most one params value")
	}
	query := url.Values{}
	if len(options) == 1 {
		if options[0].EndActiveSessions {
			query.Set("endActiveSessions", "1")
		}
		setIfNotEmpty(query, "reason", options[0].Reason)
	}
	response, err := s.client.deleteQuery(ctx, s.path(eventKey, "/"+escape(channelID)+
		"/access-links/"+escape(linkID)), query)
	if err != nil {
		return AccessLinkRevokeResult{}, err
	}
	return decodeResponse[AccessLinkRevokeResult](response)
}
