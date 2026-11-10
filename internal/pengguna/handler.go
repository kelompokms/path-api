package pengguna

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

type UpdateEmailRequest struct {
	Email string `json:"email" validate:"required,email"`
}

type UpdatePhoneRequest struct {
	Phone string `json:"phone" validate:"required,e164"`
}

type UpdateProfilePictureRequest struct {
	ProfilePicture *multipart.FileHeader `form:"profile_picture" validate:"required"`
}

type ResetPasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required,min=8"`
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type GetPenggunaSuccessResponse struct {
	ID             int32  `json:"id" example:"1"`
	Username       string `json:"username" example:"John Doe"`
	Email          string `json:"email" example:"johndoe@example.com"`
	Phone          string `json:"phone" example:"123123123"`
	ProfilePicture string `json:"profile_picture" example:"joe_mama.jpg"`
}

type GetPenggunaFailure401Response struct {
	Message string `json:"message" example:"Tidak dapat mengakses data klaim"`
}

type GetPenggunaFailure400Response struct {
	Message string `json:"message" example:"Data pengguna tidak ditemukan"`
}

type PenggunaDataResponse struct {
	ID             int32  `json:"id" example:"1"`
	Nama           string `json:"nama" example:"John Doe"`
	Email          string `json:"email" example:"johndoe@example.com"`
	Phone          string `json:"phone" example:"08123456789"`
	ProfilePicture string `json:"profile_picture" example:"uploads/pfp/2026/11/10/user1_abc.jpg"`
}

type UpdatePenggunaResponse struct {
	Message string               `json:"message" example:"Data berhasil diperbarui"`
	Data    PenggunaDataResponse `json:"data"`
}
