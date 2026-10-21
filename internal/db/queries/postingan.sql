-- name: ListPostingan :many
SELECT p.id, p.judul, p.deskripsi, p.lampiran, p.tipe, p.pengguna_id, p.kelas_id, p.dibuat
FROM postingan p
JOIN kelas k ON p.kelas_id = k.id
WHERE p.tipe = $1
  AND p.pengguna_id = $2
  AND p.kelas_id = $3
  AND p.mata_pelajaran = $4;

-- name: CreatePostingan :one
INSERT INTO postingan (judul, deskripsi, lampiran, tipe, pengguna_id, kelas_id, mata_pelajaran)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdatePostingan :one
UPDATE postingan
SET judul = $2, deskripsi = $3, lampiran = $4, tipe = $5
WHERE id = $1
RETURNING *;
