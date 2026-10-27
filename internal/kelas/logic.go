package kelas

import (
	"log"
	"net/http"
	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

// List godoc
//
//	@Summary		List Kelas
//	@Description	List kelas menggunakan Bearer Token dan mengembalikan response list kelas
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Success		200			{object}	[]ListKelasResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/kelas [get]
func (h *Handler) List(c *echo.Context) error {
	claims, err := utils.GetClaims(c)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Tidak dapat mengakses data klaim")
	}

	kelas, err := h.db.ListKelas(c.Request().Context(), claims.UserID)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusUnauthorized, "tidak ada kelas")
	}

	return c.JSON(http.StatusOK, kelas)
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

	var tingkatParam pgtype.Text
	if req.Tingkat != "" {
		tingkatParam.Valid = true
		tingkatParam.String = req.Tingkat
	}

	kodeKelas := utils.GenerateHashCode()

	err = h.db.CreateKelas(c.Request().Context(), db.CreateKelasParams{
		Nama:      req.Nama,
		Tingkat:   tingkatParam,
		Pembuat:   claims.UserID,
		KodeKelas: kodeKelas,
	})

	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal membuat kelas")
	}

	return c.JSON(http.StatusCreated, map[string]string{
		"message": "Kelas berhasil dibuat",
	})
}

// Join godoc
//
//	@Summary		Join Kelas
//	@Description	Join Kelas menggunakan kode dan mengembalikan message
//	@Tags			Kelas
//	@Accept			json
//	@Produce		json
//	@Param			request		path		JoinDanKeluarKelasRequest	true	"Create Kelas Request Payload"
//	@Success		200			{object}	utils.HTTPResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/gabung [post]
func (h *Handler) Join(c *echo.Context) error {
	var req JoinDanKeluarKelasRequest

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

	kelasID, err := h.db.GetKelasIDByKodeKelas(c.Request().Context(), req.KodeKelas)
	if err != nil {
		log.Println(err)
		return echo.NewHTTPError(http.StatusBadRequest, "kelas tidak ditemukan")
	}

	exists, err := h.db.CheckAnggotaKelas(c.Request().Context(), db.CheckAnggotaKelasParams{
		PenggunaID: claims.UserID,
		KelasID:    kelasID,
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
//	@Param			request		path		JoinDanKeluarKelasRequest	true	"Leave Kelas Request Payload"
//	@Success		200			{object}	utils.HTTPResponse
//	@Failure		400,401,500	{object}	utils.HTTPResponse
//	@Router			/kelas/{kode_kelas}/keluar [post]
func (h *Handler) Leave(c *echo.Context) error {
	var req JoinDanKeluarKelasRequest
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

	kelasID, err := h.db.GetKelasIDByKodeKelas(c.Request().Context(), req.KodeKelas)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "kelas tidak ditemukan")
	}

	err = h.db.LeaveKelas(c.Request().Context(), db.LeaveKelasParams{
		KelasID:    kelasID,
		PenggunaID: claims.UserID,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Gagal keluar dari kelas")
	}

	return c.JSON(http.StatusOK, map[string]string{
		"message": "Berhasil keluar dari kelas",
	})
}
