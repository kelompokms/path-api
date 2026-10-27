package postingan

import (
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

func (h *Handler) Create(c *echo.Context) error {
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
		log.Println(err)
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	var deskripsiParam pgtype.Text
	if req.Deskripsi != "" {
		deskripsiParam = pgtype.Text{String: req.Deskripsi, Valid: true}
	}

	log.Println(claims.UserID)

	// Simpan data ke database
	post, err := h.db.CreatePostingan(c.Request().Context(), db.CreatePostinganParams{
		Judul:      req.Judul,
		Deskripsi:  deskripsiParam,
		Lampiran:   req.Lampiran,
		Tipe:       req.Tipe,
		PenggunaID: claims.UserID,
		KelasID:    req.KelasID,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat postingan")
	}

	return c.JSON(http.StatusCreated, post)
}
func (h *Handler) List(c *echo.Context) error {
	var req ListPostinganRequest

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	kelasID, err := h.db.GetKelasID(c.Request().Context(), req.KodeKelas)

	// Konversi string req.Mapel menjadi pgtype.Text yang diminta oleh sqlc
	mapelParam := pgtype.Text{
		String: req.Mapel,
		Valid:  req.Mapel != "",
	}

	tugas, err := h.db.ListPostingan(c.Request().Context(), db.ListPostinganParams{
		Tipe:          req.Tipe,
		PenggunaID:    claims.UserID,
		KelasID:       kelasID,
		MataPelajaran: mapelParam,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil data "+string(req.Tipe))
	}
	return c.JSON(http.StatusOK, tugas)
}
func (h *Handler) Edit(c *echo.Context) error {
	req := new(EditPostinganRequest)

	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	var deskripsiParam pgtype.Text
	if req.Deskripsi != "" {
		deskripsiParam = pgtype.Text{String: req.Deskripsi, Valid: true}
	}

	updatedPost, err := h.db.UpdatePostingan(c.Request().Context(), db.UpdatePostinganParams{
		ID:        req.PostinganID,
		Judul:     req.Judul,
		Deskripsi: deskripsiParam,
		Tipe:      req.Tipe,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui postingan")
	}

	return c.JSON(http.StatusOK, updatedPost)
}
