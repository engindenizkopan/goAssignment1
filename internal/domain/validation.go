package domain

import (
	"errors"
	"fmt"
	"time"
)

// FieldError represents a single field's validation error.
type FieldError struct {
	Field string `json:"field"`
	Msg   string `json:"message"`
}

func (e FieldError) Error() string { return fmt.Sprintf("%s: %s", e.Field, e.Msg) }

// ValidateEvent performs strict checks on the event.
// now: reference time (injectable for tests)
// skew: allowable future skew (positive duration)
func ValidateEvent(ev *Event, now time.Time, skew time.Duration) []FieldError {
	var errs []FieldError

	// Required fields
	if ev.EventName == "" {
		errs = append(errs, FieldError{"event_name", "required"})
	} else if len(ev.EventName) > MaxEventNameLen {
		errs = append(errs, FieldError{"event_name", fmt.Sprintf("max length %d", MaxEventNameLen)})
	}

	if ev.UserID == "" {
		errs = append(errs, FieldError{"user_id", "required"})
	} else if len(ev.UserID) > MaxUserIDLen {
		errs = append(errs, FieldError{"user_id", fmt.Sprintf("max length %d", MaxUserIDLen)})
	}

	// Timestamp: epoch seconds, not in the future (allow small skew)
	if ev.Timestamp == 0 {
		errs = append(errs, FieldError{"timestamp", "required epoch seconds (UTC)"})
	} else {
		ts := time.Unix(ev.Timestamp, 0).UTC()
		if ts.After(now.Add(skew)) {
			errs = append(errs, FieldError{"timestamp", "must not be in the future (beyond allowed skew)"})
		}
	}

	// Optional fields length limits
	if ev.Channel != "" && len(ev.Channel) > MaxChannelLen {
		errs = append(errs, FieldError{"channel", fmt.Sprintf("max length %d", MaxChannelLen)})
	}
	if ev.CampaignID != "" && len(ev.CampaignID) > MaxCampaignIDLen {
		errs = append(errs, FieldError{"campaign_id", fmt.Sprintf("max length %d", MaxCampaignIDLen)})
	}

	// Tags: max count & item length
	if len(ev.Tags) > MaxTagsCount {
		errs = append(errs, FieldError{"tags", fmt.Sprintf("max %d items", MaxTagsCount)})
	} else {
		for i, t := range ev.Tags {
			if t == "" {
				errs = append(errs, FieldError{fmt.Sprintf("tags[%d]", i), "must be non-empty"})
				continue
			}
			if len(t) > MaxTagLen {
				errs = append(errs, FieldError{fmt.Sprintf("tags[%d]", i), fmt.Sprintf("max length %d", MaxTagLen)})
			}
		}
	}

	return errs
}

// ValidateBulk enforces top-level bulk constraints (count caps) and per-item validation.
// maxItems: cap for number of events (e.g., 100).
func ValidateBulk(events []*Event, maxItems int, now time.Time, skew time.Duration) (allErrs [][]FieldError, topErr error) {
	if len(events) == 0 {
		return nil, errors.New("events: required and must contain at least one item")
	}
	if len(events) > maxItems {
		return nil, fmt.Errorf("events: max %d items", maxItems)
	}
	allErrs = make([][]FieldError, len(events))
	var any bool
	for i := range events {
		fe := ValidateEvent(events[i], now, skew)
		if len(fe) > 0 {
			allErrs[i] = fe
			any = true
		}
	}
	if any {
		return allErrs, fmt.Errorf("one or more events failed validation")
	}
	return nil, nil
}
