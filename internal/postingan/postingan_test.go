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

	t.Run("successful create postingan as guru", func(t *testing.T) {
		reqBody := CreatePostinganRequest{
			Judul:     "Tugas 1",
			Deskripsi: "Kerjakan halaman 10",
			Lampiran:  []string{"https://example.com/file.pdf"},
			TipeID:    2,
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

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Tugas 1", pgtype.Text{String: "Kerjakan halaman 10", Valid: true}, []string{"https://example.com/file.pdf"}, int32(1), int32(1), int32(2), pgtype.Timestamp{Time: time.Now(), Valid: true})

		mockPool.ExpectQuery("INSERT").WithArgs("Tugas 1", pgtype.Text{String: "Kerjakan halaman 10", Valid: true}, []string{"https://example.com/file.pdf"}, int32(2), int32(1), int32(1)).WillReturnRows(rows)

		err := h.Create(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when non-guru creates materi", func(t *testing.T) {
		reqBody := CreatePostinganRequest{
			Judul:     "Materi Ilegal",
			Deskripsi: "Siswa tidak boleh membuat materi",
			TipeID:    2,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/posts", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(2), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(false),
		)

		err := h.Create(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("not found when kelas does not exist", func(t *testing.T) {
		reqBody := CreatePostinganRequest{
			Judul:  "Materi",
			TipeID: 2,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPost, "/kelas/UNKNOWN/posts", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "UNKNOWN"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("UNKNOWN").WillReturnError(pgx.ErrNoRows)

		err := h.Create(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
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

	t.Run("successful list postingan with tipe filter as non-author member", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/posts?tipe=1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		// Member user 2 reads post created by guru user 1
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(2)).WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).AddRow(true),
		)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "tipe_id", "pengguna_id", "kelas_id", "dibuat"}).
			AddRow(int32(1), "Tugas 1", pgtype.Text{String: "Deskripsi", Valid: true}, []string(nil), int32(1), int32(1), int32(10), pgtype.Timestamp{Time: time.Now(), Valid: true})

		mockPool.ExpectQuery("SELECT p.id").WithArgs(int32(10), pgtype.Int4{Int32: 1, Valid: true}).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []PostinganResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, int32(1), resp[0].PenggunaID)
		assert.Equal(t, []string{}, resp[0].Lampiran)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("successful list postingan without tipe filter returns empty json array when empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/posts", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).AddRow(true),
		)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "tipe_id", "pengguna_id", "kelas_id", "dibuat"})
		mockPool.ExpectQuery("SELECT p.id").WithArgs(int32(10), pgtype.Int4{Valid: false}).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "[]\n", rec.Body.String())
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when user is not a member of kelas", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/posts?tipe=1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 99, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(99)).WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).AddRow(false),
		)

		err := h.List(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("admin can list postingan without class membership", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/posts", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 1, true)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(10)),
		)
		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "tipe_id", "pengguna_id", "kelas_id", "dibuat"})
		mockPool.ExpectQuery("SELECT p.id").WithArgs(int32(10), pgtype.Int4{Valid: false}).WillReturnRows(rows)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("not found when kelas does not exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/UNKNOWN/posts?tipe=2", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "UNKNOWN"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("UNKNOWN").WillReturnError(pgx.ErrNoRows)

		err := h.List(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("bad request when tipe out of range", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/posts?tipe=99", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
		})
		setAuthContext(c, 1, false)

		err := h.List(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
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

	t.Run("successful edit postingan as guru", func(t *testing.T) {
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

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Updated Judul", pgtype.Text{String: "Updated Deskripsi", Valid: true}, []string{"https://example.com/file.pdf"}, int32(1), int32(1), int32(1), pgtype.Timestamp{Time: time.Now(), Valid: true})

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), int32(1), "Updated Judul", pgtype.Text{String: "Updated Deskripsi", Valid: true}, []string{"https://example.com/file.pdf"}, int32(1)).WillReturnRows(rows)

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

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Updated Judul", pgtype.Text{String: "", Valid: false}, []string{"existing.pdf"}, int32(1), int32(1), int32(1), pgtype.Timestamp{Time: time.Now(), Valid: true})

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), int32(1), "Updated Judul", pgtype.Text{String: "", Valid: false}, []string(nil), int32(1)).WillReturnRows(rows)

		err := h.Edit(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("empty lampiran clears existing attachments", func(t *testing.T) {
		empty := []string{}
		reqBody := EditPostinganRequest{
			PostinganID: 1,
			Judul:       "Updated Judul",
			Lampiran:    &empty,
			TipeID:      2,
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

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		rows := pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(int32(1), "Updated Judul", pgtype.Text{String: "", Valid: false}, []string{}, int32(1), int32(1), int32(2), pgtype.Timestamp{Time: time.Now(), Valid: true})

		mockPool.ExpectQuery("UPDATE").WithArgs(int32(1), int32(1), "Updated Judul", pgtype.Text{String: "", Valid: false}, []string{}, int32(2)).WillReturnRows(rows)

		err := h.Edit(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp PostinganResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, []string{}, resp.Lampiran)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when non-guru edits postingan", func(t *testing.T) {
		reqBody := EditPostinganRequest{
			PostinganID: 1,
			Judul:       "Updated Judul",
			TipeID:      2,
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
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(2), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(false),
		)

		err := h.Edit(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("not found when postingan does not belong to kelas", func(t *testing.T) {
		reqBody := EditPostinganRequest{
			PostinganID: 99,
			Judul:       "Updated Judul",
			TipeID:      2,
		}
		jsonBody, _ := json.Marshal(reqBody)

		req := httptest.NewRequest(http.MethodPatch, "/kelas/ABC123/postingan/99", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "99"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("UPDATE").WithArgs(int32(99), int32(1), "Updated Judul", pgtype.Text{String: "", Valid: false}, []string(nil), int32(2)).WillReturnError(pgx.ErrNoRows)

		err := h.Edit(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
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

func expectAuthorizeMuridTugas(mockPool pgxmock.PgxPoolIface, kodeKelas string, kelasID, userID, postID, tipeID int32) {
	mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs(kodeKelas).WillReturnRows(
		pgxmock.NewRows([]string{"id"}).AddRow(kelasID),
	)
	mockPool.ExpectQuery("SELECT EXISTS").WithArgs(kodeKelas, userID).WillReturnRows(
		pgxmock.NewRows([]string{"exists"}).AddRow(true),
	)
	mockPool.ExpectQuery("SELECT COALESCE").WithArgs(userID, kelasID).WillReturnRows(
		pgxmock.NewRows([]string{"is_guru"}).AddRow(false),
	)
	mockPool.ExpectQuery("SELECT id, judul, deskripsi, lampiran").WithArgs(postID, kelasID).WillReturnRows(
		pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
			AddRow(postID, "Tugas 1", pgtype.Text{String: "Kerjakan", Valid: true}, []string(nil), int32(1), kelasID, tipeID, pgtype.Timestamp{Time: time.Now(), Valid: true}),
	)
}

func TestGetPengumpulan(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("returns unsubmitted state when no row exists", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/postingan/20/pengumpulan/saya", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 20, 1)
		mockPool.ExpectQuery("SELECT id, postingan_id, pengguna_id, lampiran, dibuat").
			WithArgs(int32(20), int32(2)).
			WillReturnError(pgx.ErrNoRows)

		err := h.GetPengumpulan(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp PengumpulanResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.False(t, resp.Submitted)
		assert.Equal(t, int32(20), resp.PostinganID)
		assert.Equal(t, int32(2), resp.PenggunaID)
		assert.Equal(t, []string{}, resp.Lampiran)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("returns existing submission", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/postingan/20/pengumpulan/saya", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 20, 1)
		mockPool.ExpectQuery("SELECT id, postingan_id, pengguna_id, lampiran, dibuat").
			WithArgs(int32(20), int32(2)).
			WillReturnRows(
				pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "lampiran", "dibuat"}).
					AddRow(int32(5), int32(20), int32(2), []string{"https://example.com/jawaban"}, pgtype.Timestamp{Time: time.Now(), Valid: true}),
			)

		err := h.GetPengumpulan(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp PengumpulanResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.True(t, resp.Submitted)
		assert.Equal(t, int32(5), resp.ID)
		assert.Equal(t, []string{"https://example.com/jawaban"}, resp.Lampiran)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestSubmitPengumpulan(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()
	uploadPath := "/uploads/2026/11/16/" + strings.Repeat("a", 32) + ".pdf"

	t.Run("creates new submission as murid", func(t *testing.T) {
		jsonBody, _ := json.Marshal(SubmitPengumpulanRequest{
			Lampiran: []string{uploadPath, "https://example.com/karya"},
		})
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/postingan/20/pengumpulan", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 20, 1)
		mockPool.ExpectQuery("SELECT id, postingan_id, pengguna_id, lampiran, dibuat").
			WithArgs(int32(20), int32(2)).
			WillReturnError(pgx.ErrNoRows)
		mockPool.ExpectQuery("INSERT INTO pengumpulan_tugas").
			WithArgs(int32(20), int32(2), []string{uploadPath, "https://example.com/karya"}).
			WillReturnRows(
				pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "lampiran", "dibuat"}).
					AddRow(int32(9), int32(20), int32(2), []string{uploadPath, "https://example.com/karya"}, pgtype.Timestamp{Time: time.Now(), Valid: true}),
			)

		err := h.SubmitPengumpulan(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)

		var resp PengumpulanResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.True(t, resp.Submitted)
		assert.Equal(t, int32(9), resp.ID)
		assert.Equal(t, []string{uploadPath, "https://example.com/karya"}, resp.Lampiran)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("updates existing submission and allows empty lampiran for tandai selesai", func(t *testing.T) {
		jsonBody, _ := json.Marshal(SubmitPengumpulanRequest{
			Lampiran: []string{},
		})
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/postingan/20/pengumpulan", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 20, 1)
		mockPool.ExpectQuery("SELECT id, postingan_id, pengguna_id, lampiran, dibuat").
			WithArgs(int32(20), int32(2)).
			WillReturnRows(
				pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "lampiran", "dibuat"}).
					AddRow(int32(9), int32(20), int32(2), []string{uploadPath}, pgtype.Timestamp{Time: time.Now(), Valid: true}),
			)
		mockPool.ExpectQuery("UPDATE pengumpulan_tugas").
			WithArgs(int32(9), int32(2), []string{}).
			WillReturnRows(
				pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "lampiran", "dibuat"}).
					AddRow(int32(9), int32(20), int32(2), []string{}, pgtype.Timestamp{Time: time.Now(), Valid: true}),
			)

		err := h.SubmitPengumpulan(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp PengumpulanResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.True(t, resp.Submitted)
		assert.Equal(t, []string{}, resp.Lampiran)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("rejects invalid attachment path", func(t *testing.T) {
		jsonBody, _ := json.Marshal(SubmitPengumpulanRequest{
			Lampiran: []string{"javascript:alert(1)"},
		})
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/postingan/20/pengumpulan", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 20, 1)

		err := h.SubmitPengumpulan(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestTarikPengumpulan(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()
	uploadPath := "/uploads/2026/11/16/" + strings.Repeat("b", 32) + ".pdf"

	t.Run("successful withdraw deletes submission and returns lampiran", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/kelas/ABC123/postingan/20/pengumpulan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 20, 1)
		mockPool.ExpectQuery("DELETE FROM pengumpulan_tugas").
			WithArgs(int32(20), int32(2)).
			WillReturnRows(
				pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "lampiran", "dibuat"}).
					AddRow(int32(11), int32(20), int32(2), []string{uploadPath}, pgtype.Timestamp{Time: time.Now(), Valid: true}),
			)

		err := h.TarikPengumpulan(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp TarikPengumpulanResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		assert.Equal(t, "Pengumpulan tugas berhasil ditarik", resp.Message)
		assert.Equal(t, []string{uploadPath}, resp.Lampiran)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("returns 404 when murid has no submission to withdraw", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/kelas/ABC123/postingan/20/pengumpulan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 20, 1)
		mockPool.ExpectQuery("DELETE FROM pengumpulan_tugas").
			WithArgs(int32(20), int32(2)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "lampiran", "dibuat"}))

		err := h.TarikPengumpulan(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusNotFound, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when guru tries to withdraw", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/kelas/ABC123/postingan/20/pengumpulan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)

		err := h.TarikPengumpulan(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("bad request when postingan is materi", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/kelas/ABC123/postingan/10/pengumpulan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "10"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 10, 2)

		err := h.TarikPengumpulan(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestListPengumpulan(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()
	uploadPath := "/uploads/2026/11/16/" + strings.Repeat("c", 32) + ".pdf"

	t.Run("successful list pengumpulan as guru", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/postingan/20/pengumpulan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT id, judul, deskripsi, lampiran").WithArgs(int32(20), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
				AddRow(int32(20), "Tugas 1", pgtype.Text{String: "Kerjakan", Valid: true}, []string(nil), int32(1), int32(1), int32(1), pgtype.Timestamp{Time: time.Now(), Valid: true}),
		)
		mockPool.ExpectQuery("SELECT\\s+pt\\.id").WithArgs(int32(20)).WillReturnRows(
			pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "nama", "email", "profile_picture", "lampiran", "dibuat"}).
				AddRow(int32(5), int32(20), int32(2), "Budi Santoso", "budi@example.com", pgtype.Text{String: "uploads/pfp/budi.jpg", Valid: true}, []string{uploadPath}, pgtype.Timestamp{Time: time.Now(), Valid: true}).
				AddRow(int32(6), int32(20), int32(3), "Siti Aminah", "siti@example.com", pgtype.Text{Valid: false}, []string(nil), pgtype.Timestamp{Time: time.Now(), Valid: true}),
		)

		err := h.ListPengumpulan(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []PengumpulanItemResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp, 2)
		assert.Equal(t, "Budi Santoso", resp[0].Nama)
		assert.Equal(t, "budi@example.com", resp[0].Email)
		assert.Equal(t, "uploads/pfp/budi.jpg", resp[0].ProfilePicture)
		assert.Equal(t, []string{uploadPath}, resp[0].Lampiran)
		assert.Equal(t, "Siti Aminah", resp[1].Nama)
		assert.Equal(t, "", resp[1].ProfilePicture)
		assert.Equal(t, []string{}, resp[1].Lampiran)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("returns empty json array when no submissions exist", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/postingan/20/pengumpulan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT id, judul, deskripsi, lampiran").WithArgs(int32(20), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
				AddRow(int32(20), "Tugas 1", pgtype.Text{String: "Kerjakan", Valid: true}, []string(nil), int32(1), int32(1), int32(1), pgtype.Timestamp{Time: time.Now(), Valid: true}),
		)
		mockPool.ExpectQuery("SELECT\\s+pt\\.id").WithArgs(int32(20)).WillReturnRows(
			pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "nama", "email", "profile_picture", "lampiran", "dibuat"}),
		)

		err := h.ListPengumpulan(c)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "[]\n", rec.Body.String())
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when non-guru lists pengumpulan", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/postingan/20/pengumpulan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(2), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(false),
		)

		err := h.ListPengumpulan(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("bad request when postingan is materi", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/postingan/10/pengumpulan", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "10"},
		})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT id, judul, deskripsi, lampiran").WithArgs(int32(10), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
				AddRow(int32(10), "Materi 1", pgtype.Text{String: "Baca", Valid: true}, []string(nil), int32(1), int32(1), int32(2), pgtype.Timestamp{Time: time.Now(), Valid: true}),
		)

		err := h.ListPengumpulan(c)
		assert.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestPostinganDibuatUsesGMT7(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()

	utcTime := time.Date(2026, 11, 18, 2, 16, 0, 0, time.UTC)
	expectedWIB := `"dibuat":"2026-11-18T09:16:00+07:00"`

	t.Run("Create returns dibuat in +07:00", func(t *testing.T) {
		jsonBody, _ := json.Marshal(CreatePostinganRequest{
			Judul:  "Materi 1",
			TipeID: 2,
		})
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/posts", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "ABC123"}})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("INSERT").WithArgs("Materi 1", pgtype.Text{}, []string(nil), int32(2), int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
				AddRow(int32(1), "Materi 1", pgtype.Text{}, []string(nil), int32(1), int32(1), int32(2), pgtype.Timestamp{Time: utcTime, Valid: true}),
		)

		require.NoError(t, h.Create(c))
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), expectedWIB)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("List returns dibuat in +07:00", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/posts", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "ABC123"}})
		setAuthContext(c, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT EXISTS").WithArgs("ABC123", int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"exists"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT p.id").WithArgs(int32(1), pgtype.Int4{Valid: false}).WillReturnRows(
			pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "tipe_id", "pengguna_id", "kelas_id", "dibuat"}).
				AddRow(int32(1), "Materi 1", pgtype.Text{}, []string(nil), int32(2), int32(1), int32(1), pgtype.Timestamp{Time: utcTime, Valid: true}),
		)

		require.NoError(t, h.List(c))
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), expectedWIB)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("SubmitPengumpulan and ListPengumpulan return dibuat in +07:00", func(t *testing.T) {
		jsonBody, _ := json.Marshal(SubmitPengumpulanRequest{Lampiran: []string{}})
		req := httptest.NewRequest(http.MethodPost, "/kelas/ABC123/postingan/20/pengumpulan", bytes.NewReader(jsonBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(c, 2, false)

		expectAuthorizeMuridTugas(mockPool, "ABC123", 1, 2, 20, 1)
		mockPool.ExpectQuery("SELECT id, postingan_id, pengguna_id, lampiran, dibuat").
			WithArgs(int32(20), int32(2)).
			WillReturnError(pgx.ErrNoRows)
		mockPool.ExpectQuery("INSERT INTO pengumpulan_tugas").
			WithArgs(int32(20), int32(2), []string{}).
			WillReturnRows(
				pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "lampiran", "dibuat"}).
					AddRow(int32(9), int32(20), int32(2), []string{}, pgtype.Timestamp{Time: utcTime, Valid: true}),
			)

		require.NoError(t, h.SubmitPengumpulan(c))
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), expectedWIB)

		listReq := httptest.NewRequest(http.MethodGet, "/kelas/ABC123/postingan/20/pengumpulan", nil)
		listRec := httptest.NewRecorder()
		listCtx := e.NewContext(listReq, listRec)
		listCtx.SetPathValues(echo.PathValues{
			{Name: "kode_kelas", Value: "ABC123"},
			{Name: "postinganID", Value: "20"},
		})
		setAuthContext(listCtx, 1, false)

		mockPool.ExpectQuery("SELECT id FROM kelas").WithArgs("ABC123").WillReturnRows(
			pgxmock.NewRows([]string{"id"}).AddRow(int32(1)),
		)
		mockPool.ExpectQuery("SELECT COALESCE").WithArgs(int32(1), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"is_guru"}).AddRow(true),
		)
		mockPool.ExpectQuery("SELECT id, judul, deskripsi, lampiran").WithArgs(int32(20), int32(1)).WillReturnRows(
			pgxmock.NewRows([]string{"id", "judul", "deskripsi", "lampiran", "pengguna_id", "kelas_id", "tipe_id", "dibuat"}).
				AddRow(int32(20), "Tugas 1", pgtype.Text{}, []string(nil), int32(1), int32(1), int32(1), pgtype.Timestamp{Time: utcTime, Valid: true}),
		)
		mockPool.ExpectQuery("SELECT pt.id").WithArgs(int32(20)).WillReturnRows(
			pgxmock.NewRows([]string{"id", "postingan_id", "pengguna_id", "nama", "email", "profile_picture", "lampiran", "dibuat"}).
				AddRow(int32(9), int32(20), int32(2), "Budi", "budi@example.com", pgtype.Text{}, []string(nil), pgtype.Timestamp{Time: utcTime, Valid: true}),
		)

		require.NoError(t, h.ListPengumpulan(listCtx))
		assert.Equal(t, http.StatusOK, listRec.Code)
		assert.Contains(t, listRec.Body.String(), expectedWIB)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
