-- +goose Up
ALTER TABLE kelas
	ADD COLUMN IF NOT EXISTS mapel TEXT NOT NULL DEFAULT '';

ALTER TABLE postingan
	ADD COLUMN IF NOT EXISTS menerima_kiriman BOOLEAN NOT NULL DEFAULT TRUE;

CREATE TABLE IF NOT EXISTS bantuan_konten (
	kunci TEXT PRIMARY KEY,
	judul TEXT NOT NULL,
	isi TEXT NOT NULL,
	diperbarui TIMESTAMP NOT NULL DEFAULT now()
);

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

CREATE TABLE IF NOT EXISTS ai_usage_log (
	id SERIAL PRIMARY KEY,
	pengguna_id INTEGER NOT NULL REFERENCES pengguna (id) ON DELETE CASCADE,
	fitur TEXT NOT NULL,
	dibuat TIMESTAMP NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_ai_usage_log_pengguna_dibuat
	ON ai_usage_log (pengguna_id, dibuat);

-- +goose Down
DROP INDEX IF EXISTS idx_ai_usage_log_pengguna_dibuat;
DROP TABLE IF EXISTS ai_usage_log;
DROP TABLE IF EXISTS bantuan_konten;
ALTER TABLE postingan DROP COLUMN IF EXISTS menerima_kiriman;
ALTER TABLE kelas DROP COLUMN IF EXISTS mapel;
