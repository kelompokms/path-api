package utils

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
)

type JWTCustomClaims struct {
	UserID int32 `json:"user_id"`
	jwt.RegisteredClaims
}

var JWTConfig echojwt.Config = echojwt.Config{
	NewClaimsFunc: func(c *echo.Context) jwt.Claims {
		return &JWTCustomClaims{}
	},
	ErrorHandler: func(c *echo.Context, err error) error {
		return echo.NewHTTPError(http.StatusUnauthorized, "JWT tidak valid atau tidak ada")
	},
	SigningKey: []byte(GetSecret()),
}

func NewToken(UserID int32) (string, error) {
	claims := &JWTCustomClaims{
		UserID: UserID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour * 168)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	t, err := token.SignedString([]byte(GetSecret()))
	if err != nil {
		return "", err
	}

	return t, nil
}

func GetClaims(c *echo.Context) (*JWTCustomClaims, error) {
	token, err := echo.ContextGet[*jwt.Token](c, "user")
	if err != nil {
		return nil, err
	}
	return token.Claims.(*JWTCustomClaims), nil
}
