package config

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestClassifyConfigLoadError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "none", err: nil, want: "none"},
		{name: "validation", err: errors.New("validate config: DATABASE_URL is required"), want: "validation"},
		{name: "parse", err: errors.New("parse JWT_ACCESS_TTL: invalid duration"), want: "parse"},
		{name: "other", err: errors.New("some other load error"), want: "load"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyConfigLoadError(tc.err); got != tc.want {
				t.Fatalf("classifyConfigLoadError()=%q want %q", got, tc.want)
			}
		})
	}
}

func TestNormalizeConfigProfile(t *testing.T) {
	if got := normalizeConfigProfile("  ProD  "); got != "prod" {
		t.Fatalf("expected prod, got %q", got)
	}
	if got := normalizeConfigProfile("   "); got != "unknown" {
		t.Fatalf("expected unknown, got %q", got)
	}
}

func FuzzNormalizeConfigProfileRobustness(f *testing.F) {
	f.Add("  ProD  ")
	f.Add("   ")
	f.Add("")
	f.Add("🔥PROD🔥")
	f.Add(strings.Repeat("A", 4096))

	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 8192 {
			raw = raw[:8192]
		}

		got := normalizeConfigProfile(raw)
		if got == "" {
			t.Fatal("normalized profile must not be empty")
		}
		if strings.TrimSpace(raw) == "" && got != "unknown" {
			t.Fatalf("expected unknown for empty/whitespace input, got %q", got)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("normalized profile must be valid UTF-8: %q", got)
		}

		again := normalizeConfigProfile(raw)
		if got != again {
			t.Fatalf("normalizeConfigProfile must be deterministic: first=%q second=%q", got, again)
		}
	})
}
