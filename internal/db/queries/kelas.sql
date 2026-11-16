-- name: CreateKelas :exec
WITH new_kelas AS (
    INSERT INTO kelas (nama, tingkat, tahun_ajaran, pengampu, kode_kelas)
    VALUES ($1, $2, $3, $4, $5)
    RETURNING id, pengampu
)
INSERT INTO anggota_kelas (pengguna_id, kelas_id, guru)
SELECT pengampu, id, TRUE FROM new_kelas
WHERE pengampu IS NOT NULL;

-- name: ListKelas :many
SELECT
    k.id,
    k.nama,
    k.tingkat,
    k.tahun_ajaran,
    k.kode_kelas,
    k.dibuat,
    k.pengampu,
    COALESCE(
        BOOL_OR(ak.guru),
        k.pengampu = sqlc.arg(pengguna_id)::integer,
        FALSE
    )::boolean AS guru
FROM kelas k
LEFT JOIN anggota_kelas ak
    ON ak.kelas_id = k.id AND ak.pengguna_id = sqlc.arg(pengguna_id)::integer
WHERE ak.pengguna_id = sqlc.arg(pengguna_id)::integer OR k.pengampu = sqlc.arg(pengguna_id)::integer
GROUP BY k.id
ORDER BY k.dibuat DESC, k.id DESC;

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

-- name: ListAnggotaKelas :many
SELECT
    p.id AS pengguna_id,
    p.nama,
    p.email,
    p.profile_picture,
    ak.guru,
    ak.dibuat
FROM anggota_kelas ak
JOIN pengguna p ON p.id = ak.pengguna_id
JOIN kelas k ON k.id = ak.kelas_id
WHERE k.kode_kelas = $1
UNION
SELECT
    p.id AS pengguna_id,
    p.nama,
    p.email,
    p.profile_picture,
    TRUE AS guru,
    k.dibuat
FROM kelas k
JOIN pengguna p ON p.id = k.pengampu
WHERE k.kode_kelas = $1
  AND NOT EXISTS (
      SELECT 1 FROM anggota_kelas ak2
      WHERE ak2.kelas_id = k.id AND ak2.pengguna_id = k.pengampu
  )
ORDER BY guru DESC, nama ASC;

-- name: IsGuruKelas :one
SELECT COALESCE(
    (SELECT BOOL_OR(ak.guru) FROM anggota_kelas ak WHERE ak.pengguna_id = $1 AND ak.kelas_id = $2),
    (SELECT k.pengampu = $1 FROM kelas k WHERE k.id = $2),
    FALSE
)::boolean AS is_guru;

-- name: IsAnggotaKelas :one
SELECT EXISTS (
    SELECT 1 FROM anggota_kelas ak
    JOIN kelas k ON k.id = ak.kelas_id
    WHERE k.kode_kelas = $1 AND ak.pengguna_id = $2
    UNION
    SELECT 1 FROM kelas k
    WHERE k.kode_kelas = $1 AND k.pengampu = $2
)::boolean AS is_anggota;

-- name: CheckAnggotaKelas :one
SELECT 1 FROM (
    SELECT 1 FROM anggota_kelas ak
    JOIN kelas k ON k.id = ak.kelas_id
    WHERE ak.pengguna_id = $1 AND k.kode_kelas = $2
    UNION ALL
    SELECT 1 FROM kelas k2
    WHERE k2.pengampu = $1 AND k2.kode_kelas = $2
) AS membership
LIMIT 1;

-- name: LeaveKelas :exec
DELETE FROM anggota_kelas
WHERE pengguna_id = $1 AND kelas_id = (SELECT id FROM kelas WHERE kode_kelas = $2 LIMIT 1);
