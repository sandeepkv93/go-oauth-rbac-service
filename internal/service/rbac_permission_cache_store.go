package service

import (
	"context"
	"sync"
	"time"
)

type RBACPermissionCacheStore interface {
	Get(ctx context.Context, userID uint, sessionTokenID string) ([]string, bool, error)
	Set(ctx context.Context, userID uint, sessionTokenID string, permissions []string, ttl time.Duration) error
	InvalidateUser(ctx context.Context, userID uint) error
	InvalidateAll(ctx context.Context) error
}

type NoopRBACPermissionCacheStore struct{}

func NewNoopRBACPermissionCacheStore() *NoopRBACPermissionCacheStore {
	return &NoopRBACPermissionCacheStore{}
}

func (s *NoopRBACPermissionCacheStore) Get(context.Context, uint, string) ([]string, bool, error) {
	return nil, false, nil
}

func (s *NoopRBACPermissionCacheStore) Set(context.Context, uint, string, []string, time.Duration) error {
	return nil
}

func (s *NoopRBACPermissionCacheStore) InvalidateUser(context.Context, uint) error {
	return nil
}

func (s *NoopRBACPermissionCacheStore) InvalidateAll(context.Context) error {
	return nil
}

type rbacCacheEntry struct {
	permissions []string
	expiresAt   time.Time
}

type InMemoryRBACPermissionCacheStore struct {
	mu          sync.RWMutex
	data        map[string]rbacCacheEntry
	globalEpoch uint64
	userEpoch   map[uint]uint64
}

func NewInMemoryRBACPermissionCacheStore() *InMemoryRBACPermissionCacheStore {
	return &InMemoryRBACPermissionCacheStore{
		data:      make(map[string]rbacCacheEntry),
		userEpoch: make(map[uint]uint64),
	}
}

func (s *InMemoryRBACPermissionCacheStore) Get(_ context.Context, userID uint, sessionTokenID string) ([]string, bool, error) {
	now := time.Now().UTC()
	s.mu.RLock()
	key := s.cacheKeyLocked(userID, sessionTokenID)
	entry, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if now.After(entry.expiresAt) {
		s.mu.Lock()
		delete(s.data, key)
		s.mu.Unlock()
		return nil, false, nil
	}
	return append([]string(nil), entry.permissions...), true, nil
}

func (s *InMemoryRBACPermissionCacheStore) Set(_ context.Context, userID uint, sessionTokenID string, permissions []string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := s.cacheKeyLocked(userID, sessionTokenID)
	s.data[key] = rbacCacheEntry{
		permissions: append([]string(nil), permissions...),
		expiresAt:   time.Now().UTC().Add(ttl),
	}
	return nil
}

func (s *InMemoryRBACPermissionCacheStore) InvalidateUser(_ context.Context, userID uint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.userEpoch[userID]++
	return nil
}

func (s *InMemoryRBACPermissionCacheStore) InvalidateAll(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.globalEpoch++
	return nil
}

func (s *InMemoryRBACPermissionCacheStore) cacheKeyLocked(userID uint, sessionTokenID string) string {
	return buildRBACPermissionCacheKey(s.globalEpoch, s.userEpoch[userID], userID, sessionTokenID)
}
