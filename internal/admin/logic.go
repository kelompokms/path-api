package admin

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

func requireAdminClaims(c *echo.Context) (*utils.JWTCustomClaims, error) {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses informasi user")
	}
	if !claims.IsAdmin() {
		return nil, echo.NewHTTPError(http.StatusForbidden, "Anda tidak memiliki akses")
	}
	return claims, nil
}

// UpdateGuru godoc
//
//	@Summary		Update Role Guru
//	@Description	Mengupdate status role guru pengguna pada kelas tertentu
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas	path		string				true	"Kode Kelas"
//	@Param			request		body		UpdateGuruPayload	true	"Update Guru Request Payload"
//	@Success		200			{object}	utils.HTTPResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/{kode_kelas}/guru [patch]
func (h *Handler) UpdateGuru(c *echo.Context) error {
	var req UpdateGuruRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses informasi user")
	}

	if !claims.IsAdmin() {
		_, err := h.db.IsGuru(c.Request().Context(), claims.UserID)
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "Anda tidak memiliki akses")
		}
	}

	err = h.db.UpdateRoleKelas(c.Request().Context(), db.UpdateRoleKelasParams{
		PenggunaID: claims.UserID,
		KodeKelas:  req.KodeKelas,
		Guru:       req.RoleGuru,
	})

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengupdate role guru")
	}

	return c.JSON(http.StatusOK, utils.HTTPResponse{Message: "Berhasil mengupdate role guru"})
}

// ListGuru godoc
//
//	@Summary		List Guru
//	@Description	Mendapatkan daftar semua guru beserta kelas yang diampu
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Success		200			{object}	[]ListGuruResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/guru [get]
func (h *Handler) ListGuru(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses informasi user")
	}

	if !claims.IsAdmin() {
		return echo.NewHTTPError(http.StatusUnauthorized, "Anda tidak memiliki akses")
	}

	guru, err := h.db.ListGuru(c.Request().Context())
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Gagal mendapatkan list admin")
	}

	return c.JSON(http.StatusOK, guru)
}

// ListAdmin godoc
//
//	@Summary		List Admin
//	@Description	Mendapatkan daftar semua admin
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Success		200				{object}	[]ListAdminResponse
//	@Failure		400,401,403,500	{object}	utils.HTTPResponse
//	@Router			/admin [get]
func (h *Handler) ListAdmin(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses informasi user")
	}

	if !claims.IsAdmin() {
		return echo.NewHTTPError(http.StatusForbidden, "Anda tidak memiliki akses")
	}

	adminRows, err := h.db.ListAdmin(c.Request().Context())
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Gagal mendapatkan list admin")
	}

	items := make([]ListAdminResponse, 0, len(adminRows))
	for _, r := range adminRows {
		items = append(items, ListAdminResponse{
			ID:             r.ID,
			Nama:           r.Nama,
			Email:          r.Email,
			Phone:          r.Phone.String,
			ProfilePicture: r.ProfilePicture.String,
			Role:           r.Role,
			Admin:          true,
			Dibuat:         utils.ToGMT7(r.Dibuat.Time),
		})
	}

	return c.JSON(http.StatusOK, items)
}

// ListPengguna godoc
//
//	@Summary		List Semua Guru dan Murid
//	@Description	Mendapatkan daftar semua guru dan murid beserta kelas terkait (khusus admin)
//	@Tags			Admin
//	@Produce		json
//	@Success		200				{object}	ListGuruMuridResponse
//	@Failure		401,403,500		{object}	utils.HTTPResponse
//	@Router			/admin/pengguna [get]
func (h *Handler) ListPengguna(c *echo.Context) error {
	if _, err := requireAdminClaims(c); err != nil {
		return err
	}

	ctx := c.Request().Context()
	guruRows, err := h.db.ListAllGuru(ctx)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil daftar guru")
	}

	muridRows, err := h.db.ListAllMurid(ctx)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil daftar murid")
	}

	guruItems := make([]AdminPenggunaItem, 0, len(guruRows))
	for _, r := range guruRows {
		kelas := r.Kelas
		if kelas == nil {
			kelas = []string{}
		}
		guruItems = append(guruItems, AdminPenggunaItem{
			ID:             r.ID,
			Nama:           r.Nama,
			Email:          r.Email,
			Phone:          r.Phone,
			ProfilePicture: r.ProfilePicture,
			Role:           "guru",
			Kelas:          kelas,
		})
	}

	muridItems := make([]AdminPenggunaItem, 0, len(muridRows))
	for _, r := range muridRows {
		kelas := r.Kelas
		if kelas == nil {
			kelas = []string{}
		}
		muridItems = append(muridItems, AdminPenggunaItem{
			ID:             r.ID,
			Nama:           r.Nama,
			Email:          r.Email,
			Phone:          r.Phone,
			ProfilePicture: r.ProfilePicture,
			Role:           "murid",
			Kelas:          kelas,
		})
	}

	return c.JSON(http.StatusOK, ListGuruMuridResponse{
		Guru:  guruItems,
		Murid: muridItems,
	})
}

// ListKelas godoc
//
//	@Summary		List Semua Kelas (Admin)
//	@Description	Mendapatkan daftar seluruh kelas sekolah dengan filter tahun_ajaran, tingkat, dan nama (khusus admin)
//	@Tags			Admin
//	@Produce		json
//	@Param			tahun_ajaran	query		string	false	"Filter tahun ajaran"
//	@Param			tingkat			query		int		false	"Filter tingkat kelas"
//	@Param			nama			query		string	false	"Filter nama kelas"
//	@Success		200				{object}	[]AdminKelasResponse
//	@Failure		400,401,403,500	{object}	utils.HTTPResponse
//	@Router			/admin/kelas [get]
func (h *Handler) ListKelas(c *echo.Context) error {
	if _, err := requireAdminClaims(c); err != nil {
		return err
	}

	tahunAjaran := strings.TrimSpace(c.QueryParam("tahun_ajaran"))
	tingkatStr := strings.TrimSpace(c.QueryParam("tingkat"))
	nama := strings.TrimSpace(c.QueryParam("nama"))

	var tingkatParam pgtype.Int2
	if tingkatStr != "" {
		parsed, err := strconv.ParseInt(tingkatStr, 10, 16)
		if err != nil || parsed <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "Parameter tingkat harus berupa angka positif")
		}
		tingkatParam = pgtype.Int2{Int16: int16(parsed), Valid: true}
	}

	rows, err := h.db.ListAllKelas(c.Request().Context(), db.ListAllKelasParams{
		TahunAjaran: pgtype.Text{String: tahunAjaran, Valid: tahunAjaran != ""},
		Tingkat:     tingkatParam,
		Nama:        pgtype.Text{String: nama, Valid: nama != ""},
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil daftar kelas")
	}

	items := make([]AdminKelasResponse, 0, len(rows))
	for _, r := range rows {
		var pengampu *int32
		if r.Pengampu.Valid {
			val := r.Pengampu.Int32
			pengampu = &val
		}
		items = append(items, AdminKelasResponse{
			ID:           r.ID,
			Nama:         r.Nama,
			Deskripsi:    r.Deskripsi,
			Tingkat:      r.Tingkat,
			TahunAjaran:  r.TahunAjaran,
			KodeKelas:    r.KodeKelas,
			Dibuat:       utils.ToGMT7(r.Dibuat.Time),
			Pengampu:     pengampu,
			NamaPengampu: r.NamaPengampu,
			Guru:         false,
		})
	}

	return c.JSON(http.StatusOK, items)
}

// UpdateRolePengguna godoc
//
//	@Summary		Ubah Role Pengguna (Guru/Murid)
//	@Description	Mengubah role universal pengguna antara guru dan murid pada tabel role_pengguna (khusus admin)
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			pengguna_id	path		int							true	"ID Pengguna"
//	@Param			request		body		UpdateRolePenggunaRequest	true	"Role baru (guru atau murid)"
//	@Success		200			{object}	utils.HTTPResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/admin/pengguna/{pengguna_id}/role [patch]
func (h *Handler) UpdateRolePengguna(c *echo.Context) error {
	if _, err := requireAdminClaims(c); err != nil {
		return err
	}

	var req UpdateRolePenggunaRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	targetRole := strings.ToLower(strings.TrimSpace(req.Role))
	if targetRole != "guru" && targetRole != "murid" {
		return echo.NewHTTPError(http.StatusBadRequest, "Role pengguna harus 'guru' atau 'murid'")
	}

	ctx := c.Request().Context()
	existing, err := h.db.GetPenggunaRoleByID(ctx, req.PenggunaID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Pengguna tidak ditemukan")
		}
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa pengguna")
	}
	if existing.Role == "admin" {
		return echo.NewHTTPError(http.StatusBadRequest, "Tidak dapat mengubah role akun admin")
	}

	affected, err := h.db.UpdateRolePengguna(ctx, db.UpdateRolePenggunaParams{
		ID:   req.PenggunaID,
		Role: targetRole,
	})
	if err != nil || affected == 0 {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui role pengguna")
	}

	return c.JSON(http.StatusOK, utils.HTTPResponse{
		Message: "Berhasil memperbarui role pengguna",
	})
}

// CreateKelas godoc
//
//	@Summary		Buat Kelas Manual (Admin)
//	@Description	Membuat satu kelas baru dengan pengampu default null (khusus admin)
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			request				body		AdminCreateKelasRequest	true	"Data kelas baru"
//	@Success		201					{object}	utils.HTTPResponse
//	@Failure		400,401,403,500		{object}	utils.HTTPResponse
//	@Router			/admin/kelas [post]
func (h *Handler) CreateKelas(c *echo.Context) error {
	if _, err := requireAdminClaims(c); err != nil {
		return err
	}

	var req AdminCreateKelasRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	nama := strings.TrimSpace(req.Nama)
	tahunAjaran := strings.TrimSpace(req.TahunAjaran)
	if nama == "" || tahunAjaran == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Nama dan tahun ajaran wajib diisi")
	}

	err := h.db.CreateKelas(c.Request().Context(), db.CreateKelasParams{
		Nama:        nama,
		Deskripsi:   strings.TrimSpace(req.Deskripsi),
		Tingkat:     req.Tingkat,
		TahunAjaran: tahunAjaran,
		Pengampu:    pgtype.Int4{Valid: false},
		KodeKelas:   utils.GenerateHashCode(),
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat kelas")
	}

	return c.JSON(http.StatusCreated, utils.HTTPResponse{Message: "Kelas berhasil dibuat"})
}

func parseKelasCsvTingkat(raw string) (int16, bool) {
	cleaned := strings.TrimSpace(strings.ToUpper(raw))
	cleaned = strings.TrimSpace(strings.ReplaceAll(cleaned, "KELAS", ""))
	switch cleaned {
	case "X":
		return 10, true
	case "XI":
		return 11, true
	case "XII":
		return 12, true
	}
	parsed, err := strconv.ParseInt(cleaned, 10, 16)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return int16(parsed), true
}

type kelasCsvColumns struct {
	nama        int
	tingkat     int
	tahunAjaran int
	deskripsi   int
	hasHeader   bool
}

func detectKelasCsvColumns(firstRow []string) kelasCsvColumns {
	cols := kelasCsvColumns{
		nama:        0,
		tingkat:     1,
		tahunAjaran: 2,
		deskripsi:   3,
	}
	if len(firstRow) == 0 {
		return cols
	}

	namaIdx, tingkatIdx, tahunIdx, deskIdx := -1, -1, -1, -1
	for i, raw := range firstRow {
		header := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(raw, "\uFEFF")))
		header = strings.ReplaceAll(header, "_", " ")
		switch header {
		case "nama", "nama kelas", "mata pelajaran", "mapel":
			if namaIdx == -1 {
				namaIdx = i
			}
		case "tingkat", "kelas":
			if tingkatIdx == -1 {
				tingkatIdx = i
			}
		case "tahun ajaran":
			if tahunIdx == -1 {
				tahunIdx = i
			}
		case "deskripsi", "keterangan":
			if deskIdx == -1 {
				deskIdx = i
			}
		}
	}

	if namaIdx != -1 && tingkatIdx != -1 && tahunIdx != -1 {
		return kelasCsvColumns{
			nama:        namaIdx,
			tingkat:     tingkatIdx,
			tahunAjaran: tahunIdx,
			deskripsi:   deskIdx,
			hasHeader:   true,
		}
	}
	first := strings.TrimSpace(strings.TrimPrefix(firstRow[0], "\uFEFF"))
	if strings.EqualFold(first, "nama") || strings.EqualFold(first, "nama_kelas") {
		cols.hasHeader = true
	}
	return cols
}

func detectCsvComma(raw []byte) rune {
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.Contains(trimmed, ";") && !strings.Contains(trimmed, ",") {
			return ';'
		}
		break
	}
	return ','
}

// UploadKelasCSV godoc
//
//	@Summary		Buat Banyak Kelas dari CSV (Admin)
//	@Description	Mengunggah file CSV (kolom: nama,tingkat,tahun_ajaran,deskripsi) untuk membuat banyak kelas dengan pengampu default null (khusus admin)
//	@Tags			Admin
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			file_csv			formData	file	true	"File CSV daftar kelas"
//	@Success		201					{object}	utils.HTTPResponse
//	@Failure		400,401,403,500		{object}	utils.HTTPResponse
//	@Router			/admin/kelas/csv [post]
func (h *Handler) UploadKelasCSV(c *echo.Context) error {
	if _, err := requireAdminClaims(c); err != nil {
		return err
	}

	req := new(UploadKelasCSVRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Format file tidak valid")
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	src, err := req.FileCSV.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuka file CSV")
	}
	defer src.Close()

	rawBytes, err := io.ReadAll(io.LimitReader(src, 5<<20))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membaca file CSV")
	}
	rawBytes = bytes.TrimPrefix(rawBytes, []byte("\xEF\xBB\xBF"))

	reader := csv.NewReader(bytes.NewReader(rawBytes))
	reader.Comma = detectCsvComma(rawBytes)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Gagal membaca format CSV")
	}
	if len(records) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "File CSV kosong atau tidak memiliki data kelas")
	}

	cols := detectKelasCsvColumns(records[0])
	startIdx := 0
	if cols.hasHeader {
		startIdx = 1
	}

	validRows := 0
	berhasil := 0
	var lastDBErr error
	ctx := c.Request().Context()
	for _, row := range records[startIdx:] {
		if len(row) == 0 {
			continue
		}
		if cols.nama >= len(row) || cols.tingkat >= len(row) || cols.tahunAjaran >= len(row) {
			continue
		}

		nama := strings.TrimSpace(strings.TrimPrefix(row[cols.nama], "\uFEFF"))
		tingkat, ok := parseKelasCsvTingkat(row[cols.tingkat])
		tahunAjaran := strings.TrimSpace(row[cols.tahunAjaran])
		if nama == "" || !ok || tahunAjaran == "" {
			continue
		}

		deskripsi := ""
		if cols.deskripsi >= 0 && cols.deskripsi < len(row) {
			deskripsi = strings.TrimSpace(row[cols.deskripsi])
		}

		validRows++
		err := h.db.CreateKelas(ctx, db.CreateKelasParams{
			Nama:        nama,
			Deskripsi:   deskripsi,
			Tingkat:     tingkat,
			TahunAjaran: tahunAjaran,
			Pengampu:    pgtype.Int4{Valid: false},
			KodeKelas:   utils.GenerateHashCode(),
		})
		if err != nil {
			log.Println(err)
			lastDBErr = err
			continue
		}
		berhasil++
	}

	if validRows == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Tidak ada data kelas valid pada file CSV")
	}
	if berhasil == 0 && lastDBErr != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menyimpan data kelas dari CSV")
	}

	return c.JSON(http.StatusCreated, utils.HTTPResponse{
		Message: fmt.Sprintf("Berhasil membuat %d kelas dari CSV", berhasil),
	})
}

// UpdatePengampuKelas godoc
//
//	@Summary		Ubah Pengampu Kelas (Admin)
//	@Description	Menetapkan atau mengubah pengampu utama kelas dari daftar pengguna ber-role guru (khusus admin)
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			kode_kelas			path		string						true	"Kode Kelas"
//	@Param			request				body		UpdatePengampuKelasRequest	true	"ID guru pengampu (null untuk mengosongkan)"
//	@Success		200					{object}	utils.HTTPResponse
//	@Failure		400,401,403,404,500	{object}	utils.HTTPResponse
//	@Router			/admin/kelas/{kode_kelas}/pengampu [patch]
func (h *Handler) UpdatePengampuKelas(c *echo.Context) error {
	if _, err := requireAdminClaims(c); err != nil {
		return err
	}

	var req UpdatePengampuKelasRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(&req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	ctx := c.Request().Context()
	if _, err := h.db.GetKelasID(ctx, req.KodeKelas); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return echo.NewHTTPError(http.StatusNotFound, "Kelas tidak ditemukan")
		}
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa kelas")
	}

	var pengampuParam pgtype.Int4
	if req.PengampuID != nil {
		if *req.PengampuID <= 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "ID pengampu tidak valid")
		}
		targetUser, err := h.db.GetPenggunaRoleByID(ctx, *req.PengampuID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return echo.NewHTTPError(http.StatusNotFound, "Guru tidak ditemukan")
			}
			log.Println(err)
			return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memeriksa role pengguna")
		}
		if targetUser.Role != "guru" {
			return echo.NewHTTPError(http.StatusBadRequest, "Pengampu kelas harus memiliki role guru")
		}
		pengampuParam = pgtype.Int4{Int32: *req.PengampuID, Valid: true}
	}

	if _, err := h.db.UpdatePengampuKelas(ctx, db.UpdatePengampuKelasParams{
		KodeKelas: req.KodeKelas,
		Pengampu:  pengampuParam,
	}); err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal memperbarui pengampu kelas")
	}

	return c.JSON(http.StatusOK, utils.HTTPResponse{
		Message: "Berhasil memperbarui pengampu kelas",
	})
}

func toKalenderAgendaResponse(item db.KalenderAkademik) KalenderAgendaResponse {
	return KalenderAgendaResponse{
		ID:             item.ID,
		NamaKegiatan:   item.NamaKegiatan,
		Keterangan:     item.Keterangan,
		TanggalMulai:   item.TanggalMulai.Time.Format("2006-01-02"),
		TanggalSelesai: item.TanggalSelesai.Time.Format("2006-01-02"),
		Dibuat:         utils.ToGMT7(item.Dibuat.Time),
	}
}

// CreateAgendaKalender godoc
//
//	@Summary		Tambah Agenda Kalender Akademik
//	@Description	Menambahkan satu kegiatan kalender akademik beserta keterangan agar dapat dilihat semua pengguna (khusus admin)
//	@Tags			Admin
//	@Accept			json
//	@Produce		json
//	@Param			request			body		CreateKalenderAgendaRequest	true	"Agenda Request"
//	@Success		201				{object}	KalenderAgendaResponse
//	@Failure		400,401,403,500	{object}	utils.HTTPResponse
//	@Router			/admin/kalender/agenda [post]
func (h *Handler) CreateAgendaKalender(c *echo.Context) error {
	if _, err := requireAdminClaims(c); err != nil {
		return err
	}

	req := new(CreateKalenderAgendaRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	namaKegiatan := strings.TrimSpace(req.NamaKegiatan)
	if namaKegiatan == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Nama event wajib diisi")
	}

	tglMulai, err := time.Parse("2006-01-02", strings.TrimSpace(req.TanggalMulai))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Format tanggal_mulai harus YYYY-MM-DD")
	}

	tglSelesai := tglMulai
	if strings.TrimSpace(req.TanggalSelesai) != "" {
		parsedEnd, err := time.Parse("2006-01-02", strings.TrimSpace(req.TanggalSelesai))
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Format tanggal_selesai harus YYYY-MM-DD")
		}
		if parsedEnd.Before(tglMulai) {
			return echo.NewHTTPError(http.StatusBadRequest, "tanggal_selesai tidak boleh sebelum tanggal_mulai")
		}
		tglSelesai = parsedEnd
	}

	created, err := h.db.CreateKegiatanKalender(c.Request().Context(), db.CreateKegiatanKalenderParams{
		NamaKegiatan:   namaKegiatan,
		Keterangan:     strings.TrimSpace(req.Keterangan),
		TanggalMulai:   pgtype.Date{Time: tglMulai, Valid: true},
		TanggalSelesai: pgtype.Date{Time: tglSelesai, Valid: true},
	})
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal menambahkan agenda kalender")
	}

	return c.JSON(http.StatusCreated, toKalenderAgendaResponse(created))
}

func (h *Handler) UploadKalenderAkademik(c *echo.Context) error {
	req := new(UploadKalenderRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Format file tidak valid")
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	if _, err := requireAdminClaims(c); err != nil {
		return err
	}

	src, err := req.FileCSV.Open()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuka file CSV")
	}
	defer src.Close()

	reader := csv.NewReader(src)
	records, err := reader.ReadAll()
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Gagal membaca format CSV")
	}

	if len(records) <= 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "File CSV kosong atau tidak memiliki data")
	}

	_ = h.db.DeleteSemuaKegiatanKalender(c.Request().Context())

	berhasil := 0
	for i, row := range records {
		if i == 0 {
			continue
		}
		if len(row) < 3 {
			continue
		}

		namaKegiatan := row[0]

		tglMulai, err1 := time.Parse("2006-01-02", row[1])
		tglSelesai, err2 := time.Parse("2006-01-02", row[2])

		if err1 != nil || err2 != nil {
			continue
		}

		keterangan := ""
		if len(row) >= 4 {
			keterangan = strings.TrimSpace(row[3])
		}

		_, err := h.db.CreateKegiatanKalender(c.Request().Context(), db.CreateKegiatanKalenderParams{
			NamaKegiatan:   namaKegiatan,
			Keterangan:     keterangan,
			TanggalMulai:   pgtype.Date{Time: tglMulai, Valid: true},
			TanggalSelesai: pgtype.Date{Time: tglSelesai, Valid: true},
		})

		if err == nil {
			berhasil++
		}
	}

	return c.JSON(http.StatusCreated, utils.HTTPResponse{
		Message: fmt.Sprintf("Berhasil mengunggah %d kegiatan kalender akademik", berhasil),
	})
}

// ListKalenderAkademik godoc
//
//	@Summary		List Agenda Kalender Akademik
//	@Description	Mendapatkan seluruh kegiatan dan agenda pada kalender akademik untuk semua pengguna
//	@Tags			Kalender
//	@Produce		json
//	@Success		200		{object}	[]KalenderAgendaResponse
//	@Failure		401,500	{object}	utils.HTTPResponse
//	@Router			/kalender [get]
func (h *Handler) ListKalenderAkademik(c *echo.Context) error {
	_, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses informasi user")
	}

	jadwal, err := h.db.ListKegiatanKalender(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil data kalender")
	}

	items := make([]KalenderAgendaResponse, 0, len(jadwal))
	for _, row := range jadwal {
		items = append(items, toKalenderAgendaResponse(row))
	}

	return c.JSON(http.StatusOK, items)
}
