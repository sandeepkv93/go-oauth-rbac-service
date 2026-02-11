package service

import (
	"context"
	"testing"
	"time"
)

func TestRedisIdempotencyStoreStateTransitionsAndTTLRefresh(t *testing.T) {
	ctx := context.Background()
	_, client := newRedisClientForTest(t)
	store := NewRedisIdempotencyStore(client, "idem_test")

	scope := "register"
	key := "idem-key"
	fingerprint := "fp-1"

	res, err := store.Begin(ctx, scope, key, fingerprint, time.Second)
	if err != nil {
		t.Fatalf("begin new: %v", err)
	}
	if res.State != IdempotencyStateNew {
		t.Fatalf("expected new, got %s", res.State)
	}

	res, err = store.Begin(ctx, scope, key, fingerprint, time.Second)
	if err != nil {
		t.Fatalf("begin in-progress: %v", err)
	}
	if res.State != IdempotencyStateInProgress {
		t.Fatalf("expected in_progress, got %s", res.State)
	}

	res, err = store.Begin(ctx, scope, key, "fp-conflict", time.Second)
	if err != nil {
		t.Fatalf("begin conflict: %v", err)
	}
	if res.State != IdempotencyStateConflict {
		t.Fatalf("expected conflict, got %s", res.State)
	}

	redisKey := store.redisKey(scope, key)
	initialTTL := client.PTTL(ctx, redisKey).Val()
	if initialTTL <= 0 {
		t.Fatalf("expected positive ttl before complete, got %v", initialTTL)
	}

	completeTTL := 3 * time.Second
	if err := store.Complete(ctx, scope, key, fingerprint, CachedHTTPResponse{
		StatusCode:  201,
		ContentType: "application/json",
		Body:        []byte(`{"ok":true}`),
	}, completeTTL); err != nil {
		t.Fatalf("complete: %v", err)
	}

	postCompleteTTL := client.PTTL(ctx, redisKey).Val()
	if postCompleteTTL <= initialTTL {
		t.Fatalf("expected ttl refresh on complete, before=%v after=%v", initialTTL, postCompleteTTL)
	}

	replay, err := store.Begin(ctx, scope, key, fingerprint, time.Second)
	if err != nil {
		t.Fatalf("begin replay: %v", err)
	}
	if replay.State != IdempotencyStateReplay || replay.Cached == nil {
		t.Fatalf("expected replay with cached response, got state=%s cached=%v", replay.State, replay.Cached != nil)
	}
	if replay.Cached.StatusCode != 201 || string(replay.Cached.Body) != `{"ok":true}` {
		t.Fatalf("unexpected replay payload: %#v", replay.Cached)
	}
}

func TestRedisIdempotencyStoreMalformedReplayPayloads(t *testing.T) {
	ctx := context.Background()
	_, client := newRedisClientForTest(t)
	store := NewRedisIdempotencyStore(client, "idem_test")

	scope := "register"
	key := "idem-malformed"
	fp := "fp-1"
	redisKey := store.redisKey(scope, key)

	if err := client.HSet(ctx, redisKey,
		"fingerprint", fp,
		"status", "completed",
		"response_status", "NaN",
		"content_type", "application/json",
		"response_body", "eyJvayI6dHJ1ZX0=",
	).Err(); err != nil {
		t.Fatalf("seed malformed status: %v", err)
	}
	if _, err := store.Begin(ctx, scope, key, fp, time.Second); err == nil {
		t.Fatal("expected parse replay status error")
	}

	if err := client.HSet(ctx, redisKey,
		"fingerprint", fp,
		"status", "completed",
		"response_status", "200",
		"content_type", "application/json",
		"response_body", "!!!not-base64!!!",
	).Err(); err != nil {
		t.Fatalf("seed malformed body: %v", err)
	}
	if _, err := store.Begin(ctx, scope, key, fp, time.Second); err == nil {
		t.Fatal("expected decode replay body error")
	}

}
