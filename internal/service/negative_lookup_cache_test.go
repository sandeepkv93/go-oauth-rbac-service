package service

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryNegativeLookupCacheStoreGetSetInvalidate(t *testing.T) {
	store := NewInMemoryNegativeLookupCacheStore()
	ctx := context.Background()

	if err := store.Set(ctx, "admin.role.not_found", "42", time.Minute); err != nil {
		t.Fatalf("set negative cache: %v", err)
	}
	ok, err := store.Get(ctx, "admin.role.not_found", "42")
	if err != nil {
		t.Fatalf("get negative cache: %v", err)
	}
	if !ok {
		t.Fatal("expected negative cache hit")
	}

	if err := store.InvalidateNamespace(ctx, "admin.role.not_found"); err != nil {
		t.Fatalf("invalidate negative cache namespace: %v", err)
	}
	ok, err = store.Get(ctx, "admin.role.not_found", "42")
	if err != nil {
		t.Fatalf("get cache after invalidate: %v", err)
	}
	if ok {
		t.Fatal("expected negative cache miss after invalidate")
	}
}

func TestInMemoryNegativeLookupCacheStoreExpiry(t *testing.T) {
	store := NewInMemoryNegativeLookupCacheStore()
	ctx := context.Background()

	if err := store.Set(ctx, "admin.permission.not_found", "77", 25*time.Millisecond); err != nil {
		t.Fatalf("set negative cache: %v", err)
	}
	time.Sleep(40 * time.Millisecond)
	ok, err := store.Get(ctx, "admin.permission.not_found", "77")
	if err != nil {
		t.Fatalf("get negative cache: %v", err)
	}
	if ok {
		t.Fatal("expected negative cache entry to expire")
	}
}

func TestNoopNegativeLookupCacheStoreAlwaysMisses(t *testing.T) {
	store := NewNoopNegativeLookupCacheStore()
	ctx := context.Background()
	if err := store.Set(ctx, "admin.role.not_found", "404", time.Minute); err != nil {
		t.Fatalf("set noop negative cache: %v", err)
	}
	ok, err := store.Get(ctx, "admin.role.not_found", "404")
	if err != nil {
		t.Fatalf("get noop negative cache: %v", err)
	}
	if ok {
		t.Fatal("expected noop negative cache miss")
	}
	if err := store.InvalidateNamespace(ctx, "admin.role.not_found"); err != nil {
		t.Fatalf("invalidate noop negative cache namespace: %v", err)
	}
}
