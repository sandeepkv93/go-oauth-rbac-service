package repository

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sandeepkv93/secure-observable-go-backend-starter-kit/internal/domain"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSessionRepositoryListActiveByUserID(t *testing.T) {
	repo := newSessionRepoForTest(t)

	active := &domain.Session{
		UserID:           1,
		RefreshTokenHash: "h1",
		TokenID:          strPtr("tok-1"),
		FamilyID:         strPtr("fam-1"),
		ExpiresAt:        time.Now().Add(2 * time.Hour),
	}
	revokedAt := time.Now().UTC()
	revoked := &domain.Session{
		UserID:           1,
		RefreshTokenHash: "h2",
		TokenID:          strPtr("tok-2"),
		FamilyID:         strPtr("fam-2"),
		ExpiresAt:        time.Now().Add(2 * time.Hour),
		RevokedAt:        &revokedAt,
	}
	expired := &domain.Session{
		UserID:           1,
		RefreshTokenHash: "h3",
		TokenID:          strPtr("tok-3"),
		FamilyID:         strPtr("fam-3"),
		ExpiresAt:        time.Now().Add(-time.Hour),
	}
	otherUser := &domain.Session{
		UserID:           2,
		RefreshTokenHash: "h4",
		TokenID:          strPtr("tok-4"),
		FamilyID:         strPtr("fam-4"),
		ExpiresAt:        time.Now().Add(2 * time.Hour),
	}

	if err := repo.Create(active); err != nil {
		t.Fatalf("create active: %v", err)
	}
	if err := repo.Create(revoked); err != nil {
		t.Fatalf("create revoked: %v", err)
	}
	if err := repo.Create(expired); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	if err := repo.Create(otherUser); err != nil {
		t.Fatalf("create other user: %v", err)
	}

	sessions, err := repo.ListActiveByUserID(1)
	if err != nil {
		t.Fatalf("list active: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 active session, got %d", len(sessions))
	}
	if sessions[0].RefreshTokenHash != "h1" {
		t.Fatalf("unexpected active session: %+v", sessions[0])
	}
}

func TestSessionRepositoryRevokeScopeByUser(t *testing.T) {
	repo := newSessionRepoForTest(t)

	s1 := &domain.Session{
		UserID:           1,
		RefreshTokenHash: "u1s1",
		TokenID:          strPtr("tok-u1s1"),
		FamilyID:         strPtr("fam-u1"),
		ExpiresAt:        time.Now().Add(2 * time.Hour),
	}
	s2 := &domain.Session{
		UserID:           2,
		RefreshTokenHash: "u2s1",
		TokenID:          strPtr("tok-u2s1"),
		FamilyID:         strPtr("fam-u2"),
		ExpiresAt:        time.Now().Add(2 * time.Hour),
	}

	if err := repo.Create(s1); err != nil {
		t.Fatalf("create s1: %v", err)
	}
	if err := repo.Create(s2); err != nil {
		t.Fatalf("create s2: %v", err)
	}

	if _, err := repo.RevokeByIDForUser(1, s2.ID, "manual"); err == nil {
		t.Fatal("expected not found when revoking another user's session")
	}

	changed, err := repo.RevokeByIDForUser(2, s2.ID, "manual")
	if err != nil {
		t.Fatalf("revoke owned session: %v", err)
	}
	if !changed {
		t.Fatal("expected changed=true on first revoke")
	}

	changed, err = repo.RevokeByIDForUser(2, s2.ID, "manual")
	if err != nil {
		t.Fatalf("idempotent revoke: %v", err)
	}
	if changed {
		t.Fatal("expected changed=false on already revoked session")
	}

	revokedCount, err := repo.RevokeOthersByUser(1, s1.ID, "revoke_others")
	if err != nil {
		t.Fatalf("revoke others: %v", err)
	}
	if revokedCount != 0 {
		t.Fatalf("expected 0 revoked for user 1 with one kept session, got %d", revokedCount)
	}

	if _, err := repo.FindByIDForUser(1, s1.ID); err != nil {
		t.Fatalf("find own session: %v", err)
	}
}

func newSessionRepoForTest(t *testing.T) SessionRepository {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&domain.Session{}); err != nil {
		t.Fatalf("migrate session: %v", err)
	}
	return NewSessionRepository(db)
}

func strPtr(v string) *string { return &v }
