-- name: CreateKegiatanKalender :one
INSERT INTO kalender_akademik (nama_kegiatan, tanggal_mulai, tanggal_selesai)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListKegiatanKalender :many
SELECT * FROM kalender_akademik
ORDER BY tanggal_mulai ASC;

-- name: DeleteSemuaKegiatanKalender :exec
DELETE FROM kalender_akademik;