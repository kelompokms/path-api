package pengguna

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func setAuthContext(c *echo.Context, userID int32, admin bool) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &utils.JWTCustomClaims{
		UserID: userID,
		Admin:  admin,
	})
	c.Set("user", token)
}

func TestGetPengguna(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful get", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/pengguna", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture"}).
			AddRow(int32(1), "John Doe", "john@example.com", pgtype.Text{String: "", Valid: false}, pgtype.Text{String: "", Valid: false})

		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(rows)

		err := h.Get(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestUpdateEmail(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful update email", func(t *testing.T) {
		reqBody := UpdateEmailRequest{
			Email: "newemail@example.com",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/pengguna/email", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		// Check if email already exists (returns ErrNoRows)
		mockPool.ExpectQuery("SELECT").WithArgs("newemail@example.com").WillReturnError(pgx.ErrNoRows)

		rows := pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture"}).
			AddRow(int32(1), "John Doe", "newemail@example.com", pgtype.Text{String: "", Valid: false}, pgtype.Text{String: "", Valid: false})

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), "newemail@example.com").WillReturnRows(rows)

		err := h.UpdateEmail(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestUpdatePhone(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful update phone", func(t *testing.T) {
		reqBody := UpdatePhoneRequest{
			Phone: "+6281234567890",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/pengguna/phone", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture"}).
			AddRow(int32(1), "John Doe", "john@example.com", pgtype.Text{String: "+6281234567890", Valid: true}, pgtype.Text{String: "", Valid: false})

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), pgtype.Text{String: "+6281234567890", Valid: true}).WillReturnRows(rows)

		err := h.UpdatePhone(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestUpdateProfilePicture(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful update profile picture", func(t *testing.T) {
		defer os.RemoveAll("uploads")

		img := image.NewRGBA(image.Rect(0, 0, 8, 8))
		img.Set(0, 0, color.RGBA{R: 255, A: 255})
		var pngBuf bytes.Buffer
		require.NoError(t, png.Encode(&pngBuf, img))

		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("profile_picture", "avatar.png")
		require.NoError(t, err)
		_, err = part.Write(pngBuf.Bytes())
		require.NoError(t, err)
		require.NoError(t, w.Close())

		req := httptest.NewRequest(http.MethodPatch, "/pengguna/profile-picture", &body)
		req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture"}).
			AddRow(int32(1), "John Doe", "john@example.com", pgtype.Text{String: "", Valid: false}, pgtype.Text{String: "uploads/pfp/avatar.png", Valid: true})

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), pgxmock.AnyArg()).WillReturnRows(rows)

		err = h.UpdateProfilePicture(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestResetPassword(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte("oldpassword123"), bcrypt.DefaultCost)

	t.Run("successful reset password", func(t *testing.T) {
		reqBody := ResetPasswordRequest{
			OldPassword: "oldpassword123",
			NewPassword: "newpassword123",
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/pengguna/reset-password", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, false)

		// GetPasswordPenggunaByID
		mockPool.ExpectQuery("SELECT").WithArgs(int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"password"}).AddRow(string(hashedPassword)),
		)
		// UpdatePasswordPengguna (Exec)
		mockPool.ExpectExec("UPDATE").WithArgs(int32(1), pgxmock.AnyArg()).WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := h.ResetPassword(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
