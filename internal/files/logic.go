package files

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path-api/internal/utils"
	"path/filepath"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/labstack/echo/v5"
)

const publicUploadPrefix = "/uploads"

// uploadRoot is a variable so tests can point uploads at a temporary directory.
var uploadRoot = "uploads"

func SaveProfilePicture(userID int32, fh *multipart.FileHeader) (string, error) {
	if fh.Size == 0 {
		return "", errors.New("file tidak ada")
	}
	if fh.Size > 5<<20 {
		return "", errors.New("ukuran file terlalu besar (maks. 5 MB)")
	}

	src, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	// 1. Detect the content type from the first 512 bytes.
	sniff := make([]byte, 512)
	n, err := io.ReadFull(src, sniff)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read upload: %w", err)
	}
	mimeType := http.DetectContentType(sniff[:n])
	ext, ok := allowedTypes[mimeType]
	if !ok {
		return "", errors.New("Extension tidak didukung")
	}

	// 2. Read dimensions only (cheap: does not decode the whole image).
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("seek upload: %w", err)
	}
	cfg, _, err := image.DecodeConfig(src)
	if err != nil {
		return "", errors.New("Gambar tidak valid")
	}
	if cfg.Width == 0 || cfg.Width != cfg.Height {
		return "", errors.New("Gambar harus rasio 1:1")
	}

	// 3. Build destination path: {root}/uploads/pfp/YYYY/MM/DD/
	now := time.Now()
	relDir := filepath.Join(uploadRoot, "pfp",
		now.Format("2006"), now.Format("01"), now.Format("02"))
	if err := os.MkdirAll(relDir, 0o755); err != nil {
		return "", fmt.Errorf("create upload dir: %w", err)
	}

	// 4. Generate unique file name: user{id}_{random}.{ext}
	suffix, err := utils.RandomHex(16)
	if err != nil {
		return "", fmt.Errorf("generate file name: %w", err)
	}
	filename := fmt.Sprintf("user%d_%s.%s", userID, suffix, ext)
	absPath := filepath.Join(relDir, filename)

	// 5. Write the file. O_EXCL guarantees we never overwrite an existing file.
	dst, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		dst.Close()
		os.Remove(absPath)
		return "", fmt.Errorf("seek upload: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(absPath)
		return "", fmt.Errorf("write file: %w", err)
	}
	if err := dst.Close(); err != nil {
		os.Remove(absPath)
		return "", fmt.Errorf("close file: %w", err)
	}

	return filepath.ToSlash(filepath.Join(relDir, filename)), nil
}

// UploadPostinganFiles handles the first step of the postingan workflow: the
// frontend uploads files, receives their paths, and sends those paths back in
// postingan.lampiran when creating the postingan.
func (h *Handler) UploadPostinganFiles(c *echo.Context) error {
	req := new(UploadPostinganFilesRequest)

	if err := c.Bind(req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	if err := c.Validate(req); err != nil {
		return utils.ValidationErrorHandler(c, err)
	}

	// Files larger than the in-memory limit are spooled to temp files by
	// ParseMultipartForm; make sure they are removed when the request ends.
	if form := c.Request().MultipartForm; form != nil {
		defer form.RemoveAll()
	}

	files, err := SavePostinganFiles(req.Files)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusCreated, UploadPostinganFilesResponse{Files: files})
}

// SavePostinganFiles validates and stores every file, removing any files that
// were already written if one of them fails.
func SavePostinganFiles(files []*multipart.FileHeader) ([]UploadedPostinganFile, error) {
	if len(files) == 0 {
		return nil, errors.New("file tidak ada")
	}
	if len(files) > MaxPostinganFiles {
		return nil, fmt.Errorf("maksimal %d file", MaxPostinganFiles)
	}

	saved := make([]UploadedPostinganFile, 0, len(files))
	for _, fh := range files {
		file, err := savePostinganFile(fh)
		if err != nil {
			removeUploadedFiles(saved)
			return nil, err
		}
		saved = append(saved, file)
	}

	return saved, nil
}

func savePostinganFile(fh *multipart.FileHeader) (UploadedPostinganFile, error) {
	if fh == nil || fh.Size == 0 {
		return UploadedPostinganFile{}, errors.New("file tidak ada")
	}
	if fh.Size > MaxPostinganFileSize {
		return UploadedPostinganFile{}, fmt.Errorf(
			"ukuran file %s terlalu besar (maks. 10 MB)", sanitizeFilename(fh.Filename))
	}

	src, err := fh.Open()
	if err != nil {
		return UploadedPostinganFile{}, fmt.Errorf("open upload: %w", err)
	}
	defer src.Close()

	mimeType, err := detectMIME(src)
	if err != nil {
		return UploadedPostinganFile{}, err
	}

	name := sanitizeFilename(fh.Filename)
	ext, ok := postinganFileExtension(mimeType, name)
	if !ok {
		return UploadedPostinganFile{}, fmt.Errorf("tipe file %s tidak didukung", name)
	}

	// Store under uploads/YYYY/MM/DD/<random>.<ext>.
	now := time.Now()
	datePath := now.Format("2006/01/02")
	diskDir := filepath.Join(uploadRoot, filepath.FromSlash(datePath))
	if err := os.MkdirAll(diskDir, 0o755); err != nil {
		return UploadedPostinganFile{}, fmt.Errorf("create upload dir: %w", err)
	}

	suffix, err := utils.RandomHex(16)
	if err != nil {
		return UploadedPostinganFile{}, fmt.Errorf("generate file name: %w", err)
	}
	filename := suffix + "." + ext
	absPath := filepath.Join(diskDir, filename)

	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return UploadedPostinganFile{}, fmt.Errorf("seek upload: %w", err)
	}

	// O_EXCL guarantees we never overwrite an existing file.
	dst, err := os.OpenFile(absPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return UploadedPostinganFile{}, fmt.Errorf("create file: %w", err)
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		os.Remove(absPath)
		return UploadedPostinganFile{}, fmt.Errorf("write file: %w", err)
	}
	if err := dst.Close(); err != nil {
		os.Remove(absPath)
		return UploadedPostinganFile{}, fmt.Errorf("close file: %w", err)
	}

	return UploadedPostinganFile{
		Nama:   name,
		Path:   path.Join(publicUploadPrefix, datePath, filename),
		Ukuran: fh.Size,
	}, nil
}

// detectMIME reads up to 1 MB from the start of the file. 1 MB is enough for
// mimetype to recognise OOXML documents as well as ordinary formats.
func detectMIME(src io.Reader) (string, error) {
	head := make([]byte, 1<<20)
	n, err := io.ReadFull(src, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read upload: %w", err)
	}

	mediaType, _, err := mime.ParseMediaType(mimetype.Detect(head[:n]).String())
	if err != nil {
		return "", fmt.Errorf("detect mime: %w", err)
	}
	return mediaType, nil
}

func postinganFileExtension(mediaType, originalName string) (string, bool) {
	allowed, ok := allowedPostinganTypes[mediaType]
	if !ok {
		return "", false
	}

	originalExt := strings.ToLower(strings.TrimPrefix(path.Ext(originalName), "."))
	for _, ext := range allowed {
		if originalExt == ext {
			return ext, true
		}
	}

	return allowed[0], true
}

func sanitizeFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return "file"
	}

	runes := []rune(name)
	if len(runes) > 255 {
		name = string(runes[:255])
	}
	return name
}

func removeUploadedFiles(files []UploadedPostinganFile) {
	for _, f := range files {
		rel := strings.TrimPrefix(f.Path, publicUploadPrefix+"/")
		_ = os.Remove(filepath.Join(uploadRoot, filepath.FromSlash(rel)))
	}
}
