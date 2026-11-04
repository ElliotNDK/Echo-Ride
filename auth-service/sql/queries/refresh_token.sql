-- name: CreateRefreshToken :one
INSERT INTO t_refresh_tokens (
    refresh_token_identity_id,
    refresh_token_hash,
    refresh_token_device_info,
    refresh_token_ip_address,
    refresh_token_user_agent,
    refresh_token_expires_at
) VALUES (
    $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM t_refresh_tokens
WHERE refresh_token_hash = $1
LIMIT 1;

-- name: RevokeRefreshTokenByHash :exec
UPDATE t_refresh_tokens
SET refresh_token_revoked_at = NOW()
WHERE refresh_token_hash = $1 AND refresh_token_revoked_at IS NULL;

-- name: RevokeAllRefreshTokensByIdentity :exec
UPDATE t_refresh_tokens
SET refresh_token_revoked_at = NOW()
WHERE refresh_token_identity_id = $1 AND refresh_token_revoked_at IS NULL;

-- name: TouchRefreshTokenLastUsed :exec
UPDATE t_refresh_tokens
SET refresh_token_last_used_at = NOW()
WHERE refresh_token_hash = $1;
