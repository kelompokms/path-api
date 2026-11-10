package admin

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	e.GET("/admin", h.ListAdmin)
	e.GET("/guru", h.ListGuru)
	e.PATCH("/:kode_kelas/guru", h.UpdateGuru)
	e.POST("/admin/kalender", h.UploadKalenderAkademik) 
	e.GET("/kalender", h.ListKalenderAkademik)
}
