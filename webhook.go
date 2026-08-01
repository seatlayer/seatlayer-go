package seatlayer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// ErrWebhookVerification means the delivery did not come from SeatLayer.
// Respond 400 and do not process it.
var ErrWebhookVerification = errors.New("seatlayer: webhook verification failed")

// VerifyWebhook checks a delivery's signature and returns its decoded payload.
//
// payload must be the RAW request body — in net/http that is io.ReadAll(r.Body)
// before any decoding. Re-serialising a decoded body reorders keys and changes
// whitespace, so verification fails; the usual "fix" for that is to disable
// verification, which is why this takes bytes and does the work for you.
//
// Errors wrap ErrWebhookVerification, so callers can test with errors.Is.
//
// NOTE ON REPLAY: deliveries are signed over the body, which carries an "at"
// timestamp — but nothing enforces a freshness window, so a captured delivery
// stays valid indefinitely. Replay protection is yours: every event carries an
// occurrenceId, and the correct pattern is to record processed ids and ignore
// repeats. Do not skip this.
func VerifyWebhook(payload []byte, signature, secret string) (map[string]any, error) {
	if secret == "" {
		return nil, fmt.Errorf("%w: a signing secret is required", ErrWebhookVerification)
	}
	if signature == "" {
		return nil, fmt.Errorf("%w: missing X-SeatLayer-Signature header", ErrWebhookVerification)
	}

	scheme, provided, found := strings.Cut(signature, "=")
	if !found || scheme != "sha256" || provided == "" {
		return nil, fmt.Errorf(
			"%w: unsupported signature format %q, expected \"sha256=<hex>\"", ErrWebhookVerification, signature)
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	// hmac.Equal is constant time and handles a length mismatch without leaking
	// which of the two failures occurred.
	if !hmac.Equal([]byte(expected), []byte(provided)) {
		return nil, fmt.Errorf("%w: signature did not match", ErrWebhookVerification)
	}

	var event map[string]any
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf(
			"%w: signature verified but the body is not valid JSON: %v", ErrWebhookVerification, err)
	}
	return event, nil
}
