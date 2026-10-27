package files

import (
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path-api/internal/utils"
	"path/filepath"
	"time"
)

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
	mime := http.DetectContentType(sniff[:n])
	ext, ok := allowedTypes[mime]
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
	relDir := filepath.Join("uploads", "pfp",
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
