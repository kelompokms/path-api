package postingan

import (
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
	Deskripsi string      `json:"deskripsi" validate:"required"`
	Lampiran  []string    `json:"lampiran"`
	Tipe      db.TipePost `json:"tipe" validate:"required,oneof=tugas materi"`
}
type ListPostinganRequest struct {
	Tipe    db.TipePost `query:"tipe" validate:"required,oneof=tugas materi"`
	KelasID int32       `param:"kelasID" validate:"required"`
	Mapel   string      `query:"mapel"`
}
type EditPostinganRequest struct {
	KelasID     int32       `param:"kelasID" validate:"required"`
	PostinganID int32       `param:"postinganID" validate:"required"`
	Judul       string      `json:"judul"`
	Deskripsi   string      `json:"deskripsi"`
	Lampiran    []string    `json:"lampiran"`
	Tipe        db.TipePost `json:"tipe" validate:"omitempty,oneof=tugas materi"`
}
