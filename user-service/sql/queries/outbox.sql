-- name: CreateOutboxEvent :one
INSERT INTO t_outbox_events (
    event_aggregate_type,
    event_aggregate_id,
    event_type,
    event_payload
) VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetPendingOutboxEvents :many
SELECT * FROM t_outbox_events
WHERE event_status = 'PENDING'
ORDER BY event_created_at ASC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxEventAsPublished :exec
UPDATE t_outbox_events
SET event_status = 'PUBLISHED', event_published_at = NOW(), event_updated_at = NOW()
WHERE event_id = $1;

-- name: MarkOutboxEventAsFailed :exec
UPDATE t_outbox_events
SET event_status = 'FAILED', event_updated_at = NOW()
WHERE event_id = $1;
