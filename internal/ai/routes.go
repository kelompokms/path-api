package ai

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Group) {
	group := e.Group("/ai")

	group.POST("/chat", h.Chat)
	group.POST("/materi/explain", h.ExplainMateri)
	group.POST("/materi/generate", h.GenerateMateri)
	group.POST("/dokumen/ringkas", h.RingkasDokumen)
	group.POST("/koreksi", h.KoreksiTugas)
	group.POST("/similarity", h.CekKemiripan)
	group.POST("/indikasi", h.CekIndikasiAI)
	group.GET("/ops/usage", h.OpsUsage)
}
