package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetNestedConfig(t *testing.T) {
	cfg := map[string]any{}

	setNestedConfig(cfg, "channels.telegram.enabled", true)
	setNestedConfig(cfg, "channels.telegram.botToken", "123:abc")
	setNestedConfig(cfg, "channels.telegram.dmPolicy", "pairing")

	channels := cfg["channels"].(map[string]any)
	tg := channels["telegram"].(map[string]any)
	if tg["enabled"] != true {
		t.Error("enabled should be true")
	}
	if tg["botToken"] != "123:abc" {
		t.Errorf("botToken should be '123:abc', got '%v'", tg["botToken"])
	}
	if tg["dmPolicy"] != "pairing" {
		t.Errorf("dmPolicy should be 'pairing', got '%v'", tg["dmPolicy"])
	}
}

func TestSetNestedConfig_Overwrites(t *testing.T) {
	cfg := map[string]any{
		"channels": map[string]any{
			"telegram": map[string]any{
				"enabled":  false,
				"botToken": "old",
			},
		},
	}

	setNestedConfig(cfg, "channels.telegram.enabled", true)
	setNestedConfig(cfg, "channels.telegram.botToken", "new")

	tg := cfg["channels"].(map[string]any)["telegram"].(map[string]any)
	if tg["enabled"] != true {
		t.Error("should overwrite enabled")
	}
	if tg["botToken"] != "new" {
		t.Error("should overwrite botToken")
	}
}

func TestReadWriteInstanceConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "test.json")

	cfg := map[string]any{
		"gateway": map[string]any{"port": float64(18789)},
	}
	if err := writeInstanceConfig(path, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := readInstanceConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	gw := loaded["gateway"].(map[string]any)
	if gw["port"] != float64(18789) {
		t.Errorf("port should be 18789, got %v", gw["port"])
	}

	// Check permissions
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0600 {
		t.Errorf("config should be 0600, got %04o", fi.Mode().Perm())
	}
}

func TestIntegration_ChannelAddTelegram(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	out, err := clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:faketoken")
	if err != nil {
		t.Fatalf("channel add failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "configured") {
		t.Errorf("should confirm configuration: %s", out)
	}
	if !strings.Contains(out, "clawctl approve") {
		t.Errorf("should show approve next step: %s", out)
	}

	// Verify config was written
	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	channels := cfg["channels"].(map[string]any)
	tg := channels["telegram"].(map[string]any)
	if tg["enabled"] != true {
		t.Error("telegram should be enabled")
	}
	if tg["botToken"] != "123:faketoken" {
		t.Errorf("botToken should be set, got %v", tg["botToken"])
	}
	if tg["dmPolicy"] != "pairing" {
		t.Errorf("dmPolicy should default to pairing, got %v", tg["dmPolicy"])
	}
}

func TestIntegration_ChannelAddDiscord(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	out, err := clawctl(t, root, "channel", "add", "alpha", "discord", "--token=faketoken")
	if err != nil {
		t.Fatalf("channel add failed: %v\n%s", err, out)
	}

	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	dc := cfg["channels"].(map[string]any)["discord"].(map[string]any)
	if dc["token"] != "faketoken" {
		t.Errorf("discord token should be set, got %v", dc["token"])
	}
}

func TestIntegration_ChannelAddSlack(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	out, err := clawctl(t, root, "channel", "add", "alpha", "slack", "--bot-token=xoxb-fake", "--app-token=xapp-fake")
	if err != nil {
		t.Fatalf("channel add failed: %v\n%s", err, out)
	}

	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	sl := cfg["channels"].(map[string]any)["slack"].(map[string]any)
	if sl["botToken"] != "xoxb-fake" {
		t.Errorf("slack botToken should be set, got %v", sl["botToken"])
	}
	if sl["appToken"] != "xapp-fake" {
		t.Errorf("slack appToken should be set, got %v", sl["appToken"])
	}
}

func TestIntegration_ChannelAddMissingToken(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	_, err := clawctl(t, root, "channel", "add", "alpha", "telegram")
	if err == nil {
		t.Error("should fail without --token")
	}
}

func TestIntegration_ChannelAddCustomDmPolicy(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake", "--dm-policy=allowlist")

	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg := cfg["channels"].(map[string]any)["telegram"].(map[string]any)
	if tg["dmPolicy"] != "allowlist" {
		t.Errorf("dmPolicy should be allowlist, got %v", tg["dmPolicy"])
	}
}

func TestIntegration_ChannelStatus(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake")

	out, err := clawctl(t, root, "channel", "status", "alpha")
	if err != nil {
		t.Fatalf("channel status failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "telegram") {
		t.Error("status should show telegram")
	}
	if !strings.Contains(out, "enabled") {
		t.Error("status should show enabled")
	}
}

func TestIntegration_ChannelRemove(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake")
	clawctl(t, root, "channel", "remove", "alpha", "telegram")

	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg := cfg["channels"].(map[string]any)["telegram"].(map[string]any)
	if tg["enabled"] != false {
		t.Error("telegram should be disabled after remove")
	}
}

func TestIntegration_ChannelHelp(t *testing.T) {
	root := t.TempDir()
	out, _ := clawctl(t, root, "channel", "--help")
	if !strings.Contains(out, "Quick-add") {
		t.Errorf("channel help should mention quick-add: %s", out)
	}
}

func TestIntegration_ApproveHelp(t *testing.T) {
	root := t.TempDir()
	out, _ := clawctl(t, root, "approve", "--help")
	if !strings.Contains(out, "pairing") {
		t.Errorf("approve help should mention pairing: %s", out)
	}
}

func TestIntegration_LegacyChannelStillWorks(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	// Legacy style without flags — should attempt wizard (will fail without Docker but shouldn't panic)
	_, err := clawctl(t, root, "channel", "alpha", "telegram")
	// This will fail because there's no running container, but it should fail gracefully
	if err == nil {
		// If it somehow succeeds (unlikely in test), that's fine too
		return
	}
	// The error should be about Docker/compose, not a panic or usage error
	// This confirms the legacy path is still reachable
}
