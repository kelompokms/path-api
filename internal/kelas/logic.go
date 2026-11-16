package kelas

import (
	"errors"
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

// List godoc
//
//	@Summary		List Kelas
//	@Description	List kelas menggunakan Bearer Token dan mengembalikan response list kelas beserta status role guru
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			guru		query		bool	false	"Filter berdasarkan role guru (true: pengajar, false: murid)"
//	@Success		200			{object}	[]ListKelasResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/kelas [get]
func (h *Handler) List(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	guruFilterParam := strings.TrimSpace(c.QueryParam("guru"))
	var filterByGuru *bool
	if guruFilterParam != "" {
		switch strings.ToLower(guruFilterParam) {
		case "true":
			v := true
			filterByGuru = &v
		case "false":
			v := false
			filterByGuru = &v
		default:
			return echo.NewHTTPError(http.StatusBadRequest, "Parameter guru harus true atau false")
		}
	}

	rows, err := h.db.ListKelas(c.Request().Context(), claims.UserID)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil daftar kelas")
	}

	kelas := make([]db.ListKelasRow, 0, len(rows))
	for _, row := range rows {
		if filterByGuru != nil && row.Guru != *filterByGuru {
			continue
		}
		kelas = append(kelas, row)
	}

	return c.JSON(http.StatusOK, kelas)
}

// Get godoc
//
//	@Summary		Get Detail Kelas
//	@Description	Mendapatkan detail informasi kelas berdasarkan kode kelas
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas	path		string	true	"Kode Kelas"
//	@Success		200			{object}	ListKelasResponse
//	@Failure		400,500		{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas} [get]
func (h *Handler) Get(c *echo.Context) error {
	var req KodeKelasRequest

	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	kelas, err := h.db.GetKelas(c.Request().Context(), req.KodeKelas)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Kelas tidak ditemukan")
	}

	isGuru := false
	if claims, err := utils.GetClaims(c); err == nil {
		isGuru, err = h.db.IsGuruKelas(c.Request().Context(), db.IsGuruKelasParams{
			PenggunaID: claims.UserID,
			KelasID:    kelas.ID,
		})
		if err != nil {
			log.Println(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa role kelas")
		}
	}

	return c.JSON(http.StatusOK, DetailKelasResponse{
		ID:          kelas.ID,
		Nama:        kelas.Nama,
		Tingkat:     kelas.Tingkat,
		TahunAjaran: kelas.TahunAjaran,
		KodeKelas:   kelas.KodeKelas,
		Dibuat:      kelas.Dibuat,
		Pengampu:    kelas.Pengampu,
		Guru:        isGuru,
	})
}

// ListAnggota godoc
//
//	@Summary		List Anggota Kelas
//	@Description	Mendapatkan daftar seluruh anggota kelas (guru dan murid) berdasarkan kode kelas
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string	true	"Kode Kelas"
//	@Success		200					{object}	[]AnggotaKelasResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/anggota [get]
func (h *Handler) ListAnggota(c *echo.Context) error {
	var req KodeKelasRequest

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

	_, err = h.db.GetKelasID(c.Request().Context(), req.KodeKelas)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
		}
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa kelas")
	}

	isMember, err := h.db.IsAnggotaKelas(c.Request().Context(), db.IsAnggotaKelasParams{
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

	rows, err := h.db.ListAnggotaKelas(c.Request().Context(), req.KodeKelas)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil daftar anggota kelas")
	}

	anggota := make([]AnggotaKelasResponse, 0, len(rows))
	for _, row := range rows {
		role := "murid"
		if row.Guru {
			role = "guru"
		}
		anggota = append(anggota, AnggotaKelasResponse{
			PenggunaID:     row.PenggunaID,
			Nama:           row.Nama,
			Email:          row.Email,
			ProfilePicture: row.ProfilePicture.String,
			Guru:           row.Guru,
			Role:           role,
			Dibuat:         row.Dibuat.Time,
		})
	}

	return c.JSON(http.StatusOK, anggota)
}

// Create godoc
//
//	@Summary		Buat Kelas
//	@Description	Buat Kelas menggunakan nama dan tingkat dan mengembalikan message
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			request		body		CreateKelasRequest	true	"Create Kelas Request Payload"
//	@Success		200			{object}	utils.HTTPResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/kelas [post]
func (h *Handler) Create(c *echo.Context) error {
	req := CreateKelasRequest{}

	err := c.Bind(&req)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err = c.Validate(&req)
	if err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	kodeKelas := utils.GenerateHashCode()

	err = h.db.CreateKelas(c.Request().Context(), db.CreateKelasParams{
		Nama:        req.Nama,
		Tingkat:     req.Tingkat,
		TahunAjaran: req.TahunAjaran,
		Pengampu:    pgtype.Int4{Int32: claims.UserID, Valid: true},
		KodeKelas:   kodeKelas,
	})

	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat kelas")
	}

	return c.JSON(http.StatusCreated, utils.HTTPResponse{Message: "Kelas berhasil dibuat"})
}

// Join godoc
//
//	@Summary		Join Kelas
//	@Description	Join Kelas menggunakan kode dan mengembalikan message
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			request		path		KodeKelasRequest	true	"Create Kelas Request Payload"
//	@Success		200			{object}	utils.HTTPResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/gabung [post]
func (h *Handler) Join(c *echo.Context) error {
	var req KodeKelasRequest

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
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "kelas tidak ditemukan")
	}

	exists, err := h.db.CheckAnggotaKelas(c.Request().Context(), db.CheckAnggotaKelasParams{
		PenggunaID: claims.UserID,
		KodeKelas:  req.KodeKelas,
	})
	if exists != 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "anda sudah bergabung di kelas ini")
	}

	err = h.db.JoinKelas(c.Request().Context(), db.JoinKelasParams{
		PenggunaID: claims.UserID,
		KelasID:    kelasID,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal join kelas")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Berhasil bergabung ke dalam kelas",
	})
}

// Leave godoc
//
//	@Summary		Leave Kelas
//	@Description	Leave Kelas menggunakan kode dan mengembalikan message
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			request		path		KodeKelasRequest	true	"Leave Kelas Request Payload"
//	@Success		200			{object}	utils.HTTPResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/keluar [post]
func (h *Handler) Leave(c *echo.Context) error {
	var req KodeKelasRequest
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

	err = h.db.LeaveKelas(c.Request().Context(), db.LeaveKelasParams{
		KodeKelas:  req.KodeKelas,
		PenggunaID: claims.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal keluar dari kelas")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Berhasil keluar dari kelas",
	})
}
