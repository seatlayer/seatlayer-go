package seatlayer

import (
	"fmt"
)

// APIError is the base error returned for any non-2xx response.
//
// Go has no exception hierarchy, so the pattern here is errors.As against the
// specific types below rather than catch blocks. A sold-out seat is a business
// outcome that belongs in an if, not lumped in with a bad key:
//
//	var conflict *ConflictError
//	if errors.As(err, &conflict) && conflict.SoldOut() {
//		return offerAlternativeDates()
//	}
type APIError struct {
	// Status is the HTTP status the API answered with.
	Status int
	// Code is the machine-readable slug: body "code", falling back to "error".
	Code string
	// Message is the human-readable message, when the API sent one.
	Message string
	// Body is the decoded error body, for fields this SDK does not model.
	Body map[string]any
	// RequestID comes from X-Request-ID. Quote it in support requests.
	RequestID string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("seatlayer: %d %s: %s", e.Status, e.Code, e.Message)
	}
	return fmt.Sprintf("seatlayer: %d %s", e.Status, e.Code)
}

// AuthError is a 401 or 403 — bad key, revoked key, or a live key used against
// a test event.
type AuthError struct{ APIError }

// ModeMismatch reports whether the key's mode and the event's mode disagree.
// This is the most common cause of a "works locally, 403s in production" report.
func (e *AuthError) ModeMismatch() bool { return e.Code == "mode_mismatch" }

// NotFoundError is a 404, including another organisation's resource.
//
// Asking for something owned by a different organisation answers 404, never
// 403: a 403 would confirm the resource exists, which is not something one
// customer should be able to learn about another.
type NotFoundError struct{ APIError }

// ConflictError is a 409 — the seats moved under you.
//
// Normal in ticketing, not exceptional: two buyers wanted the same seat and one
// lost.
type ConflictError struct{ APIError }

// Conflicts returns the per-object conflicts, when the endpoint reports them.
func (e *ConflictError) Conflicts() []map[string]any {
	raw, ok := e.Body["conflicts"].([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// SoldOut reports whether best-available could not find enough free inventory.
func (e *ConflictError) SoldOut() bool {
	reason, _ := e.Body["reason"].(string)
	return reason == "sold_out" || reason == "not_enough_together"
}

// ValidationError is a 422 — the request was understood and rejected.
type ValidationError struct{ APIError }

// RateLimitError is a 429. RetryAfter prefers the header over the JSON field.
type RateLimitError struct {
	APIError
	// RetryAfter is how long to wait, in seconds.
	RetryAfter float64
}

// ConnectionError means the request never got an answer: DNS, TLS, socket, or
// a context deadline.
type ConnectionError struct {
	Op  string
	Err error
}

func (e *ConnectionError) Error() string {
	return fmt.Sprintf("seatlayer: %s: %v", e.Op, e.Err)
}

// Unwrap lets errors.Is reach the underlying cause, so a caller can still test
// for context.DeadlineExceeded or a net error.
func (e *ConnectionError) Unwrap() error { return e.Err }

func errorFromResponse(status int, body map[string]any, requestID string, retryAfter float64) error {
	base := APIError{
		Status:    status,
		Code:      firstString(body["code"], body["error"], "unknown_error"),
		Message:   stringOr(body["message"], ""),
		Body:      body,
		RequestID: requestID,
	}

	switch {
	case status == 401 || status == 403:
		return &AuthError{base}
	case status == 404:
		return &NotFoundError{base}
	case status == 409:
		return &ConflictError{base}
	case status == 422:
		return &ValidationError{base}
	case status == 429:
		return &RateLimitError{APIError: base, RetryAfter: retryAfter}
	default:
		return &base
	}
}

func firstString(first, second any, fallback string) string {
	if s, ok := first.(string); ok && s != "" {
		return s
	}
	if s, ok := second.(string); ok && s != "" {
		return s
	}
	return fallback
}

func stringOr(value any, fallback string) string {
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return fallback
}
