package integration

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestHealthLiveAndReadyEndpoints(t *testing.T) {
	baseURL, client, closeFn := newAuthTestServer(t)
	defer closeFn()

	t.Run("live endpoint stable 200 payload", func(t *testing.T) {
		resp, env := doJSON(t, client, http.MethodGet, baseURL+"/health/live", nil, nil)
		if resp.StatusCode != http.StatusOK || !env.Success {
			t.Fatalf("health live failed: status=%d success=%v", resp.StatusCode, env.Success)
		}
		var data map[string]any
		if err := json.Unmarshal(env.Data, &data); err != nil {
			t.Fatalf("decode live data: %v", err)
		}
		if got, _ := data["status"].(string); got != "ok" {
			t.Fatalf("expected status=ok, got %+v", data)
		}
	})

	t.Run("ready endpoint nil-runner ready payload", func(t *testing.T) {
		resp, env := doJSON(t, client, http.MethodGet, baseURL+"/health/ready", nil, nil)
		if resp.StatusCode != http.StatusOK || !env.Success {
			t.Fatalf("health ready failed: status=%d success=%v", resp.StatusCode, env.Success)
		}
		var data map[string]any
		if err := json.Unmarshal(env.Data, &data); err != nil {
			t.Fatalf("decode ready data: %v", err)
		}
		if got, _ := data["status"].(string); got != "ready" {
			t.Fatalf("expected status=ready, got %+v", data)
		}
		checks, ok := data["checks"].([]any)
		if !ok {
			t.Fatalf("expected checks array in ready payload, got %+v", data)
		}
		if len(checks) != 0 {
			t.Fatalf("expected empty checks for nil runner, got %+v", checks)
		}
	})
}
