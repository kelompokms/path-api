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

		rows := pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture", "password", "id_role_pengguna", "role", "dibuat"}).
			AddRow(int32(1), "Admin", "admin@example.com", pgtype.Text{String: "", Valid: false}, pgtype.Text{String: "", Valid: false}, pgtype.Text{String: "hash", Valid: true}, int32(1), "admin", time.Now())

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

		rows := pgxmock.NewRows([]string{"id", "nama_kegiatan", "keterangan", "tanggal_mulai", "tanggal_selesai", "dibuat"}).
			AddRow(int32(1), "UTS Semester Ganjil", "", pgtype.Date{Time: tglMulai, Valid: true}, pgtype.Date{Time: tglSelesai, Valid: true}, now)

		mockPool.ExpectQuery("INSERT INTO kalender_akademik").
			WithArgs("UTS Semester Ganjil", "", pgtype.Date{Time: tglMulai, Valid: true}, pgtype.Date{Time: tglSelesai, Valid: true}).
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

		rows := pgxmock.NewRows([]string{"id", "nama_kegiatan", "keterangan", "tanggal_mulai", "tanggal_selesai", "dibuat"}).
			AddRow(int32(1), "UTS Semester Ganjil", "Pelaksanaan ujian", pgtype.Date{Time: tglMulai, Valid: true}, pgtype.Date{Time: tglSelesai, Valid: true}, now)

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

func TestListPenggunaAdmin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful list all guru and murid for admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/pengguna", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		guruRows := pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture", "kelas"}).
			AddRow(int32(2), "Eko Kusnandar", "eko@example.com", "08123", "", []string{"Pemrograman Web"})
		muridRows := pgxmock.NewRows([]string{"id", "nama", "email", "phone", "profile_picture", "kelas"}).
			AddRow(int32(3), "Aji Sasongko", "aji@example.com", "", "", []string{"X PPLG 1"})

		mockPool.ExpectQuery("FROM pengguna p").WillReturnRows(guruRows)
		mockPool.ExpectQuery("FROM pengguna p").WillReturnRows(muridRows)

		err := h.ListPengguna(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp ListGuruMuridResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp.Guru, 1)
		assert.Equal(t, "Eko Kusnandar", resp.Guru[0].Nama)
		assert.Equal(t, "guru", resp.Guru[0].Role)
		require.Len(t, resp.Murid, 1)
		assert.Equal(t, "Aji Sasongko", resp.Murid[0].Nama)
		assert.Equal(t, "murid", resp.Murid[0].Role)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden for non-admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/pengguna", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 2, false)

		err := h.ListPengguna(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
	})
}

func TestListKelasAdmin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("filters all classes by tahun_ajaran, tingkat, and nama", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/kelas?tahun_ajaran=2026/2027&tingkat=10&nama=PPLG", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}
		rows := pgxmock.NewRows([]string{"id", "nama", "deskripsi", "tingkat", "tahun_ajaran", "kode_kelas", "dibuat", "pengampu", "nama_pengampu"}).
			AddRow(int32(10), "X PPLG 1", "Kelas pemrograman", int16(10), "2026/2027", "PPLG10", now, pgtype.Int4{Int32: 2, Valid: true}, "Eko Kusnandar")

		mockPool.ExpectQuery("FROM kelas k").
			WithArgs(
				pgtype.Text{String: "2026/2027", Valid: true},
				pgtype.Int2{Int16: 10, Valid: true},
				pgtype.Text{String: "PPLG", Valid: true},
			).
			WillReturnRows(rows)

		err := h.ListKelas(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp []AdminKelasResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Len(t, resp, 1)
		assert.Equal(t, "X PPLG 1", resp[0].Nama)
		assert.Equal(t, "Eko Kusnandar", resp[0].NamaPengampu)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("rejects invalid tingkat filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/kelas?tingkat=abc", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		err := h.ListKelas(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	})

	t.Run("forbidden for non-admin", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/kelas", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 2, false)

		err := h.ListKelas(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
	})
}

func TestCreateAgendaKalender(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	queries := db.New(mockPool)
	h := New(queries)

	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("successful create agenda with keterangan", func(t *testing.T) {
		body := `{"nama_kegiatan":"Rapat Orang Tua","keterangan":"Pertemuan semester ganjil","tanggal_mulai":"2026-11-20","tanggal_selesai":"2026-11-21"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/kalender/agenda", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		tglMulai, _ := time.Parse("2006-01-02", "2026-11-20")
		tglSelesai, _ := time.Parse("2006-01-02", "2026-11-21")
		now := pgtype.Timestamp{Time: time.Now(), Valid: true}

		rows := pgxmock.NewRows([]string{"id", "nama_kegiatan", "keterangan", "tanggal_mulai", "tanggal_selesai", "dibuat"}).
			AddRow(int32(5), "Rapat Orang Tua", "Pertemuan semester ganjil", pgtype.Date{Time: tglMulai, Valid: true}, pgtype.Date{Time: tglSelesai, Valid: true}, now)

		mockPool.ExpectQuery("INSERT INTO kalender_akademik").
			WithArgs("Rapat Orang Tua", "Pertemuan semester ganjil", pgtype.Date{Time: tglMulai, Valid: true}, pgtype.Date{Time: tglSelesai, Valid: true}).
			WillReturnRows(rows)

		err := h.CreateAgendaKalender(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "Rapat Orang Tua")
		assert.Contains(t, rec.Body.String(), "Pertemuan semester ganjil")
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("rejects tanggal_selesai before tanggal_mulai", func(t *testing.T) {
		body := `{"nama_kegiatan":"Acara","keterangan":"Info","tanggal_mulai":"2026-11-20","tanggal_selesai":"2026-11-18"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/kalender/agenda", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		err := h.CreateAgendaKalender(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	})

	t.Run("forbidden for non-admin", func(t *testing.T) {
		body := `{"nama_kegiatan":"Acara","keterangan":"Info","tanggal_mulai":"2026-11-20"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/kalender/agenda", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 2, false)

		err := h.CreateAgendaKalender(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusForbidden, httpErr.Code)
	})
}

func TestUpdateRolePenggunaAdmin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("updates user role between murid and guru", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/admin/pengguna/5/role", bytes.NewBufferString(`{"role":"guru"}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "pengguna_id", Value: "5"}})
		setAuthContext(c, 1, true)

		mockPool.ExpectQuery("SELECT p.id, rp.nama").
			WithArgs(int32(5)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "role"}).AddRow(int32(5), "murid"))
		mockPool.ExpectExec("UPDATE pengguna").
			WithArgs(int32(5), "guru").
			WillReturnResult(pgxmock.NewResult("UPDATE", 1))

		err := h.UpdateRolePengguna(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("rejects changing admin account role or invalid role name", func(t *testing.T) {
		reqInvalid := httptest.NewRequest(http.MethodPatch, "/admin/pengguna/5/role", bytes.NewBufferString(`{"role":"admin"}`))
		reqInvalid.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		recInvalid := httptest.NewRecorder()
		cInvalid := e.NewContext(reqInvalid, recInvalid)
		cInvalid.SetPathValues(echo.PathValues{{Name: "pengguna_id", Value: "5"}})
		setAuthContext(cInvalid, 1, true)

		err := h.UpdateRolePengguna(cInvalid)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)

		reqAdminTarget := httptest.NewRequest(http.MethodPatch, "/admin/pengguna/1/role", bytes.NewBufferString(`{"role":"guru"}`))
		reqAdminTarget.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		recAdminTarget := httptest.NewRecorder()
		cAdminTarget := e.NewContext(reqAdminTarget, recAdminTarget)
		cAdminTarget.SetPathValues(echo.PathValues{{Name: "pengguna_id", Value: "1"}})
		setAuthContext(cAdminTarget, 1, true)

		mockPool.ExpectQuery("SELECT p.id, rp.nama").
			WithArgs(int32(1)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "role"}).AddRow(int32(1), "admin"))

		err = h.UpdateRolePengguna(cAdminTarget)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, err.(*echo.HTTPError).Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}

func TestCreateAndUploadKelasCSVAdmin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("admin creates single class manually with pengampu default null", func(t *testing.T) {
		body := `{"nama":"X PPLG 1","deskripsi":"Pemrograman","tingkat":10,"tahun_ajaran":"2026/2027"}`
		req := httptest.NewRequest(http.MethodPost, "/admin/kelas", bytes.NewBufferString(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		mockPool.ExpectExec("INSERT").
			WithArgs("X PPLG 1", "Pemrograman", int16(10), "2026/2027", pgtype.Int4{Valid: false}, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err := h.CreateKelas(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("admin creates batch classes via CSV with pengampu null", func(t *testing.T) {
		csvData := "nama,tingkat,tahun_ajaran,deskripsi\nX PPLG 1,10,2026/2027,Pemrograman\nXI RPL 1,XI,2026/2027,Basis Data\n"
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("file_csv", "kelas.csv")
		require.NoError(t, err)
		_, err = part.Write([]byte(csvData))
		require.NoError(t, err)
		require.NoError(t, w.Close())

		req := httptest.NewRequest(http.MethodPost, "/admin/kelas/csv", &body)
		req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		mockPool.ExpectExec("INSERT").
			WithArgs("X PPLG 1", "Pemrograman", int16(10), "2026/2027", pgtype.Int4{Valid: false}, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mockPool.ExpectExec("INSERT").
			WithArgs("XI RPL 1", "Basis Data", int16(11), "2026/2027", pgtype.Int4{Valid: false}, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = h.UploadKelasCSV(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "Berhasil membuat 2 kelas dari CSV")
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("admin creates batch classes via semicolon CSV template with pengampu null", func(t *testing.T) {
		csvData := "\xEF\xBB\xBFkelas;tahun ajaran;mata pelajaran;guru\nX;2026/2027;Fiber Optic;Yahya\nXI;2026/2027;Bahasa Indonesia;\n"
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("file_csv", "kelas.csv")
		require.NoError(t, err)
		_, err = part.Write([]byte(csvData))
		require.NoError(t, err)
		require.NoError(t, w.Close())

		req := httptest.NewRequest(http.MethodPost, "/admin/kelas/csv", &body)
		req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		setAuthContext(c, 1, true)

		mockPool.ExpectExec("INSERT").
			WithArgs("Fiber Optic", "", int16(10), "2026/2027", pgtype.Int4{Valid: false}, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))
		mockPool.ExpectExec("INSERT").
			WithArgs("Bahasa Indonesia", "", int16(11), "2026/2027", pgtype.Int4{Valid: false}, pgxmock.AnyArg()).
			WillReturnResult(pgxmock.NewResult("INSERT", 1))

		err = h.UploadKelasCSV(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), "Berhasil membuat 2 kelas dari CSV")
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("forbidden when non-admin creates manual or CSV kelas", func(t *testing.T) {
		reqManual := httptest.NewRequest(http.MethodPost, "/admin/kelas", bytes.NewBufferString(`{"nama":"X","tingkat":10,"tahun_ajaran":"2026/2027"}`))
		reqManual.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		recManual := httptest.NewRecorder()
		cManual := e.NewContext(reqManual, recManual)
		cManual.Set("user", jwt.NewWithClaims(jwt.SigningMethodHS256, &utils.JWTCustomClaims{UserID: 2, Role: "guru"}))

		err := h.CreateKelas(cManual)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, err.(*echo.HTTPError).Code)

		var body bytes.Buffer
		w := multipart.NewWriter(&body)
		part, err := w.CreateFormFile("file_csv", "kelas.csv")
		require.NoError(t, err)
		_, err = part.Write([]byte("nama,tingkat,tahun_ajaran\nX,10,2026/2027\n"))
		require.NoError(t, err)
		require.NoError(t, w.Close())

		reqCsv := httptest.NewRequest(http.MethodPost, "/admin/kelas/csv", &body)
		reqCsv.Header.Set(echo.HeaderContentType, w.FormDataContentType())
		recCsv := httptest.NewRecorder()
		cCsv := e.NewContext(reqCsv, recCsv)
		cCsv.Set("user", jwt.NewWithClaims(jwt.SigningMethodHS256, &utils.JWTCustomClaims{UserID: 3, Role: "murid"}))

		err = h.UploadKelasCSV(cCsv)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, err.(*echo.HTTPError).Code)
	})
}

func TestUpdatePengampuKelasAdmin(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := New(db.New(mockPool))
	e := echo.New()
	e.Validator = utils.NewValidator()

	t.Run("assigns user with guru role as class pengampu", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/admin/kelas/PPLG10/pengampu", bytes.NewBufferString(`{"pengampu_id":2}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "PPLG10"}})
		setAuthContext(c, 1, true)

		mockPool.ExpectQuery("SELECT id FROM kelas").
			WithArgs("PPLG10").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(10)))
		mockPool.ExpectQuery("SELECT p.id, rp.nama").
			WithArgs(int32(2)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "role"}).AddRow(int32(2), "guru"))

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}
		mockPool.ExpectQuery("UPDATE kelas").
			WithArgs("PPLG10", pgtype.Int4{Int32: 2, Valid: true}).
			WillReturnRows(pgxmock.NewRows([]string{"id", "nama", "tingkat", "deskripsi", "tahun_ajaran", "kode_kelas", "dibuat", "pengampu"}).
				AddRow(int32(10), "X PPLG 1", int16(10), "Desc", "2026/2027", "PPLG10", now, pgtype.Int4{Int32: 2, Valid: true}))

		err := h.UpdatePengampuKelas(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("rejects assigning murid as main pengampu", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/admin/kelas/PPLG10/pengampu", bytes.NewBufferString(`{"pengampu_id":5}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "PPLG10"}})
		setAuthContext(c, 1, true)

		mockPool.ExpectQuery("SELECT id FROM kelas").
			WithArgs("PPLG10").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(10)))
		mockPool.ExpectQuery("SELECT p.id, rp.nama").
			WithArgs(int32(5)).
			WillReturnRows(pgxmock.NewRows([]string{"id", "role"}).AddRow(int32(5), "murid"))

		err := h.UpdatePengampuKelas(c)
		require.Error(t, err)
		httpErr, ok := err.(*echo.HTTPError)
		require.True(t, ok)
		assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("clears pengampu back to null", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPatch, "/admin/kelas/PPLG10/pengampu", bytes.NewBufferString(`{"pengampu_id":null}`))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetPathValues(echo.PathValues{{Name: "kode_kelas", Value: "PPLG10"}})
		setAuthContext(c, 1, true)

		mockPool.ExpectQuery("SELECT id FROM kelas").
			WithArgs("PPLG10").
			WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(10)))

		now := pgtype.Timestamp{Time: time.Now(), Valid: true}
		mockPool.ExpectQuery("UPDATE kelas").
			WithArgs("PPLG10", pgtype.Int4{Valid: false}).
			WillReturnRows(pgxmock.NewRows([]string{"id", "nama", "tingkat", "deskripsi", "tahun_ajaran", "kode_kelas", "dibuat", "pengampu"}).
				AddRow(int32(10), "X PPLG 1", int16(10), "Desc", "2026/2027", "PPLG10", now, pgtype.Int4{Valid: false}))

		err := h.UpdatePengampuKelas(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})
}
