-- +goose Up
SELECT 'up SQL query';

CREATE TYPE account_role AS ENUM ('RIDER', 'DRIVER', 'ADMIN');
CREATE TYPE account_status AS ENUM ('ACTIVE', 'SUSPENDED', 'BANNED');

CREATE TABLE t_users (
    user_id UUID PRIMARY KEY,
    user_email VARCHAR(255),
    user_phone VARCHAR(20),
    user_display_name VARCHAR(100),
    user_avatar_url TEXT,
    user_role account_role NOT NULL DEFAULT 'RIDER',
    user_status account_status NOT NULL DEFAULT 'ACTIVE',
    user_created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    user_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_t_users_user_role ON t_users (user_role);
CREATE INDEX idx_t_users_user_status ON t_users (user_status);

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
DROP TABLE IF EXISTS t_users;
DROP TYPE IF EXISTS account_status;
DROP TYPE IF EXISTS account_role;
