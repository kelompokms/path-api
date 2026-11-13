package kelas

import (
	"bytes"
	"encoding/json"
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

		rows := pgxmock.NewRows([]string{"id", "nama", "tingkat", "kode_kelas", "dibuat", "pengampu"}).
			AddRow(int32(1), "Matematika", pgtype.Text{String: "SMA", Valid: true}, "ABC123", time.Now(), pgtype.Int4{Int32: 1, Valid: true})

		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
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

		rows := pgxmock.NewRows([]string{"id", "nama", "tingkat", "kode_kelas", "dibuat", "pengampu"}).
			AddRow(int32(1), "Matematika", pgtype.Text{String: "SMA", Valid: true}, "ABC123", time.Now(), pgtype.Int4{Int32: 1, Valid: true})

		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(rows)

		err := h.Get(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
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
			Nama:    "Fisika",
			Tingkat: 10,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/kelas", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows([]string{"id", "nama", "tingkat", "kode_kelas", "dibuat", "pengampu"}).
			AddRow(int32(2), "Fisika", pgtype.Text{String: "SMA", Valid: true}, "XYZ789", time.Now(), pgtype.Int4{Int32: 1, Valid: false})

		mockPool.ExpectQuery("INSERT").WithArgs("Fisika", pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).WillReturnRows(rows)

		err := h.Create(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
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
}
