-- +goose Up
SELECT 'up SQL query';

ALTER TABLE t_identities DROP COLUMN identity_role;
ALTER TABLE t_identities DROP COLUMN identity_status;

DROP TYPE IF EXISTS account_role;
DROP TYPE IF EXISTS account_status;

CREATE TABLE t_refresh_tokens (
    refresh_token_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    refresh_token_identity_id UUID NOT NULL REFERENCES t_identities(identity_id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) NOT NULL UNIQUE,
    refresh_token_device_info VARCHAR(255),
    refresh_token_ip_address VARCHAR(64),
    refresh_token_user_agent TEXT,
    refresh_token_expires_at TIMESTAMPTZ NOT NULL,
    refresh_token_revoked_at TIMESTAMPTZ,
    refresh_token_last_used_at TIMESTAMPTZ,
    refresh_token_created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_t_refresh_tokens_identity_id ON t_refresh_tokens (refresh_token_identity_id);
CREATE INDEX idx_t_refresh_tokens_hash ON t_refresh_tokens (refresh_token_hash);

CREATE TABLE t_outbox_events (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_aggregate_id VARCHAR(255) NOT NULL,
    event_aggregate_type VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    event_payload JSONB NOT NULL,
    event_status VARCHAR(50) NOT NULL DEFAULT 'PENDING',
    event_created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    event_published_at TIMESTAMPTZ
);

CREATE INDEX idx_t_outbox_events_status ON t_outbox_events (event_status, event_created_at);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS t_outbox_events;
DROP TABLE IF EXISTS t_refresh_tokens;

CREATE TYPE account_role AS ENUM ('RIDER', 'DRIVER', 'ADMIN');
CREATE TYPE account_status AS ENUM ('ACTIVE', 'BANNED', 'SUSPENDED');

ALTER TABLE t_identities ADD COLUMN identity_role account_role NOT NULL DEFAULT 'RIDER';
ALTER TABLE t_identities ADD COLUMN identity_status account_status NOT NULL DEFAULT 'ACTIVE';
