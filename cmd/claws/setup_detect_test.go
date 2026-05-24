package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestIsAuthLikeName(t *testing.T) {
	yes := []string{
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENROUTER_API_KEY",
		"TELEGRAM_BOT_TOKEN", "DISCORD_TOKEN", "SLACK_BOT_TOKEN",
		"CLAUDE_AI_SESSION_KEY", "CLAUDE_WEB_SESSION_KEY", "CLAUDE_WEB_COOKIE",
		"GITHUB_TOKEN", "SOMETHING_SECRET",
	}
	for _, k := range yes {
		if !isAuthLikeName(k) {
			t.Errorf("expected %q to be auth-like", k)
		}
	}
	no := []string{
		"INSTANCE_NAME", "OPENCLAW_CONFIG_DIR", "OPENCLAW_GATEWAY_PORT",
		"OPENCLAW_IMAGE", "OPENCLAW_HOST_BIND", "INSTANCE_ROLE",
		"OPENCLAW_GATEWAY_TOKEN", // explicit skip — per-instance shared secret
	}
	for _, k := range no {
		if isAuthLikeName(k) {
			t.Errorf("expected %q NOT to be auth-like", k)
		}
	}
}

func TestReadRelevantKeys(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, "instance.env")
	content := strings.Join([]string{
		"# comment line — should be skipped",
		"",
		"INSTANCE_NAME=alice",
		"OPENCLAW_GATEWAY_PORT=8080",
		"OPENAI_API_KEY=sk-fake-not-real",
		"ANTHROPIC_API_KEY=sk-ant-fake-not-real",
		"TELEGRAM_BOT_TOKEN=12345:fake",
		"CLAUDE_AI_SESSION_KEY=session-fake",
		"EMPTY_KEY=",                 // present but empty — skip
		"OPENCLAW_GATEWAY_TOKEN=xyz", // looks auth-like but explicitly skipped
	}, "\n")
	if err := os.WriteFile(envFile, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	got := readRelevantKeys(envFile)
	sort.Strings(got)
	want := []string{
		"ANTHROPIC_API_KEY",
		"CLAUDE_AI_SESSION_KEY",
		"OPENAI_API_KEY",
		"TELEGRAM_BOT_TOKEN",
	}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got[%d]=%q want %q", i, got[i], want[i])
		}
	}
}

func TestReadRelevantKeys_MissingFile(t *testing.T) {
	if got := readRelevantKeys("/no/such/path/instance.env"); got != nil {
		t.Errorf("expected nil for missing file, got %v", got)
	}
}

func TestHasClaudeOAuthBlock(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"valid", `{"claudeAiOauth": {"accessToken": "x"}}`, true},
		{"missing block", `{"otherStuff": {}}`, false},
		{"not json", `not-json`, false},
		{"empty", ``, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := filepath.Join(dir, c.name+".json")
			if err := os.WriteFile(p, []byte(c.content), 0600); err != nil {
				t.Fatal(err)
			}
			if got := hasClaudeOAuthBlock(p); got != c.want {
				t.Errorf("got %v want %v", got, c.want)
			}
		})
	}
	// Nonexistent file
	if hasClaudeOAuthBlock(filepath.Join(dir, "nope.json")) {
		t.Error("expected false for missing file")
	}
}

func TestShortenHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	if home == "" {
		t.Skip("no home dir")
	}
	cases := []struct {
		in, out string
	}{
		{home + "/.claude/.credentials.json", "~/.claude/.credentials.json"},
		{home, "~"},
		{"/tmp/elsewhere", "/tmp/elsewhere"},
	}
	for _, c := range cases {
		if got := shortenHome(c.in); got != c.out {
			t.Errorf("shortenHome(%q) = %q, want %q", c.in, got, c.out)
		}
	}
}

func TestDetectExistingAuth_EnvKeys(t *testing.T) {
	// Use a sandbox HOME so we don't see whatever's on the dev machine.
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	d := detectExistingAuth()
	if d.EnvAnthropicKey != "sk-ant-test" {
		t.Errorf("EnvAnthropicKey=%q want %q", d.EnvAnthropicKey, "sk-ant-test")
	}
	if d.EnvOpenAIKey != "" {
		t.Errorf("EnvOpenAIKey should be empty, got %q", d.EnvOpenAIKey)
	}
	if !d.Any() {
		t.Error("Any() should be true when an env key is present")
	}
}

func TestDetectExistingAuth_ExistingAgent(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	// Create a fake openclaw-style agent instance.env
	agentDir := filepath.Join(tmpHome, ".openclaw", "team", "alice")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(agentDir, "instance.env")
	body := "INSTANCE_NAME=alice\nOPENAI_API_KEY=sk-fake\nOPENCLAW_GATEWAY_PORT=8080\n"
	if err := os.WriteFile(envFile, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}

	d := detectExistingAuth()
	if len(d.ExistingAgents) != 1 {
		t.Fatalf("got %d existing agents, want 1: %+v", len(d.ExistingAgents), d.ExistingAgents)
	}
	a := d.ExistingAgents[0]
	if a.Team != "team" || a.Name != "alice" {
		t.Errorf("team/name = %q/%q want team/alice", a.Team, a.Name)
	}
	if len(a.HasKeys) != 1 || a.HasKeys[0] != "OPENAI_API_KEY" {
		t.Errorf("HasKeys = %v want [OPENAI_API_KEY]", a.HasKeys)
	}
}

func TestDetectExistingAuth_Empty(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("OPENAI_API_KEY", "")
	t.Setenv("OPENROUTER_API_KEY", "")

	d := detectExistingAuth()
	if d.Any() {
		t.Error("Any() should be false on a clean home with no env keys")
	}
}
