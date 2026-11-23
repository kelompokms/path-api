package ai

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path-api/internal/db"
	"path-api/internal/files"
	"path-api/internal/utils"
	"path/filepath"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setAuthClaims(c *echo.Context, userID int32, role string) {
	claims := &utils.JWTCustomClaims{
		UserID: userID,
		Admin:  role == "admin",
		Role:   role,
	}
	c.Set("user", &jwt.Token{Claims: claims})
}

func TestChatProxiesAndLogsUsage(t *testing.T) {
	var gotKey, gotActor string
	var gotPayload map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/chat", r.URL.Path)
		gotKey = r.Header.Get("X-API-Key")
		gotActor = r.Header.Get("X-Actor-Id")
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &gotPayload)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jawaban":"Algoritma adalah urutan langkah.","model":"gemma-4","route":"paid","berbasis_materi":true}`))
	}))
	defer upstream.Close()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectExec("INSERT INTO ai_usage_log").
		WithArgs(int32(7), "bantuan_ai").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := &Handler{
		db:         db.New(mock),
		httpClient: upstream.Client(),
		baseURL:    upstream.URL,
		apiKey:     "test-ai-key",
	}

	e := echo.New()
	e.Validator = utils.NewValidator()
	req := httptest.NewRequest(http.MethodPost, "/ai/chat", bytes.NewBufferString(`{"pesan":"Apa itu algoritma?","isi_materi":"Materi algoritma","modes":["Tabel"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthClaims(c, 7, "murid")

	require.NoError(t, h.Chat(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "test-ai-key", gotKey)
	assert.Equal(t, "siswa:7", gotActor)
	assert.Contains(t, gotPayload["pesan"], "Tabel")

	var resp ChatResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Algoritma adalah urutan langkah.", resp.Jawaban)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGenerateMateriPollsAndSavesArtifact(t *testing.T) {
	origRoot := files.UploadRoot
	files.UploadRoot = t.TempDir()
	defer func() { files.UploadRoot = origRoot }()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/materials/generate":
			assert.Equal(t, "guru:3", r.Header.Get("X-Actor-Id"))
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"job-123","status":"queued"}`))
		case "/v1/jobs/job-123":
			_, _ = w.Write([]byte(`{"job_id":"job-123","status":"done","result":{"judul":"HTML Dasar","deskripsi_singkat":"Ringkasan HTML"},"artifact":{"name":"html-dasar.pdf","availability":"/v1/jobs/job-123/artifact","size_bytes":16}}`))
		case "/v1/jobs/job-123/artifact":
			_, _ = w.Write([]byte("%PDF-1.4 sample"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectExec("INSERT INTO ai_usage_log").
		WithArgs(int32(3), "buat_materi").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := &Handler{
		db:           db.New(mock),
		httpClient:   upstream.Client(),
		baseURL:      upstream.URL,
		apiKey:       "test-ai-key",
		pollInterval: 5 * time.Millisecond,
		maxPolls:     5,
	}

	e := echo.New()
	e.Validator = utils.NewValidator()
	req := httptest.NewRequest(http.MethodPost, "/ai/materi/generate", bytes.NewBufferString(`{"judul":"HTML Dasar","deskripsi_materi":"Pengenalan tag HTML","tabel":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthClaims(c, 3, "guru")

	require.NoError(t, h.GenerateMateri(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp GenerateMateriResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "HTML Dasar", resp.Judul)
	assert.Equal(t, "Ringkasan HTML", resp.Deskripsi)
	require.Len(t, resp.Lampiran, 1)
	assert.Equal(t, "html-dasar.pdf", resp.Lampiran[0].Nama)
	assert.Contains(t, resp.Lampiran[0].Path, "/uploads/")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestKoreksiTugasReadsSubmissionsAndReturnsScores(t *testing.T) {
	origRoot := files.UploadRoot
	files.UploadRoot = t.TempDir()
	defer func() { files.UploadRoot = origRoot }()

	// Seed a student uploaded answer file on disk
	subDir := filepath.Join(files.UploadRoot, "2026", "11", "22")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "jwb.txt"), []byte("HyperText Markup Language"), 0o644))

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/assessments/grade":
			assert.NoError(t, r.ParseMultipartForm(4<<20))
			assert.Equal(t, "X PPLG 1", r.FormValue("mata_pelajaran"))
			assert.Equal(t, "X PPLG 1", r.FormValue("kelas"))
			assert.Equal(t, "siswa1@school.id", r.FormValue("email"))
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"grade-99","status":"queued"}`))
		case "/v1/jobs/grade-99":
			_, _ = w.Write([]byte(`{"job_id":"grade-99","status":"done","result":{"deskripsi_hasil_analisa":["Rata-rata 95"],"siswa":[{"email":"siswa1@school.id","nama":"Siswa Satu","nilai":94.6,"label":"Sangat Baik","saran":"Bagus"}]},"artifact":{"name":"rekap.xlsx","availability":"/v1/jobs/grade-99/artifact"}}`))
		case "/v1/jobs/grade-99/artifact":
			_, _ = w.Write([]byte("PK-fake-xlsx"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	now := pgtype.Timestamp{Time: time.Now(), Valid: true}
	mock.ExpectQuery("SELECT id, nama, tingkat, deskripsi, tahun_ajaran, kode_kelas, dibuat, pengampu FROM kelas").
		WithArgs("PPLG10").
		WillReturnRows(pgxmock.NewRows([]string{"id", "nama", "tingkat", "deskripsi", "tahun_ajaran", "kode_kelas", "dibuat", "pengampu"}).
			AddRow(int32(1), "X PPLG 1", int16(10), "Desc", "2026/2027", "PPLG10", now, pgtype.Int4{Int32: 2, Valid: true}))
	mock.ExpectQuery("SELECT COALESCE").
		WithArgs(int32(2), int32(1)).
		WillReturnRows(pgxmock.NewRows([]string{"is_guru"}).AddRow(true))
	mock.ExpectQuery("SELECT id, judul, deskripsi, lampiran, pengguna_id, kelas_id, tipe_id, dibuat, menerima_kiriman, tenggat_waktu").
		WithArgs(int32(10), int32(1)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat", "menerima_kiriman", "tenggat_waktu"}).
			AddRow(int32(10), "Kuis HTML", pgtype.Text{String: "Kerjakan", Valid: true}, []string{}, int32(2), int32(1), int32(1), now, true, pgtype.Timestamp{}))
	mock.ExpectQuery("SELECT p.id, p.nama, p.email, p.phone, p.profile_picture").
		WithArgs(int32(2)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture", "role", "admin"}).
			AddRow(int32(2), "Pak Budi", "budi@school.id", pgtype.Text{}, pgtype.Text{}, "guru", false))
	mock.ExpectQuery("FROM pengumpulan_tugas pt").
		WithArgs(int32(10)).
		WillReturnRows(pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "nama", "email", "profile_picture", "lampiran", "nilai", "dibuat"}).
			AddRow(int32(1), int32(10), int32(5), "Siswa Satu", "siswa1@school.id", pgtype.Text{}, []string{"/uploads/2026/11/22/jwb.txt"}, pgtype.Int2{}, now))
	mock.ExpectExec("INSERT INTO ai_usage_log").
		WithArgs(int32(2), "koreksi_ai").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := &Handler{
		db:           db.New(mock),
		httpClient:   upstream.Client(),
		baseURL:      upstream.URL,
		apiKey:       "test-ai-key",
		pollInterval: 5 * time.Millisecond,
		maxPolls:     5,
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	_ = mw.WriteField("kode_kelas", "PPLG10")
	_ = mw.WriteField("postingan_id", "10")
	_ = mw.WriteField("pengguna_ids", "5")
	_ = mw.WriteField("deskripsi_kunci", "HyperText Markup Language")
	require.NoError(t, mw.Close())

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/ai/koreksi", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthClaims(c, 2, "guru")

	require.NoError(t, h.KoreksiTugas(c))
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp KoreksiTugasResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Hasil, 1)
	assert.Equal(t, int32(5), resp.Hasil[0].PenggunaID)
	assert.Equal(t, 95, resp.Hasil[0].Nilai)
	require.NotNil(t, resp.RekapFile)
	assert.Equal(t, "rekap.xlsx", resp.RekapFile.Nama)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestExplainMateriPollsAndSavesArtifact(t *testing.T) {
	origRoot := files.UploadRoot
	files.UploadRoot = t.TempDir()
	defer func() { files.UploadRoot = origRoot }()

	var gotDiagram string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/materials/explain":
			assert.Equal(t, "siswa:7", r.Header.Get("X-Actor-Id"))
			assert.NoError(t, r.ParseMultipartForm(4<<20))
			gotDiagram = r.FormValue("diagram")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"exp-45","status":"queued"}`))
		case "/v1/jobs/exp-45":
			_, _ = w.Write([]byte(`{"job_id":"exp-45","status":"done","result":{"judul":"Alur Golang","deskripsi_singkat":"Penjelasan visual dengan diagram panah."},"artifact":{"name":"alur-golang.pdf","availability":"/v1/jobs/exp-45/artifact","size_bytes":24}}`))
		case "/v1/jobs/exp-45/artifact":
			_, _ = w.Write([]byte("%PDF-1.4 diagram pdf"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	mock.ExpectExec("INSERT INTO ai_usage_log").
		WithArgs(int32(7), "bantuan_ai").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := &Handler{
		db:           db.New(mock),
		httpClient:   upstream.Client(),
		baseURL:      upstream.URL,
		apiKey:       "test-ai-key",
		pollInterval: 5 * time.Millisecond,
		maxPolls:     5,
	}

	e := echo.New()
	e.Validator = utils.NewValidator()
	req := httptest.NewRequest(http.MethodPost, "/ai/materi/explain", bytes.NewBufferString(`{"pertanyaan":"Jelaskan alur Golang","isi_materi":"Materi Go","diagram":true}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthClaims(c, 7, "murid")

	require.NoError(t, h.ExplainMateri(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "true", gotDiagram)

	var resp ExplainMateriResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "Alur Golang", resp.Judul)
	assert.Equal(t, "Penjelasan visual dengan diagram panah.", resp.Jawaban)
	require.NotNil(t, resp.File)
	assert.Equal(t, "alur-golang.pdf", resp.File.Nama)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestImageGenerationForcesPaidModel(t *testing.T) {
	var gotExplainFree, gotGenerateFree string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/materials/explain":
			assert.NoError(t, r.ParseMultipartForm(4<<20))
			gotExplainFree = r.FormValue("gunakan_model_gratis")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"exp-img","status":"queued"}`))
		case "/v1/jobs/exp-img":
			_, _ = w.Write([]byte(`{"job_id":"exp-img","status":"done","result":{"judul":"Ilustrasi","deskripsi_singkat":"OK"}}`))
		case "/v1/materials/generate":
			assert.NoError(t, r.ParseMultipartForm(4<<20))
			gotGenerateFree = r.FormValue("gunakan_model_gratis")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"gen-img","status":"queued"}`))
		case "/v1/jobs/gen-img":
			_, _ = w.Write([]byte(`{"job_id":"gen-img","status":"done","result":{"judul":"Materi","deskripsi_singkat":"OK"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()
	mock.ExpectExec("INSERT INTO ai_usage_log").WithArgs(int32(7), "bantuan_ai").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	mock.ExpectExec("INSERT INTO ai_usage_log").WithArgs(int32(3), "buat_materi").WillReturnResult(pgxmock.NewResult("INSERT", 1))

	h := &Handler{
		db:           db.New(mock),
		httpClient:   upstream.Client(),
		baseURL:      upstream.URL,
		apiKey:       "test-ai-key",
		pollInterval: 5 * time.Millisecond,
		maxPolls:     5,
	}
	e := echo.New()
	e.Validator = utils.NewValidator()

	req1 := httptest.NewRequest(http.MethodPost, "/ai/materi/explain", bytes.NewBufferString(`{"pertanyaan":"Gambar sel","ilustrasi":true,"gunakan_model_gratis":true}`))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	c1 := e.NewContext(req1, rec1)
	setAuthClaims(c1, 7, "murid")
	require.NoError(t, h.ExplainMateri(c1))
	assert.Equal(t, "false", gotExplainFree)

	req2 := httptest.NewRequest(http.MethodPost, "/ai/materi/generate", bytes.NewBufferString(`{"judul":"Sel","kombinasi":true,"gunakan_model_gratis":true}`))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	setAuthClaims(c2, 3, "guru")
	require.NoError(t, h.GenerateMateri(c2))
	assert.Equal(t, "false", gotGenerateFree)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSimilarityIndikasiRingkasAndOpsUsage(t *testing.T) {
	origRoot := files.UploadRoot
	files.UploadRoot = t.TempDir()
	defer func() { files.UploadRoot = origRoot }()

	subDir := filepath.Join(files.UploadRoot, "2026", "11", "22")
	require.NoError(t, os.MkdirAll(subDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, "doc.txt"), []byte("Isi dokumen panjang materi algoritma."), 0o644))

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/assessments/similarity":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"sim-1","status":"queued"}`))
		case "/v1/jobs/sim-1":
			_, _ = w.Write([]byte(`{"job_id":"sim-1","status":"done","result":{"deskripsi_hasil_analisa":["Terdeteksi 1 pasangan mirip"],"pasangan":[{"siswa_a":"Budi","siswa_b":"Siti","persentase":85,"label":"Tinggi"}]},"artifact":{"name":"sim.xlsx","availability":"/v1/jobs/sim-1/artifact"}}`))
		case "/v1/jobs/sim-1/artifact":
			_, _ = w.Write([]byte("PK-sim-xlsx"))
		case "/v1/assessments/ai-usage":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"aiu-1","status":"queued"}`))
		case "/v1/jobs/aiu-1":
			_, _ = w.Write([]byte(`{"job_id":"aiu-1","status":"done","result":{"deskripsi_hasil_analisa":["1 siswa indikasi sedang"],"siswa":[{"nama":"Budi","persentase":60,"label":"Sedang"}]},"artifact":{"name":"aiu.xlsx","availability":"/v1/jobs/aiu-1/artifact"}}`))
		case "/v1/jobs/aiu-1/artifact":
			_, _ = w.Write([]byte("PK-aiu-xlsx"))
		case "/v1/documents/summarize":
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"job_id":"sum-1","status":"queued"}`))
		case "/v1/jobs/sum-1":
			_, _ = w.Write([]byte(`{"job_id":"sum-1","status":"done","result":{"judul":"Ringkasan Algoritma","deskripsi_singkat":"Poin utama algoritma."},"artifact":{"name":"ringkasan.pdf","availability":"/v1/jobs/sum-1/artifact"}}`))
		case "/v1/jobs/sum-1/artifact":
			_, _ = w.Write([]byte("%PDF-1.4 summary"))
		case "/v1/ops/usage":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"budget":{"limit":1000000,"used":250000,"remaining":750000},"free_quota":{"used":5,"limit":50,"remaining":45}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer upstream.Close()

	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	now := pgtype.Timestamp{Time: time.Now(), Valid: true}
	expectAssessmentDB := func(fitur string) {
		mock.ExpectQuery("SELECT id, nama, tingkat, deskripsi, tahun_ajaran, kode_kelas, dibuat, pengampu FROM kelas").
			WithArgs("PPLG10").
			WillReturnRows(pgxmock.NewRows([]string{"id", "nama", "tingkat", "deskripsi", "tahun_ajaran", "kode_kelas", "dibuat", "pengampu"}).
				AddRow(int32(1), "X PPLG 1", int16(10), "Desc", "2026/2027", "PPLG10", now, pgtype.Int4{Int32: 2, Valid: true}))
		mock.ExpectQuery("SELECT COALESCE").WithArgs(int32(2), int32(1)).WillReturnRows(pgxmock.NewRows([]string{"is_guru"}).AddRow(true))
		mock.ExpectQuery("SELECT id, judul, deskripsi, lampiran, pengguna_id, kelas_id, tipe_id, dibuat, menerima_kiriman, tenggat_waktu").
			WithArgs(int32(10), int32(1)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat", "menerima_kiriman", "tenggat_waktu"}).
				AddRow(int32(10), "Tugas 1", pgtype.Text{}, []string{}, int32(2), int32(1), int32(1), now, true, pgtype.Timestamp{}))
		mock.ExpectQuery("SELECT p.id, p.nama, p.email, p.phone, p.profile_picture").
			WithArgs(int32(2)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture", "role", "admin"}).
				AddRow(int32(2), "Pak Budi", "budi@school.id", pgtype.Text{}, pgtype.Text{}, "guru", false))
		mock.ExpectQuery("FROM pengumpulan_tugas pt").
			WithArgs(int32(10)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "nama", "email", "profile_picture", "lampiran", "nilai", "dibuat"}).
				AddRow(int32(1), int32(10), int32(5), "Budi", "budi@s.id", pgtype.Text{}, []string{"/uploads/2026/11/22/doc.txt"}, pgtype.Int2{}, now).
				AddRow(int32(2), int32(10), int32(6), "Siti", "siti@s.id", pgtype.Text{}, []string{"/uploads/2026/11/22/doc.txt"}, pgtype.Int2{}, now))
		mock.ExpectExec("INSERT INTO ai_usage_log").WithArgs(int32(2), fitur).WillReturnResult(pgxmock.NewResult("INSERT", 1))
	}

	h := &Handler{
		db:           db.New(mock),
		httpClient:   upstream.Client(),
		baseURL:      upstream.URL,
		apiKey:       "test-ai-key",
		pollInterval: 5 * time.Millisecond,
		maxPolls:     5,
	}
	e := echo.New()
	e.Validator = utils.NewValidator()

	// 1. Similarity
	expectAssessmentDB("similarity_ai")
	simReq := httptest.NewRequest(http.MethodPost, "/ai/similarity", bytes.NewBufferString(`{"kode_kelas":"PPLG10","postingan_id":10,"gunakan_model_gratis":true}`))
	simReq.Header.Set("Content-Type", "application/json")
	simRec := httptest.NewRecorder()
	simCtx := e.NewContext(simReq, simRec)
	setAuthClaims(simCtx, 2, "guru")
	require.NoError(t, h.CekKemiripan(simCtx))
	assert.Equal(t, http.StatusOK, simRec.Code)
	var simResp AssessmentCheckResponse
	require.NoError(t, json.Unmarshal(simRec.Body.Bytes(), &simResp))
	require.NotNil(t, simResp.RekapFile)
	assert.Contains(t, simResp.Ringkasan[0], "Budi ↔ Siti")

	// 2. AI Usage Indikasi
	expectAssessmentDB("indikasi_ai")
	aiuReq := httptest.NewRequest(http.MethodPost, "/ai/indikasi", bytes.NewBufferString(`{"kode_kelas":"PPLG10","postingan_id":10}`))
	aiuReq.Header.Set("Content-Type", "application/json")
	aiuRec := httptest.NewRecorder()
	aiuCtx := e.NewContext(aiuReq, aiuRec)
	setAuthClaims(aiuCtx, 2, "guru")
	require.NoError(t, h.CekIndikasiAI(aiuCtx))
	assert.Equal(t, http.StatusOK, aiuRec.Code)

	// 3. Ringkas Dokumen
	mock.ExpectExec("INSERT INTO ai_usage_log").WithArgs(int32(7), "bantuan_ai").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	sumReq := httptest.NewRequest(http.MethodPost, "/ai/dokumen/ringkas", bytes.NewBufferString(`{"lampiran_paths":["/uploads/2026/11/22/doc.txt"],"judul":"Algoritma"}`))
	sumReq.Header.Set("Content-Type", "application/json")
	sumRec := httptest.NewRecorder()
	sumCtx := e.NewContext(sumReq, sumRec)
	setAuthClaims(sumCtx, 7, "murid")
	require.NoError(t, h.RingkasDokumen(sumCtx))
	assert.Equal(t, http.StatusOK, sumRec.Code)

	// 4. Ops Usage (admin only)
	opsReq := httptest.NewRequest(http.MethodGet, "/ai/ops/usage", nil)
	opsRec := httptest.NewRecorder()
	opsCtx := e.NewContext(opsReq, opsRec)
	setAuthClaims(opsCtx, 1, "admin")
	require.NoError(t, h.OpsUsage(opsCtx))
	assert.Equal(t, http.StatusOK, opsRec.Code)
	assert.Contains(t, opsRec.Body.String(), "750000")

	require.NoError(t, mock.ExpectationsWereMet())
}
