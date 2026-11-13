package admin

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

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

	if claims.Admin == false {
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

	if claims.Admin == false {
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

	if claims.Admin == false {
		return echo.NewHTTPError(http.StatusForbidden, "Anda tidak memiliki akses")
	}

	admin, err := h.db.ListAdmin(c.Request().Context())
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "Gagal mendapatkan list admin")
	}

	return c.JSON(http.StatusOK, admin)
}

func (h *Handler) UploadKalenderAkademik(c *echo.Context) error {
	req := new(UploadKalenderRequest)
	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Format file tidak valid")
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}
	
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses informasi user")
	}
	if claims.Admin == false {
		return echo.NewHTTPError(http.StatusForbidden, "Anda tidak memiliki akses")
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

		_, err := h.db.CreateKegiatanKalender(c.Request().Context(), db.CreateKegiatanKalenderParams{
			NamaKegiatan:   namaKegiatan,
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

func (h *Handler) ListKalenderAkademik(c *echo.Context) error {
	_, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses informasi user")
	}

	jadwal, err := h.db.ListKegiatanKalender(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal mengambil data kalender")
	}

	return c.JSON(http.StatusOK, jadwal)
}
