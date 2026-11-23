-- name: CreateKegiatanKalender :one
INSERT INTO kalender_akademik (nama_kegiatan, keterangan, tanggal_mulai, tanggal_selesai)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpdateKegiatanKalender :one
UPDATE kalender_akademik
SET nama_kegiatan = $2, keterangan = $3, tanggal_mulai = $4, tanggal_selesai = $5
WHERE id = $1
RETURNING *;

-- name: DeleteKegiatanKalenderByID :execrows
DELETE FROM kalender_akademik
WHERE id = $1;

-- name: ListKegiatanKalender :many
SELECT * FROM kalender_akademik
ORDER BY tanggal_mulai ASC;

-- name: DeleteSemuaKegiatanKalender :exec
DELETE FROM kalender_akademik;