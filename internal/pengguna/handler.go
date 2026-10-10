package pengguna

import (
	"path-api/internal/db"
)

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

type UpdateBiodataRequest struct {
	Email          string  `json:"email" validate:"required,email"`
	Phone          *string `json:"phone"`
	ProfilePicture *string `json:"profile_picture"`
}

type ResetPasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}