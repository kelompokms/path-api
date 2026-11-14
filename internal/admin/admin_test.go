package admin

import (
	"bytes"
	"encoding/json"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setAuthContext(c *echo.Context, userID int32, admin bool) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &utils.JWTCustomClaims{
		UserID: userID,
		Admin:  admin,
	})
	c.Set("user", token)
}

func TestListAdmin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful list admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		rows := pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture", "password", "admin", "dibuat"}).
			AddRow(int32(1), "Admin", "admin@example.com", pgtype.Text{String: "", Valid: false}, pgtype.Text{String: "", Valid: false}, "hash", true, time.Now())

		mockPool.ExpectQuery("SELECT").WillReturnRows(rows)

		err := h.ListAdmin(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden for non-admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 2, false)

		err := h.ListAdmin(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
	})
}

func TestListGuru(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful list guru", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/guru", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		rows := pgxmock.NewRows([]string{"kelas_id", "nama_kelas", "pengguna_id", "nama_pengguna"}).
			AddRow(int32(1), "Matematika", int32(2), "Guru Budi")

		mockPool.ExpectQuery("SELECT").WillReturnRows(rows)

		err := h.ListGuru(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("unauthorized for non-admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/guru", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 2, false)

		err := h.ListGuru(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
	})
}

func TestUpdateGuru(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful update guru by admin", func(t *testing.T) {
		reqBody := UpdateGuruRequest{
			KodeKelas: "ABC123",
			RoleGuru:  true,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/ABC123/guru", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 1, true)

		mockPool.ExpectExec("UPDATE").WithArgs(int32(1), "ABC123", true).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := h.UpdateGuru(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("successful update guru by existing guru", func(t *testing.T) {
		reqBody := UpdateGuruRequest{
			KodeKelas: "ABC123",
			RoleGuru:  true,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/ABC123/guru", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT").WithArgs(int32(2)).WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).AddRow(true),
		)
		mockPool.ExpectExec("UPDATE").WithArgs(int32(2), "ABC123", true).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := h.UpdateGuru(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestUploadKalenderAkademik(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	newCSVRequest := func(t *testing.T, csvContent string) (*echo.Context, *httptest.ResponseRecorder) {
		t.Helper()
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("file_csv", "kalender.csv")
		require.NoError(t, err)
		_, err = part.Write([]byte(csvContent))
		require.NoError(t, err)
		require.NoError(t, w.Close())

		req := httptest.NewRequest(http.MethodPost, "/admin/kalender", &body)
		req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
		rec := httptest.NewRecorder()
		return e.NewContext(req, rec), rec
	}

	t.Run("successful upload kalender akademik", func(t *testing.T) {
		csvData := "nama_kegiatan,tanggal_mulai,tanggal_selesai\nUTS Semester Ganjil,2026-10-01,2026-10-07\n"
		c, rec := newCSVRequest(t, csvData)
		setAuthContext(c, 1, true)

		tglMulai, _ := time.Parse("2006-01-02", "2026-10-01")
		tglSelesai, _ := time.Parse("2006-01-02", "2026-10-07")
		now := pgtype.Timestamp{Time: time.Now(), Valid: true}

		mockPool.ExpectExec("DELETE FROM kalender_akademik").
			WillReturnResult(pgxmock.NewResult("DELETE", 1))

		rows := pgxmock.NewRows([]string{"id", "nama_kegiatan", "tanggal_mulai", "tanggal_selesai", "dibuat"}).
			AddRow(int32(1), "UTS Semester Ganjil", pgtype.Date{Time: tglMulai, Valid: true}, pgtype.Date{Time: tglSelesai, Valid: true}, now)

		mockPool.ExpectQuery("INSERT INTO kalender_akademik").
			WithArgs("UTS Semester Ganjil", pgtype.Date{Time: tglMulai, Valid: true}, pgtype.Date{Time: tglSelesai, Valid: true}).
			WillReturnRows(rows)

		err := h.UploadKalenderAkademik(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "Berhasil mengunggah 1 kegiatan kalender akademik")
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden for non-admin", func(t *testing.T) {
		csvData := "nama_kegiatan,tanggal_mulai,tanggal_selesai\nUTS,2026-10-01,2026-10-07\n"
		c, _ := newCSVRequest(t, csvData)
		setAuthContext(c, 2, false)

		err := h.UploadKalenderAkademik(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
	})

	t.Run("empty csv without data rows", func(t *testing.T) {
		csvData := "nama_kegiatan,tanggal_mulai,tanggal_selesai\n"
		c, _ := newCSVRequest(t, csvData)
		setAuthContext(c, 1, true)

		err := h.UploadKalenderAkademik(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	})
}

func TestListKalenderAkademik(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful list kalender akademik", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kalender", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		tglMulai, _ := time.Parse("2006-01-02", "2026-10-01")
		tglSelesai, _ := time.Parse("2006-01-02", "2026-10-07")
		now := pgtype.Timestamp{Time: time.Now(), Valid: true}

		rows := pgxmock.NewRows([]string{"id", "nama_kegiatan", "tanggal_mulai", "tanggal_selesai", "dibuat"}).
			AddRow(int32(1), "UTS Semester Ganjil", pgtype.Date{Time: tglMulai, Valid: true}, pgtype.Date{Time: tglSelesai, Valid: true}, now)

		mockPool.ExpectQuery("SELECT").WillReturnRows(rows)

		err := h.ListKalenderAkademik(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("unauthorized without claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kalender", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.ListKalenderAkademik(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
	})

	t.Run("database error", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kalender", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT").WillReturnError(errors.New("db error"))

		err := h.ListKalenderAkademik(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
