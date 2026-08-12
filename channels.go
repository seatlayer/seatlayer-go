package seatlayer

import (
	"context"
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

// Archive retires a channel and moves its inventory to destination.
func (s *ChannelsService) Archive(
	ctx context.Context, eventKey, channelID, destination, reason string,
) (map[string]any, error) {
	return s.client.post(ctx, s.path(eventKey, "/"+escape(channelID)+"/archive"),
		params("destination", destination, "reason", stringOrNil(reason)), "")
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
	return s.client.post(ctx, "/v1/events/"+escape(eventKey)+"/buyer-access-sessions",
		body, p.IdempotencyKey)
}

// BuyerAccessSessionListParams filters and pages buyer access sessions.
type BuyerAccessSessionListParams struct {
	State  string
	Limit  int
	Cursor string
}

// ListBuyerAccessSessions returns one page of buyer access sessions.
func (s *ChannelsService) ListBuyerAccessSessions(
	ctx context.Context, eventKey string, p BuyerAccessSessionListParams,
) (map[string]any, error) {
	query := url.Values{}
	if p.State != "" {
		query.Set("state", p.State)
	}
	if p.Limit != 0 {
		query.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Cursor != "" {
		query.Set("cursor", p.Cursor)
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
