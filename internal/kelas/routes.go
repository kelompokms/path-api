package kelas

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	kelas := e.Group("/kelas")

	kelas.GET("", h.List)
	kelas.POST("", h.Create)
	kelas.GET("/:kode_kelas", h.Get)
	kelas.POST("/:kode_kelas/gabung", h.Join)
	kelas.POST("/:kode_kelas/keluar", h.Leave)
}
