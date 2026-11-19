package auth

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestRegister(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful registration", func(t *testing.T) {
		reqBody := RegisterRequest{
			Nama:     "John Doe",
			Email:    "john@example.com",
			Password: "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockPool.ExpectQuery("SELECT").WithArgs("john@example.com").WillReturnError(pgx.ErrNoRows)
		mockPool.ExpectQuery("INSERT").WithArgs("John Doe", "john@example.com", pgxmock.AnyArg(), "murid").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)

		err := h.Register(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "token")
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("email already registered", func(t *testing.T) {
		reqBody := RegisterRequest{
			Nama:     "John Doe",
			Email:    "john@example.com",
			Password: "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockPool.ExpectQuery("SELECT").WithArgs("john@example.com").WillReturnRows(
			pgxmock.NewRows([]string{"id", "password", "role", "aktif"}).AddRow(int32(1), pgtype.Text{String: "hashed", Valid: true}, "murid", true),
		)

		err := h.Register(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Equal(t, "email sudah terdaftarkan", httpErr.Message)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestLogin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)

	t.Run("successful login", func(t *testing.T) {
		reqBody := LoginRequest{
			Email:    "john@example.com",
			Password: "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockPool.ExpectQuery("SELECT").WithArgs("john@example.com").WillReturnRows(
			pgxmock.NewRows([]string{"id", "password", "role", "aktif"}).AddRow(int32(1), pgtype.Text{String: string(hashedPassword), Valid: true}, "murid", true),
		)

		err := h.Login(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), "token")
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("invalid password", func(t *testing.T) {
		reqBody := LoginRequest{
			Email:    "john@example.com",
			Password: "wrongpassword",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockPool.ExpectQuery("SELECT").WithArgs("john@example.com").WillReturnRows(
			pgxmock.NewRows([]string{"id", "password", "role", "aktif"}).AddRow(int32(1), pgtype.Text{String: string(hashedPassword), Valid: true}, "murid", true),
		)

		err := h.Login(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Equal(t, "username atau password salah", httpErr.Message)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("oauth-only user cannot login with password", func(t *testing.T) {
		reqBody := LoginRequest{
			Email:    "oauth@example.com",
			Password: "somepassword123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockPool.ExpectQuery("SELECT").WithArgs("oauth@example.com").WillReturnRows(
			pgxmock.NewRows([]string{"id", "password", "role", "aktif"}).AddRow(int32(2), pgtype.Text{Valid: false}, "murid", true),
		)

		err := h.Login(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Equal(t, "akun ini terdaftar melalui OAuth, silakan login dengan provider OAuth", httpErr.Message)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("inactive user cannot login", func(t *testing.T) {
		reqBody := LoginRequest{
			Email:    "inactive@example.com",
			Password: "password123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/login", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		mockPool.ExpectQuery("SELECT").WithArgs("inactive@example.com").WillReturnRows(
			pgxmock.NewRows([]string{"id", "password", "role", "aktif"}).AddRow(int32(3), pgtype.Text{String: string(hashedPassword), Valid: true}, "murid", false),
		)

		err := h.Login(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.Equal(t, "akun tidak aktif", httpErr.Message)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
