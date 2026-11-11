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
	Nama        string `json:"nama" validate:"required"`
	Tingkat     int16  `json:"tingkat" validate:"required"`
	TahunAjaran string `json:"tahun_ajaran" validate:"required"`
}

type ListKelasResponse struct {
	ID          int32     `json:"id" example:"2"`
	Nama        string    `json:"nama" example:"Pemrograman Dasar"`
	Tingkat     int16     `json:"tingkat" example:"10"`
	TahunAjaran string    `json:"tahun_ajaran" example:"2026/2027"`
	Pengampu    int32     `json:"pengampu" example:"1"`
	KodeKelas   string    `json:"kode_kelas" example:"A18D23"`
	Dibuat      time.Time `json:"dibuat" example:"2006-01-02T15:04:05Z07:00"`
}

type KodeKelasRequest struct {
	KodeKelas string `json:"kode_kelas" param:"kode_kelas" validate:"required"`
}

type SuccessResponse struct {
	Message string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}
