-- name: CreateKelas :one
INSERT INTO kelas (nama, tingkat, pembuat, kode_kelas)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: ListKelas :many
SELECT kelas.* FROM anggota_kelas
JOIN kelas ON kelas_id = kelas.id
WHERE pengguna_id = $1;

-- name: GetKelas :one
SELECT * FROM kelas WHERE kode_kelas = $1;

-- name: JoinKelas :exec
INSERT INTO anggota_kelas (pengguna_id, kelas_id, role)
VALUES ($1, $2, $3);

-- name: GetKelasID :one
SELECT id FROM kelas WHERE kode_kelas = $1;

-- name: UpdateRoleKelas :exec
UPDATE anggota_kelas
SET role = $2
WHERE id = $1;

-- name: ListAnggotaKelas :one
SELECT * FROM anggota_kelas
WHERE kelas_id = $1;

-- name: CheckAnggotaKelas :one
SELECT 1 FROM anggota_kelas
WHERE pengguna_id = $1 AND kelas_id = $2;

-- name: LeaveKelas :exec
DELETE FROM anggota_kelas
WHERE kelas_id = $1 AND pengguna_id = $2;
