package postingan

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	postingan := e.Group("/kelas/:kelasID")

	postingan.GET("", h.ListPostingan)
	postingan.POST("", h.CreatePostingan)
	postingan.PATCH("/postingan/:postinganID", h.EditPostingan)
}


