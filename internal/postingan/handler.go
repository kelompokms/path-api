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

type ListPostinganRequest struct {
	TipeID    int32  `query:"tipe" validate:"omitempty,gte=1,lte=2"`
	KodeKelas string `param:"kode_kelas" validate:"required"`
}

type EditPostinganRequest struct {
	KodeKelas   string    `param:"kode_kelas" json:"-" validate:"required"`
	PostinganID int32     `param:"postinganID" json:"-" validate:"required"`
	Judul       string    `json:"judul" validate:"required"`
	Deskripsi   string    `json:"deskripsi"`
	Lampiran    *[]string `json:"lampiran"`
	TipeID      int32     `json:"tipe" validate:"required,gte=1,lte=2"`
}

type PostinganResponse struct {
	ID         int32     `json:"id" example:"1"`
	Judul      string    `json:"judul" example:"Materi Bab 1"`
	Deskripsi  string    `json:"deskripsi" example:"Pelajari materi berikut"`
	Lampiran   []string  `json:"lampiran" example:"[\"/uploads/2026/11/08/abc.pdf\"]"`
	PenggunaID int32     `json:"pengguna_id" example:"1"`
	KelasID    int32     `json:"kelas_id" example:"1"`
	TipeID     int32     `json:"tipe_id" example:"1"`
	Dibuat     time.Time `json:"dibuat" example:"2026-01-01T07:00:00+07:00"`
}

type TipePostinganResponse struct {
	ID   int32  `json:"id" example:"1"`
	Nama string `json:"nama" example:"Materi"`
}

type PengumpulanPathRequest struct {
	KodeKelas   string `param:"kode_kelas" json:"-" validate:"required"`
	PostinganID int32  `param:"postinganID" json:"-" validate:"required,gt=0"`
}

type SubmitPengumpulanRequest struct {
	KodeKelas   string   `param:"kode_kelas" json:"-" validate:"required"`
	PostinganID int32    `param:"postinganID" json:"-" validate:"required,gt=0"`
	Lampiran    []string `json:"lampiran"`
}

type PengumpulanResponse struct {
	ID          int32      `json:"id" example:"1"`
	PostinganID int32      `json:"postingan_id" example:"1"`
	PenggunaID  int32      `json:"pengguna_id" example:"2"`
	Submitted   bool       `json:"submitted" example:"true"`
	Lampiran    []string   `json:"lampiran" example:"[\"/uploads/2026/11/08/abc.pdf\"]"`
	Dibuat      *time.Time `json:"dibuat,omitempty" example:"2026-01-01T07:00:00+07:00"`
}

type PengumpulanItemResponse struct {
	ID             int32     `json:"id" example:"1"`
	PostinganID    int32     `json:"postingan_id" example:"1"`
	PenggunaID     int32     `json:"pengguna_id" example:"2"`
	Nama           string    `json:"nama" example:"Budi Santoso"`
	Email          string    `json:"email" example:"budi@example.com"`
	ProfilePicture string    `json:"profile_picture" example:"uploads/pfp/2026/11/10/user2_abc.jpg"`
	Lampiran       []string  `json:"lampiran" example:"[\"/uploads/2026/11/08/abc.pdf\"]"`
	Dibuat         time.Time `json:"dibuat" example:"2026-01-01T07:00:00+07:00"`
}

type TarikPengumpulanResponse struct {
	Message  string   `json:"message" example:"Pengumpulan tugas berhasil ditarik"`
	Lampiran []string `json:"lampiran" example:"[\"/uploads/2026/11/08/abc.pdf\"]"`
}
