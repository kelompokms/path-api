-- name: GetPasswordPenggunaByEmail :one
SELECT p.id, p.password, rp.nama AS role
FROM pengguna p
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
WHERE p.email = $1;

-- name: GetPasswordPenggunaByID :one
SELECT password FROM pengguna WHERE id = $1;

-- name: GetPenggunaByEmail :one
SELECT p.id, p.nama, p.email, p.profile_picture, rp.nama AS role
FROM pengguna p
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
WHERE p.email = $1;

-- name: CreateOAuthPengguna :one
INSERT INTO pengguna (nama, email, profile_picture, id_role_pengguna)
SELECT $1, $2, $3, id FROM role_pengguna WHERE nama = 'murid'
RETURNING id;

-- name: GetPenggunaByOAuthAccount :one
SELECT p.id, rp.nama AS role
FROM oauth_accounts oa
JOIN pengguna p ON p.id = oa.pengguna_id
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
WHERE oa.provider = $1 AND oa.provider_user_id = $2;

-- name: LinkOAuthAccount :exec
INSERT INTO oauth_accounts (pengguna_id, provider, provider_user_id, email)
VALUES ($1, $2, $3, $4)
ON CONFLICT (pengguna_id, provider) DO UPDATE
SET provider_user_id = EXCLUDED.provider_user_id,
    email = EXCLUDED.email;
