package postingan

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

func TestListTipe(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful list tipe", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/posts/tipe", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		rows := pgxmock.NewRows([]string{"id", "nama"}).
			AddRow(int32(1), "Tugas").
			AddRow(int32(2), "Materi")

		mockPool.ExpectQuery("SELECT").WillReturnRows(rows)

		err := h.ListTipe(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestCreatePostingan(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful create postingan", func(t *testing.T) {
		reqBody := CreatePostinganRequest{
			KelasID:   1,
			Judul:     "Tugas 1",
			Deskripsi: "Kerjakan halaman 10",
			Lampiran:  []string{"file.pdf"},
			TipeID:    1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/kelas/1/posts", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kelasID", Value: "1"},
		})
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "mata_pelajaran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Tugas 1", pgtype.Text{String: "Kerjakan halaman 10", Valid: true}, []string{"file.pdf"}, pgtype.Text{String: "", Valid: false}, int32(1), int32(1), int32(1), time.Now())

		mockPool.ExpectQuery("INSERT").WithArgs("Tugas 1", pgtype.Text{String: "Kerjakan halaman 10", Valid: true}, []string{"file.pdf"}, int32(1), int32(1), int32(1), pgtype.Text{String: "", Valid: false}).WillReturnRows(rows)

		err := h.Create(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestListPostingan(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful list postingan", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/posts?tipe=1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 1, false)

		// GetKelasID
		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "tipe_id", "pengguna_id", "kelas_id", "dibuat"}).
			AddRow(int32(1), "Tugas 1", pgtype.Text{String: "Deskripsi", Valid: true}, []string{}, int32(1), int32(1), int32(1), pgtype.Timestamp{Time: time.Now(), Valid: true})

		mockPool.ExpectQuery("SELECT").WithArgs(int32(1), int32(1), int32(1), pgtype.Text{String: "", Valid: false}).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestEditPostingan(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful edit postingan", func(t *testing.T) {
		reqBody := EditPostinganRequest{
			KelasID:     1,
			PostinganID: 1,
			Judul:       "Updated Judul",
			Deskripsi:   "Updated Deskripsi",
			TipeID:      1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/kelas/1/postingan/1", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kelasID", Value: "1"},
			{Name: "postinganID", Value: "1"},
		})
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "mata_pelajaran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Updated Judul", pgtype.Text{String: "Updated Deskripsi", Valid: true}, []string{}, pgtype.Text{String: "", Valid: false}, int32(1), int32(1), int32(1), time.Now())

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), "Updated Judul", pgtype.Text{String: "Updated Deskripsi", Valid: true}, []string(nil), int32(1)).WillReturnRows(rows)

		err := h.Edit(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
