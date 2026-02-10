package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/http/middleware"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/service"
)

func TestRedisRateLimiterConcurrentBurstHonorsLimit(t *testing.T) {
	redisClient, cleanup := startRedisContainer(t)
	defer cleanup()

	limiter := middleware.NewRedisFixedWindowLimiter(redisClient, "itest:rl")
	policy := middleware.RateLimitPolicy{
		SustainedLimit:    20,
		SustainedWindow:   10 * time.Minute,
		BurstCapacity:     20,
		BurstRefillPerSec: float64(20) / (10 * time.Minute).Seconds(),
	}

	const attempts = 100
	var allowed atomic.Int64
	errCh := make(chan error, attempts)
	var wg sync.WaitGroup

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := limiter.Allow(context.Background(), "same-actor", policy)
			if err != nil {
				errCh <- err
				return
			}
			if decision.Allowed {
				allowed.Add(1)
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("limiter allow failed: %v", err)
	}

	if got := allowed.Load(); got != int64(policy.SustainedLimit) {
		t.Fatalf("expected exactly %d allowed requests, got %d", policy.SustainedLimit, got)
	}

	decision, err := limiter.Allow(context.Background(), "same-actor", policy)
	if err != nil {
		t.Fatalf("final allow call failed: %v", err)
	}
	if decision.Allowed {
		t.Fatal("expected next request after burst to be limited")
	}
}

func TestRedisIdempotencyConcurrentInProgressAndReplayConsistency(t *testing.T) {
	redisClient, cleanup := startRedisContainer(t)
	defer cleanup()

	store := service.NewRedisIdempotencyStore(redisClient, "itest:idem")
	idem := middleware.NewIdempotencyMiddleware(store, 5*time.Minute)

	release := make(chan struct{})
	var executions atomic.Int64

	handler := idem.Middleware("register")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		executions.Add(1)
		<-release

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"status":"created","id":"user-123"}`)
	}))

	srv := httptest.NewServer(handler)
	defer srv.Close()

	type result struct {
		statusCode int
		body       string
		replayed   string
	}
	results := make(chan result, 12)
	payload := registerPayload{
		Email:    "idem-race@example.com",
		Name:     "Redis Race",
		Password: "Valid#Pass1234",
	}

	const concurrentCalls = 12
	var wg sync.WaitGroup
	for i := 0; i < concurrentCalls; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, body := postJSONWithIdempotencyKey(t, srv.URL, "idem-race-001", payload)
			results <- result{
				statusCode: resp.StatusCode,
				body:       body,
				replayed:   resp.Header.Get("X-Idempotency-Replayed"),
			}
		}()
	}

	waitForExecutionStart(t, &executions, 3*time.Second)
	close(release)
	wg.Wait()
	close(results)

	var createdCount int
	var inProgressCount int
	var firstCreatedBody string
	for res := range results {
		switch res.statusCode {
		case http.StatusCreated:
			createdCount++
			if firstCreatedBody == "" {
				firstCreatedBody = res.body
			}
			if res.replayed == "true" {
				t.Fatalf("did not expect concurrent in-flight response to be replayed: body=%q", res.body)
			}
		case http.StatusConflict:
			inProgressCount++
			if !strings.Contains(res.body, "request with this idempotency key is in progress") {
				t.Fatalf("expected in-progress conflict body, got %q", res.body)
			}
		default:
			t.Fatalf("unexpected status code from concurrent request: %d body=%q", res.statusCode, res.body)
		}
	}

	if createdCount != 1 {
		t.Fatalf("expected exactly one successful execution, got %d", createdCount)
	}
	if inProgressCount != concurrentCalls-1 {
		t.Fatalf("expected %d in-progress conflicts, got %d", concurrentCalls-1, inProgressCount)
	}
	if got := executions.Load(); got != 1 {
		t.Fatalf("expected handler to execute exactly once, got %d", got)
	}

	replayResp, replayBody := postJSONWithIdempotencyKey(t, srv.URL, "idem-race-001", payload)
	if replayResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected replay status 201, got %d body=%q", replayResp.StatusCode, replayBody)
	}
	if got := replayResp.Header.Get("X-Idempotency-Replayed"); got != "true" {
		t.Fatalf("expected replay header true, got %q", got)
	}
	if replayBody != firstCreatedBody {
		t.Fatalf("expected replay body to match original response\noriginal=%s\nreplay=%s", firstCreatedBody, replayBody)
	}
}

func postJSONWithIdempotencyKey(t *testing.T, baseURL, key string, body any) (*http.Response, string) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, baseURL, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", key)
	req.Header.Set("X-Forwarded-For", "10.0.0.10")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

type registerPayload struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func waitForExecutionStart(t *testing.T, executions *atomic.Int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if executions.Load() >= 1 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for handler execution to start")
}

func startRedisContainer(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping redis container integration test in short mode")
	}
	if !dockerAvailable() {
		t.Skip("docker is not available; skipping redis container integration test")
	}

	hostPort := reserveLocalPort(t)
	containerName := "sogbsk-redis-it-" + strconv.FormatInt(time.Now().UnixNano(), 10) + "-" + strconv.Itoa(rand.Intn(1000))

	runCmd := exec.Command("docker", "run", "-d", "--rm",
		"--name", containerName,
		"-p", fmt.Sprintf("127.0.0.1:%d:6379", hostPort),
		"redis:7-alpine",
		"redis-server", "--save", "", "--appendonly", "no",
	)
	out, err := runCmd.CombinedOutput()
	if err != nil {
		t.Skipf("unable to start redis container: %v output=%s", err, strings.TrimSpace(string(out)))
	}

	client := redis.NewClient(&redis.Options{Addr: fmt.Sprintf("127.0.0.1:%d", hostPort)})
	ctx := context.Background()
	deadline := time.Now().Add(20 * time.Second)
	for {
		if time.Now().After(deadline) {
			_ = client.Close()
			_ = exec.Command("docker", "rm", "-f", containerName).Run()
			t.Fatalf("timed out waiting for redis container %s to become ready", containerName)
		}
		if err := client.Ping(ctx).Err(); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	cleanup := func() {
		_ = client.Close()
		_ = exec.Command("docker", "rm", "-f", containerName).Run()
	}
	return client, cleanup
}

func dockerAvailable() bool {
	cmd := exec.Command("docker", "version", "--format", "{{.Server.Version}}")
	return cmd.Run() == nil
}

func reserveLocalPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve local port: %v", err)
	}
	defer func() { _ = l.Close() }()
	addr, ok := l.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected addr type %T", l.Addr())
	}
	return addr.Port
}
