package pengguna

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Echo) {
	pengguna := e.Group("/pengguna")

	pengguna.GET("", h.List)
	pengguna.GET("/:id", h.Get)
	pengguna.POST("/logout", h.Logout)
	pengguna.PUT("/:id", h.UpdateBiodata) 
	pengguna.PUT("/:id/reset-password", h.ResetPassword)
}
