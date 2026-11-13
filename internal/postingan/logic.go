package postingan

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path-api/internal/db"
	"path-api/internal/utils"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

const maxLampiran = 5

// uploadPathPattern matches the paths produced by files.SavePostinganFiles:
// /uploads/YYYY/MM/DD/<32 hex chars>.<ext>
var uploadPathPattern = regexp.MustCompile(`^uploads/\d{4}/\d{2}/\d{2}/[a-f0-9]{32}\.[a-z0-9]+$`)

// Create godoc
//
//	@Summary		Buat Postingan
//	@Description	Membuat postingan baru (materi atau tugas) pada kelas tertentu
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas		path		string					true	"Kode Kelas"
//	@Param			request			body		CreatePostinganPayload	true	"Create Postingan Request Payload"
//	@Success		201				{object}	PostinganResponse
//	@Failure		400,401,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/posts [post]
func (h *Handler) Create(c *echo.Context) error {
	req := new(CreatePostinganRequest)

	// Proses binding request body (JSON) dan parameter URL (kode_kelas)
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

	kelasID, err := h.db.GetKelasID(c.Request().Context(), req.KodeKelas)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
	}

	var deskripsiParam pgtype.Text
	if req.Deskripsi != "" {
		deskripsiParam = pgtype.Text{String: req.Deskripsi, Valid: true}
	}

	lampiran, err := normalizeLampiran(req.Lampiran)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	// Simpan data ke database
	post, err := h.db.CreatePostingan(c.Request().Context(), db.CreatePostinganParams{
		Judul:      req.Judul,
		Deskripsi:  deskripsiParam,
		Lampiran:   lampiran,
		TipeID:     req.TipeID,
		PenggunaID: claims.UserID,
		KelasID:    kelasID,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat postingan")
	}

	return c.JSON(http.StatusCreated, post)
}

// List godoc
//
//	@Summary		List Postingan
//	@Description	Mendapatkan daftar postingan pada kelas berdasarkan tipe dan mata pelajaran
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas	path		string	true	"Kode Kelas"
//	@Param			tipe		query		int		true	"Tipe Postingan (1: Tugas, 2: Materi)"
//	@Param			mapel		query		string	false	"Mata Pelajaran"
//	@Success		200			{object}	[]PostinganResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/posts [get]
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
		TipeID:        req.TipeID,
		PenggunaID:    claims.UserID,
		KelasID:       kelasID,
		MataPelajaran: mapelParam,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil postingan")
	}
	return c.JSON(http.StatusOK, tugas)
}

// Edit godoc
//
//	@Summary		Edit Postingan
//	@Description	Memperbarui postingan yang ada pada kelas tertentu
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas	path		string					true	"Kode Kelas"
//	@Param			postinganID	path		int						true	"Postingan ID"
//	@Param			request		body		EditPostinganPayload	true	"Edit Postingan Request Payload"
//	@Success		200			{object}	PostinganResponse
//	@Failure		400,500		{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/postingan/{postinganID} [patch]
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

	// A nil Lampiran means "keep the existing attachments". An empty slice
	// explicitly clears them.
	var lampiran []string
	if req.Lampiran != nil {
		var err error
		lampiran, err = normalizeLampiran(*req.Lampiran)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}
	}

	updatedPost, err := h.db.UpdatePostingan(c.Request().Context(), db.UpdatePostinganParams{
		ID:        req.PostinganID,
		Judul:     req.Judul,
		Deskripsi: deskripsiParam,
		Lampiran:  lampiran,
		TipeID:    req.TipeID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui postingan")
	}

	return c.JSON(http.StatusOK, updatedPost)
}

// ListTipe godoc
//
//	@Summary		List Tipe Postingan
//	@Description	Mendapatkan daftar tipe postingan yang tersedia
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	[]TipePostinganResponse
//	@Failure		500	{object}	utils.HTTPResponse
//	@Router			/posts/tipe [get]
func (h *Handler) ListTipe(c *echo.Context) error {
	tipe, err := h.db.ListTipePostingan(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui postingan")
	}

	return c.JSON(http.StatusOK, tipe)
}

// normalizeLampiran accepts either an http(s) link or a path returned by
// POST /files/postingan, and enforces the maximum attachment count.
func normalizeLampiran(lampiran []string) ([]string, error) {
	if len(lampiran) > maxLampiran {
		return nil, fmt.Errorf("lampiran maksimal %d", maxLampiran)
	}
	if lampiran == nil {
		return nil, nil
	}

	normalized := make([]string, 0, len(lampiran))
	for _, item := range lampiran {
		item = strings.TrimSpace(item)
		if item == "" {
			return nil, errors.New("lampiran tidak boleh kosong")
		}

		if p, ok := normalizeUploadPath(item); ok {
			normalized = append(normalized, p)
			continue
		}

		u, err := url.Parse(item)
		if err != nil {
			return nil, fmt.Errorf("lampiran %q harus berupa link http(s) atau file yang diunggah", item)
		}
		scheme := strings.ToLower(u.Scheme)
		if u.Host == "" || (scheme != "http" && scheme != "https") {
			return nil, fmt.Errorf("lampiran %q harus berupa link http(s) atau file yang diunggah", item)
		}
		normalized = append(normalized, item)
	}

	return normalized, nil
}

// normalizeUploadPath turns "uploads/..." into "/uploads/..." and only accepts
// paths with the shape produced by the upload endpoint.
func normalizeUploadPath(item string) (string, bool) {
	p := strings.TrimPrefix(item, "/")
	if !uploadPathPattern.MatchString(p) {
		return "", false
	}
	return "/" + p, true
}
