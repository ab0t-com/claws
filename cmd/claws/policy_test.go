package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern, s string
		want       bool
	}{
		{"*", "anything", true},
		{"openclaw:*", "openclaw:local", true},
		{"openclaw:*", "openclaw:v2026.3", true},
		{"openclaw:*", "other:local", false},
		{"openclaw:local", "openclaw:local", true},
		{"openclaw:local", "openclaw:v1", false},
		{"ghcr.io/openclaw/*", "ghcr.io/openclaw/gateway:v1", true},
		{"ghcr.io/openclaw/*", "docker.io/other:v1", false},
	}
	for _, tc := range tests {
		got := matchGlob(tc.pattern, tc.s)
		if got != tc.want {
			t.Errorf("matchGlob(%q, %q) = %v, want %v", tc.pattern, tc.s, got, tc.want)
		}
	}
}

func TestPolicyEnforceBind(t *testing.T) {
	p := Policy{AllowedBindModes: []string{"loopback"}}
	if err := p.enforceBindPolicy("loopback"); err != nil {
		t.Errorf("loopback should be allowed: %v", err)
	}
	if err := p.enforceBindPolicy("lan"); err == nil {
		t.Error("lan should be blocked")
	}

	// Empty policy = no restriction
	p2 := Policy{}
	if err := p2.enforceBindPolicy("wan"); err != nil {
		t.Errorf("empty policy should allow anything: %v", err)
	}
}

func TestPolicyEnforceImage(t *testing.T) {
	p := Policy{AllowedImages: []string{"openclaw:*"}}
	if err := p.enforceImagePolicy("openclaw:local"); err != nil {
		t.Errorf("openclaw:local should be allowed: %v", err)
	}
	if err := p.enforceImagePolicy("other:latest"); err == nil {
		t.Error("other:latest should be blocked")
	}
}

func TestPolicyEnforceChannel(t *testing.T) {
	p := Policy{BlockedChannels: []string{"irc", "twitch"}}
	if err := p.enforceChannelPolicy("telegram"); err != nil {
		t.Errorf("telegram should be allowed: %v", err)
	}
	if err := p.enforceChannelPolicy("irc"); err == nil {
		t.Error("irc should be blocked")
	}
}

func TestPolicyEnforceDmPairing(t *testing.T) {
	p := Policy{RequireDmPairing: true}
	if err := p.enforceDmPolicy("pairing"); err != nil {
		t.Errorf("pairing should be allowed: %v", err)
	}
	if err := p.enforceDmPolicy("allowlist"); err != nil {
		t.Errorf("allowlist should be allowed (stricter than pairing): %v", err)
	}
	if err := p.enforceDmPolicy("disabled"); err != nil {
		t.Errorf("disabled should be allowed (strictest): %v", err)
	}
	if err := p.enforceDmPolicy("open"); err == nil {
		t.Error("open should be blocked when pairing required")
	}
}

func TestIntegration_PolicyInit(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "policy", "init")
	if err != nil {
		t.Fatalf("policy init failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Policy created") {
		t.Errorf("should confirm creation: %s", out)
	}

	// Verify file
	data, err := os.ReadFile(filepath.Join(root, "policy.json"))
	if err != nil {
		t.Fatal("policy.json not created")
	}
	var p Policy
	json.Unmarshal(data, &p)
	if len(p.AllowedBindModes) == 0 || p.AllowedBindModes[0] != "loopback" {
		t.Error("default policy should restrict to loopback")
	}
	if !p.RequireDmPairing {
		t.Error("default policy should require DM pairing")
	}
}

func TestIntegration_PolicyShow(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "policy", "init")
	out, _ := claws(t, root, "policy", "show")
	if !strings.Contains(out, "loopback") {
		t.Errorf("policy show should contain loopback: %s", out)
	}
}

func TestIntegration_PolicyBlocksBindLan(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "policy", "init")

	_, err := claws(t, root, "create", "alpha", "--bind=lan")
	if err == nil {
		t.Error("policy should block --bind=lan when only loopback is allowed")
	}
}

func TestIntegration_PolicyAllowsLoopback(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "policy", "init")

	_, err := claws(t, root, "create", "alpha")
	if err != nil {
		t.Errorf("default create (loopback) should be allowed by policy: %v", err)
	}
}

func TestIntegration_PolicyValidate(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "policy", "init")

	out, err := claws(t, root, "policy", "validate")
	if err != nil {
		// May have violations since alpha was created before policy
		_ = out
	}
	if !strings.Contains(out, "alpha") {
		t.Errorf("validate should check alpha: %s", out)
	}
}

func TestIntegration_PolicyEnforce(t *testing.T) {
	root := t.TempDir()

	// Create instance with lan bind (before policy exists)
	claws(t, root, "create", "alpha", "--bind=lan")

	// Verify it's lan
	envFile := filepath.Join(root, "alpha", "instance.env")
	bind := readEnvFromFile(t, envFile, "OPENCLAW_GATEWAY_BIND")
	if bind != "lan" {
		t.Fatalf("expected lan, got %s", bind)
	}

	// Init policy (restricts to loopback)
	claws(t, root, "policy", "init")

	// Enforce
	out, err := claws(t, root, "policy", "enforce")
	if err != nil {
		t.Fatalf("enforce failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "loopback") {
		t.Errorf("should report bind fix: %s", out)
	}

	// Verify fixed
	bind = readEnvFromFile(t, envFile, "OPENCLAW_GATEWAY_BIND")
	if bind != "loopback" {
		t.Errorf("bind should be loopback after enforce, got %s", bind)
	}
}

func TestIntegration_PolicyEnforceDmPolicy(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "channel", "add", "alpha", "telegram", "--token=fake", "--dm-policy=open")

	// Init policy (requires pairing)
	claws(t, root, "policy", "init")

	// Enforce
	out, _ := claws(t, root, "policy", "enforce")
	if !strings.Contains(out, "pairing") {
		t.Errorf("should fix DM policy to pairing: %s", out)
	}

	// Verify
	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg := cfg["channels"].(map[string]any)["telegram"].(map[string]any)
	if tg["dmPolicy"] != "pairing" {
		t.Errorf("dmPolicy should be pairing after enforce, got %v", tg["dmPolicy"])
	}
}

func TestIntegration_PolicyEnforceClean(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "policy", "init")
	claws(t, root, "create", "alpha") // created under policy, should be clean

	out, _ := claws(t, root, "policy", "enforce")
	if !strings.Contains(out, "No violations") {
		t.Errorf("clean instance should have no violations: %s", out)
	}
}

func TestPolicyEnforceOutboundAllowlist(t *testing.T) {
	p := Policy{RequireOutboundAllowlist: true}

	// sendMessage enabled without allowFrom = violation
	chMap := map[string]any{
		"actions": map[string]any{"sendMessage": true},
	}
	if err := p.enforceOutboundAllowlist("whatsapp", chMap); err == nil {
		t.Error("should fail: sendMessage enabled without allowFrom")
	}

	// sendMessage enabled with allowFrom = ok
	chMap["allowFrom"] = []any{"+1234567890"}
	if err := p.enforceOutboundAllowlist("whatsapp", chMap); err != nil {
		t.Errorf("should pass with allowFrom: %v", err)
	}

	// sendMessage disabled = ok (no allowFrom needed)
	chMap2 := map[string]any{
		"actions": map[string]any{"sendMessage": false},
	}
	if err := p.enforceOutboundAllowlist("whatsapp", chMap2); err != nil {
		t.Errorf("should pass when sendMessage is false: %v", err)
	}

	// No actions set = ok
	chMap3 := map[string]any{}
	if err := p.enforceOutboundAllowlist("whatsapp", chMap3); err != nil {
		t.Errorf("should pass when no actions: %v", err)
	}

	// Policy disabled = ok
	p2 := Policy{RequireOutboundAllowlist: false}
	if err := p2.enforceOutboundAllowlist("whatsapp", chMap); err != nil {
		t.Errorf("should pass when policy disabled: %v", err)
	}
}

func TestPolicyEnforceOutboundDiscord(t *testing.T) {
	p := Policy{RequireOutboundAllowlist: true}

	// Discord uses "messages" not "sendMessage"
	chMap := map[string]any{
		"actions": map[string]any{"messages": true},
	}
	if err := p.enforceOutboundAllowlist("discord", chMap); err == nil {
		t.Error("should fail: discord messages enabled without allowFrom")
	}
}

func TestIntegration_PolicyInitOutbound(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "policy", "init")

	data, _ := os.ReadFile(filepath.Join(root, "policy.json"))
	var p Policy
	json.Unmarshal(data, &p)
	if !p.RequireOutboundAllowlist {
		t.Error("default policy should require outbound allowlist")
	}
}

func TestIntegration_PolicyEnforceOutbound(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")
	claws(t, root, "channel", "add", "alpha", "telegram", "--token=fake", "--allow-send")

	// Init policy (requires outbound allowlist)
	claws(t, root, "policy", "init")

	// Enforce — should disable sendMessage since no allowFrom
	out, _ := claws(t, root, "policy", "enforce")
	if !strings.Contains(out, "sendMessage") && !strings.Contains(out, "disabled") {
		t.Errorf("should disable sendMessage without allowFrom: %s", out)
	}

	// Verify
	cfg := readJSON(t, filepath.Join(root, "alpha", "openclaw.json"))
	tg := cfg["channels"].(map[string]any)["telegram"].(map[string]any)
	actions := tg["actions"].(map[string]any)
	if actions["sendMessage"] != false {
		t.Error("sendMessage should be false after enforce")
	}
}

func TestIntegration_PolicyBlocksChannel(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// Create policy that blocks IRC
	p := Policy{BlockedChannels: []string{"irc"}}
	paths := Paths{Root: root, PortRegistry: filepath.Join(root, ".port-registry")}
	writePolicy(paths, p)

	_, err := claws(t, root, "channel", "add", "alpha", "telegram", "--token=fake")
	if err != nil {
		t.Errorf("telegram should not be blocked: %v", err)
	}
}
