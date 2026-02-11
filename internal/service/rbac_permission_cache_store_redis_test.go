package service

import (
	"context"
	"testing"
	"time"
)

func TestRedisRBACPermissionCacheStoreKeyingAndInvalidation(t *testing.T) {
	ctx := context.Background()
	_, client := newRedisClientForTest(t)
	store := NewRedisRBACPermissionCacheStore(client, "rbac_test")

	userID := uint(42)
	session := "sess-1"
	perms := []string{"role.read", "role.write"}

	if err := store.Set(ctx, userID, session, perms, time.Minute); err != nil {
		t.Fatalf("set initial perms: %v", err)
	}
	got, ok, err := store.Get(ctx, userID, session)
	if err != nil {
		t.Fatalf("get initial perms: %v", err)
	}
	if !ok {
		t.Fatal("expected cache hit after set")
	}
	if len(got) != 2 || got[0] != perms[0] || got[1] != perms[1] {
		t.Fatalf("unexpected cached permissions: %#v", got)
	}

	if err := store.InvalidateUser(ctx, userID); err != nil {
		t.Fatalf("invalidate user: %v", err)
	}
	_, ok, err = store.Get(ctx, userID, session)
	if err != nil {
		t.Fatalf("get after user invalidation: %v", err)
	}
	if ok {
		t.Fatal("expected miss after user invalidation")
	}

	if err := store.Set(ctx, userID, session, perms, time.Minute); err != nil {
		t.Fatalf("set after user invalidation: %v", err)
	}
	if err := store.InvalidateAll(ctx); err != nil {
		t.Fatalf("invalidate all: %v", err)
	}
	_, ok, err = store.Get(ctx, userID, session)
	if err != nil {
		t.Fatalf("get after global invalidation: %v", err)
	}
	if ok {
		t.Fatal("expected miss after global invalidation")
	}
}

func TestRedisRBACPermissionCacheStoreMalformedEpochValue(t *testing.T) {
	ctx := context.Background()
	_, client := newRedisClientForTest(t)
	store := NewRedisRBACPermissionCacheStore(client, "rbac_test")

	if err := client.Set(ctx, store.globalEpochKey(), "NaN", time.Minute).Err(); err != nil {
		t.Fatalf("seed malformed epoch: %v", err)
	}

	if _, _, err := store.Get(ctx, 7, "sess"); err == nil {
		t.Fatal("expected parse error for malformed epoch")
	}
}
