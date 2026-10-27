-- name: CreateFile :exec
INSERT INTO file (nama, path, ukuran, tanggal) VALUES ($1, $2, $3, $4);
