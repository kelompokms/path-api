package files

import (
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func (h *Handler) Routes(e *echo.Group) {
	e.Static("/uploads", uploadRoot)
	e.POST("/files/postingan", h.UploadPostinganFiles, middleware.BodyLimit(maxPostinganUploadBytes))
}
