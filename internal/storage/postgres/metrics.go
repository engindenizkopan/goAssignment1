package postgres

import (
	"context"
	"fmt"
)

type MetricsTotals struct {
	Count       int64 `json:"count"`
	UniqueUsers int64 `json:"unique_users"`
}

type MetricsBucket struct {
	BucketStart int64 `json:"bucket_start"`
	Count       int64 `json:"count"`
	UniqueUsers int64 `json:"unique_users"`
}

// eventName and channel are optional (nil or empty string means "no filter")
func (db *DB) QueryTotals(ctx context.Context, eventName *string, from, to int64, channel *string) (MetricsTotals, error) {
	var res MetricsTotals

	cond := "WHERE ts_epoch >= $1 AND ts_epoch <= $2"
	args := []any{from, to}
	idx := 3

	if eventName != nil && *eventName != "" {
		cond += fmt.Sprintf(" AND event_name=$%d", idx)
		args = append(args, *eventName)
		idx++
	}
	if channel != nil && *channel != "" {
		cond += fmt.Sprintf(" AND channel=$%d", idx)
		args = append(args, *channel)
	}

	sql := "SELECT COUNT(*)::bigint, COUNT(DISTINCT user_id)::bigint FROM events " + cond
	row := db.Pool.QueryRow(ctx, sql, args...)
	if err := row.Scan(&res.Count, &res.UniqueUsers); err != nil {
		return res, fmt.Errorf("scan totals: %w", err)
	}
	return res, nil
}

func (db *DB) QueryBucketsDaily(ctx context.Context, eventName *string, from, to int64, channel *string) ([]MetricsBucket, error) {
	cond := "WHERE ts_epoch >= $1 AND ts_epoch <= $2"
	args := []any{from, to}
	idx := 3

	if eventName != nil && *eventName != "" {
		cond += fmt.Sprintf(" AND event_name=$%d", idx)
		args = append(args, *eventName)
		idx++
	}
	if channel != nil && *channel != "" {
		cond += fmt.Sprintf(" AND channel=$%d", idx)
		args = append(args, *channel)
	}

	sql := fmt.Sprintf(`
SELECT
  EXTRACT(EPOCH FROM date_trunc('day', to_timestamp(ts_epoch)))::bigint AS bucket_start,
  COUNT(*)::bigint AS cnt,
  COUNT(DISTINCT user_id)::bigint AS uniq
FROM events
%s
GROUP BY 1
ORDER BY 1 ASC`, cond)

	rows, err := db.Pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []MetricsBucket
	for rows.Next() {
		var b MetricsBucket
		if err := rows.Scan(&b.BucketStart, &b.Count, &b.UniqueUsers); err != nil {
			return nil, fmt.Errorf("scan bucket: %w", err)
		}
		out = append(out, b)
	}
	return out, rows.Err()
}
