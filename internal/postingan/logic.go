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
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

const (
	maxLampiran = 5
	tipeTugasID = 1
)

// uploadPathPattern matches the paths produced by files.SavePostinganFiles:
// /uploads/YYYY/MM/DD/<32 hex chars>.<ext>
var uploadPathPattern = regexp.MustCompile(`^uploads/\d{4}/\d{2}/\d{2}/[a-f0-9]{32}\.[a-z0-9]+$`)

func toPostinganResponse(
	id int32,
	judul string,
	deskripsi pgtype.Text,
	lampiran []string,
	penggunaID, kelasID, tipeID int32,
	dibuat pgtype.Timestamp,
) PostinganResponse {
	if lampiran == nil {
		lampiran = []string{}
	}
	return PostinganResponse{
		ID:         id,
		Judul:      judul,
		Deskripsi:  deskripsi.String,
		Lampiran:   lampiran,
		PenggunaID: penggunaID,
		KelasID:    kelasID,
		TipeID:     tipeID,
		Dibuat:     dibuat.Time,
	}
}

func (h *Handler) authorizeGuruKelas(c *echo.Context, kodeKelas, forbiddenMsg string) (int32, int32, error) {
	claims, err := utils.GetClaims(c)
	if err != nil {
		log.Println(err)
		return 0, 0, echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	kelasID, err := h.db.GetKelasID(c.Request().Context(), kodeKelas)
	if err != nil {
		log.Println(err)
		return 0, 0, echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
	}

	isGuru, err := h.db.IsGuruKelas(c.Request().Context(), db.IsGuruKelasParams{
		PenggunaID: claims.UserID,
		KelasID:    kelasID,
	})
	if err != nil {
		log.Println(err)
		return 0, 0, echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa role kelas")
	}
	if !isGuru {
		return 0, 0, echo.NewHTTPError(http.StatusForbidden, forbiddenMsg)
	}

	return claims.UserID, kelasID, nil
}

// Create godoc
//
//	@Summary		Buat Postingan
//	@Description	Membuat postingan baru (materi atau tugas) pada kelas tertentu (khusus guru kelas)
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string					true	"Kode Kelas"
//	@Param			request				body		CreatePostinganRequest	true	"Create Postingan Request Payload"
//	@Success		201					{object}	PostinganResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
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

	userID, kelasID, err := h.authorizeGuruKelas(c, req.KodeKelas, "Hanya guru yang dapat membuat materi atau tugas")
	if err != nil {
		return err
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
		PenggunaID: userID,
		KelasID:    kelasID,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat postingan")
	}

	return c.JSON(http.StatusCreated, toPostinganResponse(
		post.ID,
		post.Judul,
		post.Deskripsi,
		post.Lampiran,
		post.PenggunaID,
		post.KelasID,
		post.TipeID,
		post.Dibuat,
	))
}

// List godoc
//
//	@Summary		List Postingan
//	@Description	Mendapatkan daftar postingan pada kelas, opsional difilter berdasarkan tipe postingan
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string	true	"Kode Kelas"
//	@Param			tipe				query		int		false	"Tipe Postingan (1: Tugas, 2: Materi)"
//	@Success		200					{object}	[]PostinganResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
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

	ctx := c.Request().Context()
	kelasID, err := h.db.GetKelasID(ctx, req.KodeKelas)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
		}
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa kelas")
	}

	isMember, err := h.db.IsAnggotaKelas(ctx, db.IsAnggotaKelasParams{
		KodeKelas:  req.KodeKelas,
		PenggunaID: claims.UserID,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa keanggotaan kelas")
	}
	if !isMember {
		return echo.NewHTTPError(http.StatusForbidden, "Anda bukan anggota kelas ini")
	}

	rows, err := h.db.ListPostingan(ctx, db.ListPostinganParams{
		KelasID: kelasID,
		TipeID: pgtype.Int4{
			Int32: req.TipeID,
			Valid: req.TipeID > 0,
		},
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil postingan")
	}

	posts := make([]PostinganResponse, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, toPostinganResponse(
			row.ID,
			row.Judul,
			row.Deskripsi,
			row.Lampiran,
			row.PenggunaID,
			row.KelasID,
			row.TipeID,
			row.Dibuat,
		))
	}

	return c.JSON(http.StatusOK, posts)
}

// Edit godoc
//
//	@Summary		Edit Postingan
//	@Description	Memperbarui postingan yang ada pada kelas tertentu (khusus guru kelas)
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string					true	"Kode Kelas"
//	@Param			postinganID			path		int						true	"Postingan ID"
//	@Param			request				body		EditPostinganRequest	true	"Edit Postingan Request Payload"
//	@Success		200					{object}	PostinganResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/postingan/{postinganID} [patch]
func (h *Handler) Edit(c *echo.Context) error {
	req := new(EditPostinganRequest)

	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	_, kelasID, err := h.authorizeGuruKelas(c, req.KodeKelas, "Hanya guru yang dapat mengubah postingan")
	if err != nil {
		return err
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
		KelasID:   kelasID,
		Judul:     req.Judul,
		Deskripsi: deskripsiParam,
		Lampiran:  lampiran,
		TipeID:    req.TipeID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Postingan tidak ditemukan")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui postingan")
	}

	return c.JSON(http.StatusOK, toPostinganResponse(
		updatedPost.ID,
		updatedPost.Judul,
		updatedPost.Deskripsi,
		updatedPost.Lampiran,
		updatedPost.PenggunaID,
		updatedPost.KelasID,
		updatedPost.TipeID,
		updatedPost.Dibuat,
	))
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

func toPengumpulanResponse(row db.PengumpulanTuga) PengumpulanResponse {
	lampiran := row.Lampiran
	if lampiran == nil {
		lampiran = []string{}
	}
	var dibuat *time.Time
	if row.Dibuat.Valid {
		t := row.Dibuat.Time
		dibuat = &t
	}
	return PengumpulanResponse{
		ID:          row.ID,
		PostinganID: row.PostinganID,
		PenggunaID:  row.PenggunaID,
		Submitted:   true,
		Lampiran:    lampiran,
		Dibuat:      dibuat,
	}
}

func (h *Handler) authorizeMuridTugas(c *echo.Context, kodeKelas string, postinganID int32) (int32, error) {
	ctx := c.Request().Context()

	claims, err := utils.GetClaims(c)
	if err != nil {
		return 0, echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	kelasID, err := h.db.GetKelasID(ctx, kodeKelas)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
		}
		log.Println(err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa kelas")
	}

	isMember, err := h.db.IsAnggotaKelas(ctx, db.IsAnggotaKelasParams{
		KodeKelas:  kodeKelas,
		PenggunaID: claims.UserID,
	})
	if err != nil {
		log.Println(err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa keanggotaan kelas")
	}
	if !isMember {
		return 0, echo.NewHTTPError(http.StatusForbidden, "Anda bukan anggota kelas ini")
	}

	isGuru, err := h.db.IsGuruKelas(ctx, db.IsGuruKelasParams{
		PenggunaID: claims.UserID,
		KelasID:    kelasID,
	})
	if err != nil {
		log.Println(err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa role kelas")
	}
	if isGuru {
		return 0, echo.NewHTTPError(http.StatusForbidden, "Hanya murid yang dapat mengelola pengumpulan tugas")
	}

	post, err := h.db.GetPostinganByID(ctx, db.GetPostinganByIDParams{
		ID:      postinganID,
		KelasID: kelasID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, echo.NewHTTPError(http.StatusNotFound, "Postingan tidak ditemukan")
		}
		log.Println(err)
		return 0, echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa postingan")
	}
	if post.TipeID != tipeTugasID {
		return 0, echo.NewHTTPError(http.StatusBadRequest, "Pengumpulan hanya berlaku untuk postingan tugas")
	}

	return claims.UserID, nil
}

// GetPengumpulan godoc
//
//	@Summary		Get Pengumpulan Tugas Murid
//	@Description	Mendapatkan status dan lampiran pengumpulan tugas milik murid yang sedang login
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string	true	"Kode Kelas"
//	@Param			postinganID			path		int		true	"Postingan ID"
//	@Success		200					{object}	PengumpulanResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/postingan/{postinganID}/pengumpulan [get]
func (h *Handler) GetPengumpulan(c *echo.Context) error {
	req := new(PengumpulanPathRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	userID, err := h.authorizeMuridTugas(c, req.KodeKelas, req.PostinganID)
	if err != nil {
		return err
	}

	row, err := h.db.GetPengumpulanTugas(c.Request().Context(), db.GetPengumpulanTugasParams{
		PostinganID: req.PostinganID,
		PenggunaID:  userID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(http.StatusOK, PengumpulanResponse{
				PostinganID: req.PostinganID,
				PenggunaID:  userID,
				Submitted:   false,
				Lampiran:    []string{},
			})
		}
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memuat pengumpulan tugas")
	}

	return c.JSON(http.StatusOK, toPengumpulanResponse(row))
}

// SubmitPengumpulan godoc
//
//	@Summary		Kumpulkan Tugas
//	@Description	Mengumpulkan atau memperbarui pengumpulan tugas murid (lampiran kosong untuk tandai selesai)
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string						true	"Kode Kelas"
//	@Param			postinganID			path		int							true	"Postingan ID"
//	@Param			request				body		SubmitPengumpulanRequest	true	"Submit Pengumpulan Request Payload"
//	@Success		200					{object}	PengumpulanResponse
//	@Success		201					{object}	PengumpulanResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/postingan/{postinganID}/pengumpulan [post]
func (h *Handler) SubmitPengumpulan(c *echo.Context) error {
	req := new(SubmitPengumpulanRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	userID, err := h.authorizeMuridTugas(c, req.KodeKelas, req.PostinganID)
	if err != nil {
		return err
	}

	lampiran, err := normalizeLampiran(req.Lampiran)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if lampiran == nil {
		lampiran = []string{}
	}

	ctx := c.Request().Context()
	existing, err := h.db.GetPengumpulanTugas(ctx, db.GetPengumpulanTugasParams{
		PostinganID: req.PostinganID,
		PenggunaID:  userID,
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa pengumpulan tugas")
	}

	if errors.Is(err, pgx.ErrNoRows) {
		created, err := h.db.CreatePengumpulanTugas(ctx, db.CreatePengumpulanTugasParams{
			PostinganID: req.PostinganID,
			PenggunaID:  userID,
			Lampiran:    lampiran,
		})
		if err != nil {
			log.Println(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengumpulkan tugas")
		}
		return c.JSON(http.StatusCreated, toPengumpulanResponse(created))
	}

	updated, err := h.db.UpdatePengumpulanTugas(ctx, db.UpdatePengumpulanTugasParams{
		ID:         existing.ID,
		PenggunaID: userID,
		Lampiran:   lampiran,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui pengumpulan tugas")
	}
	return c.JSON(http.StatusOK, toPengumpulanResponse(updated))
}

// TarikPengumpulan godoc
//
//	@Summary		Tarik Pengumpulan Tugas
//	@Description	Menarik kembali pengumpulan tugas yang telah diserahkan oleh murid
//	@Tags			Postingan
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string	true	"Kode Kelas"
//	@Param			postinganID			path		int		true	"Postingan ID"
//	@Success		200					{object}	TarikPengumpulanResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/postingan/{postinganID}/pengumpulan [delete]
func (h *Handler) TarikPengumpulan(c *echo.Context) error {
	req := new(PengumpulanPathRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	userID, err := h.authorizeMuridTugas(c, req.KodeKelas, req.PostinganID)
	if err != nil {
		return err
	}

	deleted, err := h.db.DeletePengumpulanTugas(c.Request().Context(), db.DeletePengumpulanTugasParams{
		PostinganID: req.PostinganID,
		PenggunaID:  userID,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menarik pengumpulan tugas")
	}
	if len(deleted) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "Pengumpulan tugas tidak ditemukan")
	}

	latest := deleted[0]
	for _, row := range deleted[1:] {
		if row.ID > latest.ID {
			latest = row
		}
	}
	lampiran := latest.Lampiran
	if lampiran == nil {
		lampiran = []string{}
	}

	return c.JSON(http.StatusOK, TarikPengumpulanResponse{
		Message:  "Pengumpulan tugas berhasil ditarik",
		Lampiran: lampiran,
	})
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
