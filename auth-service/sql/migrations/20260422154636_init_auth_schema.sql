-- +goose Up
SELECT 'up SQL query';
CREATE TYPE account_role AS ENUM ('RIDER', 'DRIVER', 'ADMIN');
CREATE TYPE account_status AS ENUM ('ACTIVE', 'BANNED', 'SUSPENDED');

CREATE TABLE t_identities (
    identity_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    identity_email VARCHAR(255) UNIQUE NOT NULL ,
    identity_phone VARCHAR(20) UNIQUE NOT NULL,
    identity_password_hash VARCHAR(255) NOT NULL,
    identity_role account_role NOT NULL DEFAULT 'RIDER',
    identity_status account_status NOT NULL DEFAULT 'ACTIVE',
    identity_created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    identity_updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_t_identities_identity_email ON t_identities (identity_email);
CREATE INDEX idx_t_identities_identity_phone ON t_identities (identity_phone);

-- +goose Down
SELECT 'down SQL query';
DROP TABLE IF EXISTS t_identities;
DROP TYPE IF EXISTS account_role;
DROP TYPE IF EXISTS account_status;
