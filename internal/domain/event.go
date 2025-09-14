package domain

import "time"

// Event is the canonical domain object for ingestion.
// timestamp is epoch seconds (UTC) per the spec.
type Event struct {
	EventID    string         `json:"event_id,omitempty"`
	EventName  string         `json:"event_name"`
	UserID     string         `json:"user_id"`
	Timestamp  int64          `json:"timestamp"`
	Channel    string         `json:"channel,omitempty"`
	CampaignID string         `json:"campaign_id,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Validation constraints (MVP defaults; keep in sync with OpenAPI)
const (
	MaxEventNameLen  = 128
	MaxUserIDLen     = 128
	MaxChannelLen    = 64
	MaxCampaignIDLen = 64
	MaxTagLen        = 64
	MaxTagsCount     = 50
	DefaultClockSkew = 5 * time.Minute
)
