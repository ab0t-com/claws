package main

import (
	"strings"
	"testing"
	"time"
)

func TestHumanAgo(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want string
	}{
		{45 * time.Second, "45s ago"},
		{90 * time.Second, "1m ago"},
		{45 * time.Minute, "45m ago"},
		{2*time.Hour + 30*time.Minute, "2h ago"},
		{49 * time.Hour, "2d ago"},
	}
	for _, c := range cases {
		if got := humanAgo(c.in); got != c.want {
			t.Errorf("humanAgo(%v)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestCompact(t *testing.T) {
	long := strings.Repeat("x", 100)
	got := compact(long, 20)
	// compact truncates at byte index max-1 then appends "…" (3 bytes
	// UTF-8). We care that the truncation kicked in and the trailing
	// rune is the ellipsis — not the exact byte count.
	if !strings.HasSuffix(got, "…") {
		t.Errorf("compact should end with ellipsis, got %q", got)
	}
	if strings.Count(got, "x") != 19 {
		t.Errorf("compact should keep 19 x's, got %d", strings.Count(got, "x"))
	}
	if got := compact("line1\nline2", 50); strings.Contains(got, "\n") {
		t.Errorf("compact should strip newlines, got %q", got)
	}
}

func TestDetectRisks_BunchedAuth(t *testing.T) {
	now := time.Now()
	rows := []authDiagRow{
		{Name: "team/sarah", Provider: "openai-codex", LastAuthAt: now},
		{Name: "team/john", Provider: "openai-codex", LastAuthAt: now.Add(2 * time.Minute)},
		{Name: "team/lead", Provider: "openai-codex", LastAuthAt: now.Add(5 * time.Minute)},
	}
	out := detectRisks(rows)
	combined := strings.Join(out, "\n")
	if !strings.Contains(combined, "3 agents authed within") {
		t.Errorf("expected bunched-auth warning, got:\n%s", combined)
	}
	if !strings.Contains(combined, "openai-codex") {
		t.Errorf("expected provider in warning, got:\n%s", combined)
	}
	if !strings.Contains(combined, "claws auth fleet codex") {
		t.Errorf("expected remediation, got:\n%s", combined)
	}
}

func TestDetectRisks_NotBunched(t *testing.T) {
	now := time.Now()
	rows := []authDiagRow{
		{Name: "team/sarah", Provider: "openai-codex", LastAuthAt: now.Add(-30 * 24 * time.Hour)},
		{Name: "team/john", Provider: "openai-codex", LastAuthAt: now},
	}
	out := detectRisks(rows)
	combined := strings.Join(out, "\n")
	if strings.Contains(combined, "authed within") {
		t.Errorf("auth events 30 days apart should NOT trigger bunched-auth, got:\n%s", combined)
	}
}

func TestDetectRisks_ReuseClassConfirmed(t *testing.T) {
	rows := []authDiagRow{
		{Name: "team/sarah", Provider: "openai-codex", VerifyState: "failed",
			VerifyDetail: "refresh_token_reused"},
		{Name: "team/john", Provider: "openai-codex", VerifyState: "failed",
			VerifyDetail: "Token refresh failed: 401"},
	}
	out := detectRisks(rows)
	combined := strings.Join(out, "\n")
	if !strings.Contains(combined, "failing with refresh-token-reuse") {
		t.Errorf("expected reuse-class warning, got:\n%s", combined)
	}
}

func TestDetectRisks_SingleAgentNoTrigger(t *testing.T) {
	rows := []authDiagRow{
		{Name: "team/sarah", Provider: "openai-codex", LastAuthAt: time.Now()},
	}
	out := detectRisks(rows)
	if len(out) != 0 {
		t.Errorf("single-agent setup should trigger no risks, got: %v", out)
	}
}
