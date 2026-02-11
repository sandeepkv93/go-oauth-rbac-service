package service

import (
	"context"
	"testing"
	"time"
)

func TestRedisAdminListCacheStoreNamespaceIndexAndInvalidateIdempotency(t *testing.T) {
	ctx := context.Background()
	_, client := newRedisClientForTest(t)
	store := NewRedisAdminListCacheStore(client, "admin_test")

	namespace := "admin.roles"
	if err := store.Set(ctx, namespace, "k1", []byte(`{"a":1}`), time.Minute); err != nil {
		t.Fatalf("set k1: %v", err)
	}
	if err := store.Set(ctx, namespace, "k2", []byte(`{"a":2}`), time.Minute); err != nil {
		t.Fatalf("set k2: %v", err)
	}

	members, err := client.SMembers(ctx, store.namespaceIndexKey(namespace)).Result()
	if err != nil {
		t.Fatalf("smembers namespace index: %v", err)
	}
	if len(members) != 4 {
		t.Fatalf("expected 4 namespace index members (2 data + 2 meta), got %d", len(members))
	}

	if err := store.InvalidateNamespace(ctx, namespace); err != nil {
		t.Fatalf("invalidate namespace first pass: %v", err)
	}
	if err := store.InvalidateNamespace(ctx, namespace); err != nil {
		t.Fatalf("invalidate namespace second pass should be idempotent: %v", err)
	}

	_, ok, err := store.Get(ctx, namespace, "k1")
	if err != nil {
		t.Fatalf("get after invalidate: %v", err)
	}
	if ok {
		t.Fatal("expected cache miss after invalidation")
	}
}

func TestRedisAdminListCacheStoreGetWithAgeMetaFallbacks(t *testing.T) {
	ctx := context.Background()
	_, client := newRedisClientForTest(t)
	store := NewRedisAdminListCacheStore(client, "admin_test")

	namespace := "admin.permissions"
	key := "k1"
	dataKey := store.dataKey(namespace, key)
	metaKey := store.metaKey(namespace, key)

	if err := client.Set(ctx, dataKey, []byte(`{"x":1}`), time.Minute).Err(); err != nil {
		t.Fatalf("seed data key: %v", err)
	}

	payload, ok, age, err := store.GetWithAge(ctx, namespace, key)
	if err != nil {
		t.Fatalf("get with missing meta should not error: %v", err)
	}
	if !ok {
		t.Fatal("expected hit with missing meta")
	}
	if string(payload) != `{"x":1}` {
		t.Fatalf("unexpected payload %s", string(payload))
	}
	if age != 0 {
		t.Fatalf("expected age 0 when meta missing, got %v", age)
	}

	if err := client.Set(ctx, metaKey, "not-a-number", time.Minute).Err(); err != nil {
		t.Fatalf("seed invalid meta: %v", err)
	}
	_, ok, age, err = store.GetWithAge(ctx, namespace, key)
	if err != nil {
		t.Fatalf("get with malformed meta should not error: %v", err)
	}
	if !ok {
		t.Fatal("expected hit with malformed meta")
	}
	if age != 0 {
		t.Fatalf("expected age 0 with malformed meta, got %v", age)
	}
}
