package seatlayer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

const testSecret = "whsec_test"

func sign(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func TestVerifyWebhookAcceptsSignedDelivery(t *testing.T) {
	payload := `{"type":"booking.created","occurrenceId":"occ_1"}`
	event, err := VerifyWebhook([]byte(payload), sign(payload, testSecret), testSecret)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if event["type"] != "booking.created" {
		t.Fatalf("event = %v", event)
	}
}

func TestVerifyWebhookRejectsReserialisedBody(t *testing.T) {
	// The classic integration bug: decoding the body and re-encoding it changes
	// the bytes, so the signature no longer matches.
	//
	// The mechanism is Go-specific. encoding/json marshals a map with its keys
	// SORTED, while a real delivery arrives in the order we serialised it —
	// deliveryId, occurrenceId, event, at. Round-tripping therefore reorders it.
	// (An alphabetical payload would survive the round trip by coincidence, which
	// is exactly why this test uses a realistic delivery shape instead.)
	original := `{"deliveryId":"d_1","occurrenceId":"occ_1","event":"booking.created","at":1754006400000}`

	var decoded map[string]any
	if err := json.Unmarshal([]byte(original), &decoded); err != nil {
		t.Fatalf("setup: %v", err)
	}
	reserialised, _ := json.Marshal(decoded)
	if string(reserialised) == original {
		t.Fatal("setup: re-serialising did not change the bytes, so this test proves nothing")
	}

	_, err := VerifyWebhook(reserialised, sign(original, testSecret), testSecret)
	if !errors.Is(err, ErrWebhookVerification) {
		t.Fatalf("want ErrWebhookVerification, got %v", err)
	}
}

func TestVerifyWebhookRejectsWrongSecret(t *testing.T) {
	payload := `{"ok":true}`
	_, err := VerifyWebhook([]byte(payload), sign(payload, "whsec_other"), testSecret)
	if !errors.Is(err, ErrWebhookVerification) || !strings.Contains(err.Error(), "did not match") {
		t.Fatalf("want a signature mismatch, got %v", err)
	}
}

func TestVerifyWebhookRejectsMissingHeader(t *testing.T) {
	_, err := VerifyWebhook([]byte(`{}`), "", testSecret)
	if !errors.Is(err, ErrWebhookVerification) {
		t.Fatalf("want ErrWebhookVerification, got %v", err)
	}
}

func TestVerifyWebhookRejectsUnknownScheme(t *testing.T) {
	_, err := VerifyWebhook([]byte(`{}`), "md5=abc", testSecret)
	if !strings.Contains(err.Error(), "unsupported signature format") {
		t.Fatalf("want an unsupported-format error, got %v", err)
	}
}

func TestVerifyWebhookRejectsTruncatedSignature(t *testing.T) {
	payload := `{"ok":true}`
	truncated := sign(payload, testSecret)[:20]
	if _, err := VerifyWebhook([]byte(payload), truncated, testSecret); !errors.Is(err, ErrWebhookVerification) {
		t.Fatalf("want ErrWebhookVerification, got %v", err)
	}
}

func TestVerifyWebhookRequiresSecret(t *testing.T) {
	payload := `{}`
	_, err := VerifyWebhook([]byte(payload), sign(payload, testSecret), "")
	if !strings.Contains(err.Error(), "signing secret is required") {
		t.Fatalf("want a missing-secret error, got %v", err)
	}
}

func TestVerifyWebhookReportsUnparseableBodyDistinctly(t *testing.T) {
	payload := `not json`
	_, err := VerifyWebhook([]byte(payload), sign(payload, testSecret), testSecret)
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Fatalf("want a JSON error distinct from a signature failure, got %v", err)
	}
}

func TestVerifyWebhookHandlesNonASCII(t *testing.T) {
	// A venue named "Théâtre" must verify — this is where a string/byte mismatch
	// in the HMAC would show up.
	payload := `{"venue":"Théâtre du Châtelet — 日本"}`
	event, err := VerifyWebhook([]byte(payload), sign(payload, testSecret), testSecret)
	if err != nil {
		t.Fatalf("VerifyWebhook: %v", err)
	}
	if event["venue"] != "Théâtre du Châtelet — 日本" {
		t.Fatalf("venue = %v", event["venue"])
	}
}
