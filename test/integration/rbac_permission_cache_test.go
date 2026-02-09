package integration

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/config"
)

func TestRBACPermissionCacheEnforcesRoleChangeWithoutRelogin(t *testing.T) {
	baseURL, client, closeFn := newAuthTestServerWithOptions(t, authTestServerOptions{
		cfgOverride: func(cfg *config.Config) {
			cfg.BootstrapAdminEmail = "rbac-cache-admin@example.com"
		},
	})
	defer closeFn()

	registerAndLogin(t, client, baseURL, "rbac-cache-admin@example.com", "Valid#Pass1234")

	resp, env := doJSON(t, client, http.MethodGet, baseURL+"/api/v1/me", nil, nil)
	if resp.StatusCode != http.StatusOK || !env.Success {
		t.Fatalf("load current user failed: status=%d success=%v", resp.StatusCode, env.Success)
	}
	var me struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &me); err != nil {
		t.Fatalf("decode me payload: %v", err)
	}
	if me.ID == 0 {
		t.Fatal("expected non-zero user id")
	}

	resp, env = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/admin/roles", map[string]any{
		"name":        "rbac-cache-limited",
		"description": "limited role for cache test",
		"permissions": []string{"roles:read"},
	}, nil)
	if resp.StatusCode != http.StatusCreated || !env.Success {
		t.Fatalf("create limited role failed: status=%d success=%v", resp.StatusCode, env.Success)
	}
	var createdRole struct {
		ID uint `json:"id"`
	}
	if err := json.Unmarshal(env.Data, &createdRole); err != nil {
		t.Fatalf("decode created role: %v", err)
	}
	if createdRole.ID == 0 {
		t.Fatal("expected role id")
	}

	resp, env = doJSON(t, client, http.MethodPatch, baseURL+"/api/v1/admin/users/"+itoa(me.ID)+"/roles", map[string]any{
		"role_ids": []uint{createdRole.ID},
	}, nil)
	if resp.StatusCode != http.StatusOK || !env.Success {
		t.Fatalf("set roles failed: status=%d success=%v", resp.StatusCode, env.Success)
	}

	resp, env = doJSON(t, client, http.MethodPost, baseURL+"/api/v1/admin/permissions", map[string]any{
		"resource": "cachetest",
		"action":   "write",
	}, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden after permission change, got %d", resp.StatusCode)
	}
	if env.Error == nil || env.Error.Code != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN error envelope, got %#v", env.Error)
	}
}
