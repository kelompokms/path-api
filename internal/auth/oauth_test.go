package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"path-api/internal/db"
	"path-api/internal/utils"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/labstack/echo/v5"
	"github.com/pashagolub/pgxmock/v6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMockOAuthServer(t *testing.T, expectedVerifier *string, userPayload any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			require.NoError(t, r.ParseForm())
			assert.Equal(t, "authorization_code", r.Form.Get("grant_type"))
			assert.Equal(t, "auth-code-123", r.Form.Get("code"))
			if expectedVerifier != nil && *expectedVerifier != "" {
				assert.Equal(t, *expectedVerifier, r.Form.Get("code_verifier"))
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"access_token": "provider-access-token",
				"token_type":   "Bearer",
			})
		case "/userinfo":
			assert.Equal(t, "Bearer provider-access-token", r.Header.Get("Authorization"))
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(userPayload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func startOAuthFlow(t *testing.T, e *echo.Echo, provider string, extraQuery ...string) (*http.Cookie, string) {
	t.Helper()
	target := "/auth/oauth/" + provider
	if len(extraQuery) > 0 && extraQuery[0] != "" {
		target += "?" + extraQuery[0]
	}
	req := httptest.NewRequest(http.MethodGet, target, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	assert.Contains(t, rec.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, "no-cache", rec.Header().Get("Pragma"))
	location := rec.Header().Get("Location")
	require.NotEmpty(t, location)

	parsed, err := url.Parse(location)
	require.NoError(t, err)
	state := parsed.Query().Get("state")
	require.NotEmpty(t, state)
	assert.Equal(t, "S256", parsed.Query().Get("code_challenge_method"))
	assert.NotEmpty(t, parsed.Query().Get("code_challenge"))

	cookies := rec.Result().Cookies()
	var stateCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == oauthStateCookieName {
			stateCookie = c
			break
		}
	}
	require.NotNil(t, stateCookie)
	assert.Equal(t, "/", stateCookie.Path)
	assert.True(t, stateCookie.HttpOnly)

	return stateCookie, state
}

func TestOAuthLoginValidation(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	h := NewWithOAuth(db.New(mockPool), map[string]OAuthProviderConfig{
		"google": {
			Name:    "google",
			AuthURL: "https://accounts.google.com/o/oauth2/v2/auth",
		},
	}, nil, "")

	e := echo.New()
	h.Routes(e)

	t.Run("unsupported provider returns 400", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/oauth/unknown", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unconfigured provider returns 503", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})
}

func TestOAuthCallbackGoogleNewUser(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	var expectedVerifier string
	srv := setupMockOAuthServer(t, &expectedVerifier, map[string]any{
		"sub":            "google-sub-1",
		"name":           "Jane Doe",
		"email":          "jane@example.com",
		"email_verified": true,
		"picture":        "https://example.com/avatar.png",
	})
	defer srv.Close()

	h := NewWithOAuth(db.New(mockPool), map[string]OAuthProviderConfig{
		"google": {
			Name:         "google",
			ClientID:     "google-client-id",
			ClientSecret: "google-client-secret",
			RedirectURL:  "http://localhost:3000/auth/oauth/google/callback",
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     srv.URL + "/token",
			UserInfoURL:  srv.URL + "/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
		},
	}, srv.Client(), "")

	e := echo.New()
	h.Routes(e)

	stateCookie, state := startOAuthFlow(t, e, "google")
	payload, err := verifySignedState(stateCookie.Value, utils.GetSecret())
	require.NoError(t, err)
	expectedVerifier = payload.CodeVerifier

	mockPool.ExpectQuery("SELECT p.id, rp.nama").
		WithArgs("google", "google-sub-1").
		WillReturnError(pgx.ErrNoRows)

	mockPool.ExpectQuery("SELECT p.id, p.nama, p.email, p.profile_picture, rp.nama").
		WithArgs("jane@example.com").
		WillReturnError(pgx.ErrNoRows)

	mockPool.ExpectQuery("INSERT INTO pengguna").
		WithArgs("Jane Doe", "jane@example.com", pgtype.Text{String: "https://example.com/avatar.png", Valid: true}).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(int32(10)))

	mockPool.ExpectExec("INSERT INTO oauth_accounts").
		WithArgs(int32(10), "google", "google-sub-1", "jane@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state="+url.QueryEscape(state), nil)
	cbReq.AddCookie(stateCookie)
	cbRec := httptest.NewRecorder()
	e.ServeHTTP(cbRec, cbReq)

	require.Equal(t, http.StatusOK, cbRec.Code)
	assert.Contains(t, cbRec.Header().Get("Cache-Control"), "no-store")
	assert.Equal(t, "no-cache", cbRec.Header().Get("Pragma"))
	var clearedCookie *http.Cookie
	for _, c := range cbRec.Result().Cookies() {
		if c.Name == oauthStateCookieName {
			clearedCookie = c
			break
		}
	}
	require.NotNil(t, clearedCookie)
	assert.Equal(t, "/", clearedCookie.Path)
	assert.Less(t, clearedCookie.MaxAge, 0)
	var resp SuccessResponse
	require.NoError(t, json.Unmarshal(cbRec.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Token)

	parsedToken, err := jwt.ParseWithClaims(resp.Token, &utils.JWTCustomClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(utils.GetSecret()), nil
	})
	require.NoError(t, err)
	claims := parsedToken.Claims.(*utils.JWTCustomClaims)
	assert.Equal(t, int32(10), claims.UserID)
	assert.False(t, claims.Admin)
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestOAuthCallbackDiscordLinkExistingEmailAndFrontendRedirect(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	srv := setupMockOAuthServer(t, nil, map[string]any{
		"id":          "discord-id-99",
		"username":    "nelly",
		"global_name": "Nelly Discord",
		"avatar":      "abc123",
		"email":       "existing@example.com",
		"verified":    true,
	})
	defer srv.Close()

	h := NewWithOAuth(db.New(mockPool), map[string]OAuthProviderConfig{
		"discord": {
			Name:         "discord",
			ClientID:     "discord-client-id",
			ClientSecret: "discord-client-secret",
			RedirectURL:  "http://localhost:3000/auth/oauth/discord/callback",
			AuthURL:      "https://discord.com/oauth2/authorize",
			TokenURL:     srv.URL + "/token",
			UserInfoURL:  srv.URL + "/userinfo",
			Scopes:       []string{"identify", "email"},
		},
	}, srv.Client(), "https://app.patclass.app/oauth/done")

	e := echo.New()
	h.Routes(e)

	stateCookie, state := startOAuthFlow(t, e, "discord")

	mockPool.ExpectQuery("SELECT p.id, rp.nama").
		WithArgs("discord", "discord-id-99").
		WillReturnError(pgx.ErrNoRows)

	mockPool.ExpectQuery("SELECT p.id, p.nama, p.email, p.profile_picture, rp.nama").
		WithArgs("existing@example.com").
		WillReturnRows(pgxmock.NewRows([]string{"id", "nama", "email", "profile_picture", "role", "aktif"}).
			AddRow(int32(42), "Existing User", "existing@example.com", pgtype.Text{Valid: false}, "admin", true))

	mockPool.ExpectExec("INSERT INTO oauth_accounts").
		WithArgs(int32(42), "discord", "discord-id-99", "existing@example.com").
		WillReturnResult(pgxmock.NewResult("INSERT", 1))

	cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/discord/callback?code=auth-code-123&state="+url.QueryEscape(state), nil)
	cbReq.AddCookie(stateCookie)
	cbRec := httptest.NewRecorder()
	e.ServeHTTP(cbRec, cbReq)

	require.Equal(t, http.StatusFound, cbRec.Code)
	location := cbRec.Header().Get("Location")
	assert.True(t, strings.HasPrefix(location, "https://app.patclass.app/oauth/done#token="))
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestOAuthCallbackReturningLinkedUser(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	srv := setupMockOAuthServer(t, nil, map[string]any{
		"sub":            "google-sub-existing",
		"name":           "Returning User",
		"email":          "returning@example.com",
		"email_verified": true,
	})
	defer srv.Close()

	h := NewWithOAuth(db.New(mockPool), map[string]OAuthProviderConfig{
		"google": {
			Name:         "google",
			ClientID:     "cid",
			ClientSecret: "csec",
			RedirectURL:  "http://localhost:3000/auth/oauth/google/callback",
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     srv.URL + "/token",
			UserInfoURL:  srv.URL + "/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
		},
	}, srv.Client(), "")

	e := echo.New()
	h.Routes(e)

	stateCookie, state := startOAuthFlow(t, e, "google")

	mockPool.ExpectQuery("SELECT p.id, rp.nama").
		WithArgs("google", "google-sub-existing").
		WillReturnRows(pgxmock.NewRows([]string{"id", "role", "aktif"}).AddRow(int32(7), "murid", true))

	cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state="+url.QueryEscape(state), nil)
	cbReq.AddCookie(stateCookie)
	cbRec := httptest.NewRecorder()
	e.ServeHTTP(cbRec, cbReq)

	require.Equal(t, http.StatusOK, cbRec.Code)
	assert.Contains(t, cbRec.Body.String(), "token")
	assert.NoError(t, mockPool.ExpectationsWereMet())
}

func TestOAuthCallbackRejectsInvalidStateAndUnverifiedEmail(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	srv := setupMockOAuthServer(t, nil, map[string]any{
		"sub":            "google-sub-unverified",
		"name":           "Unverified User",
		"email":          "unverified@example.com",
		"email_verified": false,
	})
	defer srv.Close()

	h := NewWithOAuth(db.New(mockPool), map[string]OAuthProviderConfig{
		"google": {
			Name:         "google",
			ClientID:     "cid",
			ClientSecret: "csec",
			RedirectURL:  "http://localhost:3000/auth/oauth/google/callback",
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     srv.URL + "/token",
			UserInfoURL:  srv.URL + "/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
		},
	}, srv.Client(), "")

	e := echo.New()
	h.Routes(e)

	t.Run("missing state cookie", func(t *testing.T) {
		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state=abc", nil)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)
		assert.Equal(t, http.StatusBadRequest, cbRec.Code)
		assert.Contains(t, cbRec.Header().Get("Cache-Control"), "no-store")
		assert.Empty(t, cbRec.Result().Cookies())
	})

	t.Run("mismatched state", func(t *testing.T) {
		stateCookie, _ := startOAuthFlow(t, e, "google")
		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state=wrong-state", nil)
		cbReq.AddCookie(stateCookie)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)
		assert.Equal(t, http.StatusBadRequest, cbRec.Code)
		assert.Contains(t, cbRec.Header().Get("Cache-Control"), "no-store")
		cookies := cbRec.Result().Cookies()
		require.Len(t, cookies, 1)
		assert.Equal(t, "/", cookies[0].Path)
		assert.Less(t, cookies[0].MaxAge, 0)
	})

	t.Run("provider error parameter", func(t *testing.T) {
		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?error=access_denied", nil)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)
		assert.Equal(t, http.StatusBadRequest, cbRec.Code)
	})

	t.Run("unverified email rejected", func(t *testing.T) {
		stateCookie, state := startOAuthFlow(t, e, "google")
		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state="+url.QueryEscape(state), nil)
		cbReq.AddCookie(stateCookie)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)
		assert.Equal(t, http.StatusBadRequest, cbRec.Code)
	})
}

func TestOAuthMobileDeepLinkRedirectAndAllowlist(t *testing.T) {
	mockPool, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mockPool.Close()

	srv := setupMockOAuthServer(t, nil, map[string]any{
		"sub":            "google-mobile-1",
		"name":           "Mobile User",
		"email":          "mobile@example.com",
		"email_verified": true,
	})
	defer srv.Close()

	h := NewWithOAuth(db.New(mockPool), map[string]OAuthProviderConfig{
		"google": {
			Name:         "google",
			ClientID:     "cid",
			ClientSecret: "csec",
			RedirectURL:  "http://localhost:3000/auth/oauth/google/callback",
			AuthURL:      "https://accounts.google.com/o/oauth2/v2/auth",
			TokenURL:     srv.URL + "/token",
			UserInfoURL:  srv.URL + "/userinfo",
			Scopes:       []string{"openid", "email", "profile"},
		},
	}, srv.Client(), "")

	e := echo.New()
	h.Routes(e)

	t.Run("disallowed redirect_uri rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google?redirect_uri="+url.QueryEscape("https://evil.example/steal"), nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("mobile deep link receives token query param", func(t *testing.T) {
		stateCookie, state := startOAuthFlow(t, e, "google", "redirect_uri="+url.QueryEscape(defaultMobileOAuthRedirectURI))

		mockPool.ExpectQuery("SELECT p.id, rp.nama").
			WithArgs("google", "google-mobile-1").
			WillReturnRows(pgxmock.NewRows([]string{"id", "role", "aktif"}).AddRow(int32(55), "murid", true))

		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state="+url.QueryEscape(state), nil)
		cbReq.AddCookie(stateCookie)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)

		require.Equal(t, http.StatusFound, cbRec.Code)
		parsedLoc, err := url.Parse(cbRec.Header().Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, "pathmobile", parsedLoc.Scheme)
		assert.Equal(t, "app", parsedLoc.Host)
		assert.Equal(t, "/oauth/callback", parsedLoc.Path)
		assert.NotEmpty(t, parsedLoc.Query().Get("token"))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("mobile deep link receives error query param when cancelled", func(t *testing.T) {
		stateCookie, state := startOAuthFlow(t, e, "google", "redirect_uri="+url.QueryEscape(defaultMobileOAuthRedirectURI))

		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?error=access_denied&state="+url.QueryEscape(state), nil)
		cbReq.AddCookie(stateCookie)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)

		require.Equal(t, http.StatusFound, cbRec.Code)
		parsedLoc, err := url.Parse(cbRec.Header().Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, "access_denied", parsedLoc.Query().Get("error"))
	})

	t.Run("invalid client_state rejected", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/auth/oauth/google?redirect_uri="+url.QueryEscape(defaultMobileOAuthRedirectURI)+"&client_state=short", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)

		reqBadChar := httptest.NewRequest(http.MethodGet, "/auth/oauth/google?redirect_uri="+url.QueryEscape(defaultMobileOAuthRedirectURI)+"&client_state="+url.QueryEscape("valid-prefix-but bad spaces"), nil)
		recBadChar := httptest.NewRecorder()
		e.ServeHTTP(recBadChar, reqBadChar)
		assert.Equal(t, http.StatusBadRequest, recBadChar.Code)
	})

	t.Run("mobile client_state succeeds when browser drops oauth_state cookie", func(t *testing.T) {
		clientState := "mobile-client-nonce-1234567890"
		stateCookie, state := startOAuthFlow(
			t,
			e,
			"google",
			"redirect_uri="+url.QueryEscape(defaultMobileOAuthRedirectURI)+"&client_state="+url.QueryEscape(clientState),
		)
		payload, err := verifySignedState(stateCookie.Value, utils.GetSecret())
		require.NoError(t, err)
		assert.NotContains(t, state, payload.CodeVerifier)
		assert.NotContains(t, state, clientState)

		mockPool.ExpectQuery("SELECT p.id, rp.nama").
			WithArgs("google", "google-mobile-1").
			WillReturnRows(pgxmock.NewRows([]string{"id", "role", "aktif"}).AddRow(int32(55), "murid", true))

		// Intentionally omit oauth_state cookie to simulate Custom Tab / ITP dropping cookies.
		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state="+url.QueryEscape(state), nil)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)

		require.Equal(t, http.StatusFound, cbRec.Code)
		parsedLoc, err := url.Parse(cbRec.Header().Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, "pathmobile", parsedLoc.Scheme)
		assert.Equal(t, "app", parsedLoc.Host)
		assert.Equal(t, "/oauth/callback", parsedLoc.Path)
		assert.Equal(t, clientState, parsedLoc.Query().Get("state"))
		assert.NotEmpty(t, parsedLoc.Query().Get("token"))
		assert.NoError(t, mockPool.ExpectationsWereMet())
	})

	t.Run("mobile client_state forwards error and state when cookie missing", func(t *testing.T) {
		clientState := "mobile-cancel-nonce-1234567890"
		_, state := startOAuthFlow(
			t,
			e,
			"google",
			"redirect_uri="+url.QueryEscape(defaultMobileOAuthRedirectURI)+"&client_state="+url.QueryEscape(clientState),
		)

		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?error=access_denied&state="+url.QueryEscape(state), nil)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)

		require.Equal(t, http.StatusFound, cbRec.Code)
		parsedLoc, err := url.Parse(cbRec.Header().Get("Location"))
		require.NoError(t, err)
		assert.Equal(t, "access_denied", parsedLoc.Query().Get("error"))
		assert.Equal(t, clientState, parsedLoc.Query().Get("state"))
	})

	t.Run("expired or tampered sealed state rejected without cookie", func(t *testing.T) {
		expiredState, err := sealOAuthQueryState(oauthQueryStatePayload{
			Provider:     "google",
			Nonce:        "nonce-1",
			CodeVerifier: "verifier-1",
			RedirectURI:  defaultMobileOAuthRedirectURI,
			ClientNonce:  "mobile-expired-nonce-123456",
			ExpiresAt:    time.Now().Add(-time.Minute).Unix(),
		}, utils.GetSecret())
		require.NoError(t, err)

		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state="+url.QueryEscape(expiredState), nil)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)
		assert.Equal(t, http.StatusBadRequest, cbRec.Code)

		tamperedReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state="+url.QueryEscape(expiredState+"tamper"), nil)
		tamperedRec := httptest.NewRecorder()
		e.ServeHTTP(tamperedRec, tamperedReq)
		assert.Equal(t, http.StatusBadRequest, tamperedRec.Code)
	})

	t.Run("client_state without redirect_uri still requires cookie", func(t *testing.T) {
		_, state := startOAuthFlow(t, e, "google", "client_state=direct-browser-nonce-123456")

		cbReq := httptest.NewRequest(http.MethodGet, "/auth/oauth/google/callback?code=auth-code-123&state="+url.QueryEscape(state), nil)
		cbRec := httptest.NewRecorder()
		e.ServeHTTP(cbRec, cbReq)
		assert.Equal(t, http.StatusBadRequest, cbRec.Code)
	})
}
