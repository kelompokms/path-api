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

-- name: GetPostinganByID :one
SELECT id, judul, deskripsi, lampiran, pengguna_id, kelas_id, tipe_id, dibuat
FROM postingan
WHERE id = $1 AND kelas_id = $2;

-- name: GetPengumpulanTugas :one
SELECT id, postingan_id, pengguna_id, lampiran, dibuat
FROM pengumpulan_tugas
WHERE postingan_id = $1 AND pengguna_id = $2
ORDER BY id DESC
LIMIT 1;

-- name: ListPengumpulanTugas :many
SELECT
    pt.id,
    pt.postingan_id,
    pt.pengguna_id,
    u.nama,
    u.email,
    u.profile_picture,
    pt.lampiran,
    pt.dibuat
FROM pengumpulan_tugas pt
JOIN pengguna u ON u.id = pt.pengguna_id
WHERE pt.postingan_id = $1
ORDER BY pt.dibuat DESC, pt.id DESC;

-- name: CreatePengumpulanTugas :one
INSERT INTO pengumpulan_tugas (postingan_id, pengguna_id, lampiran)
VALUES ($1, $2, $3)
RETURNING id, postingan_id, pengguna_id, lampiran, dibuat;

-- name: UpdatePengumpulanTugas :one
UPDATE pengumpulan_tugas
SET lampiran = $3, dibuat = now()
WHERE id = $1 AND pengguna_id = $2
RETURNING id, postingan_id, pengguna_id, lampiran, dibuat;

-- name: DeletePengumpulanTugas :many
DELETE FROM pengumpulan_tugas
WHERE postingan_id = $1 AND pengguna_id = $2
RETURNING id, postingan_id, pengguna_id, lampiran, dibuat;

-- name: ListTipePostingan :many
SELECT * FROM tipe_postingan;
