-- +goose Up
CREATE TABLE IF NOT EXISTS tipe_postingan (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pengguna (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	phone TEXT,
	profile_picture TEXT,
	password TEXT,
	admin BOOLEAN NOT NULL DEFAULT FALSE,
	dibuat TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS kelas (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL,
	tingkat SMALLINT NOT NULL,
	deskripsi TEXT NOT NULL DEFAULT '',
	tahun_ajaran TEXT NOT NULL,
	kode_kelas VARCHAR(6) NOT NULL UNIQUE,
	dibuat TIMESTAMP NOT NULL DEFAULT now(),
	pengampu INTEGER REFERENCES pengguna (id)
);

CREATE TABLE IF NOT EXISTS anggota_kelas (
	id SERIAL PRIMARY KEY,
	pengguna_id INTEGER REFERENCES pengguna (id) NOT NULL,
	kelas_id INTEGER REFERENCES kelas (id) NOT NULL,
	guru BOOLEAN NOT NULL DEFAULT FALSE,
	dibuat TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS postingan (
	id SERIAL PRIMARY KEY,
	judul TEXT NOT NULL,
	deskripsi TEXT,
	lampiran TEXT ARRAY,
	mata_pelajaran TEXT,
	pengguna_id INTEGER REFERENCES pengguna (id) NOT NULL,
	kelas_id INTEGER REFERENCES kelas (id) NOT NULL,
	tipe_id INTEGER REFERENCES tipe_postingan (id) NOT NULL,
	dibuat TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS pengumpulan_tugas (
	id SERIAL PRIMARY KEY,
	postingan_id INTEGER REFERENCES postingan (id) NOT NULL,
	pengguna_id INTEGER REFERENCES pengguna (id) NOT NULL,
	lampiran TEXT ARRAY,
	dibuat TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS file (
    id SERIAL PRIMARY KEY,
    nama TEXT NOT NULL,
    path TEXT NOT NULL,
    ukuran INTEGER NOT NULL,
    tanggal DATE NOT NULL
);
CREATE TABLE IF NOT EXISTS kalender_akademik (
	id SERIAL PRIMARY KEY,
	nama_kegiatan TEXT NOT NULL,
	tanggal_mulai DATE NOT NULL,
	tanggal_selesai DATE NOT NULL,
	dibuat TIMESTAMP NOT NULL DEFAULT now()
);


CREATE TABLE IF NOT EXISTS oauth_accounts (
	id SERIAL PRIMARY KEY,
	pengguna_id INTEGER NOT NULL REFERENCES pengguna (id) ON DELETE CASCADE,
	provider TEXT NOT NULL,
	provider_user_id TEXT NOT NULL,
	email TEXT NOT NULL,
	dibuat TIMESTAMP NOT NULL DEFAULT now(),
	UNIQUE (provider, provider_user_id),
	UNIQUE (pengguna_id, provider)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_accounts_provider_user_uidx
	ON oauth_accounts (provider, provider_user_id);

CREATE UNIQUE INDEX IF NOT EXISTS idx_oauth_accounts_pengguna_provider_uidx
	ON oauth_accounts (pengguna_id, provider);


-- +goose StatementBegin
INSERT INTO tipe_postingan(nama) VALUES
('materi'),
('tugas');

INSERT INTO pengguna(nama, email, password, admin) VALUES
('admin', 'admin@example.com', '$2a$10$olMNj43m1Rdw2u1XlPIs.uasf0WIluzVHjaqk0UH3TzSoZcUVzF.i', true);

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_oauth_accounts_pengguna_provider_uidx;
DROP INDEX IF EXISTS idx_oauth_accounts_provider_user_uidx;
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS file;
DROP TABLE IF EXISTS pengumpulan_tugas;
DROP TABLE IF EXISTS postingan;
DROP TABLE IF EXISTS anggota_kelas;
DROP TABLE IF EXISTS kelas;
DROP TABLE IF EXISTS pengguna;
DROP TABLE IF EXISTS tipe_postingan;
DROP TABLE IF EXISTS kalender_akademik;
