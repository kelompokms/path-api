package pengguna

import (
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/files"
	"path-api/internal/utils"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"golang.org/x/crypto/bcrypt"
)

// Get godoc
//
//	@Summary		Mendapatkan informasi pengguna
//	@Description	Mendapatkan informasi pengguna dengan parsing ID pengguna dari token
//	@Tags			Pengguna
//	@Produce		json
//	@Param			Authorization	header		string	true	"Token"
//	@Success		200				{object}	GetPenggunaSuccessResponse
//	@Failure		401				{object}	GetPenggunaFailure401Response
//	@Failure		400				{object}	GetPenggunaFailure400Response
//	@Router			/pengguna [get]
func (h *Handler) Get(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	pengguna, err := h.db.GetPengguna(c.Request().Context(), claims.UserID)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusNotFound, "Data pengguna tidak ditemukan")
	}

	return c.JSON(http.StatusOK, pengguna)
}

func (h *Handler) UpdateEmail(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	req := new(UpdateEmailRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	_, err = h.db.GetPasswordPenggunaByEmail(c.Request().Context(), req.Email)
	if err == nil {
		log.Println("email sudah terdaftarkan")
		return echo.NewHTTPError(http.StatusBadRequest, "email sudah terdaftarkan!")
	}

	updatedUser, err := h.db.UpdateEmailPengguna(c.Request().Context(), db.UpdateEmailPenggunaParams{
		ID:    claims.UserID,
		Email: req.Email,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui email")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Email berhasil diperbarui",
		"data":    updatedUser,
	})
}

func (h *Handler) UpdatePhone(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	req := new(UpdatePhoneRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	phonePg := pgtype.Text{String: req.Phone, Valid: true}

	updatedUser, err := h.db.UpdatePhonePengguna(c.Request().Context(), db.UpdatePhonePenggunaParams{
		ID:    claims.UserID,
		Phone: phonePg,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui nomor HP")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Nomor HP berhasil diperbarui",
		"data":    updatedUser,
	})
}

func (h *Handler) UpdateProfilePicture(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	req := new(UpdateProfilePictureRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "request tidak sesuai")
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	path, err := files.SaveProfilePicture(claims.UserID, req.ProfilePicture)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui foto profil")
	}

	updatedPengguna, err := h.db.UpdateProfilePicturePengguna(c.Request().Context(), db.UpdateProfilePicturePenggunaParams{
		ID:             claims.UserID,
		ProfilePicture: pgtype.Text{String: path, Valid: true},
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui foto profil")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Foto profil berhasil diperbarui",
		"data":    updatedPengguna,
	})
}

func (h *Handler) ResetPassword(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses informasi user")
	}

	req := new(ResetPasswordRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	password, err := h.db.GetPasswordPenggunaByID(c.Request().Context(), claims.UserID)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusNotFound, "Pengguna tidak ditemukan")
	}

	err = bcrypt.CompareHashAndPassword([]byte(password), []byte(req.OldPassword))
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Password tidak sesuai dengan password lama")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memproses password baru")
	}

	err = h.db.UpdatePasswordPengguna(c.Request().Context(), db.UpdatePasswordPenggunaParams{
		ID:       claims.UserID,
		Password: string(hashedPassword),
	})

	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mereset password")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Reset password berhasil",
	})
}
