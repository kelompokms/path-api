package kelas_test

import (
	"net/http"
	"net/http/httptest"
	"path-api/internal/db"
	"path-api/internal/kelas"
	"path-api/internal/utils"
	"regexp"
	"testing"
	"time"

	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/assert"
)

func TestList(t *testing.T) {
	t.Run("List Kelas", func(t *testing.T) {
		mock, err := pgxmock.NewPool()
		assert.NoError(t, err)
		defer mock.Close()

		queries := db.New(mock)

		h := kelas.New(queries)

		token, err := utils.NewToken(1)
		assert.NoError(t, err)

		e := echo.New()
		req := httptest.NewRequest(http.MethodGet, "/kelas", nil)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		jwtMiddleware := echojwt.WithConfig(utils.JWTConfig)

		rows := mock.NewRows([]string{"id", "nama", "tingkat", "pembuat", "kode_kelas", "dibuat"}).
			AddRow(1, "Matematika", "Kelas 10", 1, "ABCDEF", time.Now()).
			AddRow(2, "Fisika", "Kelas 10", 1, "FEDCBA", time.Now())

		mock.ExpectQuery(regexp.QuoteMeta(`SELECT kelas.id, kelas.nama, kelas.tingkat, kelas.pembuat, kelas.kode_kelas, kelas.dibuat FROM anggota_kelas JOIN kelas ON kelas_id = kelas.id WHERE pengguna_id = $1`)).
			WithArgs(int32(1)).
			WillReturnRows(rows)

		chain := jwtMiddleware(h.List)
		err = chain(c)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
