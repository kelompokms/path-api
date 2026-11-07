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
	KelasID   int32    `param:"kelasID" validate:"required"`
	Judul     string   `json:"judul" validate:"required"`
	Deskripsi string   `json:"deskripsi"`
	Lampiran  []string `json:"lampiran"`
	TipeID    int32    `json:"tipe" validate:"required,gte=1,lte=2"`
}

type ListPostinganRequest struct {
	TipeID    int32  `query:"tipe" validate:"required,,gte=1,lte=2"`
	KodeKelas string `param:"kode_kelas" validate:"required"`
	Mapel     string `query:"mapel"`
}
type EditPostinganRequest struct {
	KelasID     int32  `param:"kelasID" validate:"required"`
	PostinganID int32  `param:"postinganID" validate:"required"`
	Judul       string `json:"judul" validate:"required"`
	Deskripsi   string `json:"deskripsi"`
	TipeID      int32  `json:"tipe" validate:"required,gte=1,lte=2"`
}
