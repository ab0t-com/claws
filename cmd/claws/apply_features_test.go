package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All new schema fields land in the right files.
func TestIntegration_ApplyAppliesAllSchemaFields(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "feature.json")
	body := `{
  "apiVersion": "claws.ab0t.com/v1",
  "kind": "Profile",
  "metadata": {"name": "feat", "version": "1.0"},
  "policy": {"loopbackOnly": true, "dmDefault": "pairing", "outboundDefault": "off"},
  "team": {"name": "ft"},
  "agents": [{
    "name": "a",
    "sandbox": true,
    "tools": {"profile": "coding", "allow": ["bash"], "deny": ["network"]},
    "skills": ["calendar"],
    "hooks": {"onStart": "echo booted"},
    "config": {"memory.enabled": true}
  }]
}`
	if err := os.WriteFile(profile, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := claws(t, root, "apply", "--file="+profile, "--skip-audit")
	if err != nil {
		t.Fatalf("apply failed: %v\n%s", err, out)
	}

	// A1 — policy applied
	policyData, err := os.ReadFile(filepath.Join(root, "policy.json"))
	if err != nil {
		t.Fatalf("policy.json not written: %v", err)
	}
	var pol map[string]interface{}
	_ = json.Unmarshal(policyData, &pol)
	if bm, _ := pol["allowedBindModes"].([]interface{}); len(bm) == 0 || bm[0] != "loopback" {
		t.Errorf("policy.allowedBindModes should be [loopback], got %v", bm)
	}
	if rp, _ := pol["requireDmPairing"].(bool); !rp {
		t.Errorf("policy.requireDmPairing should be true")
	}

	// B1 — sandbox
	agentCfgData, err := os.ReadFile(filepath.Join(root, "ft", "a", "openclaw.json"))
	if err != nil {
		t.Fatalf("agent openclaw.json not found: %v", err)
	}
	cfg := string(agentCfgData)
	if !strings.Contains(cfg, `"sandbox": true`) {
		t.Errorf("agent config missing sandbox=true:\n%s", cfg)
	}
	// A4 — tools.profile
	if !strings.Contains(cfg, `"profile": "coding"`) {
		t.Errorf("agent config missing tools.profile=coding:\n%s", cfg)
	}
	// B5 — tools.allow / tools.deny
	if !strings.Contains(cfg, `"bash"`) || !strings.Contains(cfg, `"network"`) {
		t.Errorf("agent config missing tools.allow/deny:\n%s", cfg)
	}
	// B4 — config catch-all
	if !strings.Contains(cfg, `"enabled": true`) {
		t.Errorf("agent config missing memory.enabled=true:\n%s", cfg)
	}

	// B2 — skills manifest (v1.6: team-shared at <team>/shared/skills/)
	manifest, err := os.ReadFile(filepath.Join(root, "ft", "shared", "skills", "MANIFEST.txt"))
	if err != nil {
		t.Errorf("v1.6 team-shared skills manifest not written: %v", err)
	} else if !strings.Contains(string(manifest), "calendar") {
		t.Errorf("skills manifest missing 'calendar': %s", string(manifest))
	}

	// B3 — hooks (v1.6: team-shared at <team>/shared/hooks/)
	hookData, err := os.ReadFile(filepath.Join(root, "ft", "shared", "hooks", "onStart.sh"))
	if err != nil {
		t.Errorf("v1.6 team-shared hook onStart.sh not written: %v", err)
	} else if !strings.Contains(string(hookData), "echo booted") {
		t.Errorf("hook script missing command: %s", string(hookData))
	}
}

// Channel idempotence: same channel block applied twice → second is skipped.
func TestIntegration_ApplyChannelIdempotent(t *testing.T) {
	root := t.TempDir()
	// First, create an agent and manually add the channel (simulating an existing state)
	if _, err := claws(t, root, "init"); err != nil {
		t.Fatal(err)
	}
	if _, err := claws(t, root, "group", "create", "t"); err != nil {
		t.Fatal(err)
	}
	if _, err := claws(t, root, "create", "t/a"); err != nil {
		t.Fatal(err)
	}
	// Write a fake openclaw.json that has telegram.enabled=true
	cfgPath := filepath.Join(root, "t", "a", "openclaw.json")
	data, _ := os.ReadFile(cfgPath)
	var cfg map[string]interface{}
	_ = json.Unmarshal(data, &cfg)
	channels, _ := cfg["channels"].(map[string]interface{})
	if channels == nil {
		channels = map[string]interface{}{}
		cfg["channels"] = channels
	}
	channels["telegram"] = map[string]interface{}{"enabled": true}
	updated, _ := json.MarshalIndent(cfg, "", "  ")
	_ = os.WriteFile(cfgPath, updated, 0644)

	// Now apply a profile that declares the same channel — should skip
	profile := filepath.Join(root, "p.json")
	body := `{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "p", "version": "0.1"},
  "team": {"name": "t"},
  "agents": [{"name": "a", "channels": [{"type": "telegram", "tokenFrom": {"env": "TG"}}]}]
}`
	_ = os.WriteFile(profile, []byte(body), 0644)
	t.Setenv("TG", "fake-token")
	out, err := claws(t, root, "apply", "--file="+profile, "--skip-audit")
	if err != nil {
		t.Fatalf("apply failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "channel telegram already configured") {
		t.Errorf("expected channel idempotence skip, got:\n%s", out)
	}
}

// --template=<name> resolves from cwd/templates → XDG data dir → bundled.
func TestIntegration_TemplateResolverCWD(t *testing.T) {
	root := t.TempDir()
	tplDir := filepath.Join(root, "templates")
	_ = os.MkdirAll(tplDir, 0755)
	_ = os.WriteFile(filepath.Join(tplDir, "tinytest.json"), []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "tinytest", "version": "0.1"},
  "team": {"name": "tt"},
  "agents": [{"name": "x"}]
}`), 0644)
	// Run from inside the temp dir so cwd ./templates resolves
	out, err := clawsCwd(t, root, root, "apply", "--template=tinytest", "--skip-audit")
	if err != nil {
		t.Fatalf("apply --template=tinytest failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "apply complete") {
		t.Errorf("apply did not complete:\n%s", out)
	}
}

// template list shows templates with metadata.
func TestIntegration_TemplateList(t *testing.T) {
	root := t.TempDir()
	tplDir := filepath.Join(root, "templates")
	_ = os.MkdirAll(tplDir, 0755)
	_ = os.WriteFile(filepath.Join(tplDir, "alpha.json"), []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "alpha", "version": "2.0.0", "description": "Alpha template"},
  "team": {"name": "x"}, "agents": [{"name": "a"}]
}`), 0644)
	out, err := clawsCwd(t, root, root, "template", "list")
	if err != nil {
		t.Fatalf("template list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "2.0.0") {
		t.Errorf("template list missing entry:\n%s", out)
	}
}

// template show prints the JSON.
func TestIntegration_TemplateShow(t *testing.T) {
	root := t.TempDir()
	tplDir := filepath.Join(root, "templates")
	_ = os.MkdirAll(tplDir, 0755)
	body := `{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "beta", "version": "0.1"},
  "team": {"name": "x"}, "agents": [{"name": "a"}]
}`
	_ = os.WriteFile(filepath.Join(tplDir, "beta.json"), []byte(body), 0644)
	out, err := clawsCwd(t, root, root, "template", "show", "beta")
	if err != nil {
		t.Fatalf("template show failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "\"beta\"") {
		t.Errorf("template show didn't print profile JSON:\n%s", out)
	}
}

// template resolve errors clearly for unknown names.
func TestIntegration_TemplateResolveUnknown(t *testing.T) {
	root := t.TempDir()
	_, err := clawsCwd(t, root, root, "template", "resolve", "does-not-exist")
	if err == nil {
		t.Error("expected error for unknown template name")
	}
}
