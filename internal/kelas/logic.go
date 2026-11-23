package kelas

import (
	"errors"
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"
	"sort"
	"strconv"
	"strings"
	"time"

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

	kelas := make([]DetailKelasResponse, 0, len(rows))
	for _, row := range rows {
		if filterByGuru != nil && row.Guru != *filterByGuru {
			continue
		}
		kelas = append(kelas, DetailKelasResponse{
			ID:           row.ID,
			Nama:         row.Nama,
			Deskripsi:    row.Deskripsi,
			Tingkat:      row.Tingkat,
			TahunAjaran:  row.TahunAjaran,
			KodeKelas:    row.KodeKelas,
			Dibuat:       utils.ToGMT7(row.Dibuat.Time),
			Pengampu:     row.Pengampu,
			NamaPengampu: row.NamaPengampu,
			Guru:         row.Guru,
		})
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
		Deskripsi:   kelas.Deskripsi,
		Tingkat:     kelas.Tingkat,
		TahunAjaran: kelas.TahunAjaran,
		KodeKelas:   kelas.KodeKelas,
		Dibuat:      utils.ToGMT7(kelas.Dibuat.Time),
		Pengampu:    kelas.Pengampu,
		Guru:        isGuru,
	})
}

func (h *Handler) authorizeGuruKelas(c *echo.Context, kodeKelas, forbiddenMsg string) (int32, int32, error) {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return 0, 0, echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	kelasID, err := h.db.GetKelasID(c.Request().Context(), kodeKelas)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
		}
		log.Println(err)
		return 0, 0, echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa kelas")
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

// Update godoc
//
//	@Summary		Edit Kelas
//	@Description	Memperbarui nama dan/atau deskripsi kelas (khusus guru kelas)
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string				true	"Kode Kelas"
//	@Param			request				body		UpdateKelasPayload	true	"Update Kelas Request Payload"
//	@Success		200					{object}	ListKelasResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas} [patch]
func (h *Handler) Update(c *echo.Context) error {
	var req UpdateKelasRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	if _, _, err := h.authorizeGuruKelas(c, req.KodeKelas, "Hanya guru yang dapat mengubah informasi kelas"); err != nil {
		return err
	}

	if req.Nama == nil && req.Deskripsi == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Minimal nama atau deskripsi harus diisi")
	}

	var namaParam pgtype.Text
	if req.Nama != nil {
		trimmed := strings.TrimSpace(*req.Nama)
		if trimmed == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "Nama kelas tidak boleh kosong")
		}
		namaParam = pgtype.Text{String: trimmed, Valid: true}
	}

	var deskripsiParam pgtype.Text
	if req.Deskripsi != nil {
		deskripsiParam = pgtype.Text{String: strings.TrimSpace(*req.Deskripsi), Valid: true}
	}

	updated, err := h.db.UpdateKelas(c.Request().Context(), db.UpdateKelasParams{
		KodeKelas: req.KodeKelas,
		Nama:      namaParam,
		Deskripsi: deskripsiParam,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
		}
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui kelas")
	}

	return c.JSON(http.StatusOK, DetailKelasResponse{
		ID:          updated.ID,
		Nama:        updated.Nama,
		Deskripsi:   updated.Deskripsi,
		Tingkat:     updated.Tingkat,
		TahunAjaran: updated.TahunAjaran,
		KodeKelas:   updated.KodeKelas,
		Dibuat:      utils.ToGMT7(updated.Dibuat.Time),
		Pengampu:    updated.Pengampu,
		Guru:        true,
	})
}

// PromoteGuru godoc
//
//	@Summary		Angkat Murid Menjadi Guru
//	@Description	Mengubah role murid menjadi guru pada kelas tertentu (khusus guru kelas)
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string	true	"Kode Kelas"
//	@Param			pengguna_id			path		int		true	"ID Pengguna Murid"
//	@Success		200					{object}	utils.HTTPResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/anggota/{pengguna_id}/guru [patch]
func (h *Handler) PromoteGuru(c *echo.Context) error {
	var req PromoteGuruRequest
	req.KodeKelas = c.Param("kode_kelas")
	if rawID := strings.TrimSpace(c.Param("pengguna_id")); rawID != "" {
		parsedID, err := strconv.ParseInt(rawID, 10, 32)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "pengguna_id tidak valid")
		}
		req.PenggunaID = int32(parsedID)
	}
	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	_, kelasID, err := h.authorizeGuruKelas(c, req.KodeKelas, "Hanya guru yang dapat mengangkat murid menjadi guru")
	if err != nil {
		return err
	}

	ctx := c.Request().Context()
	isMember, err := h.db.IsAnggotaKelas(ctx, db.IsAnggotaKelasParams{
		KodeKelas:  req.KodeKelas,
		PenggunaID: req.PenggunaID,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa keanggotaan kelas")
	}
	if !isMember {
		return echo.NewHTTPError(http.StatusNotFound, "Anggota tidak ditemukan di kelas ini")
	}

	targetIsGuru, err := h.db.IsGuruKelas(ctx, db.IsGuruKelasParams{
		PenggunaID: req.PenggunaID,
		KelasID:    kelasID,
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa role anggota")
	}
	if targetIsGuru {
		return echo.NewHTTPError(http.StatusBadRequest, "Anggota tersebut sudah menjadi guru")
	}

	if err := h.db.UpdateRoleKelas(ctx, db.UpdateRoleKelasParams{
		PenggunaID: req.PenggunaID,
		KodeKelas:  req.KodeKelas,
		Guru:       true,
	}); err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengangkat murid menjadi guru")
	}

	return c.JSON(http.StatusOK, utils.HTTPResponse{Message: "Berhasil mengangkat murid menjadi guru"})
}

func resolveGuruUtamaID(pengampu pgtype.Int4, rows []db.ListAnggotaKelasRow) int32 {
	if pengampu.Valid && pengampu.Int32 > 0 {
		for _, row := range rows {
			if row.Guru && row.PenggunaID == pengampu.Int32 {
				return row.PenggunaID
			}
		}
	}
	var firstID int32
	var firstTime time.Time
	for _, row := range rows {
		if !row.Guru {
			continue
		}
		if firstID == 0 ||
			row.Dibuat.Time.Before(firstTime) ||
			(row.Dibuat.Time.Equal(firstTime) && row.PenggunaID < firstID) {
			firstID = row.PenggunaID
			firstTime = row.Dibuat.Time
		}
	}
	return firstID
}

// DemoteGuru godoc
//
//	@Summary		Turunkan Guru Menjadi Murid
//	@Description	Mengubah role guru pendamping menjadi murid pada kelas tertentu (khusus guru utama atau admin)
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string	true	"Kode Kelas"
//	@Param			pengguna_id			path		int		true	"ID Pengguna Guru"
//	@Success		200					{object}	utils.HTTPResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/anggota/{pengguna_id}/murid [patch]
func (h *Handler) DemoteGuru(c *echo.Context) error {
	var req PromoteGuruRequest
	req.KodeKelas = c.Param("kode_kelas")
	if rawID := strings.TrimSpace(c.Param("pengguna_id")); rawID != "" {
		parsedID, err := strconv.ParseInt(rawID, 10, 32)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "pengguna_id tidak valid")
		}
		req.PenggunaID = int32(parsedID)
	}
	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	ctx := c.Request().Context()
	kelasRow, err := h.db.GetKelas(ctx, req.KodeKelas)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
		}
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa kelas")
	}

	rows, err := h.db.ListAnggotaKelas(ctx, req.KodeKelas)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil daftar anggota kelas")
	}

	utamaID := resolveGuruUtamaID(kelasRow.Pengampu, rows)
	if !claims.IsAdmin() && (utamaID == 0 || claims.UserID != utamaID) {
		return echo.NewHTTPError(http.StatusForbidden, "Hanya guru utama atau admin yang dapat menurunkan guru menjadi murid")
	}

	var target *db.ListAnggotaKelasRow
	for i := range rows {
		if rows[i].PenggunaID == req.PenggunaID {
			target = &rows[i]
			break
		}
	}
	if target == nil {
		return echo.NewHTTPError(http.StatusNotFound, "Anggota tidak ditemukan di kelas ini")
	}
	if !target.Guru {
		return echo.NewHTTPError(http.StatusBadRequest, "Anggota tersebut bukan guru")
	}
	if req.PenggunaID == utamaID {
		return echo.NewHTTPError(http.StatusBadRequest, "Guru utama tidak dapat diturunkan menjadi murid")
	}
	if req.PenggunaID == claims.UserID {
		return echo.NewHTTPError(http.StatusBadRequest, "Tidak dapat menurunkan role diri sendiri")
	}

	if err := h.db.UpdateRoleKelas(ctx, db.UpdateRoleKelasParams{
		PenggunaID: req.PenggunaID,
		KodeKelas:  req.KodeKelas,
		Guru:       false,
	}); err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menurunkan guru menjadi murid")
	}

	return c.JSON(http.StatusOK, utils.HTTPResponse{Message: "Berhasil menurunkan guru menjadi murid"})
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

	ctx := c.Request().Context()
	kelasRow, err := h.db.GetKelas(ctx, req.KodeKelas)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
		}
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa kelas")
	}

	if !claims.IsAdmin() {
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
	}

	rows, err := h.db.ListAnggotaKelas(ctx, req.KodeKelas)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil daftar anggota kelas")
	}

	utamaID := resolveGuruUtamaID(kelasRow.Pengampu, rows)
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
			Utama:          row.Guru && row.PenggunaID == utamaID,
			Role:           role,
			Dibuat:         utils.ToGMT7(row.Dibuat.Time),
		})
	}
	sort.SliceStable(anggota, func(i, j int) bool {
		if anggota[i].Guru != anggota[j].Guru {
			return anggota[i].Guru
		}
		if anggota[i].Utama != anggota[j].Utama {
			return anggota[i].Utama
		}
		return strings.ToLower(anggota[i].Nama) < strings.ToLower(anggota[j].Nama)
	})

	return c.JSON(http.StatusOK, anggota)
}

// Create godoc
//
//	@Summary		Buat Kelas
//	@Description	Membuat kelas baru dengan pengampu default null (khusus admin)
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			request				body		CreateKelasRequest	true	"Create Kelas Request Payload"
//	@Success		201					{object}	utils.HTTPResponse
//	@Failure		400,401,403,500		{object}	utils.HTTPResponse
//	@Router			/kelas [post]
func (h *Handler) Create(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}
	if !claims.IsAdmin() {
		return echo.NewHTTPError(http.StatusForbidden, "Hanya admin yang dapat membuat kelas")
	}

	req := CreateKelasRequest{}

	err = c.Bind(&req)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err = c.Validate(&req)
	if err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	nama := strings.TrimSpace(req.Nama)
	tahunAjaran := strings.TrimSpace(req.TahunAjaran)
	if nama == "" || tahunAjaran == "" || req.Tingkat <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Nama, tingkat positif, dan tahun ajaran wajib diisi")
	}

	kodeKelas := utils.GenerateHashCode()

	err = h.db.CreateKelas(c.Request().Context(), db.CreateKelasParams{
		Nama:        nama,
		Deskripsi:   strings.TrimSpace(req.Deskripsi),
		Tingkat:     req.Tingkat,
		TahunAjaran: tahunAjaran,
		Pengampu:    pgtype.Int4{Valid: false},
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
	req := KodeKelasRequest{
		KodeKelas: c.Param("kode_kelas"),
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
	req := KodeKelasRequest{
		KodeKelas: c.Param("kode_kelas"),
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
