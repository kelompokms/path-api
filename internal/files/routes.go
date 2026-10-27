package files

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	e.Static("/uploads", "uploads")
}
