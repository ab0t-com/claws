package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecretsDirFallback_CuratedMap(t *testing.T) {
	activeSecretsDir = "/tmp/x"
	defer func() { activeSecretsDir = "" }()
	cases := map[string]string{
		"OPENAI_API_KEY":     "/tmp/x/openai.key",
		"ANTHROPIC_API_KEY":  "/tmp/x/anthropic.key",
		"TELEGRAM_BOT_TOKEN": "/tmp/x/telegram.token",
		"DISCORD_BOT_TOKEN":  "/tmp/x/discord.token",
		"SLACK_BOT_TOKEN":    "/tmp/x/slack.bot-token",
		"SLACK_APP_TOKEN":    "/tmp/x/slack.app-token",
	}
	for envName, want := range cases {
		if got := secretsDirFallback(envName); got != want {
			t.Errorf("secretsDirFallback(%q): got %q want %q", envName, got, want)
		}
	}
}

func TestSecretsDirFallback_DerivationRule(t *testing.T) {
	activeSecretsDir = "/tmp/x"
	defer func() { activeSecretsDir = "" }()
	cases := map[string]string{
		"WEIRD_NEW_KEY":      "/tmp/x/weird-new.key",
		"FOO_BAR_TOKEN":      "/tmp/x/foo-bar.token",
		"MY_SECRET":          "/tmp/x/my.secret",
		"UNSUFFIXED":         "/tmp/x/unsuffixed.value",
	}
	for envName, want := range cases {
		if got := secretsDirFallback(envName); got != want {
			t.Errorf("secretsDirFallback(%q): got %q want %q", envName, got, want)
		}
	}
}

func TestSecretsDirFallback_NoDirActive(t *testing.T) {
	activeSecretsDir = ""
	if got := secretsDirFallback("OPENAI_API_KEY"); got != "" {
		t.Errorf("expected empty when no dir active; got %q", got)
	}
}

func TestReadSecretFile_StripsCommentsAndBlanks(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "x.key")
	body := "# This is a comment\n\nsk-real-value-here\n# another comment\n"
	_ = os.WriteFile(f, []byte(body), 0600)
	got := readSecretFile(f)
	if got != "sk-real-value-here" {
		t.Errorf("expected just the value, got %q", got)
	}
}

func TestReadSecretFile_OnlyComments(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "x.key")
	_ = os.WriteFile(f, []byte("# Only comments\n# Nothing else\n"), 0600)
	if got := readSecretFile(f); got != "" {
		t.Errorf("expected empty for comments-only, got %q", got)
	}
}

// Apply with --secrets-dir resolves env vars from files.
func TestIntegration_ApplyWithSecretsDir(t *testing.T) {
	root := t.TempDir()
	secrets := filepath.Join(root, "secrets")
	_ = os.MkdirAll(secrets, 0700)
	_ = os.WriteFile(filepath.Join(secrets, "openai.key"), []byte("# header\nsk-fake-key\n"), 0600)
	_ = os.WriteFile(filepath.Join(secrets, "telegram.token"), []byte("111:fake-token\n"), 0600)

	profile := filepath.Join(root, "profile.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "sd", "version": "0"},
  "team": {"name": "sd"},
  "agents": [{
    "name": "a",
    "auth": {"fallbackApiKey": {"provider": "openai", "fromEnv": "OPENAI_API_KEY"}},
    "channels": [{"type": "telegram", "tokenFrom": {"env": "TELEGRAM_BOT_TOKEN"}}]
  }]
}`), 0644)

	// Ensure the env vars are absent.
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("TELEGRAM_BOT_TOKEN")

	out, err := claws(t, root, "apply", "--file="+profile, "--secrets-dir="+secrets, "--skip-audit")
	if err != nil {
		t.Fatalf("apply --secrets-dir should succeed when files present; got:\n%s", out)
	}
	if !strings.Contains(out, "apply complete") {
		t.Errorf("apply didn't complete: %s", out)
	}
}

// Apply with --secrets-dir but empty files = still missing.
func TestIntegration_ApplyWithSecretsDirEmptyFile(t *testing.T) {
	root := t.TempDir()
	secrets := filepath.Join(root, "secrets")
	_ = os.MkdirAll(secrets, 0700)
	// Comments-only files = effectively empty.
	_ = os.WriteFile(filepath.Join(secrets, "openai.key"), []byte("# placeholder, no value\n"), 0600)

	profile := filepath.Join(root, "profile.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "sd2", "version": "0"},
  "team": {"name": "sd2"},
  "agents": [{
    "name": "a",
    "auth": {"fallbackApiKey": {"provider": "openai", "fromEnv": "OPENAI_API_KEY"}}
  }]
}`), 0644)
	os.Unsetenv("OPENAI_API_KEY")
	out, err := claws(t, root, "apply", "--file="+profile, "--secrets-dir="+secrets, "--skip-audit")
	if err == nil {
		t.Errorf("expected failure when secrets-dir file is empty; got:\n%s", out)
	}
	if !strings.Contains(out, "openai.key") {
		t.Errorf("error should mention the secrets-dir file path; got:\n%s", out)
	}
}
