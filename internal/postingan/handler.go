package postingan

import (
	"path-api/internal/db"
	"time"
)

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

type CreatePostinganRequest struct {
	KodeKelas string   `param:"kode_kelas" json:"-" validate:"required"`
	Judul     string   `json:"judul" validate:"required"`
	Deskripsi string   `json:"deskripsi"`
	Lampiran  []string `json:"lampiran"`
	TipeID    int32    `json:"tipe" validate:"required,gte=1,lte=2"`
}

type CreatePostinganPayload struct {
	Judul     string   `json:"judul" validate:"required" example:"Materi Bab 1"`
	Deskripsi string   `json:"deskripsi" example:"Pelajari materi berikut"`
	Lampiran  []string `json:"lampiran" example:"[\"/uploads/2026/11/08/abc.pdf\"]"`
	TipeID    int32    `json:"tipe" validate:"required,gte=1,lte=2" example:"1"`
}

type ListPostinganRequest struct {
	TipeID    int32  `query:"tipe" validate:"required,gte=1,lte=2"`
	KodeKelas string `param:"kode_kelas" validate:"required"`
	Mapel     string `query:"mapel"`
}

type EditPostinganRequest struct {
	PostinganID int32     `param:"postinganID" json:"-" validate:"required"`
	Judul       string    `json:"judul" validate:"required"`
	Deskripsi   string    `json:"deskripsi"`
	Lampiran    *[]string `json:"lampiran"`
	TipeID      int32     `json:"tipe" validate:"required,gte=1,lte=2"`
}

type EditPostinganPayload struct {
	Judul     string    `json:"judul" validate:"required" example:"Judul Baru"`
	Deskripsi string    `json:"deskripsi" example:"Deskripsi Baru"`
	Lampiran  *[]string `json:"lampiran" example:"[\"/uploads/2026/11/08/abc.pdf\"]"`
	TipeID    int32     `json:"tipe" validate:"required,gte=1,lte=2" example:"1"`
}

type PostinganResponse struct {
	ID            int32     `json:"id" example:"1"`
	Judul         string    `json:"judul" example:"Materi Bab 1"`
	Deskripsi     string    `json:"deskripsi" example:"Pelajari materi berikut"`
	Lampiran      []string  `json:"lampiran" example:"[\"/uploads/2026/11/08/abc.pdf\"]"`
	MataPelajaran string    `json:"mata_pelajaran,omitempty" example:"Matematika"`
	PenggunaID    int32     `json:"pengguna_id" example:"1"`
	KelasID       int32     `json:"kelas_id" example:"1"`
	TipeID        int32     `json:"tipe_id" example:"1"`
	Dibuat        time.Time `json:"dibuat" example:"2026-01-01T00:00:00Z"`
}

type TipePostinganResponse struct {
	ID   int32  `json:"id" example:"1"`
	Nama string `json:"nama" example:"Materi"`
}

