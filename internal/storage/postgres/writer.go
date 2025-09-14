package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"example.com/goAssignment1/internal/domain"
)

type Writer struct {
	db *DB
}

func NewWriter(db *DB) *Writer { return &Writer{db: db} }

// InsertBatch inserts events with ON CONFLICT DO NOTHING to enforce idempotency.
func (w *Writer) InsertBatch(ctx context.Context, items []domain.Event) (int64, error) {
	if len(items) == 0 {
		return 0, nil
	}

	cols := []string{"event_id", "event_name", "user_id", "ts_epoch", "channel", "campaign_id", "tags", "metadata"}
	placeholders := make([]string, 0, len(items))
	args := make([]any, 0, len(items)*len(cols))

	argi := 1
	for _, ev := range items {
		ph := make([]string, 0, len(cols))

		// event_id (NULL if empty)
		if ev.EventID == "" {
			args = append(args, nil)
		} else {
			args = append(args, ev.EventID)
		}
		ph = append(ph, fmt.Sprintf("$%d", argi))
		argi++

		// requireds
		args = append(args, ev.EventName)
		ph = append(ph, fmt.Sprintf("$%d", argi))
		argi++

		args = append(args, ev.UserID)
		ph = append(ph, fmt.Sprintf("$%d", argi))
		argi++

		args = append(args, ev.Timestamp)
		ph = append(ph, fmt.Sprintf("$%d", argi))
		argi++

		// optionals
		if ev.Channel == "" {
			args = append(args, nil)
		} else {
			args = append(args, ev.Channel)
		}
		ph = append(ph, fmt.Sprintf("$%d", argi))
		argi++

		if ev.CampaignID == "" {
			args = append(args, nil)
		} else {
			args = append(args, ev.CampaignID)
		}
		ph = append(ph, fmt.Sprintf("$%d", argi))
		argi++

		// tags JSONB (nil or JSON string)
		if len(ev.Tags) == 0 {
			args = append(args, nil)
		} else {
			b, _ := json.Marshal(ev.Tags)
			args = append(args, string(b))
		}
		ph = append(ph, fmt.Sprintf("$%d::jsonb", argi))
		argi++

		// metadata JSONB (nil or JSON string)
		if ev.Metadata == nil {
			args = append(args, nil)
		} else {
			b, _ := json.Marshal(ev.Metadata)
			args = append(args, string(b))
		}
		ph = append(ph, fmt.Sprintf("$%d::jsonb", argi))
		argi++

		placeholders = append(placeholders, "("+strings.Join(ph, ",")+")")
	}

	sql := "INSERT INTO events (" + strings.Join(cols, ",") + ") VALUES " +
		strings.Join(placeholders, ",") +
		" ON CONFLICT DO NOTHING"

	ct, err := w.db.Pool.Exec(ctx, sql, args...)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}
