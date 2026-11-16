-- name: ListPostingan :many
SELECT p.id, p.judul, p.deskripsi, p.lampiran, p.tipe_id, p.pengguna_id, p.kelas_id, p.dibuat
FROM postingan p
WHERE p.kelas_id = $1
  AND (sqlc.narg(tipe_id)::integer IS NULL OR p.tipe_id = sqlc.narg(tipe_id))
ORDER BY p.dibuat DESC, p.id DESC;

-- name: CreatePostingan :one
INSERT INTO postingan (judul, deskripsi, lampiran, tipe_id, pengguna_id, kelas_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: UpdatePostingan :one
UPDATE postingan
SET judul = $3, deskripsi = $4, lampiran = COALESCE($5, lampiran), tipe_id = $6
WHERE id = $1 AND kelas_id = $2
RETURNING *;

-- name: ListTipePostingan :many
SELECT * FROM tipe_postingan;
