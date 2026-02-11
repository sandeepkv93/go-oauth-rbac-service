package service

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/oauth2"
)

type testOAuthProvider struct {
	exchangeFn func(ctx context.Context, code string) (*oauth2.Token, error)
	userinfoFn func(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error)
}

func (p testOAuthProvider) AuthCodeURL(_ string) string { return "" }

func (p testOAuthProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	if p.exchangeFn != nil {
		return p.exchangeFn(ctx, code)
	}
	return &oauth2.Token{AccessToken: "token"}, nil
}

func (p testOAuthProvider) FetchUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	if p.userinfoFn != nil {
		return p.userinfoFn(ctx, token)
	}
	return &OAuthUserInfo{ProviderUserID: "provider-id", Email: "user@example.com", EmailVerified: true}, nil
}

func TestOAuthServiceHandleGoogleCallbackExchangeError(t *testing.T) {
	svc := NewOAuthService(
		testOAuthProvider{exchangeFn: func(context.Context, string) (*oauth2.Token, error) {
			return nil, context.DeadlineExceeded
		}},
		nil,
		nil,
		nil,
	)

	_, err := svc.HandleGoogleCallback(context.Background(), "code")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestOAuthServiceHandleGoogleCallbackUserInfoError(t *testing.T) {
	userinfoErr := errors.New("userinfo status: 500")
	svc := NewOAuthService(
		testOAuthProvider{userinfoFn: func(context.Context, *oauth2.Token) (*OAuthUserInfo, error) {
			return nil, userinfoErr
		}},
		nil,
		nil,
		nil,
	)

	_, err := svc.HandleGoogleCallback(context.Background(), "code")
	if !errors.Is(err, userinfoErr) {
		t.Fatalf("expected userinfo error, got %v", err)
	}
}

func TestOAuthServiceHandleGoogleCallbackEmailNotVerified(t *testing.T) {
	svc := NewOAuthService(
		testOAuthProvider{userinfoFn: func(context.Context, *oauth2.Token) (*OAuthUserInfo, error) {
			return &OAuthUserInfo{ProviderUserID: "provider-id", Email: "user@example.com", EmailVerified: false}, nil
		}},
		nil,
		nil,
		nil,
	)

	_, err := svc.HandleGoogleCallback(context.Background(), "code")
	if err == nil || err.Error() != "google email not verified" {
		t.Fatalf("expected google email not verified error, got %v", err)
	}
}

func TestClassifyOAuthError(t *testing.T) {
	if got := classifyOAuthError(context.Canceled); got != "context_canceled" {
		t.Fatalf("expected context_canceled, got %q", got)
	}
	if got := classifyOAuthError(context.DeadlineExceeded); got != "timeout" {
		t.Fatalf("expected timeout, got %q", got)
	}
	if got := classifyOAuthError(errors.New("userinfo status: 401")); got != "userinfo_status" {
		t.Fatalf("expected userinfo_status, got %q", got)
	}
	if got := classifyOAuthError(errors.New("missing required userinfo fields")); got != "invalid_userinfo" {
		t.Fatalf("expected invalid_userinfo, got %q", got)
	}
	if got := classifyOAuthError(errors.New("oauth2: cannot fetch token")); got != "oauth2_exchange" {
		t.Fatalf("expected oauth2_exchange, got %q", got)
	}
}
