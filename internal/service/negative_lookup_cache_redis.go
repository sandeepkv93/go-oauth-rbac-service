package service

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisNegativeLookupCacheStore struct {
	client redis.UniversalClient
	prefix string
}

func NewRedisNegativeLookupCacheStore(client redis.UniversalClient, prefix string) *RedisNegativeLookupCacheStore {
	if prefix == "" {
		prefix = "negative_lookup_cache"
	}
	return &RedisNegativeLookupCacheStore{
		client: client,
		prefix: prefix,
	}
}

func (s *RedisNegativeLookupCacheStore) Get(ctx context.Context, namespace, key string) (bool, error) {
	if s.client == nil {
		return false, nil
	}
	_, err := s.client.Get(ctx, s.dataKey(namespace, key)).Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *RedisNegativeLookupCacheStore) Set(ctx context.Context, namespace, key string, ttl time.Duration) error {
	if s.client == nil || ttl <= 0 {
		return nil
	}
	dataKey := s.dataKey(namespace, key)
	namespaceIndex := s.namespaceIndexKey(namespace)
	pipe := s.client.TxPipeline()
	pipe.Set(ctx, dataKey, "1", ttl)
	pipe.SAdd(ctx, namespaceIndex, dataKey)
	pipe.Expire(ctx, namespaceIndex, ttl+time.Minute)
	_, err := pipe.Exec(ctx)
	return err
}

func (s *RedisNegativeLookupCacheStore) InvalidateNamespace(ctx context.Context, namespace string) error {
	if s.client == nil {
		return nil
	}
	namespaceIndex := s.namespaceIndexKey(namespace)
	keys, err := s.client.SMembers(ctx, namespaceIndex).Result()
	if err != nil && err != redis.Nil {
		return err
	}
	pipe := s.client.TxPipeline()
	if len(keys) > 0 {
		pipe.Del(ctx, keys...)
	}
	pipe.Del(ctx, namespaceIndex)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *RedisNegativeLookupCacheStore) dataKey(namespace, key string) string {
	return fmt.Sprintf("%s:data:%s:%s", s.prefix, normalizeToken(namespace), hashToken(key))
}

func (s *RedisNegativeLookupCacheStore) namespaceIndexKey(namespace string) string {
	return fmt.Sprintf("%s:index:%s", s.prefix, normalizeToken(namespace))
}
