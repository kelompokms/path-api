-- name: CreatePengguna :one
INSERT INTO pengguna (nama, email, password, admin)
VALUES ($1, $2, $3, $4)
RETURNING id;

-- name: ListAdmin :many
SELECT * FROM pengguna WHERE admin = TRUE;

-- name: ListGuru :many
SELECT
    kelas_id,
    kelas.nama as nama_kelas,
    pengguna_id,
    pengguna.nama as nama_pengguna
FROM anggota_kelas
JOIN pengguna ON pengguna_id = pengguna.id
JOIN kelas ON kelas_id = kelas.id
WHERE guru = TRUE;

-- name: IsGuru :one
SELECT TRUE FROM anggota_kelas WHERE id = $1 AND guru = TRUE;

-- name: GetPengguna :one
SELECT id, nama, email, phone, profile_picture FROM pengguna WHERE id = $1 LIMIT 1;

-- name: UpdatePengguna :one
SELECT id, nama, email, phone, profile_picture FROM pengguna WHERE id = $1 LIMIT 1;

-- name: UpdateRolePengguna :exec
UPDATE pengguna SET admin = $2 WHERE id = $1;

-- name: UpdatePasswordPengguna :exec
UPDATE pengguna SET password = $2 WHERE id = $1;

-- name: UpdateEmailPengguna :one
UPDATE pengguna SET email = $2 WHERE id = $1 RETURNING id, nama, email, phone, profile_picture;

-- name: UpdatePhonePengguna :one
UPDATE pengguna SET phone = $2 WHERE id = $1 RETURNING id, nama, email, phone, profile_picture;

-- name: UpdateProfilePicturePengguna :one
UPDATE pengguna SET profile_picture = $2 WHERE id = $1 RETURNING id, nama, email, phone, profile_picture;
