package admin

import (
	"mime/multipart"
	"path-api/internal/db"
	"time"
)

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

type UpdateGuruRequest struct {
	KodeKelas string `json:"kode_kelas" param:"kode_kelas" validate:"required"`
	RoleGuru  bool   `json:"role_guru" validate:"required"`
}

type UploadKalenderRequest struct {
	FileCSV *multipart.FileHeader `form:"file_csv" validate:"required"`
}

type UpdateGuruPayload struct {
	RoleGuru bool `json:"role_guru" validate:"required" example:"true"`
}

type ListAdminResponse struct {
	ID             int32     `json:"id" example:"1"`
	Nama           string    `json:"nama" example:"Admin"`
	Email          string    `json:"email" example:"admin@example.com"`
	Phone          string    `json:"phone" example:"08123456789"`
	ProfilePicture string    `json:"profile_picture" example:"uploads/pfp/admin.jpg"`
	Admin          bool      `json:"admin" example:"true"`
	Dibuat         time.Time `json:"dibuat" example:"2026-01-01T00:00:00Z"`
}

type ListGuruResponse struct {
	KelasID      int32  `json:"kelas_id" example:"1"`
	NamaKelas    string `json:"nama_kelas" example:"Matematika"`
	PenggunaID   int32  `json:"pengguna_id" example:"2"`
	NamaPengguna string `json:"nama_pengguna" example:"Guru Budi"`
}

