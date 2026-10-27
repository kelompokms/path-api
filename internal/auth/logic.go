package auth

import (
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/labstack/echo/v5"
	"golang.org/x/crypto/bcrypt"
)

// Register godoc
//
//	@Summary		Register pengguna baru
//	@Description	Mendaftarkan pengguna baru dengan Username, Email dan Password, mengembalikan JWT token untuk Bearer Auth
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		RegisterRequest	true	"Register Request Payload"
//	@Success		200		{object}	SuccessResponse
//	@Failure		400,500	{object}	utils.HTTPResponse
//	@Router			/register [post]
func (h *Handler) Register(c *echo.Context) error {
	req := new(RegisterRequest)

	err := c.Bind(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err = c.Validate(req)
	if err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	_, err = h.db.GetPasswordPenggunaByEmail(c.Request().Context(), req.Email)
	if err == nil {
		log.Println("email sudah terdaftarkan:", err)
		return echo.NewHTTPError(http.StatusBadRequest, "email sudah terdaftarkan")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("Error hashing password:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memproses register")
	}

	userID, err := h.db.CreatePengguna(c.Request().Context(), db.CreatePenggunaParams{
		Username: req.Username,
		Email:    req.Email,
		Password: string(hashedPassword),
	})
	if err != nil {
		log.Println("Error insert pengguna:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memproses register")
	}

	token, err := utils.NewToken(userID)
	if err != nil {
		log.Println("Error membuat token:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memproses register")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"token": token,
	})
}

// Login godoc
//
//	@Summary		Login pengguna
//	@Description	Login pengguna baru dengan Email dan Password, mengembalikan JWT token untuk Bearer Auth
//	@Tags			Auth
//	@Accept			json
//	@Produce		json
//	@Param			request	body		LoginRequest	true	"Login Request Payload"
//	@Success		200		{object}	SuccessResponse
//	@Failure		400,500	{object}	utils.HTTPResponse
//	@Router			/login [post]
func (h *Handler) Login(c *echo.Context) error {
	req := new(LoginRequest)

	err := c.Bind(req)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err = c.Validate(req)
	if err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	pengguna, err := h.db.GetPasswordPenggunaByEmail(c.Request().Context(), req.Email)
	if err != nil {
		log.Println("username atau password salah", err)
		return echo.NewHTTPError(http.StatusBadRequest, "username atau password salah")
	}

	err = bcrypt.CompareHashAndPassword([]byte(pengguna.Password), []byte(req.Password))
	if err != nil {
		log.Println("Error mengecek password:", err)
		return echo.NewHTTPError(http.StatusBadRequest, "username atau password salah")
	}

	token, err := utils.NewToken(pengguna.ID)
	if err != nil {
		log.Println("Error membuat token:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memproses login")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"token": token,
	})
}
