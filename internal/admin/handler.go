package admin

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

type UpdateGuruRequest struct {
	KodeKelas string `json:"kode_kelas" param:"kode_kelas" validate:"required"`
	RoleGuru  bool   `json:"role_guru" validate:"required"`
}

type UploadKalenderRequest struct {
	FileCSV *multipart.FileHeader `form:"file_csv" validate:"required"`
}
