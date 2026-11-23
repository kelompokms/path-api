-- name: ListBantuanKonten :many
SELECT kunci, judul, isi, diperbarui
FROM bantuan_konten
ORDER BY CASE kunci
    WHEN 'panduan_ai' THEN 1
    WHEN 'etika_ai' THEN 2
    WHEN 'kontak_kami' THEN 3
    ELSE 4
END;

-- name: UpdateBantuanKonten :one
UPDATE bantuan_konten
SET isi = $2, diperbarui = now()
WHERE kunci = $1
RETURNING kunci, judul, isi, diperbarui;

-- name: CreateAiUsageLog :exec
INSERT INTO ai_usage_log (pengguna_id, fitur)
VALUES ($1, $2);

-- name: ListAiUsageByPeriod :many
SELECT
    al.pengguna_id,
    rp.nama AS role,
    COUNT(*)::integer AS jumlah
FROM ai_usage_log al
JOIN pengguna p ON p.id = al.pengguna_id AND p.aktif = TRUE
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
WHERE al.dibuat >= $1 AND al.dibuat < $2
GROUP BY al.pengguna_id, rp.nama;
