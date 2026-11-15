package auth

import (
	"net/http"
	"os"
	"path-api/internal/db"
	"time"
)

type Handler struct {
	db               *db.Queries
	oauthProviders   map[string]OAuthProviderConfig
	httpClient       *http.Client
	frontendURL      string
	allowedRedirects map[string]struct{}
}

func New(db *db.Queries) *Handler {
	frontendURL := os.Getenv("OAUTH_FRONTEND_REDIRECT_URL")
	return &Handler{
		db:               db,
		oauthProviders:   loadOAuthProvidersFromEnv(),
		httpClient:       &http.Client{Timeout: 10 * time.Second},
		frontendURL:      frontendURL,
		allowedRedirects: loadAllowedOAuthRedirects(frontendURL),
	}
}

func NewWithOAuth(db *db.Queries, providers map[string]OAuthProviderConfig, httpClient *http.Client, frontendURL string) *Handler {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	allowed := map[string]struct{}{
		defaultMobileOAuthRedirectURI: {},
	}
	if frontendURL != "" {
		allowed[frontendURL] = struct{}{}
	}
	return &Handler{
		db:               db,
		oauthProviders:   providers,
		httpClient:       httpClient,
		frontendURL:      frontendURL,
		allowedRedirects: allowed,
	}
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email" example:"johndoe@example.com"`
	Password string `json:"password" validate:"required,min=8" example:"secret123"`
}

type RegisterRequest struct {
	Nama     string `json:"username" validate:"required" example:"John Doe"`
	Email    string `json:"email" validate:"required,email" example:"johndoe@example.com"`
	Password string `json:"password" validate:"required,min=8" example:"secret123"`
}

type SuccessResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
}
