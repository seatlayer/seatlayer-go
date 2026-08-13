package seatlayer

import (
	"context"
	"errors"
	"net/url"
	"strconv"
)

// WebhooksService manages webhook subscriptions. To VERIFY a delivery, see
// VerifyWebhook.
type WebhooksService struct{ client *Client }

// List returns the webhook subscriptions.
func (s *WebhooksService) List(ctx context.Context) (WebhookList, error) {
	response, err := s.client.get(ctx, "/v1/webhooks", nil)
	if err != nil {
		return WebhookList{}, err
	}
	return decodeResponse[WebhookList](response)
}

// Create registers a subscription. The response carries the signing secret once.
func (s *WebhooksService) Create(
	ctx context.Context, targetURL string, events []WebhookEventName,
) (WebhookCreateEnvelope, error) {
	if err := validateWebhookEvents(events); err != nil {
		return WebhookCreateEnvelope{}, err
	}
	response, err := s.client.post(ctx, "/v1/webhooks",
		params("url", targetURL, "events", events), "")
	if err != nil {
		return WebhookCreateEnvelope{}, err
	}
	return decodeResponse[WebhookCreateEnvelope](response)
}

// WebhookUpdateParams describes the only mutable subscription fields.
type WebhookUpdateParams struct {
	URL      string
	Events   []WebhookEventName
	Disabled *bool
}

// Update changes a subscription.
func (s *WebhooksService) Update(
	ctx context.Context, webhookID string, p WebhookUpdateParams,
) (WebhookEnvelope, error) {
	if p.Events != nil {
		if err := validateWebhookEvents(p.Events); err != nil {
			return WebhookEnvelope{}, err
		}
	}
	response, err := s.client.patch(ctx, "/v1/webhooks/"+escape(webhookID), params(
		"url", stringOrNil(p.URL),
		"events", webhookEventsOrNil(p.Events),
		"disabled", p.Disabled,
	))
	if err != nil {
		return WebhookEnvelope{}, err
	}
	return decodeResponse[WebhookEnvelope](response)
}

// Delete removes a subscription.
func (s *WebhooksService) Delete(ctx context.Context, webhookID string) error {
	_, err := s.client.delete(ctx, "/v1/webhooks/"+escape(webhookID))
	return err
}

// WebhookDeliveryListParams filters and pages delivery attempts.
type WebhookDeliveryListParams struct {
	Limit  int
	Status WebhookDeliveryStatus
	Before int64
}

// ListDeliveries returns recent delivery attempts for a subscription. The
// optional params value preserves the original no-filter call shape.
func (s *WebhooksService) ListDeliveries(
	ctx context.Context, webhookID string, options ...WebhookDeliveryListParams,
) (WebhookDeliveryPage, error) {
	if len(options) > 1 {
		return WebhookDeliveryPage{}, errors.New(
			"seatlayer: ListDeliveries accepts at most one params value")
	}
	p := WebhookDeliveryListParams{}
	if len(options) == 1 {
		p = options[0]
	}
	query := url.Values{}
	if p.Limit != 0 {
		query.Set("limit", strconv.Itoa(p.Limit))
	}
	if p.Status != "" {
		if p.Status != WebhookDeliveryOK && p.Status != WebhookDeliveryFailed {
			return WebhookDeliveryPage{}, errors.New(
				"seatlayer: webhook delivery status must be ok or failed")
		}
		query.Set("status", string(p.Status))
	}
	if p.Before != 0 {
		query.Set("before", strconv.FormatInt(p.Before, 10))
	}
	response, err := s.client.get(ctx, "/v1/webhooks/"+escape(webhookID)+"/deliveries", query)
	if err != nil {
		return WebhookDeliveryPage{}, err
	}
	return decodeResponse[WebhookDeliveryPage](response)
}

func webhookEventsOrNil(events []WebhookEventName) any {
	if len(events) == 0 {
		return nil
	}
	return events
}

func validateWebhookEvents(events []WebhookEventName) error {
	if len(events) == 0 {
		return errors.New("seatlayer: events must contain supported webhook event names")
	}
	for _, event := range events {
		switch event {
		case WebhookEventSeatBooked, WebhookEventSeatReleased, WebhookEventSeatBlocked,
			WebhookEventHoldExpired, WebhookEventHoldCreated, WebhookEventHoldExtended,
			WebhookEventEventCreated, WebhookEventEventSoldOut:
			continue
		default:
			return errors.New("seatlayer: events must contain supported webhook event names")
		}
	}
	return nil
}
