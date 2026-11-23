package ai

import (
	"net/http"
	"path-api/internal/db"
	"path-api/internal/files"
	"path-api/internal/utils"
	"strings"
	"time"
)

type Handler struct {
	db           *db.Queries
	httpClient   *http.Client
	baseURL      string
	apiKey       string
	pollInterval time.Duration
	maxPolls     int
}

func New(q *db.Queries) *Handler {
	return &Handler{
		db:           q,
		httpClient:   &http.Client{Timeout: 120 * time.Second},
		baseURL:      strings.TrimRight(utils.GetAIAPIURL(), "/"),
		apiKey:       utils.GetAIAPIKey(),
		pollInterval: 1500 * time.Millisecond,
		maxPolls:     80,
	}
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Pesan              string        `json:"pesan" validate:"required"`
	IsiMateri          string        `json:"isi_materi"`
	Jenjang            string        `json:"jenjang"`
	Modes              []string      `json:"modes"`
	Riwayat            []ChatMessage `json:"riwayat"`
	GunakanModelGratis bool          `json:"gunakan_model_gratis"`
}

type ChatResponse struct {
	Jawaban        string `json:"jawaban"`
	Model          string `json:"model"`
	Route          string `json:"route"`
	BerbasisMateri bool   `json:"berbasis_materi"`
}

type ExplainMateriRequest struct {
	Pertanyaan         string   `json:"pertanyaan" validate:"required"`
	DeskripsiMateri    string   `json:"deskripsi_materi"`
	IsiMateri          string   `json:"isi_materi"`
	LampiranPaths      []string `json:"lampiran_paths"`
	Jenjang            string   `json:"jenjang"`
	Tabel              bool     `json:"tabel"`
	Diagram            bool     `json:"diagram"`
	Ilustrasi          bool     `json:"ilustrasi"`
	Kombinasi          bool     `json:"kombinasi"`
	GunakanModelGratis bool     `json:"gunakan_model_gratis"`
}

type ExplainMateriResponse struct {
	Judul   string                       `json:"judul"`
	Jawaban string                       `json:"jawaban"`
	File    *files.UploadedPostinganFile `json:"file,omitempty"`
}

type GenerateMateriRequest struct {
	Judul              string `json:"judul" validate:"required"`
	DeskripsiMateri    string `json:"deskripsi_materi"`
	Jenjang            string `json:"jenjang"`
	Tabel              bool   `json:"tabel"`
	Diagram            bool   `json:"diagram"`
	Ilustrasi          bool   `json:"ilustrasi"`
	Kombinasi          bool   `json:"kombinasi"`
	GunakanModelGratis bool   `json:"gunakan_model_gratis"`
}

type GenerateMateriResponse struct {
	Judul     string                        `json:"judul"`
	Deskripsi string                        `json:"deskripsi"`
	Lampiran  []files.UploadedPostinganFile `json:"lampiran"`
}

type KoreksiSiswaResult struct {
	PenggunaID int32  `json:"pengguna_id"`
	Email      string `json:"email"`
	Nama       string `json:"nama"`
	Nilai      int    `json:"nilai"`
	Label      string `json:"label"`
	Saran      string `json:"saran"`
}

type KoreksiTugasResponse struct {
	RekapFile *files.UploadedPostinganFile `json:"rekap_file,omitempty"`
	Analisa   []string                     `json:"analisa"`
	Hasil     []KoreksiSiswaResult         `json:"hasil"`
}

type AssessmentCheckRequest struct {
	KodeKelas          string  `json:"kode_kelas" validate:"required"`
	PostinganID        int32   `json:"postingan_id" validate:"required,gt=0"`
	PenggunaIDs        []int32 `json:"pengguna_ids"`
	Jenjang            string  `json:"jenjang"`
	GunakanModelGratis bool    `json:"gunakan_model_gratis"`
}

type AssessmentCheckResponse struct {
	RekapFile *files.UploadedPostinganFile `json:"rekap_file,omitempty"`
	Analisa   []string                     `json:"analisa"`
	Ringkasan []string                     `json:"ringkasan"`
}

type RingkasDokumenRequest struct {
	LampiranPaths      []string `json:"lampiran_paths" validate:"required,min=1"`
	Judul              string   `json:"judul"`
	Fokus              string   `json:"fokus"`
	Jenjang            string   `json:"jenjang"`
	Diagram            bool     `json:"diagram"`
	GunakanModelGratis bool     `json:"gunakan_model_gratis"`
}
