package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisRBACPermissionCacheStore struct {
	client redis.UniversalClient
	prefix string
}

func NewRedisRBACPermissionCacheStore(client redis.UniversalClient, prefix string) *RedisRBACPermissionCacheStore {
	if prefix == "" {
		prefix = "rbac_perm"
	}
	return &RedisRBACPermissionCacheStore{
		client: client,
		prefix: prefix,
	}
}

func (s *RedisRBACPermissionCacheStore) Get(ctx context.Context, userID uint, sessionTokenID string) ([]string, bool, error) {
	if s.client == nil {
		return nil, false, nil
	}
	key, err := s.dataKey(ctx, userID, sessionTokenID)
	if err != nil {
		return nil, false, err
	}
	raw, err := s.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var perms []string
	if err := json.Unmarshal(raw, &perms); err != nil {
		return nil, false, err
	}
	return perms, true, nil
}

func (s *RedisRBACPermissionCacheStore) Set(ctx context.Context, userID uint, sessionTokenID string, permissions []string, ttl time.Duration) error {
	if s.client == nil || ttl <= 0 {
		return nil
	}
	key, err := s.dataKey(ctx, userID, sessionTokenID)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(permissions)
	if err != nil {
		return err
	}
	return s.client.Set(ctx, key, payload, ttl).Err()
}

func (s *RedisRBACPermissionCacheStore) InvalidateUser(ctx context.Context, userID uint) error {
	if s.client == nil {
		return nil
	}
	return s.client.Incr(ctx, s.userEpochKey(userID)).Err()
}

func (s *RedisRBACPermissionCacheStore) InvalidateAll(ctx context.Context) error {
	if s.client == nil {
		return nil
	}
	return s.client.Incr(ctx, s.globalEpochKey()).Err()
}

func (s *RedisRBACPermissionCacheStore) dataKey(ctx context.Context, userID uint, sessionTokenID string) (string, error) {
	pipe := s.client.Pipeline()
	globalEpochCmd := pipe.Get(ctx, s.globalEpochKey())
	userEpochCmd := pipe.Get(ctx, s.userEpochKey(userID))
	_, err := pipe.Exec(ctx)
	if err != nil && err != redis.Nil {
		return "", err
	}
	globalEpoch, err := parseEpoch(globalEpochCmd)
	if err != nil {
		return "", err
	}
	userEpoch, err := parseEpoch(userEpochCmd)
	if err != nil {
		return "", err
	}
	return buildRBACPermissionCacheKey(globalEpoch, userEpoch, userID, sessionTokenID), nil
}

func parseEpoch(cmd *redis.StringCmd) (uint64, error) {
	v, err := cmd.Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if v == "" {
		return 0, nil
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *RedisRBACPermissionCacheStore) globalEpochKey() string {
	return s.prefix + ":epoch:global"
}

func (s *RedisRBACPermissionCacheStore) userEpochKey(userID uint) string {
	return fmt.Sprintf("%s:epoch:user:%d", s.prefix, userID)
}
