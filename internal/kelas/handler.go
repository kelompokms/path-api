package kelas

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

type CreateKelasRequest struct {
	Nama    string `json:"nama" validate:"required"`
	Tingkat string `json:"tingkat"`
}

type ListKelasResponse struct {
	ID        int32     `json:"id" example:"2"`
	Nama      string    `json:"nama" example:"Pemrograman Dasar"`
	Tingkat   string    `json:"tingkat" example:"SMA"`
	Pembuat   int32     `json:"pembuat" example:"1"`
	KodeKelas string    `json:"kode_kelas" example:"A18D23"`
	Dibuat    time.Time `json:"dibuat" example:"2006-01-02T15:04:05Z07:00"`
}

type KodeKelasRequest struct {
	KodeKelas string `json:"kode_kelas" param:"kode_kelas" validate:"required"`
}

type SuccessResponse struct {
	Message string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}
