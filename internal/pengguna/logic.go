package pengguna

import (
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"golang.org/x/crypto/bcrypt"
)

func (h *Handler) List(c *echo.Context) error {
	pengguna, err := h.db.ListPengguna(c.Request().Context())
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mendapatkan data pengguna")
	}

	return c.JSON(200, pengguna)
}

func (h *Handler) Get(c *echo.Context) error {
	// 	idStr := c.Param("id")
	//
	// 	id, _ := strconv.Atoi(idStr)
	//
	// 	pengguna, err := h.db.GetPengguna(c.Request().Context(), int32(id))
	// 	if err != nil {
	// 		log.Println(err)
	// 		return c.String(500, "gagal mendapatkan data pengguna")
	// 	}
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Format ID tidak valid")
	}

	pengguna, err := h.db.GetPenggunaByID(c.Request().Context(), int32(id))
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusNotFound, "Data pengguna tidak ditemukan")
	}

	return c.JSON(http.StatusOK, pengguna)
}

func (h *Handler) Create(c *echo.Context) error {
	return echo.NewHTTPError(400, "Bad Request")
}

func (h *Handler) Logout(c *echo.Context) error{
	return c.JSON(http.StatusOK, map[string]string{
		"message" : "Logout Berhasil",
	})
} 

func (h *Handler) UpdateBiodata(c *echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Format ID tidak valid")
	}

	req := new(UpdateBiodataRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}
	phonePg := pgtype.Text{Valid: false}
	if req.Phone != nil {
		phonePg = pgtype.Text{String: *req.Phone, Valid: true}
	}

	ppPg := pgtype.Text{Valid: false}
	if req.ProfilePicture != nil {
		ppPg = pgtype.Text{String: *req.ProfilePicture, Valid: true}
	}
	updatedUser, err := h.db.UpdateBiodataPengguna(c.Request().Context(), db.UpdateBiodataPenggunaParams{
		ID:             int32(id),
		Email:          req.Email,
		Phone:          phonePg,
		ProfilePicture: ppPg,
	})
	if err != nil {
		log.Println("Error update biodata:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui biodata")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Biodata berhasil diperbarui",
		"data": map[string]interface{}{
			"id":              updatedUser.ID,
			"username":        updatedUser.Username,
			"email":           updatedUser.Email,
			"phone":           updatedUser.Phone.String,
			"profile_picture": updatedUser.ProfilePicture.String,
		},
	})
}

func (h *Handler) ResetPassword(c *echo.Context) error {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Format ID tidak valid")
	}
	req := new(ResetPasswordRequest) 
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c,err)
	}
	user, err := h.db.GetPenggunaByID(c.Request().Context(), int32(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Pengguna tidak ditemukan")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil{
		return echo.NewHTTPError(http.StatusUnauthorized, "Password tidak sesuai dengan Password lama")
	}
	hashedNewPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memproses password baru")
	}
	err = h.db.UpdatePassword(c.Request().Context(), db.UpdatePasswordParams{
		ID:       user.ID,
		Password: string(hashedNewPassword),
	})
	if err != nil {
		log.Println("Error update password:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mereset password")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Reset password berhasil",
	})
}