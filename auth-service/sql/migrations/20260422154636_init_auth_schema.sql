-- +goose Up
CREATE TABLE identities
(
    id                UUID PRIMARY KEY,
    email             VARCHAR(255),
    phone             VARCHAR(20),
    phone_verified_at TIMESTAMPTZ,
    status            VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_identities_status CHECK (status IN ('ACTIVE', 'SUSPENDED', 'BANNED')),
    CONSTRAINT chk_identities_contact CHECK (
        email IS NOT NULL
            OR phone IS NOT NULL
        )
);

CREATE UNIQUE INDEX ux_identities_email ON identities (lower(email)) WHERE
  email IS NOT NULL;
CREATE UNIQUE INDEX ux_identities_phone ON identities (phone) WHERE
  phone IS NOT NULL;

CREATE TABLE outbox_events
(
    id             UUID PRIMARY KEY      DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(100) NOT NULL,
    aggregate_id   VARCHAR(255) NOT NULL,
    event_type     VARCHAR(100) NOT NULL,
    payload        JSONB        NOT NULL,
    status         VARCHAR(20)  NOT NULL DEFAULT 'PENDING',
    attempts       INT          NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    published_at   TIMESTAMPTZ,
    CONSTRAINT chk_outbox_events_status CHECK (status IN ('PENDING', 'PUBLISHED', 'FAILED'))
);

CREATE INDEX idx_outbox_events_pending ON outbox_events (created_at) WHERE
  status = 'PENDING';

-- +goose Down
DROP TABLE IF EXISTS outbox_events;

DROP TABLE IF EXISTS identities;