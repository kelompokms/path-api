-- name: GetPasswordPenggunaByEmail :one
SELECT id, password, admin FROM pengguna WHERE email = $1;

-- name: GetPasswordPenggunaByID :one
SELECT password FROM pengguna WHERE id = $1;
