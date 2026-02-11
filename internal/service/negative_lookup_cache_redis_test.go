package service

import (
	"context"
	"testing"
	"time"
)

func TestRedisNegativeLookupCacheStoreSetGetInvalidateAndStale(t *testing.T) {
	ctx := context.Background()
	server, client := newRedisClientForTest(t)
	store := NewRedisNegativeLookupCacheStore(client, "neg_test")

	namespace := "admin.users"
	key := "missing:user@example.com"

	hit, err := store.Get(ctx, namespace, key)
	if err != nil {
		t.Fatalf("initial get: %v", err)
	}
	if hit {
		t.Fatal("expected initial miss")
	}

	if err := store.Set(ctx, namespace, key, 2*time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	hit, err = store.Get(ctx, namespace, key)
	if err != nil {
		t.Fatalf("get after set: %v", err)
	}
	if !hit {
		t.Fatal("expected hit after set")
	}

	server.FastForward(3 * time.Second)
	hit, err = store.Get(ctx, namespace, key)
	if err != nil {
		t.Fatalf("get after ttl expiry: %v", err)
	}
	if hit {
		t.Fatal("expected miss after ttl expiry")
	}

	if err := store.Set(ctx, namespace, key, time.Minute); err != nil {
		t.Fatalf("set before invalidate: %v", err)
	}
	if err := store.InvalidateNamespace(ctx, namespace); err != nil {
		t.Fatalf("invalidate namespace: %v", err)
	}
	hit, err = store.Get(ctx, namespace, key)
	if err != nil {
		t.Fatalf("get after invalidate: %v", err)
	}
	if hit {
		t.Fatal("expected miss after invalidate")
	}
}
