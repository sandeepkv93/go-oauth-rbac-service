package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/oauth2"

	"log/slog"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/config"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/service"
)

const oauthStateSigningKey = "0123456789abcdef0123456789abcdef"

type oauthProviderFuncStub struct {
	authCodeURLFn func(state string) string
	exchangeFn    func(ctx context.Context, code string) (*oauth2.Token, error)
	userInfoFn    func(ctx context.Context, token *oauth2.Token) (*service.OAuthUserInfo, error)
}

func (s oauthProviderFuncStub) AuthCodeURL(state string) string {
	if s.authCodeURLFn != nil {
		return s.authCodeURLFn(state)
	}
	return ""
}

func (s oauthProviderFuncStub) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if s.exchangeFn != nil {
		return s.exchangeFn(ctx, code)
	}
	return nil, errors.New("exchange not configured")
}

func (s oauthProviderFuncStub) FetchUserInfo(ctx context.Context, token *oauth2.Token) (*service.OAuthUserInfo, error) {
	if s.userInfoFn != nil {
		return s.userInfoFn(ctx, token)
	}
	return nil, errors.New("userinfo not configured")
}

func TestGoogleLoginRedirectAndDisabled(t *testing.T) {
	events := captureAuditEvents(t, func() {
		baseURL, client, closeFn := newAuthTestServerWithOptions(t, authTestServerOptions{
			cfgOverride: func(cfg *config.Config) { cfg.AuthGoogleEnabled = true },
			oauthProvider: oauthProviderFuncStub{
				authCodeURLFn: func(state string) string {
					return "https://accounts.example/oauth?state=" + state
				},
			},
		})
		defer closeFn()

		resp, _ := doRawTextNoRedirect(t, client, http.MethodGet, baseURL+"/api/v1/auth/google/login", nil, nil, nil)
		if resp.StatusCode != http.StatusFound {
			t.Fatalf("expected 302 redirect, got %d", resp.StatusCode)
		}
		location := resp.Header.Get("Location")
		if !strings.HasPrefix(location, "https://accounts.example/oauth?state=") {
			t.Fatalf("unexpected redirect location: %q", location)
		}
		assertCookieProps(t, resp, "oauth_state", "/api/v1/auth/google", true)
	})
	requireAuditEvent(t, events, "auth.google.login", "success", "redirect_issued")

	events = captureAuditEvents(t, func() {
		baseURL, client, closeFn := newAuthTestServerWithOptions(t, authTestServerOptions{
			cfgOverride: func(cfg *config.Config) { cfg.AuthGoogleEnabled = false },
			oauthProvider: oauthProviderFuncStub{
				authCodeURLFn: func(state string) string {
					return "https://accounts.example/oauth?state=" + state
				},
			},
		})
		defer closeFn()

		resp, env := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/auth/google/login", nil, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 when provider disabled, got %d", resp.StatusCode)
		}
		if env.Error == nil || env.Error.Code != "NOT_ENABLED" {
			t.Fatalf("expected NOT_ENABLED error envelope, got %#v", env.Error)
		}
	})
	requireAuditEvent(t, events, "auth.google.login", "rejected", "provider_disabled")
}

func TestGoogleCallbackValidationErrors(t *testing.T) {
	baseURL, client, closeFn := newAuthTestServerWithOptions(t, authTestServerOptions{
		cfgOverride:   func(cfg *config.Config) { cfg.AuthGoogleEnabled = true },
		oauthProvider: oauthProviderFuncStub{},
	})
	defer closeFn()

	events := captureAuditEvents(t, func() {
		resp, env := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/auth/google/callback", nil, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for missing state/code, got %d", resp.StatusCode)
		}
		if env.Error == nil || env.Error.Code != "BAD_REQUEST" {
			t.Fatalf("expected BAD_REQUEST envelope, got %#v", env.Error)
		}
	})
	requireAuditEvent(t, events, "auth.google.callback", "failure", "missing_code_or_state")

	events = captureAuditEvents(t, func() {
		resp, env := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/auth/google/callback?state=abc&code=xyz", nil, nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401 for invalid state cookie, got %d", resp.StatusCode)
		}
		if env.Error == nil || env.Error.Code != "UNAUTHORIZED" {
			t.Fatalf("expected UNAUTHORIZED envelope, got %#v", env.Error)
		}
	})
	requireAuditEvent(t, events, "auth.google.callback", "failure", "invalid_state")
}

func TestGoogleCallbackSuccessSetsCookiesAndClearsState(t *testing.T) {
	baseURL, client, closeFn := newAuthTestServerWithOptions(t, authTestServerOptions{
		cfgOverride: func(cfg *config.Config) { cfg.AuthGoogleEnabled = true },
		oauthProvider: oauthProviderFuncStub{
			authCodeURLFn: func(state string) string {
				return "https://accounts.example/oauth?state=" + state
			},
			exchangeFn: func(context.Context, string) (*oauth2.Token, error) {
				return &oauth2.Token{AccessToken: "oauth-token"}, nil
			},
			userInfoFn: func(context.Context, *oauth2.Token) (*service.OAuthUserInfo, error) {
				return &service.OAuthUserInfo{
					ProviderUserID: "google-user-1",
					Email:          "oauth-user@example.com",
					Name:           "OAuth User",
					Picture:        "https://example.com/avatar.png",
					EmailVerified:  true,
				}, nil
			},
		},
	})
	defer closeFn()

	events := captureAuditEvents(t, func() {
		loginResp, _ := doRawTextNoRedirect(t, client, http.MethodGet, baseURL+"/api/v1/auth/google/login", nil, nil, nil)
		if loginResp.StatusCode != http.StatusFound {
			t.Fatalf("expected login redirect 302, got %d", loginResp.StatusCode)
		}
		location := loginResp.Header.Get("Location")
		redirectURL, err := url.Parse(location)
		if err != nil {
			t.Fatalf("parse redirect URL: %v", err)
		}
		state := redirectURL.Query().Get("state")
		if strings.TrimSpace(state) == "" {
			t.Fatalf("expected non-empty oauth state in redirect URL: %q", location)
		}

		resp, env := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/auth/google/callback?state="+url.QueryEscape(state)+"&code=google-code-1", nil, nil)
		if resp.StatusCode != http.StatusOK || !env.Success {
			t.Fatalf("expected callback success 200, got status=%d success=%v", resp.StatusCode, env.Success)
		}

		assertCookieProps(t, resp, "access_token", "/", true)
		assertCookieProps(t, resp, "refresh_token", "/api/v1/auth", true)
		assertCookieProps(t, resp, "csrf_token", "/", false)
		assertClearingCookie(t, resp, "oauth_state")
		assertCookiePath(t, resp, "oauth_state", "/api/v1/auth/google")
	})

	requireAuditEvent(t, events, "auth.google.login", "success", "redirect_issued")
	requireAuditEvent(t, events, "auth.login", "success", "oauth_google")
}

func TestGoogleCallbackDisabledAndProviderErrors(t *testing.T) {
	t.Run("provider disabled returns not enabled", func(t *testing.T) {
		baseURL, client, closeFn := newAuthTestServerWithOptions(t, authTestServerOptions{
			cfgOverride: func(cfg *config.Config) { cfg.AuthGoogleEnabled = false },
		})
		defer closeFn()

		state := "disabled-state"
		signed := security.SignState(state, oauthStateSigningKey)
		resp, env := doRaw(t, client, http.MethodGet, baseURL+"/api/v1/auth/google/callback?state="+state+"&code=abc", nil, nil, []*http.Cookie{
			{Name: "oauth_state", Value: signed, Path: "/api/v1/auth/google"},
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 when google provider disabled, got %d", resp.StatusCode)
		}
		if env.Error == nil || env.Error.Code != "NOT_ENABLED" {
			t.Fatalf("expected NOT_ENABLED envelope, got %#v", env.Error)
		}
	})

	t.Run("oauth exchange and userinfo errors map to oauth_failed and audit", func(t *testing.T) {
		cases := []struct {
			name     string
			provider service.OAuthProvider
		}{
			{
				name: "exchange error",
				provider: oauthProviderFuncStub{
					exchangeFn: func(context.Context, string) (*oauth2.Token, error) {
						return nil, errors.New("oauth2: cannot fetch token")
					},
				},
			},
			{
				name: "userinfo error",
				provider: oauthProviderFuncStub{
					exchangeFn: func(context.Context, string) (*oauth2.Token, error) {
						return &oauth2.Token{AccessToken: "oauth-token"}, nil
					},
					userInfoFn: func(context.Context, *oauth2.Token) (*service.OAuthUserInfo, error) {
						return nil, errors.New("userinfo status: 500")
					},
				},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				baseURL, client, closeFn := newAuthTestServerWithOptions(t, authTestServerOptions{
					cfgOverride:   func(cfg *config.Config) { cfg.AuthGoogleEnabled = true },
					oauthProvider: tc.provider,
				})
				defer closeFn()

				state := "error-state"
				signed := security.SignState(state, oauthStateSigningKey)
				events := captureAuditEvents(t, func() {
					resp, env := doRaw(t, client, http.MethodGet, baseURL+"/api/v1/auth/google/callback?state="+state+"&code=abc", nil, nil, []*http.Cookie{
						{Name: "oauth_state", Value: signed, Path: "/api/v1/auth/google"},
					})
					if resp.StatusCode != http.StatusUnauthorized {
						t.Fatalf("expected 401 oauth failure, got %d", resp.StatusCode)
					}
					if env.Error == nil || env.Error.Code != "OAUTH_FAILED" {
						t.Fatalf("expected OAUTH_FAILED envelope, got %#v", env.Error)
					}
				})
				requireAuditEvent(t, events, "auth.google.callback", "failure", "oauth_exchange_error")
			})
		}
	})
}

func doRawTextNoRedirect(t *testing.T, client *http.Client, method, url string, body any, headers map[string]string, cookies []*http.Cookie) (*http.Response, string) {
	t.Helper()
	clone := *client
	clone.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return doRawText(t, &clone, method, url, body, headers, cookies)
}

func captureAuditEvents(t *testing.T, fn func()) []map[string]any {
	t.Helper()
	var logBuf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(previous)

	fn()
	return extractAuditEvents(t, logBuf.String())
}

func extractAuditEvents(t *testing.T, logs string) []map[string]any {
	t.Helper()
	lines := strings.Split(logs, "\n")
	events := make([]map[string]any, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if msg, _ := event["msg"].(string); msg == "audit.event" {
			events = append(events, event)
		}
	}
	return events
}

func requireAuditEvent(t *testing.T, events []map[string]any, eventName, outcome, reason string) {
	t.Helper()
	for _, event := range events {
		gotName, _ := event["event_name"].(string)
		gotOutcome, _ := event["outcome"].(string)
		gotReason, _ := event["reason"].(string)
		if gotName == eventName && gotOutcome == outcome && gotReason == reason {
			return
		}
	}
	t.Fatalf("expected audit event_name=%q outcome=%q reason=%q, got events=%#v", eventName, outcome, reason, events)
}

func assertCookiePath(t *testing.T, resp *http.Response, name, path string) {
	t.Helper()
	for _, c := range resp.Cookies() {
		if c.Name != name {
			continue
		}
		if c.Path != path {
			t.Fatalf("cookie %s path mismatch: got %q want %q", name, c.Path, path)
		}
		return
	}
	t.Fatalf("cookie %s not found in response", name)
}
