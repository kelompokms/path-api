-- +goose Up
CREATE TABLE IF NOT EXISTS tipe_postingan (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS role_pengguna (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS pengguna (
	id SERIAL PRIMARY KEY,
	nama TEXT NOT NULL,
	email TEXT NOT NULL UNIQUE,
	phone TEXT,
	profile_picture TEXT,
	password TEXT,
	id_role_pengguna INTEGER REFERENCES role_pengguna (id) NOT NULL,
	aktif BOOLEAN NOT NULL DEFAULT TRUE,
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
	pengguna_id INTEGER REFERENCES pengguna (id) NOT NULL,
	kelas_id INTEGER REFERENCES kelas (id) NOT NULL,
	tipe_id INTEGER REFERENCES tipe_postingan (id) NOT NULL,
	dibuat TIMESTAMP NOT NULL DEFAULT now(),
	menerima_kiriman BOOLEAN NOT NULL DEFAULT TRUE,
	tenggat_waktu TIMESTAMP
);

CREATE TABLE IF NOT EXISTS pengumpulan_tugas (
	id SERIAL PRIMARY KEY,
	postingan_id INTEGER REFERENCES postingan (id) NOT NULL,
	pengguna_id INTEGER REFERENCES pengguna (id) NOT NULL,
	lampiran TEXT ARRAY,
	nilai SMALLINT CHECK (nilai IS NULL OR (nilai >= 0 AND nilai <= 100)),
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
	keterangan TEXT NOT NULL DEFAULT '',
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

CREATE TABLE IF NOT EXISTS bantuan_konten (
	kunci TEXT PRIMARY KEY,
	judul TEXT NOT NULL,
	isi TEXT NOT NULL,
	diperbarui TIMESTAMP NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ai_usage_log (
	id SERIAL PRIMARY KEY,
	pengguna_id INTEGER NOT NULL REFERENCES pengguna (id) ON DELETE CASCADE,
	fitur TEXT NOT NULL,
	dibuat TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_log_pengguna_dibuat
	ON ai_usage_log (pengguna_id, dibuat);


-- +goose StatementBegin
INSERT INTO tipe_postingan(nama) VALUES
('materi'),
('tugas');

INSERT INTO role_pengguna(nama) VALUES
('admin'),
('guru'),
('murid');

INSERT INTO pengguna(nama, email, password, id_role_pengguna) VALUES
('admin', 'admin@example.com', '$2a$10$olMNj43m1Rdw2u1XlPIs.uasf0WIluzVHjaqk0UH3TzSoZcUVzF.i', 1),
('Budi', 'budi@example.com', '$2a$10$IgOJikpoGNP1bCDFa4C2v.7uAyxEG7jPlOP5ND7n5kDzCOamtjspC', 2);

INSERT INTO bantuan_konten (kunci, judul, isi) VALUES
(
	'panduan_ai',
	'Panduan AI',
	E'Fitur AI di PATH membantu proses belajar dan mengajar secara adaptif:\n\n1. Bantuan AI (Murid)\nBuka halaman detail materi di dalam kelas, lalu tekan tombol "Bantuan AI". Pilih gaya penjelasan yang diinginkan (Tabel, Diagram, Ilustrasi, atau Kombinasi) atau ketik pertanyaan langsung terkait materi tersebut.\n\n2. Buat Materi dengan AI (Guru)\nDi tab Materi kelas, tekan "Buat Materi" lalu pilih "Buat materi dengan AI". Isi judul, topik materi, dan elemen pendukung untuk membuat draf materi yang dapat langsung digunakan atau disesuaikan.\n\n3. Koreksi Menggunakan AI (Guru)\nDi halaman "Pengumpulan & Penilaian" pada tugas, tekan "Koreksi menggunakan AI", unggah file kunci jawaban (PDF, TXT, DOC/DOCX, XLS/XLSX, CSV) atau tulis deskripsi kunci jawaban, pilih kiriman siswa, lalu tinjau hasil penilaian sebelum disimpan.'
),
(
	'etika_ai',
	'Etika Penggunaan AI',
	E'Agar pemanfaatan AI tetap bermanfaat dan bertanggung jawab di lingkungan sekolah:\n\n• Gunakan AI sebagai alat bantu memahami konsep, bukan untuk menyalin jawaban secara langsung.\n• Selalu periksa dan verifikasi kebenaran informasi yang dihasilkan AI dengan buku pelajaran atau guru.\n• Hindari plagiarisme dan biasakan menuliskan hasil pemahaman dengan bahasa sendiri.\n• Jangan memasukkan data sensitif atau informasi pribadi ke dalam prompt AI.\n• Patuhi ketentuan penggunaan AI yang ditetapkan oleh guru dan pihak sekolah.'
),
(
	'kontak_kami',
	'Kontak Kami',
	E'MBKM PATH\nEmail: -\n\nJika mengalami kendala atau memiliki pertanyaan terkait penggunaan aplikasi PATH, silakan hubungi administrator sekolah atau tim MBKM PATH.'
)
ON CONFLICT (kunci) DO NOTHING;

-- +goose StatementEnd

-- +goose Down
DROP INDEX IF EXISTS idx_ai_usage_log_pengguna_dibuat;
DROP TABLE IF EXISTS ai_usage_log;
DROP TABLE IF EXISTS bantuan_konten;
DROP INDEX IF EXISTS idx_oauth_accounts_pengguna_provider_uidx;
DROP INDEX IF EXISTS idx_oauth_accounts_provider_user_uidx;
DROP TABLE IF EXISTS oauth_accounts;
DROP TABLE IF EXISTS file;
DROP TABLE IF EXISTS pengumpulan_tugas;
DROP TABLE IF EXISTS postingan;
DROP TABLE IF EXISTS anggota_kelas;
DROP TABLE IF EXISTS kelas;
DROP TABLE IF EXISTS pengguna;
DROP TABLE IF EXISTS role_pengguna;
DROP TABLE IF EXISTS tipe_postingan;
DROP TABLE IF EXISTS kalender_akademik;
