package loadgen

import "testing"

func TestClassifyStatusClass(t *testing.T) {
	cases := map[int]string{
		200: "2xx",
		302: "3xx",
		404: "4xx",
		500: "5xx",
		100: "other",
	}
	for status, want := range cases {
		if got := classifyStatusClass(status); got != want {
			t.Fatalf("classifyStatusClass(%d)=%q want %q", status, got, want)
		}
	}
}

func TestNormalizeProfile(t *testing.T) {
	if got := normalizeProfile(""); got != "mixed" {
		t.Fatalf("normalizeProfile empty=%q want mixed", got)
	}
	if got := normalizeProfile("  AUTH  "); got != "auth" {
		t.Fatalf("normalizeProfile auth=%q want auth", got)
	}
}
