package postingan

import (
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/labstack/echo/v5"
)
func (h *Handler) CreatePostingan(c *echo.Context) error {
	req := new(CreatePostinganRequest)
	
	// Proses binding request body (JSON) dan parameter URL (kelasID)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	// Validasi input
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	// Ambil ID pengguna dari token JWT (menggunakan utilitas yang sama seperti ListPostingan)
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	// Simpan data ke database
	post, err := h.db.CreatePostingan(c.Request().Context(), db.CreatePostinganParams{
		Judul:      req.Judul,
		Deskripsi:  req.Deskripsi,
		Lampiran:   req.Lampiran,
		Tipe:       req.Tipe,
		PenggunaID: int32(claims.UserID),
		KelasID:    req.KelasID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat postingan")
	}

	return c.JSON(http.StatusCreated, post)
}
func (h *Handler) ListPostingan(c *echo.Context) error {
	req := new(ListPostinganRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}
	tugas, err := h.db.ListPostingan(c.Request().Context(), db.ListPostinganParams{
		PenggunaID: int32(claims.UserID),
		KelasID:    req.KelasID,
		Tipe:       req.Tipe,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil data "+string(req.Tipe))
	}
	return c.JSON(http.StatusOK, tugas)
}


