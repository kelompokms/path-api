-- +goose Up
CREATE TABLE tipe_postingan (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pengguna (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	phone TEXT,
	profile_picture TEXT,
	password TEXT NOT NULL,
	admin BOOLEAN NOT NULL DEFAULT FALSE,
	dibuat TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS kelas (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL,
	tingkat TEXT,
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

CREATE TABLE file (
    id SERIAL PRIMARY KEY,
    nama TEXT NOT NULL,
    path TEXT NOT NULL,
    ukuran INTEGER NOT NULL,
    tanggal DATE NOT NULL
);

-- +goose StatementBegin
INSERT INTO tipe_postingan(nama) VALUES
('materi'),
('tugas');

INSERT INTO pengguna(nama, email, password, admin) VALUES
('admin', 'admin@example.com', '$2a$10$olMNj43m1Rdw2u1XlPIs.uasf0WIluzVHjaqk0UH3TzSoZcUVzF.i', true);

-- +goose StatementEnd

-- +goose Down
DROP TABLE file;
DROP TABLE pengumpulan_tugas;
DROP TABLE postingan;
DROP TABLE anggota_kelas;
DROP TABLE kelas;
DROP TABLE pengguna;
DROP TABLE tipe_postingan;
