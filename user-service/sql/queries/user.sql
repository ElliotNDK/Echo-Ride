-- name: CreateUser :one
INSERT INTO t_users (
    user_id,
    user_email,
    user_phone,
    user_display_name,
    user_avatar_url,
    user_role,
    user_status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
RETURNING *;

-- name: UpsertUser :one
INSERT INTO t_users (
    user_id,
    user_email,
    user_phone,
    user_role
) VALUES (
    $1, $2, $3, $4
)
ON CONFLICT (user_id) DO UPDATE
SET user_email = EXCLUDED.user_email,
    user_phone = EXCLUDED.user_phone,
    user_updated_at = NOW()
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM t_users
WHERE user_id = $1
LIMIT 1;

-- name: UpdateUserProfile :one
UPDATE t_users
SET user_display_name = COALESCE(sqlc.narg('user_display_name'), user_display_name),
    user_avatar_url = COALESCE(sqlc.narg('user_avatar_url'), user_avatar_url),
    user_updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: UpdateUserRole :one
UPDATE t_users
SET user_role = $2, user_updated_at = NOW()
WHERE user_id = $1
RETURNING *;

-- name: UpdateUserStatus :one
UPDATE t_users
SET user_status = $2, user_updated_at = NOW()
WHERE user_id = $1
RETURNING *;
