-- +goose Up
SELECT 'up SQL query';
CREATE TABLE t_outbox_events (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_aggregate_id VARCHAR(255) NOT NULL,
    event_aggregate_type VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    event_payload JSONB NOT NULL,
    event_status VARCHAR(50) NOT NULL DEFAULT 'PENDING', -- PENDING, PUBLISHED, FAILED
    event_created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_published_at TIMESTAMPTZ
);

CREATE INDEX idx_t_outbox_events_status ON t_outbox_events (event_status,event_created_at);

-- +goose Down
SELECT 'down SQL query';
DROP TABLE IF EXISTS t_outbox_events;