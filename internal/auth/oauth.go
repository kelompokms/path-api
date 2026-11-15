package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path-api/internal/db"
	"path-api/internal/utils"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
)

const (
	oauthStateCookieName          = "oauth_state"
	oauthStateCookiePath          = "/"
	oauthStateTTL                 = 10 * time.Minute
	defaultMobileOAuthRedirectURI = "pathmobile://app/oauth/callback"
	oauthSealedStatePrefix        = "v1."
	minOAuthClientStateLen        = 16
	maxOAuthClientStateLen        = 128
)

var errOAuthStateExpired = errors.New("state expired")

// OAuthProviderConfig holds OAuth2 endpoint and credential configuration for a provider.
type OAuthProviderConfig struct {
	Name         string
	ClientID     string
	ClientSecret string
	RedirectURL  string
	AuthURL      string
	TokenURL     string
	UserInfoURL  string
	Scopes       []string
}

type oauthUserInfo struct {
	ID            string
	Name          string
	Email         string
	EmailVerified bool
	AvatarURL     string
}

type oauthTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
}

type oauthStatePayload struct {
	Provider     string `json:"provider"`
	State        string `json:"state"`
	CodeVerifier string `json:"code_verifier"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
	ExpiresAt    int64  `json:"expires_at"`
}

type oauthQueryStatePayload struct {
	Provider     string `json:"p"`
	Nonce        string `json:"n"`
	CodeVerifier string `json:"v"`
	RedirectURI  string `json:"r,omitempty"`
	ClientNonce  string `json:"c,omitempty"`
	ExpiresAt    int64  `json:"e"`
}

func loadAllowedOAuthRedirects(frontendURL string) map[string]struct{} {
	allowed := make(map[string]struct{})
	if frontendURL != "" {
		allowed[frontendURL] = struct{}{}
	}

	mobileRedirect := strings.TrimSpace(os.Getenv("OAUTH_MOBILE_REDIRECT_URL"))
	if mobileRedirect == "" {
		mobileRedirect = defaultMobileOAuthRedirectURI
	}
	allowed[mobileRedirect] = struct{}{}

	for entry := range strings.SplitSeq(os.Getenv("OAUTH_ALLOWED_REDIRECT_URLS"), ",") {
		trimmed := strings.TrimSpace(entry)
		if trimmed != "" {
			allowed[trimmed] = struct{}{}
		}
	}
	return allowed
}

func loadOAuthProvidersFromEnv() map[string]OAuthProviderConfig {
	return map[string]OAuthProviderConfig{
		"google": {
			Name:         "google",
			ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
			ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     "https://oauth2.googleapis.com/token",
			UserInfoURL:  "https://openidconnect.googleapis.com/v1/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
		},
		"discord": {
			Name:         "discord",
			ClientID:     os.Getenv("DISCORD_CLIENT_ID"),
			ClientSecret: os.Getenv("DISCORD_CLIENT_SECRET"),
			RedirectURL:  os.Getenv("DISCORD_REDIRECT_URL"),
			AuthURL:      "https://discord.com/oauth2/authorize",
			TokenURL:     "https://discord.com/api/oauth2/token",
			UserInfoURL:  "https://discord.com/api/users/@me",
			Scopes:       []string{"identify", "email"},
		},
	}
}

func (p OAuthProviderConfig) isConfigured() bool {
	return p.ClientID != "" && p.ClientSecret != "" && p.RedirectURL != ""
}

// OAuthLogin godoc
//
//	@Summary		Inisiasi login OAuth2
//	@Description	Mengalihkan pengguna ke halaman persetujuan OAuth2 provider (google atau discord) dengan PKCE dan state cookie bertanda tangan
//	@Tags			Auth
//	@Produce		json
//	@Param			provider		path		string	true	"OAuth provider (google, discord)"
//	@Param			redirect_uri	query		string	false	"URL redirect tujuan yang diizinkan (mis. pathmobile://app/oauth/callback)"
//	@Param			client_state	query		string	false	"Nonce state anti-CSRF dari aplikasi mobile (16-128 karakter URL-safe)"
//	@Success		302				{string}	string	"Redirect ke halaman login OAuth provider"
//	@Failure		400,500,503		{object}	utils.HTTPResponse
//	@Router			/auth/oauth/{provider} [get]
func (h *Handler) OAuthLogin(c *echo.Context) error {
	providerName := strings.ToLower(c.Param("provider"))
	provider, ok := h.oauthProviders[providerName]
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "provider OAuth tidak didukung")
	}
	if !provider.isConfigured() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "provider OAuth belum dikonfigurasi")
	}

	redirectURI := strings.TrimSpace(c.QueryParam("redirect_uri"))
	if redirectURI != "" {
		if _, allowed := h.allowedRedirects[redirectURI]; !allowed {
			return echo.NewHTTPError(http.StatusBadRequest, "redirect_uri OAuth tidak diizinkan")
		}
	} else {
		redirectURI = h.frontendURL
	}

	clientState := strings.TrimSpace(c.QueryParam("client_state"))
	if clientState != "" && !isValidOAuthClientState(clientState) {
		return echo.NewHTTPError(http.StatusBadRequest, "client_state OAuth tidak valid")
	}

	stateNonce, err := randomBase64URL(16)
	if err != nil {
		log.Println("Error generating OAuth state:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memproses login OAuth")
	}

	codeVerifier, err := randomBase64URL(32)
	if err != nil {
		log.Println("Error generating PKCE verifier:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memproses login OAuth")
	}

	expiresAt := time.Now().Add(oauthStateTTL).Unix()
	secret := utils.GetSecret()

	signedCookieVal, err := encodeSignedState(oauthStatePayload{
		Provider:     providerName,
		State:        stateNonce,
		CodeVerifier: codeVerifier,
		RedirectURI:  redirectURI,
		ExpiresAt:    expiresAt,
	}, secret)
	if err != nil {
		log.Println("Error signing OAuth state:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memproses login OAuth")
	}

	sealedState, err := sealOAuthQueryState(oauthQueryStatePayload{
		Provider:     providerName,
		Nonce:        stateNonce,
		CodeVerifier: codeVerifier,
		RedirectURI:  redirectURI,
		ClientNonce:  clientState,
		ExpiresAt:    expiresAt,
	}, secret)
	if err != nil {
		log.Println("Error sealing OAuth state:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memproses login OAuth")
	}

	setNoCacheHeaders(c)
	c.SetCookie(&http.Cookie{
		Name:     oauthStateCookieName,
		Value:    signedCookieVal,
		Path:     oauthStateCookiePath,
		MaxAge:   int(oauthStateTTL.Seconds()),
		HttpOnly: true,
		Secure:   isHTTPSRequest(c.Request()),
		SameSite: http.SameSiteLaxMode,
	})

	authURL, err := buildAuthorizationURL(provider, sealedState, pkceChallengeS256(codeVerifier))
	if err != nil {
		log.Println("Error building OAuth auth URL:", err)
		return echo.NewHTTPError(http.StatusInternalServerError, "gagal memproses login OAuth")
	}

	return c.Redirect(http.StatusFound, authURL)
}

// OAuthCallback godoc
//
//	@Summary		Callback login OAuth2
//	@Description	Menukar authorization code dari OAuth2 provider dengan data profil terverifikasi, menautkan atau membuat akun pengguna, lalu mengembalikan JWT token atau redirect ke target yang diizinkan
//	@Tags			Auth
//	@Produce		json
//	@Param			provider	path		string	true	"OAuth provider (google, discord)"
//	@Param			code		query		string	true	"Authorization code dari OAuth provider"
//	@Param			state		query		string	true	"State token anti-CSRF"
//	@Success		200			{object}	SuccessResponse
//	@Success		302			{string}	string	"Redirect ke frontend/mobile dengan token"
//	@Failure		400,500,503	{object}	utils.HTTPResponse
//	@Router			/auth/oauth/{provider}/callback [get]
func (h *Handler) OAuthCallback(c *echo.Context) error {
	providerName := strings.ToLower(c.Param("provider"))
	provider, ok := h.oauthProviders[providerName]
	if !ok {
		return echo.NewHTTPError(http.StatusBadRequest, "provider OAuth tidak didukung")
	}
	if !provider.isConfigured() {
		return echo.NewHTTPError(http.StatusServiceUnavailable, "provider OAuth belum dikonfigurasi")
	}

	setNoCacheHeaders(c)

	stateQuery := c.QueryParam("state")
	stateCookie, cookieErr := c.Cookie(oauthStateCookieName)
	if cookieErr == nil {
		clearOAuthStateCookie(c)
	}

	secret := utils.GetSecret()
	var sealedPayload oauthQueryStatePayload
	sealedErr := errors.New("unsealed state")
	if strings.HasPrefix(stateQuery, oauthSealedStatePrefix) {
		sealedPayload, sealedErr = unsealOAuthQueryState(stateQuery, secret)
	}

	var statePayload oauthStatePayload
	var clientNonce string
	var stateValid bool

	if cookieErr == nil && stateQuery != "" {
		if parsed, err := verifySignedState(stateCookie.Value, secret); err == nil && parsed.Provider == providerName {
			if sealedErr == nil && sealedPayload.Provider == providerName && hmac.Equal([]byte(parsed.State), []byte(sealedPayload.Nonce)) {
				statePayload = parsed
				clientNonce = sealedPayload.ClientNonce
				stateValid = true
			} else if !strings.HasPrefix(stateQuery, oauthSealedStatePrefix) && hmac.Equal([]byte(parsed.State), []byte(stateQuery)) {
				statePayload = parsed
				stateValid = true
			}
		}
	} else if cookieErr != nil && sealedErr == nil && sealedPayload.Provider == providerName && sealedPayload.ClientNonce != "" && sealedPayload.RedirectURI != "" {
		if _, allowed := h.allowedRedirects[sealedPayload.RedirectURI]; allowed {
			log.Printf("OAuth callback missing %s cookie; using client_state fallback (provider=%s host=%s https=%t)",
				oauthStateCookieName, providerName, c.Request().Host, isHTTPSRequest(c.Request()))
			statePayload = oauthStatePayload{
				Provider:     sealedPayload.Provider,
				State:        sealedPayload.Nonce,
				CodeVerifier: sealedPayload.CodeVerifier,
				RedirectURI:  sealedPayload.RedirectURI,
				ExpiresAt:    sealedPayload.ExpiresAt,
			}
			clientNonce = sealedPayload.ClientNonce
			stateValid = true
		}
	}

	if oauthErr := c.QueryParam("error"); oauthErr != "" {
		if stateValid && statePayload.RedirectURI != "" {
			return c.Redirect(http.StatusFound, buildOAuthRedirectURL(statePayload.RedirectURI, "error", oauthErr, clientNonce))
		}
		return echo.NewHTTPError(http.StatusBadRequest, "otorisasi OAuth dibatalkan atau ditolak")
	}

	code := c.QueryParam("code")
	if code == "" || stateQuery == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "parameter code dan state wajib diisi")
	}
	if cookieErr != nil && !stateValid {
		if errors.Is(sealedErr, errOAuthStateExpired) {
			return echo.NewHTTPError(http.StatusBadRequest, "state OAuth tidak valid atau kedaluwarsa")
		}
		log.Printf("OAuth callback missing %s cookie (provider=%s host=%s https=%t has_state=%t err=%v)",
			oauthStateCookieName, providerName, c.Request().Host, isHTTPSRequest(c.Request()), stateQuery != "", cookieErr)
		return echo.NewHTTPError(http.StatusBadRequest, "state OAuth tidak ditemukan")
	}
	if !stateValid {
		log.Printf("OAuth callback invalid state (provider=%s host=%s has_state=%t)",
			providerName, c.Request().Host, stateQuery != "")
		return echo.NewHTTPError(http.StatusBadRequest, "state OAuth tidak valid atau kedaluwarsa")
	}

	ctx := c.Request().Context()
	accessToken, err := h.exchangeCode(ctx, provider, code, statePayload.CodeVerifier)
	if err != nil {
		log.Println("Error exchanging OAuth code:", err)
		return oauthCallbackError(c, statePayload.RedirectURI, clientNonce, "token_exchange_failed", "gagal menukar authorization code OAuth")
	}

	userInfo, err := h.fetchUserInfo(ctx, provider, accessToken)
	if err != nil {
		log.Println("Error fetching OAuth user info:", err)
		return oauthCallbackError(c, statePayload.RedirectURI, clientNonce, "userinfo_failed", "gagal mengambil profil pengguna OAuth")
	}

	if userInfo.ID == "" || userInfo.Email == "" || !userInfo.EmailVerified {
		return oauthCallbackError(c, statePayload.RedirectURI, clientNonce, "email_unverified", "email OAuth belum terverifikasi atau tidak tersedia")
	}

	userID, isAdmin, err := h.resolveOAuthUser(ctx, providerName, userInfo)
	if err != nil {
		log.Println("Error resolving OAuth user:", err)
		return oauthCallbackError(c, statePayload.RedirectURI, clientNonce, "account_error", "gagal memproses akun OAuth")
	}

	token, err := utils.NewToken(userID, isAdmin)
	if err != nil {
		log.Println("Error creating JWT token:", err)
		return oauthCallbackError(c, statePayload.RedirectURI, clientNonce, "token_creation_failed", "gagal membuat token autentikasi")
	}

	if statePayload.RedirectURI != "" {
		return c.Redirect(http.StatusFound, buildOAuthRedirectURL(statePayload.RedirectURI, "token", token, clientNonce))
	}

	return c.JSON(http.StatusOK, SuccessResponse{
		Token: token,
	})
}

func oauthCallbackError(c *echo.Context, redirectURI, clientNonce, errCode, message string) error {
	if redirectURI != "" {
		return c.Redirect(http.StatusFound, buildOAuthRedirectURL(redirectURI, "error", errCode, clientNonce))
	}
	return echo.NewHTTPError(http.StatusBadRequest, message)
}

func buildOAuthRedirectURL(baseURL, key, value, clientNonce string) string {
	params := url.Values{key: {value}}
	if clientNonce != "" {
		params.Set("state", clientNonce)
	}
	u, err := url.Parse(baseURL)
	if err != nil {
		return baseURL + "#" + params.Encode()
	}
	if u.Scheme == "http" || u.Scheme == "https" {
		u.Fragment = params.Encode()
		return u.String()
	}
	q := u.Query()
	q.Set(key, value)
	if clientNonce != "" {
		q.Set("state", clientNonce)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (h *Handler) resolveOAuthUser(ctx context.Context, provider string, userInfo oauthUserInfo) (int32, bool, error) {
	linkedUser, err := h.db.GetPenggunaByOAuthAccount(ctx, db.GetPenggunaByOAuthAccountParams{
		Provider:       provider,
		ProviderUserID: userInfo.ID,
	})
	if err == nil {
		return linkedUser.ID, linkedUser.Admin, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}

	existingUser, err := h.db.GetPenggunaByEmail(ctx, userInfo.Email)
	if err == nil {
		if err := h.db.LinkOAuthAccount(ctx, db.LinkOAuthAccountParams{
			PenggunaID:     existingUser.ID,
			Provider:       provider,
			ProviderUserID: userInfo.ID,
			Email:          userInfo.Email,
		}); err != nil {
			return 0, false, err
		}
		return existingUser.ID, existingUser.Admin, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, false, err
	}

	displayName := strings.TrimSpace(userInfo.Name)
	if displayName == "" {
		displayName = strings.Split(userInfo.Email, "@")[0]
	}

	var avatar pgtype.Text
	if userInfo.AvatarURL != "" {
		avatar = pgtype.Text{String: userInfo.AvatarURL, Valid: true}
	}

	createdUser, err := h.db.CreateOAuthPengguna(ctx, db.CreateOAuthPenggunaParams{
		Nama:           displayName,
		Email:          userInfo.Email,
		ProfilePicture: avatar,
	})
	if err != nil {
		return 0, false, err
	}

	if err := h.db.LinkOAuthAccount(ctx, db.LinkOAuthAccountParams{
		PenggunaID:     createdUser.ID,
		Provider:       provider,
		ProviderUserID: userInfo.ID,
		Email:          userInfo.Email,
	}); err != nil {
		return 0, false, err
	}

	return createdUser.ID, createdUser.Admin, nil
}

func (h *Handler) exchangeCode(ctx context.Context, provider OAuthProviderConfig, code, codeVerifier string) (string, error) {
	form := url.Values{}
	form.Set("client_id", provider.ClientID)
	form.Set("client_secret", provider.ClientSecret)
	form.Set("code", code)
	form.Set("redirect_uri", provider.RedirectURL)
	form.Set("grant_type", "authorization_code")
	form.Set("code_verifier", codeVerifier)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp oauthTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", err
	}
	if tokenResp.AccessToken == "" {
		return "", errors.New("empty access_token in token response")
	}

	return tokenResp.AccessToken, nil
}

func (h *Handler) fetchUserInfo(ctx context.Context, provider OAuthProviderConfig, accessToken string) (oauthUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, provider.UserInfoURL, nil)
	if err != nil {
		return oauthUserInfo{}, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		return oauthUserInfo{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return oauthUserInfo{}, err
	}
	if resp.StatusCode != http.StatusOK {
		return oauthUserInfo{}, fmt.Errorf("userinfo endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	switch provider.Name {
	case "google":
		var payload struct {
			Sub           string `json:"sub"`
			Name          string `json:"name"`
			Email         string `json:"email"`
			EmailVerified bool   `json:"email_verified"`
			Picture       string `json:"picture"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return oauthUserInfo{}, err
		}
		return oauthUserInfo{
			ID:            payload.Sub,
			Name:          payload.Name,
			Email:         strings.ToLower(strings.TrimSpace(payload.Email)),
			EmailVerified: payload.EmailVerified,
			AvatarURL:     payload.Picture,
		}, nil

	case "discord":
		var payload struct {
			ID         string `json:"id"`
			Username   string `json:"username"`
			GlobalName string `json:"global_name"`
			Avatar     string `json:"avatar"`
			Email      string `json:"email"`
			Verified   bool   `json:"verified"`
		}
		if err := json.Unmarshal(body, &payload); err != nil {
			return oauthUserInfo{}, err
		}
		name := payload.GlobalName
		if name == "" {
			name = payload.Username
		}
		avatarURL := ""
		if payload.Avatar != "" && payload.ID != "" {
			avatarURL = fmt.Sprintf("https://cdn.discordapp.com/avatars/%s/%s.png", payload.ID, payload.Avatar)
		}
		return oauthUserInfo{
			ID:            payload.ID,
			Name:          name,
			Email:         strings.ToLower(strings.TrimSpace(payload.Email)),
			EmailVerified: payload.Verified,
			AvatarURL:     avatarURL,
		}, nil

	default:
		return oauthUserInfo{}, fmt.Errorf("unsupported provider: %s", provider.Name)
	}
}

func buildAuthorizationURL(provider OAuthProviderConfig, state, codeChallenge string) (string, error) {
	u, err := url.Parse(provider.AuthURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("client_id", provider.ClientID)
	q.Set("redirect_uri", provider.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", strings.Join(provider.Scopes, " "))
	q.Set("state", state)
	q.Set("code_challenge", codeChallenge)
	q.Set("code_challenge_method", "S256")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func randomBase64URL(numBytes int) (string, error) {
	buf := make([]byte, numBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func pkceChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func encodeSignedState(payload oauthStatePayload, secret string) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw)
	sig := signHMAC(encoded, secret)
	return encoded + "." + sig, nil
}

func verifySignedState(token, secret string) (oauthStatePayload, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return oauthStatePayload{}, errors.New("invalid state format")
	}

	expectedSig := signHMAC(parts[0], secret)
	if !hmac.Equal([]byte(parts[1]), []byte(expectedSig)) {
		return oauthStatePayload{}, errors.New("invalid state signature")
	}

	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return oauthStatePayload{}, err
	}

	var payload oauthStatePayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return oauthStatePayload{}, err
	}

	if time.Now().Unix() > payload.ExpiresAt {
		return oauthStatePayload{}, errOAuthStateExpired
	}

	return payload, nil
}

func isValidOAuthClientState(v string) bool {
	if len(v) < minOAuthClientStateLen || len(v) > maxOAuthClientStateLen {
		return false
	}
	for i := 0; i < len(v); i++ {
		c := v[i]
		isUpper := c >= 'A' && c <= 'Z'
		isLower := c >= 'a' && c <= 'z'
		isDigit := c >= '0' && c <= '9'
		isAllowedPunct := c == '-' || c == '_' || c == '.' || c == '~'
		if !isUpper && !isLower && !isDigit && !isAllowedPunct {
			return false
		}
	}
	return true
}

func deriveOAuthAEADKey(secret string) []byte {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("path-api:oauth-state-aead:v1"))
	return mac.Sum(nil)
}

func sealOAuthQueryState(payload oauthQueryStatePayload, secret string) (string, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(deriveOAuthAEADKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, raw, []byte(oauthSealedStatePrefix))
	return oauthSealedStatePrefix + base64.RawURLEncoding.EncodeToString(sealed), nil
}

func unsealOAuthQueryState(token, secret string) (oauthQueryStatePayload, error) {
	encoded, ok := strings.CutPrefix(token, oauthSealedStatePrefix)
	if !ok {
		return oauthQueryStatePayload{}, errors.New("invalid state prefix")
	}
	sealed, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return oauthQueryStatePayload{}, err
	}
	block, err := aes.NewCipher(deriveOAuthAEADKey(secret))
	if err != nil {
		return oauthQueryStatePayload{}, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return oauthQueryStatePayload{}, err
	}
	nonceSize := gcm.NonceSize()
	if len(sealed) <= nonceSize {
		return oauthQueryStatePayload{}, errors.New("invalid sealed state length")
	}
	plaintext, err := gcm.Open(nil, sealed[:nonceSize], sealed[nonceSize:], []byte(oauthSealedStatePrefix))
	if err != nil {
		return oauthQueryStatePayload{}, err
	}
	var payload oauthQueryStatePayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return oauthQueryStatePayload{}, err
	}
	if time.Now().Unix() > payload.ExpiresAt {
		return oauthQueryStatePayload{}, errOAuthStateExpired
	}
	return payload, nil
}

func signHMAC(data, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(data))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func setNoCacheHeaders(c *echo.Context) {
	h := c.Response().Header()
	h.Set("Cache-Control", "no-store, no-cache, must-revalidate")
	h.Set("Pragma", "no-cache")
}

func clearOAuthStateCookie(c *echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     oauthStateCookieName,
		Value:    "",
		Path:     oauthStateCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   isHTTPSRequest(c.Request()),
		SameSite: http.SameSiteLaxMode,
	})
}

func isHTTPSRequest(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}
