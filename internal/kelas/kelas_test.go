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

var kelasColumns = []string{"id", "nama", "tingkat", "tahun_ajaran", "kode_kelas", "dibuat", "pengampu"}

func TestListKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}

		rowsJoined := pgxmock.NewRows(kelasColumns).
			AddRow(int32(1), "Matematika", int16(10), "2026/2027", "ABC123", now, pgtype.Int4{Int32: 2, Valid: true})
		rowsDiampu := pgxmock.NewRows(kelasColumns).
			AddRow(int32(2), "Fisika", int16(11), "2026/2027", "DEF456", now, pgtype.Int4{Int32: 1, Valid: true})

		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(rowsJoined)
		mockPool.ExpectQuery("SELECT").WithArgs(pgtype.Int4{Int32: 1, Valid: true}).WillReturnRows(rowsDiampu)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []db.Kela
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Len(t, resp, 2)
		assert.NoError(t, mockPool.ExpectationsWereMet())
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
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("error on ListKelasDiampu query", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		rowsJoined := pgxmock.NewRows(kelasColumns)
		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(rowsJoined)
		mockPool.ExpectQuery("SELECT").WithArgs(pgtype.Int4{Int32: 1, Valid: true}).WillReturnError(errors.New("db error"))

		err := h.List(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusUnauthorized, httpErr.Code)
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
			AddRow(int32(1), "Matematika", int16(10), "2026/2027", "ABC123", now, pgtype.Int4{Int32: 1, Valid: true})

		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(rows)

		err := h.Get(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
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

func TestCreateKelas(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful create", func(t *testing.T) {
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
		setAuthContext(c, 1, false)

		mockPool.ExpectExec("INSERT").
			WithArgs("Fisika", int16(10), "2026/2027", pgtype.Int4{Int32: 1, Valid: true}, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := h.Create(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
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
		setAuthContext(c, 1, false)

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
		setAuthContext(c, 1, false)

		mockPool.ExpectExec("INSERT").
			WithArgs("Fisika", int16(10), "2026/2027", pgtype.Int4{Int32: 1, Valid: true}, pgxmock.AnyArg()).
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
