package service

import (
	"context"
	"testing"
	"time"
)

func TestRedisAuthAbuseGuardCooldownGrowthResetAndIsolation(t *testing.T) {
	ctx := context.Background()
	_, client := newRedisClientForTest(t)
	policy := AuthAbusePolicy{
		FreeAttempts: 1,
		BaseDelay:    50 * time.Millisecond,
		Multiplier:   2,
		MaxDelay:     500 * time.Millisecond,
		ResetWindow:  time.Second,
	}
	guard := NewRedisAuthAbuseGuard(client, "abuse_test", policy)

	d1, err := guard.RegisterFailure(ctx, AuthAbuseScopeLogin, "u1@example.com", "10.0.0.1")
	if err != nil {
		t.Fatalf("register first failure: %v", err)
	}
	if d1 != 0 {
		t.Fatalf("expected no cooldown for first free attempt, got %v", d1)
	}

	d2, err := guard.RegisterFailure(ctx, AuthAbuseScopeLogin, "u1@example.com", "10.0.0.1")
	if err != nil {
		t.Fatalf("register second failure: %v", err)
	}
	if d2 <= 0 {
		t.Fatalf("expected cooldown after second failure, got %v", d2)
	}

	d3, err := guard.RegisterFailure(ctx, AuthAbuseScopeLogin, "u1@example.com", "10.0.0.1")
	if err != nil {
		t.Fatalf("register third failure: %v", err)
	}
	if d3 < d2 {
		t.Fatalf("expected non-decreasing cooldown, second=%v third=%v", d2, d3)
	}

	cooldown, err := guard.Check(ctx, AuthAbuseScopeLogin, "u1@example.com", "10.0.0.1")
	if err != nil {
		t.Fatalf("check cooldown: %v", err)
	}
	if cooldown <= 0 {
		t.Fatalf("expected active cooldown, got %v", cooldown)
	}

	otherCooldown, err := guard.Check(ctx, AuthAbuseScopeLogin, "u2@example.com", "10.0.0.2")
	if err != nil {
		t.Fatalf("check isolated subject/ip: %v", err)
	}
	if otherCooldown != 0 {
		t.Fatalf("expected isolated identity/ip to remain unaffected, got %v", otherCooldown)
	}

	if err := guard.Reset(ctx, AuthAbuseScopeLogin, "u1@example.com", "10.0.0.1"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	cooldown, err = guard.Check(ctx, AuthAbuseScopeLogin, "u1@example.com", "10.0.0.1")
	if err != nil {
		t.Fatalf("check after reset: %v", err)
	}
	if cooldown != 0 {
		t.Fatalf("expected no cooldown after reset, got %v", cooldown)
	}
}

func TestRedisAuthAbuseGuardMalformedRedisValue(t *testing.T) {
	ctx := context.Background()
	_, client := newRedisClientForTest(t)
	guard := NewRedisAuthAbuseGuard(client, "abuse_test", AuthAbusePolicy{})

	key := guard.stateKey(AuthAbuseScopeForgot, "id", normalizeAuthIdentity("broken@example.com"))
	if err := client.HSet(ctx, key, "last_failure_ms", "bad", "cooldown_until_ms", "still-bad").Err(); err != nil {
		t.Fatalf("seed malformed hash: %v", err)
	}

	if _, err := guard.Check(ctx, AuthAbuseScopeForgot, "broken@example.com", ""); err == nil {
		t.Fatal("expected error for malformed redis hash values")
	}
}
