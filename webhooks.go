package seatlayer

import "context"

// WebhooksService manages webhook subscriptions. To VERIFY a delivery, see
// VerifyWebhook.
type WebhooksService struct{ client *Client }

// List returns the webhook subscriptions.
func (s *WebhooksService) List(ctx context.Context) (map[string]any, error) {
	return s.client.get(ctx, "/v1/webhooks", nil)
}

// Create registers a subscription. The response carries the signing secret once.
func (s *WebhooksService) Create(ctx context.Context, targetURL string, events []string) (map[string]any, error) {
	return s.client.post(ctx, "/v1/webhooks", params("url", targetURL, "events", events), "")
}

// Update changes a subscription.
func (s *WebhooksService) Update(ctx context.Context, webhookID string, fields map[string]any) (map[string]any, error) {
	return s.client.patch(ctx, "/v1/webhooks/"+escape(webhookID), fields)
}

// Delete removes a subscription.
func (s *WebhooksService) Delete(ctx context.Context, webhookID string) error {
	_, err := s.client.delete(ctx, "/v1/webhooks/"+escape(webhookID))
	return err
}

// ListDeliveries returns recent delivery attempts for a subscription.
func (s *WebhooksService) ListDeliveries(ctx context.Context, webhookID string) (map[string]any, error) {
	return s.client.get(ctx, "/v1/webhooks/"+escape(webhookID)+"/deliveries", nil)
}
