package seatlayer

// NullableField distinguishes an omitted request field from an explicit JSON
// null. Construct one with FieldValue or FieldNull; its internals stay private
// so the three states cannot be assembled inconsistently.
type NullableField[T any] struct {
	value T
	set   bool
	null  bool
}

// FieldValue includes value in a nullable request field.
func FieldValue[T any](value T) NullableField[T] {
	return NullableField[T]{value: value, set: true}
}

// FieldNull includes an explicit JSON null in a nullable request field.
func FieldNull[T any]() NullableField[T] {
	return NullableField[T]{set: true, null: true}
}

func (f NullableField[T]) requestValue() (any, bool) {
	if !f.set {
		return nil, false
	}
	if f.null {
		return nil, true
	}
	return f.value, true
}

// EventCreateNullableFields optionally overrides the convenience scalar fields
// on EventCreateParams when a caller must send explicit JSON null.
type EventCreateNullableFields struct {
	StartsAt      NullableField[int64]
	Venue         NullableField[string]
	ExternalRef   NullableField[string]
	Currency      NullableField[string]
	Description   NullableField[string]
	EndsAt        NullableField[int64]
	Timezone      NullableField[string]
	Locale        NullableField[string]
	PosterAssetID NullableField[string]
}

func (p EventCreateNullableFields) apply(body map[string]any) {
	fields := []struct {
		key     string
		value   any
		present bool
	}{
		fieldRequestValue("startsAt", p.StartsAt),
		fieldRequestValue("venue", p.Venue),
		fieldRequestValue("externalRef", p.ExternalRef),
		fieldRequestValue("currency", p.Currency),
		fieldRequestValue("description", p.Description),
		fieldRequestValue("endsAt", p.EndsAt),
		fieldRequestValue("timezone", p.Timezone),
		fieldRequestValue("locale", p.Locale),
		fieldRequestValue("posterAssetId", p.PosterAssetID),
	}
	for _, field := range fields {
		if field.present {
			body[field.key] = field.value
		}
	}
}

// BuyerAccessSessionNullableFields sends an explicit null for optional buyer
// metadata instead of silently applying the API default.
type BuyerAccessSessionNullableFields struct {
	MaxQuantity     NullableField[int]
	BuyerRef        NullableField[string]
	PartnerRef      NullableField[string]
	ClientRequestID NullableField[string]
}

func (p BuyerAccessSessionNullableFields) apply(body map[string]any) {
	fields := []struct {
		key     string
		value   any
		present bool
	}{
		fieldRequestValue("maxQuantity", p.MaxQuantity),
		fieldRequestValue("buyerRef", p.BuyerRef),
		fieldRequestValue("partnerRef", p.PartnerRef),
		fieldRequestValue("clientRequestId", p.ClientRequestID),
	}
	for _, field := range fields {
		if field.present {
			body[field.key] = field.value
		}
	}
}

func fieldRequestValue[T any](key string, field NullableField[T]) struct {
	key     string
	value   any
	present bool
} {
	value, present := field.requestValue()
	return struct {
		key     string
		value   any
		present bool
	}{key: key, value: value, present: present}
}

// WebhookEventName is one of the event names accepted by webhook create/update.
type WebhookEventName string

// WebhookDeliveryStatus filters webhook delivery attempts.
type WebhookDeliveryStatus string

const (
	WebhookEventSeatBooked   WebhookEventName = "seat.booked"
	WebhookEventSeatReleased WebhookEventName = "seat.released"
	WebhookEventSeatBlocked  WebhookEventName = "seat.blocked"
	WebhookEventHoldExpired  WebhookEventName = "hold.expired"
	WebhookEventHoldCreated  WebhookEventName = "hold.created"
	WebhookEventHoldExtended WebhookEventName = "hold.extended"
	WebhookEventEventCreated WebhookEventName = "event.created"
	WebhookEventEventSoldOut WebhookEventName = "event.soldout"
)

const (
	WebhookDeliveryOK     WebhookDeliveryStatus = "ok"
	WebhookDeliveryFailed WebhookDeliveryStatus = "failed"
)

// ManageCapability is browser authority carried by a manage-session token.
type ManageCapability string

const (
	CapabilityView           ManageCapability = "event:view"
	CapabilityBlock          ManageCapability = "event:block"
	CapabilityCancel         ManageCapability = "event:cancel"
	CapabilityReports        ManageCapability = "event:reports"
	CapabilityChannelsView   ManageCapability = "event:channels:view"
	CapabilityChannelsManage ManageCapability = "event:channels:manage"
	CapabilityOrdersRead     ManageCapability = "event:orders:read"
	CapabilityRefund         ManageCapability = "event:refund"
	CapabilityTicketsSend    ManageCapability = "event:tickets:send"
	CapabilityDoorView       ManageCapability = "event:door:view"
	CapabilityDoorCheckin    ManageCapability = "event:door:checkin"
	CapabilityBoxOffice      ManageCapability = "event:boxoffice"
)

// InventoryItem is the authoritative priced object returned by hold APIs.
type InventoryItem struct {
	Label        string  `json:"label"`
	ObjectID     string  `json:"objectId"`
	ObjectType   string  `json:"objectType"`
	CategoryKey  string  `json:"categoryKey"`
	TierID       *string `json:"tierId"`
	UnitPrice    float64 `json:"unitPrice"`
	Currency     string  `json:"currency"`
	Quantity     *int    `json:"quantity,omitempty"`
	BookingMode  string  `json:"bookingMode,omitempty"`
	Capacity     *int    `json:"capacity,omitempty"`
	MinOccupancy *int    `json:"minOccupancy,omitempty"`
	MaxOccupancy *int    `json:"maxOccupancy,omitempty"`
	ChannelID    *string `json:"channelId,omitempty"`
	AccessSource string  `json:"accessSource,omitempty"`
	ReleaseID    *string `json:"releaseId,omitempty"`
}

// HoldInspection is the secret-key projection returned by RetrieveHold.
type HoldInspection struct {
	HoldID          string          `json:"holdId"`
	Status          string          `json:"status"`
	ExpiresAt       int64           `json:"expiresAt"`
	BookingRef      *string         `json:"bookingRef"`
	EventKey        *string         `json:"eventKey"`
	Mode            string          `json:"mode"`
	ExternalRef     *string         `json:"externalRef"`
	WorkspaceID     *string         `json:"workspaceId"`
	Items           []InventoryItem `json:"items"`
	AccessSessionID *string         `json:"accessSessionId,omitempty"`
	AccessSource    string          `json:"accessSource,omitempty"`
	BuyerRef        *string         `json:"buyerRef,omitempty"`
	PartnerRef      *string         `json:"partnerRef,omitempty"`
}

// WebhookSubscription is the public subscription projection. The signing
// secret is intentionally absent after creation.
type WebhookSubscription struct {
	ID          string             `json:"id"`
	URL         string             `json:"url"`
	Events      []WebhookEventName `json:"events"`
	Disabled    bool               `json:"disabled"`
	LastStatus  *string            `json:"lastStatus"`
	LastAt      *int64             `json:"lastAt"`
	CreatedAt   int64              `json:"createdAt"`
	Mode        *string            `json:"mode"`
	Environment *string            `json:"environment"`
	Uptime7d    *float64           `json:"uptime7d"`
}

type WebhookList struct {
	Subs []WebhookSubscription `json:"subs"`
}

type WebhookCreateEnvelope struct {
	Sub    WebhookSubscription `json:"sub"`
	Secret string              `json:"secret"`
}

type WebhookEnvelope struct {
	Sub WebhookSubscription `json:"sub"`
}

type WebhookDelivery struct {
	ID           string           `json:"id"`
	At           int64            `json:"at"`
	Event        WebhookEventName `json:"event"`
	Ref          *string          `json:"ref"`
	Status       int              `json:"status"`
	Attempt      int              `json:"attempt"`
	MaxAttempts  int              `json:"maxAttempts"`
	WillRetry    bool             `json:"willRetry"`
	OccurrenceID *string          `json:"occurrenceId"`
	Payload      any              `json:"payload"`
	ResponseBody *string          `json:"responseBody"`
	ErrorMessage *string          `json:"errorMessage"`
}

type WebhookDeliveryPage struct {
	Deliveries []WebhookDelivery `json:"deliveries"`
	NextBefore *int64            `json:"nextBefore,omitempty"`
}

type ManageSession struct {
	ID            string             `json:"id"`
	Token         string             `json:"token"`
	ExpiresAt     int64              `json:"expiresAt"`
	EventKey      string             `json:"eventKey"`
	AllowedOrigin string             `json:"allowedOrigin"`
	Capabilities  []ManageCapability `json:"capabilities"`
}

type DesignerSafeModeOptions struct {
	AllowDeletingObjects     bool `json:"allowDeletingObjects"`
	AllowEditingAreaCapacity bool `json:"allowEditingAreaCapacity"`
}

// DesignerSafeModeOptionsParams is partial request input. Pointers preserve
// explicit false values without overriding an omitted server default.
type DesignerSafeModeOptionsParams struct {
	AllowDeletingObjects     *bool `json:"allowDeletingObjects,omitempty"`
	AllowEditingAreaCapacity *bool `json:"allowEditingAreaCapacity,omitempty"`
}

type DesignerSession struct {
	ID              string                  `json:"id"`
	Token           string                  `json:"token"`
	WorkspaceID     string                  `json:"workspaceId"`
	ChartID         string                  `json:"chartId"`
	AllowedOrigin   string                  `json:"allowedOrigin"`
	Authority       string                  `json:"authority"`
	CanEdit         bool                    `json:"canEdit"`
	CanPublish      bool                    `json:"canPublish"`
	Mode            string                  `json:"mode"`
	SafeModeOptions DesignerSafeModeOptions `json:"safeModeOptions"`
	FeaturePolicy   map[string]any          `json:"featurePolicy"`
	ExpiresAt       int64                   `json:"expiresAt"`
	DesignerURL     string                  `json:"designerUrl"`
}

type DesignerSessionEnvelope struct {
	Session DesignerSession `json:"session"`
}

// AccessLinkState is the stored lifecycle state of a hosted link.
type AccessLinkState string

// AccessLinkStatus includes derived expiry and redemption-exhaustion states.
type AccessLinkStatus string

const (
	AccessLinkActive  AccessLinkState = "active"
	AccessLinkRevoked AccessLinkState = "revoked"
	AccessLinkRotated AccessLinkState = "rotated"
)

const (
	AccessLinkStatusActive    AccessLinkStatus = "active"
	AccessLinkStatusRevoked   AccessLinkStatus = "revoked"
	AccessLinkStatusRotated   AccessLinkStatus = "rotated"
	AccessLinkStatusExpired   AccessLinkStatus = "expired"
	AccessLinkStatusExhausted AccessLinkStatus = "exhausted"
)

// AccessLink is the status projection. It never contains the one-time
// capability returned by create and rotate.
type AccessLink struct {
	ID                string           `json:"id"`
	ChannelID         string           `json:"channelId"`
	Label             *string          `json:"label"`
	IncludePublic     bool             `json:"includePublic"`
	ExpiresAt         int64            `json:"expiresAt"`
	MaxRedemptions    int              `json:"maxRedemptions"`
	Redemptions       int              `json:"redemptions"`
	MaxQuantity       int              `json:"maxQuantity"`
	SessionTTLSeconds int              `json:"sessionTtlSeconds"`
	State             AccessLinkState  `json:"state"`
	Status            AccessLinkStatus `json:"status"`
	CreatedAt         int64            `json:"createdAt"`
	CreatedBy         *string          `json:"createdBy"`
	RevokedAt         *int64           `json:"revokedAt"`
	LastRedeemedAt    *int64           `json:"lastRedeemedAt"`
	RotatedFrom       *string          `json:"rotatedFrom"`
	RotatedTo         *string          `json:"rotatedTo"`
}

type AccessLinkListItem struct {
	AccessLink
	ActiveSessions int `json:"activeSessions"`
}

type AccessLinkList struct {
	Links []AccessLinkListItem `json:"links"`
}

// AccessLinkReveal contains a capability that the API will never return again.
type AccessLinkReveal struct {
	Link          AccessLink  `json:"link"`
	URL           string      `json:"url"`
	Capability    string      `json:"capability"`
	RevealedOnce  bool        `json:"revealedOnce"`
	Previous      *AccessLink `json:"previous,omitempty"`
	EndedSessions *int        `json:"endedSessions,omitempty"`
}

type AccessLinkRevokeResult struct {
	OK            bool       `json:"ok"`
	Link          AccessLink `json:"link"`
	EndedSessions int        `json:"endedSessions"`
}

type EventLogEntry struct {
	ID     int64    `json:"id"`
	At     int64    `json:"at"`
	Action string   `json:"action"`
	Labels []string `json:"labels"`
	Ref    *string  `json:"ref"`
}

type EventLogPage struct {
	Entries    []EventLogEntry `json:"entries"`
	NextBefore *int64          `json:"nextBefore"`
}

// TicketRelease is the live response shape. Consumed and Remaining are
// calculated by the service and therefore are not accepted by replacement.
type TicketRelease struct {
	ID            string  `json:"id"`
	Position      int     `json:"position"`
	Name          string  `json:"name"`
	CategoryKey   *string `json:"categoryKey"`
	Price         int     `json:"price"`
	PreviousPrice *int    `json:"previousPrice"`
	Quota         *int    `json:"quota"`
	StartsAt      *int64  `json:"startsAt"`
	EndsAt        *int64  `json:"endsAt"`
	Action        string  `json:"action"`
	ActionURL     *string `json:"actionUrl"`
	SoldOutAt     *int64  `json:"soldOutAt"`
	Consumed      *int    `json:"consumed,omitempty"`
	Remaining     *int    `json:"remaining"`
}

type TicketReleaseList struct {
	Releases []TicketRelease `json:"releases"`
}

// TicketReleaseReplaceInput is the request-only release representation. Use
// FieldNull for an explicit JSON null and FieldValue to send an optional value.
// The service validates the 12-release limit and all release rules.
type TicketReleaseReplaceInput struct {
	ID            NullableField[string]
	Name          string
	CategoryKey   NullableField[string]
	Price         int
	PreviousPrice NullableField[int]
	Quota         NullableField[int]
	StartsAt      NullableField[int64]
	EndsAt        NullableField[int64]
	Action        string
	ActionURL     NullableField[string]
}

func (p TicketReleaseReplaceInput) requestValue() map[string]any {
	body := params("name", p.Name, "price", p.Price, "action", stringOrNil(p.Action))
	fields := []struct {
		key     string
		value   any
		present bool
	}{
		fieldRequestValue("id", p.ID),
		fieldRequestValue("categoryKey", p.CategoryKey),
		fieldRequestValue("previousPrice", p.PreviousPrice),
		fieldRequestValue("quota", p.Quota),
		fieldRequestValue("startsAt", p.StartsAt),
		fieldRequestValue("endsAt", p.EndsAt),
		fieldRequestValue("actionUrl", p.ActionURL),
	}
	for _, field := range fields {
		if field.present {
			body[field.key] = field.value
		}
	}
	return body
}
