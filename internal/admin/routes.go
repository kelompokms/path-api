package admin

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	e.GET("/admin", h.ListAdmin)
	e.GET("/admin/pengguna", h.ListPengguna)
	e.POST("/admin/pengguna", h.CreatePengguna)
	e.DELETE("/admin/pengguna/:pengguna_id", h.DeletePengguna)
	e.PATCH("/admin/pengguna/:pengguna_id/role", h.UpdateRolePengguna)
	e.GET("/admin/kelas", h.ListKelas)
	e.POST("/admin/kelas", h.CreateKelas)
	e.POST("/admin/kelas/csv", h.UploadKelasCSV)
	e.PATCH("/admin/kelas/:kode_kelas/pengampu", h.UpdatePengampuKelas)
	e.POST("/admin/kalender/agenda", h.CreateAgendaKalender)
	e.GET("/guru", h.ListGuru)
	e.PATCH("/:kode_kelas/guru", h.UpdateGuru)
	e.POST("/admin/kalender", h.UploadKalenderAkademik)
	e.GET("/kalender", h.ListKalenderAkademik)
}
