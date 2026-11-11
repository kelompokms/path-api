-- name: CreateKelas :exec
INSERT INTO kelas (nama, tingkat, tahun_ajaran, pengampu, kode_kelas)
VALUES ($1, $2, $3, $4, $5);

-- name: ListKelas :many
SELECT kelas.* FROM anggota_kelas
JOIN kelas ON kelas_id = kelas.id
WHERE pengguna_id = $1 AND pengampu != NULL;

-- name: ListKelasDiampu :many
SELECT kelas.* FROM kelas
WHERE pengampu = $1;

-- name: GetKelas :one
SELECT * FROM kelas WHERE kode_kelas = $1;

-- name: JoinKelas :exec
INSERT INTO anggota_kelas (pengguna_id, kelas_id, guru)
VALUES ($1, $2, $3);

-- name: GetKelasID :one
SELECT id FROM kelas WHERE kode_kelas = $1;

-- name: UpdateRoleKelas :exec
UPDATE anggota_kelas
SET guru = $3
WHERE pengguna_id = $1
AND kelas_id = (SELECT id FROM kelas WHERE kode_kelas = $2 LIMIT 1);

-- name: ListAnggotaKelas :one
SELECT * FROM anggota_kelas
WHERE kelas_id = (SELECT id FROM kelas WHERE kode_kelas = $1 LIMIT 1);

-- name: CheckAnggotaKelas :one
SELECT 1 FROM anggota_kelas
WHERE pengguna_id = $1 AND kelas_id = (SELECT id FROM kelas WHERE kode_kelas = $2 LIMIT 1);

-- name: LeaveKelas :exec
DELETE FROM anggota_kelas
WHERE pengguna_id = $1 AND kelas_id = (SELECT id FROM kelas WHERE kode_kelas = $2 LIMIT 1);
