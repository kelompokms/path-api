package postingan

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	e.GET("/posts/tipe", h.ListTipe)

	postingan := e.Group("/kelas/:kode_kelas")

	postingan.GET("/posts", h.List)
	postingan.POST("/posts", h.Create)
	postingan.PATCH("/postingan/:postinganID", h.Edit)
}
