package postingan

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	e.GET("/posts/tipe", h.ListTipe)

	postingan := e.Group("/kelas/:kode_kelas")

	postingan.GET("/posts", h.List)
	postingan.POST("/posts", h.Create)
	postingan.PATCH("/postingan/:postinganID", h.Edit)
	postingan.DELETE("/postingan/:postinganID", h.Delete)
	postingan.GET("/postingan/:postinganID/pengumpulan/saya", h.GetPengumpulan)
	postingan.GET("/postingan/:postinganID/pengumpulan", h.ListPengumpulan)
	postingan.POST("/postingan/:postinganID/pengumpulan", h.SubmitPengumpulan)
	postingan.PATCH("/postingan/:postinganID/pengumpulan/nilai", h.NilaiPengumpulan)
	postingan.DELETE("/postingan/:postinganID/pengumpulan", h.TarikPengumpulan)
}
