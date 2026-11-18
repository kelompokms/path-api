package utils

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v5"
	"github.com/labstack/echo/v5"
)

type JWTCustomClaims struct {
	UserID int32  `json:"user_id"`
	Admin  bool   `json:"admin"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// IsAdmin returns true when the claims grant universal admin access via Role or legacy Admin flag.
func (c *JWTCustomClaims) IsAdmin() bool {
	return strings.EqualFold(c.Role, "admin") || c.Admin
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

func NewToken(userID int32, admin bool) (string, error) {
	role := "murid"
	if admin {
		role = "admin"
	}
	return NewTokenWithRole(userID, role)
}

func NewTokenWithRole(userID int32, role string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(role))
	if normalized == "" {
		normalized = "murid"
	}
	claims := &JWTCustomClaims{
		UserID: userID,
		Admin:  normalized == "admin",
		Role:   normalized,
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
