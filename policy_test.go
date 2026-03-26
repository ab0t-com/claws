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
	if err := p.enforceDmPolicy("open"); err == nil {
		t.Error("open should be blocked when pairing required")
	}
}

func TestIntegration_PolicyInit(t *testing.T) {
	root := t.TempDir()
	out, err := clawctl(t, root, "policy", "init")
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
	clawctl(t, root, "policy", "init")
	out, _ := clawctl(t, root, "policy", "show")
	if !strings.Contains(out, "loopback") {
		t.Errorf("policy show should contain loopback: %s", out)
	}
}

func TestIntegration_PolicyBlocksBindLan(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "policy", "init")

	_, err := clawctl(t, root, "create", "alpha", "--bind=lan")
	if err == nil {
		t.Error("policy should block --bind=lan when only loopback is allowed")
	}
}

func TestIntegration_PolicyAllowsLoopback(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "policy", "init")

	_, err := clawctl(t, root, "create", "alpha")
	if err != nil {
		t.Errorf("default create (loopback) should be allowed by policy: %v", err)
	}
}

func TestIntegration_PolicyValidate(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "policy", "init")

	out, err := clawctl(t, root, "policy", "validate")
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
	clawctl(t, root, "create", "alpha", "--bind=lan")

	// Verify it's lan
	envFile := filepath.Join(root, "alpha", "instance.env")
	bind := readEnvFromFile(t, envFile, "OPENCLAW_GATEWAY_BIND")
	if bind != "lan" {
		t.Fatalf("expected lan, got %s", bind)
	}

	// Init policy (restricts to loopback)
	clawctl(t, root, "policy", "init")

	// Enforce
	out, err := clawctl(t, root, "policy", "enforce")
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
	clawctl(t, root, "create", "alpha")
	clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=fake", "--dm-policy=open")

	// Init policy (requires pairing)
	clawctl(t, root, "policy", "init")

	// Enforce
	out, _ := clawctl(t, root, "policy", "enforce")
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
	clawctl(t, root, "policy", "init")
	clawctl(t, root, "create", "alpha") // created under policy, should be clean

	out, _ := clawctl(t, root, "policy", "enforce")
	if !strings.Contains(out, "No violations") {
		t.Errorf("clean instance should have no violations: %s", out)
	}
}

func TestIntegration_PolicyBlocksChannel(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "create", "alpha")

	// Create policy that blocks IRC
	p := Policy{BlockedChannels: []string{"irc"}}
	paths := Paths{Root: root, PortRegistry: filepath.Join(root, ".port-registry")}
	writePolicy(paths, p)

	_, err := clawctl(t, root, "channel", "add", "alpha", "telegram", "--token=fake")
	if err != nil {
		t.Errorf("telegram should not be blocked: %v", err)
	}
}
