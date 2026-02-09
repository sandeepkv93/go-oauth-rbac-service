package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
)

type CachedPermissionResolver struct {
	cacheStore RBACPermissionCacheStore
	userSvc    UserServiceInterface
	ttl        time.Duration
}

func NewCachedPermissionResolver(cacheStore RBACPermissionCacheStore, userSvc UserServiceInterface, ttl time.Duration) *CachedPermissionResolver {
	return &CachedPermissionResolver{
		cacheStore: cacheStore,
		userSvc:    userSvc,
		ttl:        ttl,
	}
}

func (r *CachedPermissionResolver) ResolvePermissions(ctx context.Context, claims *security.Claims) ([]string, error) {
	if claims == nil {
		return nil, fmt.Errorf("missing claims")
	}
	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid subject")
	}
	sessionTokenID := strings.TrimSpace(claims.ID)
	if sessionTokenID == "" {
		sessionTokenID = "none"
	}
	if r.cacheStore != nil && r.ttl > 0 {
		cached, ok, err := r.cacheStore.Get(ctx, uint(userID), sessionTokenID)
		if err == nil && ok {
			return cached, nil
		}
	}

	_, perms, err := r.userSvc.GetByID(uint(userID))
	if err != nil {
		return nil, err
	}
	if r.cacheStore != nil && r.ttl > 0 {
		_ = r.cacheStore.Set(ctx, uint(userID), sessionTokenID, perms, r.ttl)
	}
	return perms, nil
}

func (r *CachedPermissionResolver) InvalidateUser(ctx context.Context, userID uint) error {
	if r.cacheStore == nil {
		return nil
	}
	return r.cacheStore.InvalidateUser(ctx, userID)
}

func (r *CachedPermissionResolver) InvalidateAll(ctx context.Context) error {
	if r.cacheStore == nil {
		return nil
	}
	return r.cacheStore.InvalidateAll(ctx)
}

func buildRBACPermissionCacheKey(globalEpoch, userEpoch uint64, userID uint, sessionTokenID string) string {
	if sessionTokenID == "" {
		sessionTokenID = "none"
	}
	return fmt.Sprintf("rbacperm:g%d:u%d:user:%d:s:%s", globalEpoch, userEpoch, userID, sessionTokenID)
}
