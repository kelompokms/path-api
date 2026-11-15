-- name: GetPasswordPenggunaByEmail :one
SELECT id, password, admin FROM pengguna WHERE email = $1;

-- name: GetPasswordPenggunaByID :one
SELECT password FROM pengguna WHERE id = $1;

-- name: GetPenggunaByEmail :one
SELECT id, nama, email, profile_picture, admin FROM pengguna WHERE email = $1;

-- name: CreateOAuthPengguna :one
INSERT INTO pengguna (nama, email, profile_picture, admin)
VALUES ($1, $2, $3, FALSE)
RETURNING id, admin;

-- name: GetPenggunaByOAuthAccount :one
SELECT p.id, p.admin
FROM oauth_accounts oa
JOIN pengguna p ON p.id = oa.pengguna_id
WHERE oa.provider = $1 AND oa.provider_user_id = $2;

-- name: LinkOAuthAccount :exec
INSERT INTO oauth_accounts (pengguna_id, provider, provider_user_id, email)
VALUES ($1, $2, $3, $4)
ON CONFLICT (pengguna_id, provider) DO UPDATE
SET provider_user_id = EXCLUDED.provider_user_id,
    email = EXCLUDED.email;
