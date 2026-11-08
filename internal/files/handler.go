package files

import (
	"mime/multipart"
	"path-api/internal/db"
)

const (
	// MaxPostinganFiles is the maximum number of attachments allowed per postingan.
	MaxPostinganFiles = 5
	// MaxPostinganFileSize is the maximum size of a single postingan attachment (10 MB).
	MaxPostinganFileSize = 10 << 20
	// maxPostinganUploadBytes is the request body limit: all files plus multipart overhead.
	maxPostinganUploadBytes = MaxPostinganFiles*MaxPostinganFileSize + (1 << 20)
)

type Handler struct {
	db *db.Queries
}

func New(db *db.Queries) *Handler {
	return &Handler{db}
}

var allowedTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
}

// allowedPostinganTypes maps a detected MIME type to the extensions we are
// willing to store. The first extension is the default; the others are kept
// when the uploaded file already uses them (e.g. CSV is detected as text/plain
// and OOXML documents may be detected as a plain zip).
var allowedPostinganTypes = map[string][]string{
	// Images
	"image/jpeg": {"jpg", "jpeg"},
	"image/png":  {"png"},
	"image/gif":  {"gif"},
	"image/webp": {"webp"},
	"image/bmp":  {"bmp"},
	// Documents
	"application/pdf":    {"pdf"},
	"text/plain":         {"txt", "csv"},
	"application/msword": {"doc"},
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": {"docx"},
	"application/vnd.ms-excel": {"xls"},
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":         {"xlsx"},
	"application/vnd.ms-powerpoint":                                             {"ppt"},
	"application/vnd.openxmlformats-officedocument.presentationml.presentation": {"pptx"},
	// Archives
	"application/zip":             {"zip", "docx", "xlsx", "pptx"},
	"application/vnd.rar":         {"rar"},
	"application/x-7z-compressed": {"7z"},
	"application/gzip":            {"gz"},
	// Media
	"video/mp4":  {"mp4"},
	"audio/mpeg": {"mp3"},
}

// UploadPostinganFilesRequest is the multipart request for postingan attachments.
// The frontend sends the files under the "files" field before creating a postingan.
type UploadPostinganFilesRequest struct {
	Files []*multipart.FileHeader `form:"files" validate:"required"`
}

// UploadedPostinganFile describes a file that has been stored on disk.
type UploadedPostinganFile struct {
	Nama   string `json:"nama" example:"tugas.pdf"`
	Path   string `json:"path" example:"/uploads/2026/11/08/9f8e7d6c5b4a39281706f5e4d3c2b1a0.pdf"`
	Ukuran int64  `json:"ukuran" example:"102400"`
}

type UploadPostinganFilesResponse struct {
	Files []UploadedPostinganFile `json:"files"`
}
