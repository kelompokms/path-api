package files

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path-api/internal/utils"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type uploadFile struct {
	name string
	data []byte
}

func makePNGBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return buf.Bytes()
}

func makeFileHeader(t *testing.T, field, filename string, data []byte) *multipart.FileHeader {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile(field, filename)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	require.NoError(t, req.ParseMultipartForm(10<<20))
	return req.MultipartForm.File[field][0]
}

func newUploadContext(t *testing.T, e *echo.Echo, files ...uploadFile) (*echo.Context, *httptest.ResponseRecorder) {
	t.Helper()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for _, f := range files {
		part, err := w.CreateFormFile("files", f.name)
		require.NoError(t, err)
		_, err = part.Write(f.data)
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())

	req := httptest.NewRequest(http.MethodPost, "/files/postingan", &body)
	req.Header.Set(echo.HeaderContentType, w.FormDataContentType())
	rec := httptest.NewRecorder()

	return e.NewContext(req, rec), rec
}

func TestSaveProfilePicture(t *testing.T) {
	oldRoot := uploadRoot
	uploadRoot = t.TempDir()
	defer func() { uploadRoot = oldRoot }()

	t.Run("successful square png upload", func(t *testing.T) {
		fh := makeFileHeader(t, "profile_picture", "avatar.png", makePNGBytes(t, 10, 10))
		savedPath, err := SaveProfilePicture(1, fh)
		require.NoError(t, err)
		assert.Contains(t, savedPath, "/pfp/")
		assert.True(t, strings.HasSuffix(savedPath, ".png"))

		_, statErr := os.Stat(filepath.FromSlash(savedPath))
		assert.NoError(t, statErr)
	})

	t.Run("rejects non-square image", func(t *testing.T) {
		fh := makeFileHeader(t, "profile_picture", "rect.png", makePNGBytes(t, 10, 20))
		_, err := SaveProfilePicture(1, fh)
		require.Error(t, err)
		assert.Equal(t, "Gambar harus rasio 1:1", err.Error())
	})

	t.Run("rejects unsupported extension", func(t *testing.T) {
		fh := makeFileHeader(t, "profile_picture", "doc.txt", []byte("plain text"))
		_, err := SaveProfilePicture(1, fh)
		require.Error(t, err)
		assert.Equal(t, "Extension tidak didukung", err.Error())
	})
}

func TestUploadPostinganFiles(t *testing.T) {
	oldRoot := uploadRoot
	uploadRoot = t.TempDir()
	defer func() { uploadRoot = oldRoot }()

	e := echo.New()
	e.Validator = utils.NewValidator()
	h := New(nil)

	c, rec := newUploadContext(t, e,
		uploadFile{name: "tugas.pdf", data: []byte("%PDF-1.4\n1 0 obj\n")},
		uploadFile{name: "catatan.txt", data: []byte("hello world")},
	)

	err := h.UploadPostinganFiles(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var resp UploadPostinganFilesResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp.Files, 2)

	for _, f := range resp.Files {
		assert.Regexp(t, `^/uploads/\d{4}/\d{2}/\d{2}/[a-f0-9]{32}\.[a-z0-9]+$`, f.Path)
		assert.NotEmpty(t, f.Nama)
		assert.Greater(t, f.Ukuran, int64(0))

		rel := strings.TrimPrefix(f.Path, "/uploads/")
		_, statErr := os.Stat(filepath.Join(uploadRoot, filepath.FromSlash(rel)))
		assert.NoError(t, statErr)
	}
}

func TestUploadPostinganFilesTooMany(t *testing.T) {
	e := echo.New()
	e.Validator = utils.NewValidator()
	h := New(nil)

	files := make([]uploadFile, MaxPostinganFiles+1)
	for i := range files {
		files[i] = uploadFile{name: fmt.Sprintf("file%d.txt", i), data: []byte("hello")}
	}

	c, _ := newUploadContext(t, e, files...)

	err := h.UploadPostinganFiles(c)
	require.Error(t, err)

	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

func TestUploadPostinganFilesTooLarge(t *testing.T) {
	oldRoot := uploadRoot
	uploadRoot = t.TempDir()
	defer func() { uploadRoot = oldRoot }()

	e := echo.New()
	e.Validator = utils.NewValidator()
	h := New(nil)

	c, _ := newUploadContext(t, e, uploadFile{
		name: "big.txt",
		data: bytes.Repeat([]byte("a"), MaxPostinganFileSize+1),
	})

	err := h.UploadPostinganFiles(c)
	require.Error(t, err)

	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}

func TestUploadPostinganFilesRejectsUnsupportedType(t *testing.T) {
	oldRoot := uploadRoot
	uploadRoot = t.TempDir()
	defer func() { uploadRoot = oldRoot }()

	e := echo.New()
	e.Validator = utils.NewValidator()
	h := New(nil)

	c, _ := newUploadContext(t, e, uploadFile{
		name: "evil.html",
		data: []byte("<!doctype html><html><body>hello</body></html>"),
	})

	err := h.UploadPostinganFiles(c)
	require.Error(t, err)

	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
}
