package config

import (
	"errors"
	"testing"
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
