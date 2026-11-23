package kelas

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
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

func setAuthContextWithRole(c *echo.Context, userID int32, role string) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &utils.JWTCustomClaims{
		UserID: userID,
		Admin:  role == "admin",
		Role:   role,
	})
	c.Set("user", token)
}

var kelasColumns = []string{"id", "nama", "tingkat", "deskripsi", "tahun_ajaran", "kode_kelas", "dibuat", "pengampu", "mapel"}
var listKelasColumns = []string{"id", "nama", "deskripsi", "tingkat", "tahun_ajaran", "kode_kelas", "dibuat", "pengampu", "guru", "mapel", "nama_pengampu"}
var anggotaColumns = []string{"pengguna_id", "nama", "email", "profile_picture", "guru", "dibuat"}

func TestListKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful list returns both student and teacher classes with guru flag", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}

		rows := pgxmock.NewRows(listKelasColumns).
			AddRow(int32(1), "Matematika", "Kelas matematika", int16(10), "2026/2027", "ABC123", now, pgtype.Int4{Int32: 2, Valid: true}, false, "Matematika", "Pak Budi").
			AddRow(int32(2), "Fisika", "Kelas fisika", int16(11), "2026/2027", "DEF456", now, pgtype.Int4{Int32: 1, Valid: true}, true, "Fisika", "Pak Eko")

		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []db.ListKelasRow
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp, 2)
		assert.False(t, resp[0].Guru)
		assert.Equal(t, "Kelas matematika", resp[0].Deskripsi)
		assert.True(t, resp[1].Guru)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("filters by guru=true", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas?guru=true", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}
		rows := pgxmock.NewRows(listKelasColumns).
			AddRow(int32(1), "Matematika", "Kelas matematika", int16(10), "2026/2027", "ABC123", now, pgtype.Int4{Int32: 2, Valid: true}, false, "Matematika", "Pak Budi").
			AddRow(int32(2), "Fisika", "Kelas fisika", int16(11), "2026/2027", "DEF456", now, pgtype.Int4{Int32: 1, Valid: true}, true, "Fisika", "Pak Eko")

		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []db.ListKelasRow
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, "DEF456", resp[0].KodeKelas)
		assert.True(t, resp[0].Guru)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("filters by guru=false", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas?guru=false", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}
		rows := pgxmock.NewRows(listKelasColumns).
			AddRow(int32(1), "Matematika", "Kelas matematika", int16(10), "2026/2027", "ABC123", now, pgtype.Int4{Int32: 2, Valid: true}, false, "Matematika", "Pak Budi").
			AddRow(int32(2), "Fisika", "Kelas fisika", int16(11), "2026/2027", "DEF456", now, pgtype.Int4{Int32: 1, Valid: true}, true, "Fisika", "Pak Eko")

		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []db.ListKelasRow
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, "ABC123", resp[0].KodeKelas)
		assert.False(t, resp[0].Guru)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("returns empty json array when no classes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows(listKelasColumns)
		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "[]\n", rec.Body.String())
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("invalid guru filter returns bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas?guru=invalid", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		err := h.List(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	})

	t.Run("unauthorized without claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := h.List(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
	})

	t.Run("error on ListKelas query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnError(errors.New("db error"))

		err := h.List(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestGetKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}
		rows := pgxmock.NewRows(kelasColumns).
			AddRow(int32(1), "Matematika", int16(10), "Deskripsi kelas", "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true}, "Matematika")

		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(rows)

		err := h.Get(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("successful get with claims includes guru status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 1, false)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}
		rows := pgxmock.NewRows(kelasColumns).
			AddRow(int32(1), "Matematika", int16(10), "Deskripsi kelas", "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true}, "Matematika")

		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(rows)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		err := h.Get(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp DetailKelasResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.True(t, resp.Guru)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("kelas not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/NOTFOUND", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "NOTFOUND"},
		})

		mockPool.ExpectQuery("SELECT").WithArgs("NOTFOUND").WillReturnError(pgx.ErrNoRows)

		err := h.Get(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestListAnggotaKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("returns all members both guru and murid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/anggota", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 2, false)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}

		mockPool.ExpectQuery("SELECT id, nama, tingkat, deskripsi, tahun_ajaran, kode_kelas, dibuat, pengampu, mapel FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows(kelasColumns).
				AddRow(int32(1), "Matematika", int16(10), "Desc", "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true}, "Matematika"),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(2)).WillReturnRows(
			pgxmock.NewRows([]string{"is_anggota"}).AddRow(true),
		)
		rows := pgxmock.NewRows(anggotaColumns).
			AddRow(int32(1), "Pak Guru", "guru@example.com", pgtype.Text{String: "", Valid: false}, true, now).
			AddRow(int32(2), "Budi Murid", "budi@example.com", pgtype.Text{String: "pfp.jpg", Valid: true}, false, now)
		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(rows)

		err := h.ListAnggota(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []AnggotaKelasResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp, 2)
		assert.True(t, resp[0].Guru)
		assert.True(t, resp[0].Utama)
		assert.Equal(t, "guru", resp[0].Role)
		assert.Equal(t, "Pak Guru", resp[0].Nama)
		assert.False(t, resp[1].Guru)
		assert.False(t, resp[1].Utama)
		assert.Equal(t, "murid", resp[1].Role)
		assert.Equal(t, "Budi Murid", resp[1].Nama)
		assert.Equal(t, "pfp.jpg", resp[1].ProfilePicture)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden for non-member", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/anggota", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 99, false)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}
		mockPool.ExpectQuery("SELECT id, nama, tingkat, deskripsi, tahun_ajaran, kode_kelas, dibuat, pengampu, mapel FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows(kelasColumns).
				AddRow(int32(1), "Matematika", int16(10), "Desc", "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true}, "Matematika"),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(99)).WillReturnRows(
			pgxmock.NewRows([]string{"is_anggota"}).AddRow(false),
		)

		err := h.ListAnggota(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("not found when kelas missing", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/MISSING/anggota", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "MISSING"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT id, nama, tingkat, deskripsi, tahun_ajaran, kode_kelas, dibuat, pengampu, mapel FROM kelas").WithArgs("MISSING").WillReturnError(pgx.ErrNoRows)

		err := h.ListAnggota(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("unauthorized without claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/anggota", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})

		err := h.ListAnggota(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
	})
}

func TestCreateKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful create by admin defaults pengampu to null", func(t *testing.T) {
		reqBody := CreateKelasRequest{
			Nama:        "Fisika",
			Deskripsi:   "Fisika dasar",
			Tingkat:     10,
			TahunAjaran: "2026/2027",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/kelas", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		mockPool.ExpectExec("INSERT").
			WithArgs("Fisika", "Fisika dasar", int16(10), "2026/2027", pgtype.Int4{Valid: false}, pgxmock.AnyArg(), "").
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := h.Create(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when guru or murid tries to create kelas", func(t *testing.T) {
		reqBody := CreateKelasRequest{
			Nama:        "Fisika",
			Tingkat:     10,
			TahunAjaran: "2026/2027",
		}
		jsonBody, _ := json.Marshal(reqBody)

		for _, role := range []string{"guru", "murid"} {
			req := httptest.NewRequest(http.MethodPost, "/kelas", bytes.NewReader(jsonBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			setAuthContextWithRole(c, 2, role)

			err := h.Create(c)
			assert.Error(t, err)
			httpErr, ok := err.(*echo.HTTPError)
			require.True(t, ok)
			assert.Equal(t, http.StatusForbidden, httpErr.Code)
		}
	})

	t.Run("validation error when missing required fields", func(t *testing.T) {
		reqBody := CreateKelasRequest{
			Nama:    "Fisika",
			Tingkat: 10,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/kelas", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		err := h.Create(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("database error on create", func(t *testing.T) {
		reqBody := CreateKelasRequest{
			Nama:        "Fisika",
			Tingkat:     10,
			TahunAjaran: "2026/2027",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/kelas", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		mockPool.ExpectExec("INSERT").
			WithArgs("Fisika", "", int16(10), "2026/2027", pgtype.Int4{Valid: false}, pgxmock.AnyArg(), "").
			WillReturnError(errors.New("db error"))

		err := h.Create(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestJoinKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful join", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/gabung", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 2, false)

		// GetKelasID
		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		// CheckAnggotaKelas
		mockPool.ExpectQuery("SELECT").WithArgs(int32(2), "ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).AddRow(int32(0)),
		)
		// JoinKelas (Exec) - takes pengguna_id, kelas_id, guru
		mockPool.ExpectExec("INSERT").WithArgs(int32(2), int32(1), false).WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := h.Join(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("already joined", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/gabung", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT").WithArgs(int32(2), "ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).AddRow(int32(1)),
		)

		err := h.Join(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Equal(t, "anda sudah bergabung di kelas ini", httpErr.Message)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("kelas not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/kelas/UNKNOWN/gabung", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "UNKNOWN"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT").WithArgs("UNKNOWN").WillReturnError(pgx.ErrNoRows)

		err := h.Join(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestLeaveKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful leave", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/keluar", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectExec("DELETE").WithArgs(int32(2), "ABC123").WillReturnResult(pgxmock.NewResult("DELETE", 1))

		err := h.Leave(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("database error on leave", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/keluar", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectExec("DELETE").WithArgs(int32(2), "ABC123").WillReturnError(errors.New("db error"))

		err := h.Leave(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestUpdateKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful update nama and deskripsi as guru", func(t *testing.T) {
		nama := "Matematika Lanjutan"
		deskripsi := "Kelas persiapan olimpiade"
		jsonBody, _ := json.Marshal(UpdateKelasPayload{
			Nama:      &nama,
			Deskripsi: &deskripsi,
		})

		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "ABC123"}})
		setAuthContext(c, 1, false)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("UPDATE kelas").WithArgs(
			"ABC123",
			pgtype.Text{String: nama, Valid: true},
			pgtype.Text{String: deskripsi, Valid: true},
			pgtype.Text{},
		).WillReturnRows(
			pgxmock.NewRows(kelasColumns).
				AddRow(int32(10), nama, int16(10), deskripsi, "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true}, "Matematika"),
		)

		err := h.Update(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp DetailKelasResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, nama, resp.Nama)
		assert.Equal(t, deskripsi, resp.Deskripsi)
		assert.True(t, resp.Guru)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("rejects blank nama", func(t *testing.T) {
		nama := "   "
		jsonBody, _ := json.Marshal(UpdateKelasPayload{Nama: &nama})

		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "ABC123"}})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		err := h.Update(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("rejects empty payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123", bytes.NewReader([]byte("{}")))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "ABC123"}})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		err := h.Update(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when non-guru updates kelas", func(t *testing.T) {
		nama := "Baru"
		jsonBody, _ := json.Marshal(UpdateKelasPayload{Nama: &nama})

		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "ABC123"}})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(2), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(false),
		)

		err := h.Update(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("not found when kelas missing", func(t *testing.T) {
		nama := "Baru"
		jsonBody, _ := json.Marshal(UpdateKelasPayload{Nama: &nama})

		req := httptest.NewRequest(http.MethodPatch, "/kelas/MISSING", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "MISSING"}})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("MISSING").WillReturnError(pgx.ErrNoRows)

		err := h.Update(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPromoteGuru(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful promote student to guru as class guru", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/anggota/2/guru", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "pengguna_id", Value: "2"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(2)).WillReturnRows(
			pgxmock.NewRows([]string{"is_anggota"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(2), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(false),
		)
		mockPool.ExpectExec("UPDATE anggota_kelas").WithArgs(int32(2), "ABC123", true).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := h.PromoteGuru(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp utils.HTTPResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "Berhasil mengangkat murid menjadi guru", resp.Message)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when student attempts promote", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/anggota/3/guru", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "pengguna_id", Value: "3"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(2), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(false),
		)

		err := h.PromoteGuru(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("not found when target user is not a member", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/anggota/99/guru", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "pengguna_id", Value: "99"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(99)).WillReturnRows(
			pgxmock.NewRows([]string{"is_anggota"}).AddRow(false),
		)

		err := h.PromoteGuru(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("bad request when target user is already guru", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/anggota/2/guru", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "pengguna_id", Value: "2"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(2)).WillReturnRows(
			pgxmock.NewRows([]string{"is_anggota"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(2), int32(10)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		err := h.PromoteGuru(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("unauthorized without claims", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/anggota/2/guru", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "pengguna_id", Value: "2"},
		})

		err := h.PromoteGuru(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
	})
}

func TestKelasDibuatUsesGMT7(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()

	utcTime := time.Date(2026, 11, 18, 2, 16, 0, 0, time.UTC)
	expectedWIB := `"dibuat":"2026-11-18T09:16:00+07:00"`

	req := httptest.NewRequest(http.MethodGet, "/kelas", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	setAuthContext(c, 1, false)

	mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(
		pgxmock.NewRows(listKelasColumns).
			AddRow(int32(1), "Matematika", "Kelas matematika", int16(10), "2026/2027", "ABC123", pgtype.Timestamp{Time: utcTime, Valid: true}, pgtype.Int4{}, false, "Matematika", ""),
	)

	require.NoError(t, h.List(c))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), expectedWIB)

	getReq := httptest.NewRequest(http.MethodGet, "/kelas/ABC123", nil)
	getRec := httptest.NewRecorder()
	getCtx := e.NewContext(getReq, getRec)
	getCtx.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "ABC123"}})
	mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(
		pgxmock.NewRows(kelasColumns).
			AddRow(int32(1), "Matematika", int16(10), "Deskripsi", "2026/2027", "ABC123", pgtype.Timestamp{Time: utcTime, Valid: true}, pgtype.Int4{}, "Matematika"),
	)

	require.NoError(t, h.Get(getCtx))
	assert.Equal(t, http.StatusOK, getRec.Code)
	assert.Contains(t, getRec.Body.String(), expectedWIB)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestDemoteGuru(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()
	now := pgtype.Timestamp{Time: time.Now(), Valid: true}

	t.Run("guru utama can demote co-teacher to murid", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/anggota/2/murid", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "pengguna_id", Value: "2"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id, nama, tingkat, deskripsi, tahun_ajaran, kode_kelas, dibuat, pengampu, mapel FROM kelas").
			WithArgs("ABC123").
			WillReturnRows(pgxmock.NewRows(kelasColumns).
				AddRow(int32(10), "Matematika", int16(10), "Desc", "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true}, "Matematika"))
		mockPool.ExpectQuery("SELECT").
			WithArgs("ABC123").
			WillReturnRows(pgxmock.NewRows(anggotaColumns).
				AddRow(int32(1), "Guru Utama", "utama@example.com", pgtype.Text{}, true, now).
				AddRow(int32(2), "Guru Kedua", "kedua@example.com", pgtype.Text{}, true, now))
		mockPool.ExpectExec("UPDATE anggota_kelas").
			WithArgs(int32(2), "ABC123", false).
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		require.NoError(t, h.DemoteGuru(c))
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "Berhasil menurunkan guru menjadi murid")
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("co-teacher cannot demote another guru", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/anggota/3/murid", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "pengguna_id", Value: "3"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT id, nama, tingkat, deskripsi, tahun_ajaran, kode_kelas, dibuat, pengampu, mapel FROM kelas").
			WithArgs("ABC123").
			WillReturnRows(pgxmock.NewRows(kelasColumns).
				AddRow(int32(10), "Matematika", int16(10), "Desc", "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true}, "Matematika"))
		mockPool.ExpectQuery("SELECT").
			WithArgs("ABC123").
			WillReturnRows(pgxmock.NewRows(anggotaColumns).
				AddRow(int32(1), "Guru Utama", "utama@example.com", pgtype.Text{}, true, now).
				AddRow(int32(2), "Guru Kedua", "kedua@example.com", pgtype.Text{}, true, now).
				AddRow(int32(3), "Guru Ketiga", "ketiga@example.com", pgtype.Text{}, true, now))

		err := h.DemoteGuru(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("admin cannot demote guru utama", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/anggota/1/murid", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "pengguna_id", Value: "1"},
		})
		setAuthContext(c, 99, true)

		mockPool.ExpectQuery("SELECT id, nama, tingkat, deskripsi, tahun_ajaran, kode_kelas, dibuat, pengampu, mapel FROM kelas").
			WithArgs("ABC123").
			WillReturnRows(pgxmock.NewRows(kelasColumns).
				AddRow(int32(10), "Matematika", int16(10), "Desc", "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true}, "Matematika"))
		mockPool.ExpectQuery("SELECT").
			WithArgs("ABC123").
			WillReturnRows(pgxmock.NewRows(anggotaColumns).
				AddRow(int32(1), "Guru Utama", "utama@example.com", pgtype.Text{}, true, now))

		err := h.DemoteGuru(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
