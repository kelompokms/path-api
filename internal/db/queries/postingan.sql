-- name: ListPostingan :many
SELECT p.id, p.judul, p.deskripsi, p.lampiran, p.tipe_id, p.pengguna_id, p.kelas_id, p.dibuat
FROM postingan p
JOIN kelas k ON p.kelas_id = k.id
WHERE p.tipe_id = $1
  AND p.pengguna_id = $2
  AND p.kelas_id = $3
  AND p.mata_pelajaran = $4;

-- name: CreatePostingan :one
INSERT INTO postingan (judul, deskripsi, lampiran, tipe_id, pengguna_id, kelas_id, mata_pelajaran)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: UpdatePostingan :one
UPDATE postingan
SET judul = $2, deskripsi = $3, lampiran = COALESCE($4, lampiran), tipe_id = $5
WHERE id = $1
RETURNING *;

-- name: ListTipePostingan :many
SELECT * FROM tipe_postingan;
