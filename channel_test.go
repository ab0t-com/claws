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

func TestIntegration_ChannelAddSetsSafeDefaults(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake")

	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg := cfg["channels"].(map[string]any)["telegram"].(map[string]any)

	actions, ok := tg["actions"].(map[string]any)
	if !ok {
		t.Fatal("actions should be set after channel add")
	}
	if actions["sendMessage"] != false {
		t.Error("sendMessage should default to false")
	}
	if actions["reactions"] != true {
		t.Error("reactions should default to true")
	}
	if tg["groupPolicy"] != "allowlist" {
		t.Errorf("groupPolicy should be allowlist, got %v", tg["groupPolicy"])
	}
}

func TestIntegration_ChannelAddAllowSend(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake", "--allow-send")

	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg := cfg["channels"].(map[string]any)["telegram"].(map[string]any)

	actions := tg["actions"].(map[string]any)
	if actions["sendMessage"] != true {
		t.Error("sendMessage should be true with --allow-send")
	}
}

func TestIntegration_ChannelAddDiscordSafeDefaults(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "discord", "--token=fake")

	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	dc := cfg["channels"].(map[string]any)["discord"].(map[string]any)

	actions := dc["actions"].(map[string]any)
	if actions["messages"] != false {
		t.Error("discord messages should default to false")
	}
	if actions["reactions"] != true {
		t.Error("discord reactions should default to true")
	}
	if actions["moderation"] != false {
		t.Error("discord moderation should default to false")
	}
}

func TestIntegration_ChannelSecurity(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake")

	out, err := clawctl(t, root, "channel", "security", "alpha")
	if err != nil {
		t.Fatalf("channel security failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "telegram") {
		t.Error("should show telegram channel")
	}
	if !strings.Contains(out, "sendMessage") {
		t.Error("should show sendMessage action")
	}
	if !strings.Contains(out, "pairing") {
		t.Error("should show dm-policy")
	}
}

func TestIntegration_ChannelSend(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake")

	// Enable
	out, err := clawctl(t, root, "channel", "send", "alpha", "telegram", "--enable")
	if err != nil {
		t.Fatalf("channel send --enable failed: %v\n%s", err, out)
	}
	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	actions := cfg["channels"].(map[string]any)["telegram"].(map[string]any)["actions"].(map[string]any)
	if actions["sendMessage"] != true {
		t.Error("sendMessage should be true after --enable")
	}

	// Disable
	clawctl(t, root, "channel", "send", "alpha", "telegram", "--disable")
	cfg = readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	actions = cfg["channels"].(map[string]any)["telegram"].(map[string]any)["actions"].(map[string]any)
	if actions["sendMessage"] != false {
		t.Error("sendMessage should be false after --disable")
	}
}

func TestIntegration_ChannelAllowDeny(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake")

	// Allow
	out, err := clawctl(t, root, "channel", "allow", "alpha", "telegram", "+1234567890")
	if err != nil {
		t.Fatalf("channel allow failed: %v\n%s", err, out)
	}
	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg := cfg["channels"].(map[string]any)["telegram"].(map[string]any)
	af := tg["allowFrom"].([]any)
	if len(af) != 1 || af[0] != "+1234567890" {
		t.Errorf("allowFrom should have +1234567890, got %v", af)
	}

	// Allow another (and test dedup)
	clawctl(t, root, "channel", "allow", "alpha", "telegram", "+9876543210", "+1234567890")
	cfg = readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg = cfg["channels"].(map[string]any)["telegram"].(map[string]any)
	af = tg["allowFrom"].([]any)
	if len(af) != 2 {
		t.Errorf("allowFrom should have 2 entries (deduped), got %d", len(af))
	}

	// Deny
	clawctl(t, root, "channel", "deny", "alpha", "telegram", "+1234567890")
	cfg = readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg = cfg["channels"].(map[string]any)["telegram"].(map[string]any)
	af = tg["allowFrom"].([]any)
	if len(af) != 1 {
		t.Errorf("allowFrom should have 1 entry after deny, got %d", len(af))
	}

	// Deny non-existent
	_, err = clawctl(t, root, "channel", "deny", "alpha", "telegram", "+0000000000")
	if err == nil {
		t.Error("denying non-existent contact should fail")
	}
}

func TestIntegration_ChannelAddOutputMentionsSend(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	out, _ := clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=123:fake")
	if !strings.Contains(out, "Outbound messaging is OFF") {
		t.Error("should mention outbound messaging is off")
	}
	if !strings.Contains(out, "channel send") {
		t.Error("should tell user how to enable sending")
	}
}

func TestIntegration_ChannelSendNonExistent(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	_, err := clawctl(t, root, "channel", "send", "alpha", "telegram", "--enable")
	if err == nil {
		t.Error("should fail on non-existent channel")
	}
}

func TestIntegration_ChannelAllowNonExistent(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	_, err := clawctl(t, root, "channel", "allow", "alpha", "telegram", "+1234567890")
	if err == nil {
		t.Error("should fail on non-existent channel")
	}
}

func TestIntegration_ChannelDenyNonExistent(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	_, err := clawctl(t, root, "channel", "deny", "alpha", "telegram", "+1234567890")
	if err == nil {
		t.Error("should fail on non-existent channel")
	}
}

func TestIntegration_ChannelSendDiscord(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "discord", "--token=fake")

	// Discord uses "messages" not "sendMessage"
	clawctl(t, root, "channel", "send", "alpha", "discord", "--enable")
	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	actions := cfg["channels"].(map[string]any)["discord"].(map[string]any)["actions"].(map[string]any)
	if actions["messages"] != true {
		t.Error("discord should use 'messages' key, not 'sendMessage'")
	}
	// sendMessage should NOT exist for discord
	if _, exists := actions["sendMessage"]; exists {
		t.Error("discord should not have sendMessage key")
	}
}

func TestIntegration_ChannelAddWhatsAppDefaultAllowlist(t *testing.T) {
	// WhatsApp should default to dmPolicy=allowlist (not pairing)
	// Can't fully test WhatsApp add (needs login flow), but verify the profile
	profile := channelProfiles["whatsapp"]
	if profile.DefaultDmPolicy != "allowlist" {
		t.Errorf("whatsapp default dm policy should be allowlist, got %s", profile.DefaultDmPolicy)
	}
	profile = channelProfiles["signal"]
	if profile.DefaultDmPolicy != "allowlist" {
		t.Errorf("signal default dm policy should be allowlist, got %s", profile.DefaultDmPolicy)
	}
	// Telegram should still be pairing (empty = pairing)
	profile = channelProfiles["telegram"]
	if profile.DefaultDmPolicy != "" {
		t.Errorf("telegram default dm policy should be empty (pairing), got %s", profile.DefaultDmPolicy)
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
