-- name: CreatePengguna :one
INSERT INTO pengguna (nama, email, password, id_role_pengguna)
SELECT $1, $2, $3, rp.id FROM role_pengguna rp WHERE rp.nama = sqlc.arg(role)::text
RETURNING id;

-- name: ListAdmin :many
SELECT p.id, p.nama, p.email, p.phone, p.profile_picture, p.password, p.id_role_pengguna, rp.nama AS role, p.dibuat
FROM pengguna p
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
WHERE rp.nama = 'admin' AND p.aktif = TRUE;

-- name: ListGuru :many
SELECT
    kelas_id,
    kelas.nama as nama_kelas,
    pengguna_id,
    pengguna.nama as nama_pengguna
FROM anggota_kelas
JOIN pengguna ON pengguna_id = pengguna.id
JOIN kelas ON kelas_id = kelas.id
WHERE guru = TRUE AND pengguna.aktif = TRUE;

-- name: ListAllGuru :many
SELECT
    p.id,
    p.nama,
    p.email,
    COALESCE(p.phone, '')::text AS phone,
    COALESCE(p.profile_picture, '')::text AS profile_picture,
    COALESCE(ARRAY_AGG(DISTINCT k.nama) FILTER (WHERE k.id IS NOT NULL), '{}')::text[] AS kelas
FROM pengguna p
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
LEFT JOIN anggota_kelas ak ON ak.pengguna_id = p.id AND ak.guru = TRUE
LEFT JOIN kelas k ON k.pengampu = p.id OR k.id = ak.kelas_id
WHERE rp.nama = 'guru' AND p.aktif = TRUE
GROUP BY p.id
ORDER BY p.nama ASC;

-- name: ListAllMurid :many
SELECT
    p.id,
    p.nama,
    p.email,
    COALESCE(p.phone, '')::text AS phone,
    COALESCE(p.profile_picture, '')::text AS profile_picture,
    COALESCE(ARRAY_AGG(DISTINCT k.nama) FILTER (WHERE k.id IS NOT NULL), '{}')::text[] AS kelas
FROM pengguna p
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
LEFT JOIN anggota_kelas ak ON ak.pengguna_id = p.id
LEFT JOIN kelas k ON k.id = ak.kelas_id
WHERE rp.nama = 'murid' AND p.aktif = TRUE
GROUP BY p.id
ORDER BY p.nama ASC;

-- name: IsGuru :one
SELECT TRUE
FROM anggota_kelas ak
JOIN pengguna p ON p.id = ak.pengguna_id
WHERE ak.pengguna_id = $1 AND ak.guru = TRUE AND p.aktif = TRUE
LIMIT 1;

-- name: GetPengguna :one
SELECT p.id, p.nama, p.email, p.phone, p.profile_picture, rp.nama AS role, (rp.nama = 'admin')::boolean AS admin
FROM pengguna p
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
WHERE p.id = $1 AND p.aktif = TRUE LIMIT 1;

-- name: GetPenggunaRoleByID :one
SELECT p.id, rp.nama AS role
FROM pengguna p
JOIN role_pengguna rp ON rp.id = p.id_role_pengguna
WHERE p.id = $1 AND p.aktif = TRUE LIMIT 1;

-- name: UpdatePengguna :one
SELECT id, nama, email, phone, profile_picture FROM pengguna WHERE id = $1 AND aktif = TRUE LIMIT 1;

-- name: UpdateRolePengguna :execrows
UPDATE pengguna
SET id_role_pengguna = rp.id
FROM role_pengguna rp
WHERE pengguna.id = sqlc.arg(id)
  AND pengguna.aktif = TRUE
  AND rp.nama = sqlc.arg(role)::text;

-- name: UpdatePasswordPengguna :exec
UPDATE pengguna SET password = $2 WHERE id = $1 AND aktif = TRUE;

-- name: UpdateEmailPengguna :one
UPDATE pengguna SET email = $2 WHERE id = $1 AND aktif = TRUE RETURNING id, nama, email, phone, profile_picture;

-- name: UpdatePhonePengguna :one
UPDATE pengguna SET phone = $2 WHERE id = $1 AND aktif = TRUE RETURNING id, nama, email, phone, profile_picture;

-- name: UpdateProfilePicturePengguna :one
UPDATE pengguna SET profile_picture = $2 WHERE id = $1 AND aktif = TRUE RETURNING id, nama, email, phone, profile_picture;

-- name: DeactivatePengguna :execrows
UPDATE pengguna
SET aktif = FALSE
WHERE id = $1 AND aktif = TRUE;
