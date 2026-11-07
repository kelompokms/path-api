package admin

import "path-api/internal/db"

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

type UpdateGuruRequest struct {
	KodeKelas string `param:"kode_kelas" validate:"required"`
	RoleGuru  bool   `json:"role_guru" validate:"required"`
}
