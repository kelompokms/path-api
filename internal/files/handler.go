package files

import (
	"path-api/internal/db"
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
