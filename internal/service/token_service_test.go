package service

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/domain"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/repository"
	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/security"
)

type inMemorySessionRepo struct {
	mu      sync.Mutex
	nextID  uint
	byHash  map[string]*domain.Session
	byID    map[uint]*domain.Session
	byToken map[string]*domain.Session
}

func newInMemorySessionRepo() *inMemorySessionRepo {
	return &inMemorySessionRepo{
		nextID:  1,
		byHash:  map[string]*domain.Session{},
		byID:    map[uint]*domain.Session{},
		byToken: map[string]*domain.Session{},
	}
}

func (r *inMemorySessionRepo) Create(s *domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	copy := *s
	copy.ID = r.nextID
	r.nextID++
	r.byHash[copy.RefreshTokenHash] = &copy
	r.byID[copy.ID] = &copy
	if copy.TokenID != nil {
		r.byToken[*copy.TokenID] = &copy
	}
	return nil
}

func (r *inMemorySessionRepo) FindByHash(hash string) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byHash[hash]
	if !ok {
		return nil, repository.ErrSessionNotFound
	}
	cp := *s
	return &cp, nil
}

func (r *inMemorySessionRepo) RotateSession(oldHash string, newSession *domain.Session) (*domain.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	old, ok := r.byHash[oldHash]
	if !ok || old.RevokedAt != nil || old.ExpiresAt.Before(time.Now()) {
		return nil, repository.ErrSessionNotFound
	}
	now := time.Now().UTC()
	reason := "rotated"
	old.RevokedAt = &now
	old.RevokedReason = &reason

	copy := *newSession
	copy.ID = r.nextID
	r.nextID++
	r.byHash[copy.RefreshTokenHash] = &copy
	r.byID[copy.ID] = &copy
	if copy.TokenID != nil {
		r.byToken[*copy.TokenID] = &copy
	}

	oc := *old
	return &oc, nil
}

func (r *inMemorySessionRepo) UpdateTokenLineageByHash(hash, tokenID, familyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byHash[hash]
	if !ok {
		return repository.ErrSessionNotFound
	}
	if s.TokenID == nil || *s.TokenID == "" {
		s.TokenID = strPtr(tokenID)
	}
	if s.FamilyID == nil || *s.FamilyID == "" {
		s.FamilyID = strPtr(familyID)
	}
	return nil
}

func (r *inMemorySessionRepo) MarkReuseDetectedByHash(hash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byHash[hash]
	if !ok {
		return repository.ErrSessionNotFound
	}
	now := time.Now().UTC()
	reason := "reuse_detected"
	s.ReuseDetectedAt = &now
	s.RevokedReason = &reason
	return nil
}

func (r *inMemorySessionRepo) RevokeByHash(hash, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.byHash[hash]
	if !ok {
		return nil
	}
	if s.RevokedAt == nil {
		now := time.Now().UTC()
		s.RevokedAt = &now
	}
	s.RevokedReason = strPtr(reason)
	return nil
}

func (r *inMemorySessionRepo) RevokeByFamilyID(familyID, reason string) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var count int64
	for _, s := range r.byID {
		if s.FamilyID == nil || *s.FamilyID != familyID || s.RevokedAt != nil {
			continue
		}
		now := time.Now().UTC()
		s.RevokedAt = &now
		s.RevokedReason = strPtr(reason)
		count++
	}
	return count, nil
}

func (r *inMemorySessionRepo) RevokeByUserID(userID uint, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.byID {
		if s.UserID != userID || s.RevokedAt != nil {
			continue
		}
		now := time.Now().UTC()
		s.RevokedAt = &now
		s.RevokedReason = strPtr(reason)
	}
	return nil
}

func (r *inMemorySessionRepo) CleanupExpired() (int64, error) { return 0, nil }

func TestTokenRotateSuccessPreservesFamily(t *testing.T) {
	repo := newInMemorySessionRepo()
	svc := newTestTokenService(repo)
	user := testUser()

	_, refreshA, _, err := svc.Issue(user, []string{"users:read"}, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	claimsA, err := svc.jwtMgr.ParseRefreshToken(refreshA)
	if err != nil {
		t.Fatalf("parse refreshA: %v", err)
	}
	hashA := security.HashRefreshToken(refreshA, svc.pepper)

	_, refreshB, _, _, err := svc.Rotate(refreshA, testFetcher(user), "ua2", "127.0.0.2")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}

	sA, err := repo.FindByHash(hashA)
	if err != nil {
		t.Fatalf("find old session: %v", err)
	}
	if sA.RevokedAt == nil || sA.RevokedReason == nil || *sA.RevokedReason != "rotated" {
		t.Fatal("expected old session revoked with reason rotated")
	}

	hashB := security.HashRefreshToken(refreshB, svc.pepper)
	sB, err := repo.FindByHash(hashB)
	if err != nil {
		t.Fatalf("find new session: %v", err)
	}
	if sB.ParentTokenID == nil || *sB.ParentTokenID != claimsA.ID {
		t.Fatal("expected parent_token_id to point to old token jti")
	}
	if sB.FamilyID == nil || *sB.FamilyID != claimsA.ID {
		t.Fatal("expected family_id to be preserved")
	}
}

func TestTokenRotateReuseRevokesFamily(t *testing.T) {
	repo := newInMemorySessionRepo()
	svc := newTestTokenService(repo)
	user := testUser()

	_, refreshA, _, err := svc.Issue(user, []string{"users:read"}, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	_, refreshB, _, _, err := svc.Rotate(refreshA, testFetcher(user), "ua2", "127.0.0.2")
	if err != nil {
		t.Fatalf("first rotate: %v", err)
	}

	_, _, _, _, err = svc.Rotate(refreshA, testFetcher(user), "ua3", "127.0.0.3")
	if !errors.Is(err, ErrRefreshTokenReuseDetected) {
		t.Fatalf("expected reuse detection error, got: %v", err)
	}

	_, _, _, _, err = svc.Rotate(refreshB, testFetcher(user), "ua4", "127.0.0.4")
	if err == nil {
		t.Fatal("expected family token to fail after reuse")
	}
}

func TestTokenRotateInvalidDoesNotRevokeActiveSessions(t *testing.T) {
	repo := newInMemorySessionRepo()
	svc := newTestTokenService(repo)
	user := testUser()

	_, refreshA, _, err := svc.Issue(user, []string{"users:read"}, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	hashA := security.HashRefreshToken(refreshA, svc.pepper)

	_, _, _, _, err = svc.Rotate("not-a-valid-token", testFetcher(user), "ua", "127.0.0.1")
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("expected ErrInvalidRefreshToken, got %v", err)
	}

	sA, err := repo.FindByHash(hashA)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	if sA.RevokedAt != nil {
		t.Fatal("expected active session to remain active for malformed token")
	}
}

func TestTokenRotateBackfillsLegacyLineage(t *testing.T) {
	repo := newInMemorySessionRepo()
	svc := newTestTokenService(repo)
	user := testUser()

	_, refreshA, _, err := svc.Issue(user, []string{"users:read"}, "ua", "127.0.0.1")
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	hashA := security.HashRefreshToken(refreshA, svc.pepper)
	sA, err := repo.FindByHash(hashA)
	if err != nil {
		t.Fatalf("find session: %v", err)
	}
	// simulate legacy row without lineage metadata
	sA.TokenID = nil
	sA.FamilyID = nil
	repo.byHash[hashA] = sA
	repo.byID[sA.ID] = sA

	_, _, _, _, err = svc.Rotate(refreshA, testFetcher(user), "ua2", "127.0.0.2")
	if err != nil {
		t.Fatalf("rotate legacy: %v", err)
	}

	updated, err := repo.FindByHash(hashA)
	if err != nil {
		t.Fatalf("find updated: %v", err)
	}
	if updated.TokenID == nil || updated.FamilyID == nil {
		t.Fatal("expected legacy lineage fields to be backfilled")
	}
}

func newTestTokenService(repo repository.SessionRepository) *TokenService {
	jwtMgr := security.NewJWTManager(
		"iss",
		"aud",
		"abcdefghijklmnopqrstuvwxyz123456",
		"abcdefghijklmnopqrstuvwxyz654321",
	)
	return NewTokenService(jwtMgr, repo, "pepper-1234567890", 15*time.Minute, 24*time.Hour)
}

func testUser() *domain.User {
	return &domain.User{
		ID:    42,
		Email: "test@example.com",
		Name:  "Test",
		Roles: []domain.Role{{Name: "user"}},
	}
}

func testFetcher(user *domain.User) func(id uint) (*domain.User, []string, error) {
	return func(id uint) (*domain.User, []string, error) {
		if id != user.ID {
			return nil, nil, errors.New("not found")
		}
		return user, []string{"users:read"}, nil
	}
}

func strPtr(v string) *string { return &v }
