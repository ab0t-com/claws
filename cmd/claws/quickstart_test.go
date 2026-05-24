package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// First-run: quickstart with no args picks a random personal-assistant
// name and creates it under default/.
func TestIntegration_QuickstartFirstRun(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "quickstart")
	if err != nil {
		t.Fatalf("quickstart failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "creating personal assistant: default/") {
		t.Errorf("expected 'creating personal assistant' line, got:\n%s", out)
	}
	// Verify one of the assistant names was picked.
	picked := ""
	for _, name := range personalAssistantNames {
		if _, err := os.Stat(filepath.Join(root, "default", name, "instance.env")); err == nil {
			picked = name
			break
		}
	}
	if picked == "" {
		t.Errorf("no instance.env found for any of the curated assistant names under default/")
	}
	if _, err := os.Stat(filepath.Join(root, ".port-registry")); err != nil {
		t.Errorf("port registry missing")
	}
}

// Re-run: quickstart is idempotent — picks up the existing PA, never spawns
// a new random agent on each invocation.
func TestIntegration_QuickstartIdempotent(t *testing.T) {
	root := t.TempDir()
	if _, err := claws(t, root, "quickstart"); err != nil {
		t.Fatalf("first quickstart failed: %v", err)
	}
	// Re-run a few times; each should report 'already exists', not create new.
	for i := 0; i < 3; i++ {
		out, err := claws(t, root, "quickstart")
		if err != nil {
			t.Fatalf("re-run %d failed: %v\n%s", i, err, out)
		}
		if !strings.Contains(out, "already exists (skipping)") {
			t.Errorf("re-run %d should report 'already exists', got:\n%s", i, out)
		}
		if strings.Contains(out, "creating personal assistant") {
			t.Errorf("re-run %d should NOT create a new PA, got:\n%s", i, out)
		}
	}
	// Confirm only one agent under default/.
	entries, _ := os.ReadDir(filepath.Join(root, "default"))
	agentCount := 0
	for _, e := range entries {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(root, "default", e.Name(), "instance.env")); err == nil {
				agentCount++
			}
		}
	}
	if agentCount != 1 {
		t.Errorf("expected exactly 1 agent after multiple quickstart runs, got %d", agentCount)
	}
}

// pickAssistantName returns names only from the curated set.
func TestPickAssistantName(t *testing.T) {
	allowed := map[string]bool{}
	for _, n := range personalAssistantNames {
		allowed[n] = true
	}
	for i := 0; i < 100; i++ {
		n := pickAssistantName()
		if !allowed[n] {
			t.Fatalf("pickAssistantName returned %q which is not in the curated set", n)
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
