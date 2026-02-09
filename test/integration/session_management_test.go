package integration

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

type sessionView struct {
	ID        uint   `json:"id"`
	IsCurrent bool   `json:"is_current"`
	UserAgent string `json:"user_agent"`
	IP        string `json:"ip"`
}

func TestSessionManagementListAndRevokeByDevice(t *testing.T) {
	baseURL, client, closeFn := newAuthTestServer(t)
	defer closeFn()

	registerBody := map[string]string{
		"email":    "session-mgmt@example.com",
		"name":     "Session Manager",
		"password": "Valid#Pass1234",
	}
	resp, env := doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/local/register", registerBody, nil)
	if resp.StatusCode != http.StatusCreated || !env.Success {
		t.Fatalf("register failed: status=%d success=%v", resp.StatusCode, env.Success)
	}
	refreshA := cookieValue(t, client, baseURL, "refresh_token")
	csrfA := cookieValue(t, client, baseURL, "csrf_token")

	loginBody := map[string]string{
		"email":    registerBody["email"],
		"password": registerBody["password"],
	}
	resp, env = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/local/login", loginBody, nil)
	if resp.StatusCode != http.StatusOK || !env.Success {
		t.Fatalf("second login failed: status=%d success=%v", resp.StatusCode, env.Success)
	}
	csrfB := cookieValue(t, client, baseURL, "csrf_token")

	resp, env = doJSON(t, client, http.MethodGet, baseURL+"/api/v1/me/sessions", nil, nil)
	if resp.StatusCode != http.StatusOK || !env.Success {
		t.Fatalf("list sessions failed: status=%d success=%v", resp.StatusCode, env.Success)
	}
	var sessions []sessionView
	if err := json.Unmarshal(env.Data, &sessions); err != nil {
		t.Fatalf("decode sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 active sessions, got %d", len(sessions))
	}
	var currentCount int
	var oldSessionID uint
	for _, s := range sessions {
		if s.IsCurrent {
			currentCount++
			continue
		}
		oldSessionID = s.ID
	}
	if currentCount != 1 {
		t.Fatalf("expected exactly one current session, got %d", currentCount)
	}
	if oldSessionID == 0 {
		t.Fatal("expected one non-current session to revoke")
	}

	resp, env = doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/me/sessions/"+strconv.FormatUint(uint64(oldSessionID), 10), nil, map[string]string{
		"X-CSRF-Token": csrfB,
	})
	if resp.StatusCode != http.StatusOK || !env.Success {
		t.Fatalf("revoke session failed: status=%d success=%v", resp.StatusCode, env.Success)
	}

	resp, env = doRaw(t, client, http.MethodPost, baseURL+"/api/v1/auth/refresh", nil, map[string]string{
		"X-CSRF-Token": csrfA,
	}, []*http.Cookie{
		{Name: "refresh_token", Value: refreshA, Path: "/api/v1/auth"},
		{Name: "csrf_token", Value: csrfA, Path: "/"},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected revoked session refresh to fail with 401, got %d", resp.StatusCode)
	}
}

func TestSessionManagementRevokeOthersKeepsCurrent(t *testing.T) {
	baseURL, client, closeFn := newAuthTestServer(t)
	defer closeFn()

	registerBody := map[string]string{
		"email":    "session-revoke-others@example.com",
		"name":     "Session Revoke Others",
		"password": "Valid#Pass1234",
	}
	resp, env := doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/local/register", registerBody, nil)
	if resp.StatusCode != http.StatusCreated || !env.Success {
		t.Fatalf("register failed: status=%d success=%v", resp.StatusCode, env.Success)
	}
	refreshA := cookieValue(t, client, baseURL, "refresh_token")
	csrfA := cookieValue(t, client, baseURL, "csrf_token")

	loginBody := map[string]string{
		"email":    registerBody["email"],
		"password": registerBody["password"],
	}
	resp, env = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/local/login", loginBody, nil)
	if resp.StatusCode != http.StatusOK || !env.Success {
		t.Fatalf("second login failed: status=%d success=%v", resp.StatusCode, env.Success)
	}
	csrfB := cookieValue(t, client, baseURL, "csrf_token")

	resp, env = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/me/sessions/revoke-others", nil, map[string]string{
		"X-CSRF-Token": csrfB,
	})
	if resp.StatusCode != http.StatusOK || !env.Success {
		t.Fatalf("revoke others failed: status=%d success=%v", resp.StatusCode, env.Success)
	}

	resp, env = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/auth/refresh", nil, map[string]string{
		"X-CSRF-Token": cookieValue(t, client, baseURL, "csrf_token"),
	})
	if resp.StatusCode != http.StatusOK || !env.Success {
		t.Fatalf("current session refresh should succeed after revoke others, got %d", resp.StatusCode)
	}

	resp, env = doRaw(t, client, http.MethodPost, baseURL+"/api/v1/auth/refresh", nil, map[string]string{
		"X-CSRF-Token": csrfA,
	}, []*http.Cookie{
		{Name: "refresh_token", Value: refreshA, Path: "/api/v1/auth"},
		{Name: "csrf_token", Value: csrfA, Path: "/"},
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected old session refresh to fail with 401, got %d", resp.StatusCode)
	}
}

func TestSessionManagementRevokeErrors(t *testing.T) {
	baseURL, client, closeFn := newAuthTestServer(t)
	defer closeFn()
	registerAndLogin(t, client, baseURL, "session-errors@example.com", "Valid#Pass1234")
	csrf := cookieValue(t, client, baseURL, "csrf_token")

	resp, _ := doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/me/sessions/not-a-number", nil, map[string]string{
		"X-CSRF-Token": csrf,
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed session id, got %d", resp.StatusCode)
	}

	resp, _ = doJSON(t, client, http.MethodDelete, baseURL+"/api/v1/me/sessions/999999", nil, map[string]string{
		"X-CSRF-Token": csrf,
	})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown session id, got %d", resp.StatusCode)
	}
}
