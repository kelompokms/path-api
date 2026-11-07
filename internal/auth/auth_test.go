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
		mockPool.ExpectQuery("INSERT").WithArgs("John Doe", "john@example.com", pgxmock.AnyArg(), false).WillReturnRows(
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
			pgxmock.NewRows([]string{"id", "password", "admin"}).AddRow(int32(1), "hashed", false),
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
			pgxmock.NewRows([]string{"id", "password", "admin"}).AddRow(int32(1), string(hashedPassword), false),
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
			pgxmock.NewRows([]string{"id", "password", "admin"}).AddRow(int32(1), string(hashedPassword), false),
		)

		err := h.Login(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.Equal(t, "username atau password salah", httpErr.Message)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
