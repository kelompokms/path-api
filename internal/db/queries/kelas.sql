-- name: CreateKelas :exec
WITH new_kelas AS (
    INSERT INTO kelas (nama, deskripsi, tingkat, tahun_ajaran, pengampu, kode_kelas, mapel)
    VALUES ($1, $2, $3, $4, $5, $6, $7)
    RETURNING id, pengampu
)
INSERT INTO anggota_kelas (pengguna_id, kelas_id, guru)
SELECT pengampu, id, TRUE FROM new_kelas
WHERE pengampu IS NOT NULL;

-- name: ListKelas :many
SELECT
    k.id,
    k.nama,
    k.deskripsi,
    k.tingkat,
    k.tahun_ajaran,
    k.kode_kelas,
    k.dibuat,
    pengampu_user.id AS pengampu,
    (COALESCE(BOOL_OR(ak.guru), FALSE) OR COALESCE(k.pengampu = sqlc.arg(pengguna_id)::integer, FALSE))::boolean AS guru,
    k.mapel,
    COALESCE(pengampu_user.nama, '')::text AS nama_pengampu
FROM kelas k
JOIN pengguna caller ON caller.id = sqlc.arg(pengguna_id)::integer AND caller.aktif = TRUE
LEFT JOIN pengguna pengampu_user ON pengampu_user.id = k.pengampu AND pengampu_user.aktif = TRUE
LEFT JOIN anggota_kelas ak
    ON ak.kelas_id = k.id AND ak.pengguna_id = sqlc.arg(pengguna_id)::integer
WHERE ak.pengguna_id = sqlc.arg(pengguna_id)::integer OR k.pengampu = sqlc.arg(pengguna_id)::integer
GROUP BY k.id, pengampu_user.id, pengampu_user.nama
ORDER BY k.dibuat DESC, k.id DESC;

-- name: ListKelasDiampu :many
SELECT k.* FROM kelas k
JOIN pengguna p ON p.id = k.pengampu AND p.aktif = TRUE
WHERE k.pengampu = $1;

-- name: GetKelas :one
SELECT * FROM kelas WHERE kode_kelas = $1;

-- name: UpdateKelas :one
UPDATE kelas
SET
    nama = COALESCE(sqlc.narg(nama), nama),
    deskripsi = COALESCE(sqlc.narg(deskripsi), deskripsi),
    mapel = COALESCE(sqlc.narg(mapel), mapel)
WHERE kode_kelas = $1
RETURNING *;

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
    (ak.guru OR COALESCE(k.pengampu = p.id, FALSE))::boolean AS guru,
    ak.dibuat
FROM anggota_kelas ak
JOIN pengguna p ON p.id = ak.pengguna_id
JOIN kelas k ON k.id = ak.kelas_id
WHERE k.kode_kelas = $1
  AND p.aktif = TRUE
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
  AND p.aktif = TRUE
  AND NOT EXISTS (
      SELECT 1 FROM anggota_kelas ak2
      WHERE ak2.kelas_id = k.id AND ak2.pengguna_id = k.pengampu
  )
ORDER BY guru DESC, nama ASC;

-- name: IsGuruKelas :one
SELECT COALESCE(
    (
        (
            EXISTS (SELECT 1 FROM anggota_kelas ak WHERE ak.pengguna_id = $1 AND ak.kelas_id = $2 AND ak.guru = TRUE)
            OR EXISTS (SELECT 1 FROM kelas k WHERE k.id = $2 AND k.pengampu = $1)
        )
        AND EXISTS (SELECT 1 FROM pengguna p WHERE p.id = $1 AND p.aktif = TRUE)
    ),
    FALSE
)::boolean AS is_guru;

-- name: IsAnggotaKelas :one
SELECT EXISTS (
    SELECT 1 FROM anggota_kelas ak
    JOIN kelas k ON k.id = ak.kelas_id
    JOIN pengguna p ON p.id = ak.pengguna_id
    WHERE k.kode_kelas = $1 AND ak.pengguna_id = $2 AND p.aktif = TRUE
    UNION
    SELECT 1 FROM kelas k
    JOIN pengguna p ON p.id = k.pengampu
    WHERE k.kode_kelas = $1 AND k.pengampu = $2 AND p.aktif = TRUE
)::boolean AS is_anggota;

-- name: CheckAnggotaKelas :one
SELECT 1 FROM (
    SELECT 1 FROM anggota_kelas ak
    JOIN kelas k ON k.id = ak.kelas_id
    JOIN pengguna p ON p.id = ak.pengguna_id
    WHERE ak.pengguna_id = $1 AND k.kode_kelas = $2 AND p.aktif = TRUE
    UNION ALL
    SELECT 1 FROM kelas k2
    JOIN pengguna p2 ON p2.id = k2.pengampu
    WHERE k2.pengampu = $1 AND k2.kode_kelas = $2 AND p2.aktif = TRUE
) AS membership
LIMIT 1;

-- name: LeaveKelas :exec
DELETE FROM anggota_kelas
WHERE pengguna_id = $1 AND kelas_id = (SELECT id FROM kelas WHERE kode_kelas = $2 LIMIT 1);

-- name: ListAllKelas :many
SELECT
    k.id,
    k.nama,
    k.deskripsi,
    k.tingkat,
    k.tahun_ajaran,
    k.kode_kelas,
    k.dibuat,
    p.id AS pengampu,
    COALESCE(p.nama, '')::text AS nama_pengampu,
    k.mapel
FROM kelas k
LEFT JOIN pengguna p ON p.id = k.pengampu AND p.aktif = TRUE
WHERE (sqlc.narg(tahun_ajaran)::text IS NULL OR LOWER(k.tahun_ajaran) = LOWER(sqlc.narg(tahun_ajaran)::text))
  AND (sqlc.narg(tingkat)::smallint IS NULL OR k.tingkat = sqlc.narg(tingkat)::smallint)
  AND (sqlc.narg(nama)::text IS NULL OR k.nama ILIKE '%' || sqlc.narg(nama)::text || '%' OR k.mapel ILIKE '%' || sqlc.narg(nama)::text || '%')
ORDER BY k.tahun_ajaran DESC, k.tingkat ASC, k.mapel ASC, k.nama ASC;

-- name: UpdatePengampuKelas :one
UPDATE kelas
SET pengampu = $2
WHERE kode_kelas = $1
RETURNING *;
