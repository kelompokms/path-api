package auth

import "github.com/labstack/echo/v5"

func (h *Handler) Routes(e *echo.Echo) {
	e.POST("/register", h.Register)
	e.POST("/login", h.Login)
	e.GET("/auth/oauth/:provider", h.OAuthLogin)
	e.GET("/auth/oauth/:provider/callback", h.OAuthCallback)
}
