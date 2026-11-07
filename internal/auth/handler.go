package auth

import (
	"path-api/internal/db"
)

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"johndoe@example.com"`
	Password string `json:"password" validate:"required,min=8" example:"secret123"`
}

type RegisterRequest struct {
	Nama     string `json:"username" validate:"required" example:"John Doe"`
	Email    string `json:"email" validate:"required,email" example:"johndoe@example.com"`
	Password string `json:"password" validate:"required,min=8" example:"secret123"`
}

type SuccessResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}
