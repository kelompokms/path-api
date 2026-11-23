# Panduan Export & Deploy Aman `path-api` ke Server Live

Dokumen ini menjelaskan cara meng-export dan men-deploy perubahan `path-api-main` ke server produksi (`api.pathclass.app`) **tanpa menghapus atau merusak data yang sudah ada**.

---

## Kenapa Deploy Ini Aman untuk Database Live?

1. **Migrasi Database Otomatis & Additive (`00002_feature_updates.sql`, `00003_postingan_tenggat.sql`)**
   - `path-api` menggunakan **Goose** dengan `//go:embed migrations/*.sql` (`internal/db/connection.go`).
   - Saat server baru dinyalakan, Goose otomatis hanya menjalankan migrasi baru sambil mencatat riwayatnya di tabel `goose_db_version`.
   - Seluruh perintah SQL bersifat **additive & idempotent** (`ADD COLUMN IF NOT EXISTS`, `CREATE TABLE IF NOT EXISTS`, `ON CONFLICT DO NOTHING`):
     - `kelas.mapel` (default `''` — seluruh kelas lama tetap aman)
     - `postingan.menerima_kiriman` (default `TRUE` — seluruh tugas lama tetap bisa menerima kiriman)
     - `postingan.tenggat_waktu` (nullable `TIMESTAMP` — tugas lama tampil sebagai tanpa tenggat hingga diatur pengajar)
     - `bantuan_konten` (otomatis terisi default `panduan_ai`, `etika_ai`, `kontak_kami`)
     - `ai_usage_log` (tabel baru untuk log penggunaan AI per user)
   - **Tidak ada** `DROP TABLE`, `TRUNCATE`, atau perubahan tipe kolom destruktif.

2. **Backward-Compatible dengan Aplikasi Mobile Lama & Baru**
   - Semua endpoint lama tetap berjalan identik; field baru (`mapel`, `menerima_kiriman`, `tenggat_waktu`, `nama_pengampu`, `utama`) dan endpoint baru (`PATCH /kelas/:kode_kelas/anggota/:pengguna_id/murid`, `POST /ai/similarity`, `POST /ai/indikasi`, `POST /ai/dokumen/ringkas`, `GET /ai/ops/usage`) ditambahkan tanpa mengubah struktur lama.

---

## Langkah Deploy Aman (Urutan Wajib)

### 1. Backup Database & Folder `uploads` di Server (WAJIB Sebelum Deploy)
Masuk ke server produksi via SSH lalu jalankan:

```bash
# Backup PostgreSQL (jika pakai Docker Compose)
docker compose exec -T db pg_dump -U "$DB_USER" "$DB_NAME" > backup_path_$(date +%Y%m%d_%H%M%S).sql

# (Opsional jika PostgreSQL terpasang langsung di host)
# pg_dump -U "$DB_USER" "$DB_NAME" > backup_path_$(date +%Y%m%d_%H%M%S).sql

# Backup folder uploads (foto profil & lampiran)
tar -czf backup_uploads_$(date +%Y%m%d_%H%M%S).tar.gz uploads/
```

### 2. Pastikan `.env` di Server Sudah Lengkap
Buka file `.env` di server (JANGAN timpa file `.env` server dengan `.env.example`!). Tambahkan 2 baris baru untuk AI server:

```env
AI_API_URL=https://ai.pathclass.app
AI_API_KEY=<ISI_API_KEY_AI_PATHCLASS_DARI_SERVER>
```

> **Penting:** Pastikan `SECRET`, `DB_*`, `GOOGLE_CLIENT_*`, `JWT_*`, dan konfigurasi produksi yang lama **tidak berubah**.

### 3. Export & Kirim Source Code `path-api-main` ke Server
- **Jika pakai Git (paling disarankan):** commit & push perubahan dari folder `path-api-main`, lalu `git pull` di server.
- **Jika pakai upload folder/zip (SCP/SFTP):** zip seluruh isi `path-api-main` **KECUALI** `.env` dan `uploads/`, lalu ekstrak menimpa folder project di server.

### 4. Build & Restart API Saja (Tanpa Hapus Volume DB!)

#### Opsi A — Menggunakan Docker Compose (Sesuai `compose.yaml`)
```bash
# Build ulang image API saja dan restart container API (JANGAN pakai `docker compose down -v`!)
docker compose up -d --build api

# Cek log startup (pastikan migrasi goose 00002 sukses dan Echo berjalan)
docker compose logs -f --tail=100 api
```

#### Opsi B — Menggunakan Binary Go Langsung (Systemd / PM2)
```bash
go build -o path-api .
sudo systemctl restart path-api
journalctl -u path-api -n 100 -f
```

### 5. Verifikasi Pasca-Deploy
Pastikan semua endpoint baru & lama merespons normal:
- `GET /docs/index.html` → 200 OK
- `GET /bantuan` (dengan Bearer token) → 200 OK (berisi 3 section bantuan)
- `GET /admin/kelas/csv/template` (dengan token Admin) → 200 OK (CSV template)
- `GET /admin/kalender/template` (dengan token Admin) → 200 OK (CSV template)
- `GET /admin/ai/usage` (dengan token Admin) → 200 OK
- Upload foto profil rasio bebas (<= 2 MB) → 200 OK

---

## Cara Rollback Jika Ada Masalah
1. Kembalikan kode/commit sebelumnya dan jalankan `docker compose up -d --build api`.
2. Jika ingin membatalkan migrasi DB (opsional — biasanya tidak perlu karena migrasi bersifat additive):
   ```bash
   psql "$DB_URL" < backup_path_YYYYMMDD_HHMMSS.sql
   ```
