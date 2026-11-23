package kelas

import (
	"path-api/internal/db"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

type CreateKelasRequest struct {
	Nama        string `json:"nama" validate:"required"`
	Deskripsi   string `json:"deskripsi"`
	Tingkat     int16  `json:"tingkat" validate:"required"`
	TahunAjaran string `json:"tahun_ajaran" validate:"required"`
}

type UpdateKelasRequest struct {
	KodeKelas string  `json:"-" param:"kode_kelas" validate:"required"`
	Nama      *string `json:"nama"`
	Deskripsi *string `json:"deskripsi"`
}

type UpdateKelasPayload struct {
	Nama      *string `json:"nama" example:"X PPLG 1"`
	Deskripsi *string `json:"deskripsi" example:"Belajar algoritma dan struktur data"`
}

type ListKelasResponse struct {
	ID           int32     `json:"id" example:"2"`
	Nama         string    `json:"nama" example:"X PPLG 1"`
	Deskripsi    string    `json:"deskripsi" example:"Belajar algoritma dan struktur data"`
	Tingkat      int16     `json:"tingkat" example:"10"`
	TahunAjaran  string    `json:"tahun_ajaran" example:"2026/2027"`
	Pengampu     int32     `json:"pengampu" example:"1"`
	NamaPengampu string    `json:"nama_pengampu,omitempty" example:"Eko Kusnandar"`
	KodeKelas    string    `json:"kode_kelas" example:"A18D23"`
	Dibuat       time.Time `json:"dibuat" example:"2026-01-01T07:00:00+07:00"`
	Guru         bool      `json:"guru" example:"true"`
}

type DetailKelasResponse struct {
	ID           int32       `json:"id"`
	Nama         string      `json:"nama"`
	Deskripsi    string      `json:"deskripsi"`
	Tingkat      int16       `json:"tingkat"`
	TahunAjaran  string      `json:"tahun_ajaran"`
	KodeKelas    string      `json:"kode_kelas"`
	Dibuat       time.Time   `json:"dibuat"`
	Pengampu     pgtype.Int4 `json:"pengampu"`
	NamaPengampu string      `json:"nama_pengampu,omitempty"`
	Guru         bool        `json:"guru"`
}

type AnggotaKelasResponse struct {
	PenggunaID     int32     `json:"pengguna_id" example:"2"`
	Nama           string    `json:"nama" example:"Budi Santoso"`
	Email          string    `json:"email" example:"budi@example.com"`
	ProfilePicture string    `json:"profile_picture" example:"uploads/pfp/2026/11/10/user2_abc.jpg"`
	Guru           bool      `json:"guru" example:"false"`
	Utama          bool      `json:"utama" example:"false"`
	Role           string    `json:"role" example:"murid"`
	Dibuat         time.Time `json:"dibuat" example:"2026-11-16T15:00:00+07:00"`
}

type KodeKelasRequest struct {
	KodeKelas string `json:"kode_kelas" param:"kode_kelas" validate:"required"`
}

type PromoteGuruRequest struct {
	KodeKelas  string `param:"kode_kelas" validate:"required"`
	PenggunaID int32  `param:"pengguna_id" validate:"required,gt=0"`
}

type SuccessResponse struct {
	Message string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}
