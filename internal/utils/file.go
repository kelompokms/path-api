package utils

import (
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
)

func WriteFile(file *multipart.FileHeader) error {
	dirPath := filepath.Dir("uploads/")
	err := os.MkdirAll(dirPath, 0755)
	if err != nil {
		return err
	}

	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filepath.Join(dirPath, file.Filename))
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return err
	}

	return nil
}
