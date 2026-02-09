package service

import (
	"context"
	"sync"
	"time"
)

type NegativeLookupCacheStore interface {
	Get(ctx context.Context, namespace, key string) (bool, error)
	Set(ctx context.Context, namespace, key string, ttl time.Duration) error
	InvalidateNamespace(ctx context.Context, namespace string) error
}

type NoopNegativeLookupCacheStore struct{}

func NewNoopNegativeLookupCacheStore() *NoopNegativeLookupCacheStore {
	return &NoopNegativeLookupCacheStore{}
}

func (s *NoopNegativeLookupCacheStore) Get(context.Context, string, string) (bool, error) {
	return false, nil
}

func (s *NoopNegativeLookupCacheStore) Set(context.Context, string, string, time.Duration) error {
	return nil
}

func (s *NoopNegativeLookupCacheStore) InvalidateNamespace(context.Context, string) error {
	return nil
}

type InMemoryNegativeLookupCacheStore struct {
	mu    sync.RWMutex
	store map[string]map[string]time.Time
}

func NewInMemoryNegativeLookupCacheStore() *InMemoryNegativeLookupCacheStore {
	return &InMemoryNegativeLookupCacheStore{
		store: make(map[string]map[string]time.Time),
	}
}

func (s *InMemoryNegativeLookupCacheStore) Get(_ context.Context, namespace, key string) (bool, error) {
	now := time.Now().UTC()
	s.mu.RLock()
	ns, ok := s.store[namespace]
	if !ok {
		s.mu.RUnlock()
		return false, nil
	}
	expiresAt, ok := ns[key]
	s.mu.RUnlock()
	if !ok {
		return false, nil
	}
	if now.After(expiresAt) {
		s.mu.Lock()
		if ns2, ok2 := s.store[namespace]; ok2 {
			delete(ns2, key)
			if len(ns2) == 0 {
				delete(s.store, namespace)
			}
		}
		s.mu.Unlock()
		return false, nil
	}
	return true, nil
}

func (s *InMemoryNegativeLookupCacheStore) Set(_ context.Context, namespace, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ns, ok := s.store[namespace]
	if !ok {
		ns = make(map[string]time.Time)
		s.store[namespace] = ns
	}
	ns[key] = time.Now().UTC().Add(ttl)
	return nil
}

func (s *InMemoryNegativeLookupCacheStore) InvalidateNamespace(_ context.Context, namespace string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.store, namespace)
	return nil
}
