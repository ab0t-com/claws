package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// First-run: quickstart with no args produces default/agent-1 and registers it.
func TestIntegration_QuickstartFirstRun(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "quickstart")
	if err != nil {
		t.Fatalf("quickstart failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "default/agent-1 created") &&
		!strings.Contains(out, "Instance 'default/agent-1' created") {
		t.Errorf("expected create output, got: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "default", "agent-1", "instance.env")); err != nil {
		t.Errorf("agent instance.env missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".port-registry")); err != nil {
		t.Errorf("port registry missing")
	}
}

// Re-run: quickstart is idempotent — every step should report 'already'.
func TestIntegration_QuickstartIdempotent(t *testing.T) {
	root := t.TempDir()
	if _, err := claws(t, root, "quickstart"); err != nil {
		t.Fatalf("first quickstart failed: %v", err)
	}
	out, err := claws(t, root, "quickstart")
	if err != nil {
		t.Fatalf("re-run quickstart failed: %v\n%s", err, out)
	}
	for _, mustSay := range []string{
		"already initialized",
		"already configured",
		"already exists",
	} {
		if !strings.Contains(out, mustSay) {
			t.Errorf("idempotent re-run did not report %q; output:\n%s", mustSay, out)
		}
	}
}

// Custom names: quickstart [team] [agent] respects positional args.
func TestIntegration_QuickstartCustomNames(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "quickstart", "research", "sarah")
	if err != nil {
		t.Fatalf("quickstart research sarah failed: %v\n%s", err, out)
	}
	if _, err := os.Stat(filepath.Join(root, "research", "sarah", "instance.env")); err != nil {
		t.Errorf("research/sarah not created: %v", err)
	}
}

// Apply: minimal valid profile creates the agent.
func TestIntegration_ApplyMinimalProfile(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "minimal.json")
	if err := os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1",
  "kind": "Profile",
  "metadata": { "name": "test-minimal", "version": "0.1.0" },
  "team":     { "name": "smoke" },
  "agents": [ { "name": "agent-a" } ]
}`), 0644); err != nil {
		t.Fatal(err)
	}
	out, err := claws(t, root, "apply", "--file="+profile)
	if err != nil {
		t.Fatalf("apply failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "apply complete") {
		t.Errorf("apply did not report completion: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "smoke", "agent-a", "instance.env")); err != nil {
		t.Errorf("smoke/agent-a not created: %v", err)
	}
}

// Apply: dry-run does not mutate.
func TestIntegration_ApplyDryRun(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "p.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1",
  "kind": "Profile",
  "metadata": { "name": "dry", "version": "0.1.0" },
  "team":     { "name": "team-x" },
  "agents": [ { "name": "a" } ]
}`), 0644)
	out, err := claws(t, root, "apply", "--file="+profile, "--dry-run")
	if err != nil {
		t.Fatalf("dry-run apply failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "[dry-run]") {
		t.Errorf("expected dry-run marker in output: %s", out)
	}
	if _, err := os.Stat(filepath.Join(root, "team-x")); err == nil {
		t.Errorf("dry-run should not have created team-x dir")
	}
}

// Apply: idempotent re-run skips agent create.
func TestIntegration_ApplyIdempotent(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "p.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1",
  "kind": "Profile",
  "metadata": { "name": "idem", "version": "0.1.0" },
  "team":     { "name": "t" },
  "agents": [ { "name": "a" } ]
}`), 0644)
	if _, err := claws(t, root, "apply", "--file="+profile); err != nil {
		t.Fatalf("first apply failed: %v", err)
	}
	out, err := claws(t, root, "apply", "--file="+profile)
	if err != nil {
		t.Fatalf("second apply failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "already exists (skipping create)") {
		t.Errorf("idempotent re-run should skip create: %s", out)
	}
}

// Apply: rejects unknown apiVersion / kind.
func TestIntegration_ApplyRejectsBadSchema(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "bad.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "wrong/v1",
  "kind": "Profile",
  "metadata": { "name": "x" },
  "team":     { "name": "t" },
  "agents": [ { "name": "a" } ]
}`), 0644)
	out, err := claws(t, root, "apply", "--file="+profile)
	if err == nil {
		t.Errorf("expected apply to reject unsupported apiVersion; output: %s", out)
	}
}

// Apply: rejects file without team or agents.
func TestIntegration_ApplyRejectsIncomplete(t *testing.T) {
	root := t.TempDir()
	profile := filepath.Join(root, "bad.json")
	_ = os.WriteFile(profile, []byte(`{
  "apiVersion": "claws.ab0t.com/v1",
  "kind": "Profile",
  "metadata": { "name": "x" }
}`), 0644)
	if _, err := claws(t, root, "apply", "--file="+profile); err == nil {
		t.Error("expected apply to reject profile missing team and agents")
	}
}
