package main

import (
	"context"
	"log"
	"strings"
	_ "path-api/docs"
	"path-api/internal/admin"
	"path-api/internal/ai"
	"path-api/internal/auth"
	"path-api/internal/db"
	"path-api/internal/files"
	"path-api/internal/kelas"
	"path-api/internal/pengguna"
	"path-api/internal/postingan"
	"path-api/internal/utils"

	_ "github.com/joho/godotenv/autoload"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	echoSwagger "github.com/swaggo/echo-swagger/v2"
)

//	@title			PATH API Documentation
//	@version		1.0
//	@description	This is documentation for PATH API.

//	@license.name	Apache 2.0
//	@license.url	https://raw.githubusercontent.com/kelompokms/path-api/refs/heads/main/LICENSE

// @host		api.pathclass.app
// @BasePath	/
func main() {
	ctx := context.Background()
	q, pool, dbConn, err := db.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	defer dbConn.Close()
	defer pool.Close()

	e := echo.New()

	e.Validator = utils.NewValidator()

	e.Use(middleware.RequestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS("*"))
	// Avoid Echo v5 JSON EOF error when clients send Content-Type: application/json with an empty body.
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			if c.Request().ContentLength == 0 && strings.HasPrefix(c.Request().Header.Get(echo.HeaderContentType), echo.MIMEApplicationJSON) {
				c.Request().Header.Del(echo.HeaderContentType)
			}
			return next(c)
		}
	})

	e.GET("/docs/*", echoSwagger.WrapHandler)

	utils.CatchAll(e)

	authHandler := auth.New(q)
	authHandler.Routes(e)

	protected := e.Group("")
	protected.Use(echojwt.WithConfig(utils.JWTConfig))

	penggunaHandler := pengguna.New(q)
	penggunaHandler.Routes(protected)

	kelasHandler := kelas.New(q)
	kelasHandler.Routes(protected)

	postinganHandler := postingan.New(q)
	postinganHandler.Routes(protected)

	filesHandler := files.New(q)
	filesHandler.Routes(protected)

	adminHandler := admin.New(q)
	adminHandler.Routes(protected)

	aiHandler := ai.New(q)
	aiHandler.Routes(protected)

	e.Static("/uploads", files.UploadRoot)

	port := utils.GetPort()
	err = e.Start(port)
}
