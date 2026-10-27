package postingan

import (
	"mime/multipart"
	"path-api/internal/db"
)

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

type CreatePostinganRequest struct {
	KelasID   int32       `param:"kelasID" validate:"required"`
	Judul     string      `json:"judul" validate:"required"`
	Deskripsi string      `json:"deskripsi"`
	Lampiran  []string    `json:"lampiran"`
	Tipe      db.TipePost `json:"tipe" validate:"required,oneof=tugas materi"`
}

type ListPostinganRequest struct {
	Tipe      db.TipePost `query:"tipe" validate:"required,oneof=tugas materi"`
	KodeKelas string      `param:"kode_kelas" validate:"required"`
	Mapel     string      `query:"mapel"`
}
type EditPostinganRequest struct {
	KelasID     int32                   `param:"kelasID" validate:"required"`
	PostinganID int32                   `param:"postinganID" validate:"required"`
	Judul       string                  `json:"judul" validate:"required"`
	Deskripsi   string                  `json:"deskripsi"`
	Lampiran    []*multipart.FileHeader `json:"lampiran"`
	Tipe        db.TipePost             `json:"tipe" validate:"omitempty,oneof=tugas materi"`
}
