package postingan

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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
			Judul:     "Tugas 1",
			Deskripsi: "Kerjakan halaman 10",
			Lampiran:  []string{"https://example.com/file.pdf"},
			TipeID:    1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/posts", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "mata_pelajaran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Tugas 1", pgtype.Text{String: "Kerjakan halaman 10", Valid: true}, []string{"https://example.com/file.pdf"}, pgtype.Text{String: "", Valid: false}, int32(1), int32(1), int32(1), time.Now())

		mockPool.ExpectQuery("INSERT").WithArgs("Tugas 1", pgtype.Text{String: "Kerjakan halaman 10", Valid: true}, []string{"https://example.com/file.pdf"}, int32(1), int32(1), int32(1), pgtype.Text{String: "", Valid: false}).WillReturnRows(rows)

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
			PostinganID: 1,
			Judul:       "Updated Judul",
			Deskripsi:   "Updated Deskripsi",
			Lampiran:    &[]string{"https://example.com/file.pdf"},
			TipeID:      1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/postingan/1", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "1"},
		})
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "mata_pelajaran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Updated Judul", pgtype.Text{String: "Updated Deskripsi", Valid: true}, []string{"https://example.com/file.pdf"}, pgtype.Text{String: "", Valid: false}, int32(1), int32(1), int32(1), time.Now())

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), "Updated Judul", pgtype.Text{String: "Updated Deskripsi", Valid: true}, []string{"https://example.com/file.pdf"}, int32(1)).WillReturnRows(rows)

		err := h.Edit(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("omitted lampiran keeps existing attachments", func(t *testing.T) {
		reqBody := EditPostinganRequest{
			PostinganID: 1,
			Judul:       "Updated Judul",
			TipeID:      1,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/postingan/1", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "1"},
		})
		setAuthContext(c, 1, false)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "mata_pelajaran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Updated Judul", pgtype.Text{String: "", Valid: false}, []string{"existing.pdf"}, pgtype.Text{String: "", Valid: false}, int32(1), int32(1), int32(1), time.Now())

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), "Updated Judul", pgtype.Text{String: "", Valid: false}, []string(nil), int32(1)).WillReturnRows(rows)

		err := h.Edit(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestNormalizeLampiran(t *testing.T) {
	uploadPath := "/uploads/2026/11/08/" + strings.Repeat("a", 32) + ".pdf"

	tests := []struct {
		name    string
		input   []string
		want    []string
		wantErr bool
	}{
		{
			name:  "nil lampiran",
			input: nil,
			want:  nil,
		},
		{
			name:  "link",
			input: []string{"https://example.com/file.pdf"},
			want:  []string{"https://example.com/file.pdf"},
		},
		{
			name:  "upload path",
			input: []string{uploadPath},
			want:  []string{uploadPath},
		},
		{
			name:  "relative upload path is normalized",
			input: []string{strings.TrimPrefix(uploadPath, "/")},
			want:  []string{uploadPath},
		},
		{
			name: "too many attachments",
			input: []string{
				"https://example.com/1", "https://example.com/2", "https://example.com/3",
				"https://example.com/4", "https://example.com/5", "https://example.com/6",
			},
			wantErr: true,
		},
		{
			name:    "plain file name is rejected",
			input:   []string{"file.pdf"},
			wantErr: true,
		},
		{
			name:    "non http scheme is rejected",
			input:   []string{"javascript:alert(1)"},
			wantErr: true,
		},
		{
			name:    "empty attachment is rejected",
			input:   []string{"   "},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeLampiran(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
