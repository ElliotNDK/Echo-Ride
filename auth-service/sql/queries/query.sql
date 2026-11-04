-- name: CreateIdentity :one
INSERT INTO t_identities (
    identity_id,
    identity_email,
    identity_phone,
    identity_password_hash
) VALUES (
    $1, $2, $3, $4
)
RETURNING *;

-- name: GetIdentityByID :one
SELECT * FROM t_identities
WHERE identity_id = $1
LIMIT 1;

-- name: GetIdentityByEmail :one
SELECT * FROM t_identities
WHERE identity_email = $1
LIMIT 1;

-- name: GetIdentityByPhone :one
SELECT * FROM t_identities
WHERE identity_phone = $1
LIMIT 1;
