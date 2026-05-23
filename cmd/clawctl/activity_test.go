package main

import (
	"testing"
	"time"
)

func TestParseDockerLogTimestamp_Full(t *testing.T) {
	line := "openclaw-gateway-1 | 2026-03-17T10:30:45.123456789Z ERROR: something failed"
	ts := parseDockerLogTimestamp(line)
	if ts.Year() != 2026 || ts.Month() != 3 || ts.Day() != 17 {
		t.Errorf("expected 2026-03-17, got %v", ts)
	}
	if ts.Hour() != 10 || ts.Minute() != 30 {
		t.Errorf("expected 10:30, got %d:%d", ts.Hour(), ts.Minute())
	}
}

func TestParseDockerLogTimestamp_NoPipe(t *testing.T) {
	line := "2026-01-15T08:00:00Z fatal error in module"
	ts := parseDockerLogTimestamp(line)
	if ts.Year() != 2026 || ts.Month() != 1 || ts.Day() != 15 {
		t.Errorf("expected 2026-01-15, got %v", ts)
	}
}

func TestParseDockerLogTimestamp_NoTimestamp(t *testing.T) {
	line := "just a regular error line with no timestamp"
	ts := parseDockerLogTimestamp(line)
	// Should fallback to approximately now
	if time.Since(ts) > 2*time.Second {
		t.Errorf("fallback should be close to now, got %v", ts)
	}
}

func TestParseDockerLogTimestamp_RFC3339(t *testing.T) {
	line := "container | 2026-06-01T14:30:00+00:00 crash detected"
	ts := parseDockerLogTimestamp(line)
	if ts.Year() != 2026 || ts.Month() != 6 {
		t.Errorf("expected 2026-06, got %v", ts)
	}
}

func TestStripLogPrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"openclaw-gateway-1 | 2026-03-17T10:30:45Z error msg", "2026-03-17T10:30:45Z error msg"},
		{"error msg without pipe", "error msg without pipe"},
		{"  container | spaced  ", "spaced"},
	}
	for _, tc := range tests {
		got := stripLogPrefix(tc.input)
		if got != tc.expected {
			t.Errorf("stripLogPrefix(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 80); got != "short" {
		t.Errorf("short string should not be truncated, got %q", got)
	}
	long := "this is a very long string that exceeds the maximum length limit that we set"
	got := truncate(long, 30)
	if len(got) != 30 {
		t.Errorf("truncated length should be 30, got %d", len(got))
	}
	if got[27:] != "..." {
		t.Errorf("truncated string should end with ..., got %q", got)
	}
}
