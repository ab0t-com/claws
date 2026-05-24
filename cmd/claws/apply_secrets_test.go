package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Missing env var → loud failure with provider URL in the message.
func TestIntegration_ApplyRejectsMissingSecrets(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "missing.json")
	body := `{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "ms", "version": "0"},
  "team": {"name": "ms"},
  "agents": [{
    "name": "a",
    "auth": {"fallbackApiKey": {"provider": "openai", "fromEnv": "FAKE_MISSING_OPENAI_KEY"}},
    "channels": [{"type": "telegram", "tokenFrom": {"env": "FAKE_MISSING_TG_TOKEN"}}]
  }]
}`
	_ = os.WriteFile(profile, []byte(body), 0644)
	// Ensure the env vars are absent.
	t.Setenv("FAKE_MISSING_OPENAI_KEY", "")
	t.Setenv("FAKE_MISSING_TG_TOKEN", "")
	os.Unsetenv("FAKE_MISSING_OPENAI_KEY")
	os.Unsetenv("FAKE_MISSING_TG_TOKEN")

	out, err := claws(t, root, "apply", "--file="+profile, "--skip-audit")
	if err == nil {
		t.Errorf("expected apply to fail when secrets missing; output:\n%s", out)
	}
	for _, must := range []string{
		"FAKE_MISSING_OPENAI_KEY",
		"FAKE_MISSING_TG_TOKEN",
		"platform.openai.com",
		"t.me/BotFather",
	} {
		if !strings.Contains(out, must) {
			t.Errorf("missing-secrets error should mention %q; got:\n%s", must, out)
		}
	}
}

// --allow-missing keeps the old behavior (silent skip, exit 0).
func TestIntegration_ApplyAllowMissingPasses(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "missing.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "ms", "version": "0"},
  "team": {"name": "ms"},
  "agents": [{
    "name": "a",
    "auth": {"fallbackApiKey": {"provider": "openai", "fromEnv": "STILL_MISSING_OPENAI"}}
  }]
}`), 0644)
	os.Unsetenv("STILL_MISSING_OPENAI")
	out, err := claws(t, root, "apply", "--file="+profile, "--skip-audit", "--allow-missing")
	if err != nil {
		t.Errorf("--allow-missing should succeed; got:\n%s", out)
	}
}

// Dry-run bypasses the missing-secret check (operator inspecting the spec).
func TestIntegration_ApplyDryRunBypassesSecretCheck(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "missing.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "ms", "version": "0"},
  "team": {"name": "ms"},
  "agents": [{
    "name": "a",
    "auth": {"fallbackApiKey": {"provider": "openai", "fromEnv": "NEVER_SET_OPENAI"}}
  }]
}`), 0644)
	os.Unsetenv("NEVER_SET_OPENAI")
	out, err := claws(t, root, "apply", "--file="+profile, "--dry-run")
	if err != nil {
		t.Errorf("--dry-run should succeed even with missing secrets; got:\n%s", out)
	}
}

// providerHint + channelHint return known URLs.
func TestProviderHints(t *testing.T) {
	if providerHint("openai") == "" {
		t.Error("openai hint missing")
	}
	if providerHint("anthropic") == "" {
		t.Error("anthropic hint missing")
	}
	if providerHint("never-heard-of-it") != "" {
		t.Error("unknown provider should return empty")
	}
	if !strings.Contains(channelHint("telegram"), "BotFather") {
		t.Error("telegram hint should reference BotFather")
	}
	if !strings.Contains(channelHint("discord"), "discord.com") {
		t.Error("discord hint should reference discord.com")
	}
}
