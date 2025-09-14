package idempotency

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"example.com/goAssignment1/internal/domain"
)

type KeySource string

const (
	KeyFromEventID   KeySource = "event_id"
	KeyFromComposite KeySource = "composite"
)

// DeriveKey returns a stable idempotency key and the source used.
// - Prefer explicit EventID when provided.
// - Fallback to composite (event_name, user_id, timestamp).
// We return a hex-encoded SHA-256 when using the composite to guarantee fixed length.
func DeriveKey(ev *domain.Event) (key string, src KeySource) {
	if ev.EventID != "" {
		return ev.EventID, KeyFromEventID
	}
	composite := fmt.Sprintf("%s|%s|%d", ev.EventName, ev.UserID, ev.Timestamp)
	sum := sha256.Sum256([]byte(composite))
	return hex.EncodeToString(sum[:]), KeyFromComposite
}
