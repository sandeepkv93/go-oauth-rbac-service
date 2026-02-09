package service

import (
	"context"
	"testing"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/domain"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
)

type stubUserService struct {
	perms []string
	calls int
}

func (s *stubUserService) GetByID(id uint) (*domain.User, []string, error) {
	s.calls++
	return &domain.User{ID: id}, append([]string(nil), s.perms...), nil
}

func (s *stubUserService) List() ([]domain.User, error) {
	return nil, nil
}

func (s *stubUserService) SetRoles(uint, []uint) error {
	return nil
}

func TestCachedPermissionResolverCachesBySession(t *testing.T) {
	store := NewInMemoryRBACPermissionCacheStore()
	userSvc := &stubUserService{perms: []string{"users:read"}}
	resolver := NewCachedPermissionResolver(store, userSvc, time.Minute)

	claims := &security.Claims{}
	claims.Subject = "42"
	claims.ID = "jti-1"

	perms, err := resolver.ResolvePermissions(context.Background(), claims)
	if err != nil {
		t.Fatalf("resolve permissions first call: %v", err)
	}
	if len(perms) != 1 || perms[0] != "users:read" {
		t.Fatalf("unexpected perms: %+v", perms)
	}
	if userSvc.calls != 1 {
		t.Fatalf("expected one user service call, got %d", userSvc.calls)
	}

	perms, err = resolver.ResolvePermissions(context.Background(), claims)
	if err != nil {
		t.Fatalf("resolve permissions second call: %v", err)
	}
	if len(perms) != 1 || perms[0] != "users:read" {
		t.Fatalf("unexpected perms second call: %+v", perms)
	}
	if userSvc.calls != 1 {
		t.Fatalf("expected cache hit and unchanged user service calls, got %d", userSvc.calls)
	}
}

func TestCachedPermissionResolverInvalidateUser(t *testing.T) {
	store := NewInMemoryRBACPermissionCacheStore()
	userSvc := &stubUserService{perms: []string{"roles:read"}}
	resolver := NewCachedPermissionResolver(store, userSvc, time.Minute)

	claims := &security.Claims{}
	claims.Subject = "7"
	claims.ID = "jti-x"

	if _, err := resolver.ResolvePermissions(context.Background(), claims); err != nil {
		t.Fatalf("resolve permissions: %v", err)
	}
	if err := resolver.InvalidateUser(context.Background(), 7); err != nil {
		t.Fatalf("invalidate user: %v", err)
	}
	if _, err := resolver.ResolvePermissions(context.Background(), claims); err != nil {
		t.Fatalf("resolve permissions after invalidate: %v", err)
	}
	if userSvc.calls != 2 {
		t.Fatalf("expected cache miss after invalidate, got user service calls=%d", userSvc.calls)
	}
}
