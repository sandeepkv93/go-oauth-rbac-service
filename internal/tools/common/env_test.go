package common

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadEnvFileMissingIsNoop(t *testing.T) {
	if err := LoadEnvFile(filepath.Join(t.TempDir(), "missing.env")); err != nil {
		t.Fatalf("missing env file should be ignored: %v", err)
	}
}

func TestLoadEnvFileLoadsAndPreservesExisting(t *testing.T) {
	t.Setenv("EXISTING_KEY", "from-env")
	file := filepath.Join(t.TempDir(), "test.env")
	content := "# comment\nEXISTING_KEY=from-file\nNEW_KEY=hello\nQUOTED=\"x\"\nINVALID_LINE\n"
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatalf("write env file: %v", err)
	}

	if err := LoadEnvFile(file); err != nil {
		t.Fatalf("load env file: %v", err)
	}
	if got := os.Getenv("EXISTING_KEY"); got != "from-env" {
		t.Fatalf("expected existing var to be preserved, got %q", got)
	}
	if got := os.Getenv("NEW_KEY"); got != "hello" {
		t.Fatalf("unexpected NEW_KEY=%q", got)
	}
	if got := os.Getenv("QUOTED"); got != "x" {
		t.Fatalf("unexpected QUOTED=%q", got)
	}
}

func TestLoadEnvFileOpenError(t *testing.T) {
	dir := t.TempDir()
	if err := LoadEnvFile(dir); err == nil {
		t.Fatal("expected error when path is a directory")
	}
}

func FuzzLoadEnvFileRobustness(f *testing.F) {
	f.Add([]byte("KEY=value\nANOTHER=ok\n"))
	f.Add([]byte("INVALID_LINE\n# comment\n QUOTED = \"x\" \n"))
	f.Add([]byte("UNICODE_🔥=こんにちは\n"))
	f.Add([]byte("NO_EQUALS_LINE\nBROKEN"))
	f.Add(bytes.Repeat([]byte("A"), 70000))

	f.Fuzz(func(t *testing.T, content []byte) {
		if len(content) > 200000 {
			content = content[:200000]
		}

		dir := t.TempDir()
		file := filepath.Join(dir, "fuzz.env")
		if err := os.WriteFile(file, content, 0o600); err != nil {
			t.Fatalf("write env file: %v", err)
		}

		classify := func(err error) string {
			if err == nil {
				return "none"
			}
			msg := err.Error()
			switch {
			case strings.Contains(msg, "open env file:"):
				return "open"
			case strings.Contains(msg, "read env file:"):
				return "read"
			default:
				return "other"
			}
		}

		err1 := LoadEnvFile(file)
		err2 := LoadEnvFile(file)
		c1 := classify(err1)
		c2 := classify(err2)
		if c1 != c2 {
			t.Fatalf("error classification must be deterministic: first=%q second=%q err1=%v err2=%v", c1, c2, err1, err2)
		}
		if c1 == "other" {
			t.Fatalf("unexpected error class: err1=%v err2=%v", err1, err2)
		}
	})
}
