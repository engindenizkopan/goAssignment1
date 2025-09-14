-- MVP events table with JSONB fields and idempotency constraints.

CREATE TABLE IF NOT EXISTS events (
    id           BIGSERIAL PRIMARY KEY,
    event_id     TEXT NULL,
    event_name   TEXT NOT NULL,
    user_id      TEXT NOT NULL,
    ts_epoch     BIGINT NOT NULL,           -- epoch seconds (UTC)
    channel      TEXT NULL,
    campaign_id  TEXT NULL,
    tags         JSONB NULL,                -- []string as JSONB
    metadata     JSONB NULL,                -- flexible map
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for metrics filters
CREATE INDEX IF NOT EXISTS idx_events_event_name ON events (event_name);
CREATE INDEX IF NOT EXISTS idx_events_ts_epoch   ON events (ts_epoch);
CREATE INDEX IF NOT EXISTS idx_events_evname_ts  ON events (event_name, ts_epoch);
CREATE INDEX IF NOT EXISTS idx_events_channel    ON events (channel);

-- Idempotency (prefer event_id; fallback composite when event_id is null)
CREATE UNIQUE INDEX IF NOT EXISTS uq_events_event_id
    ON events (event_id)
    WHERE event_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_events_composite
    ON events (event_name, user_id, ts_epoch)
    WHERE event_id IS NULL;
