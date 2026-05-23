package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 2c: First-run detection — uninitialized system
func TestIntegration_FirstRunUninitialized(t *testing.T) {
	root := t.TempDir()
	os.RemoveAll(root) // ensure root doesn't exist

	out, _ := clawctl(t, root) // no args
	if !strings.Contains(out, "Welcome to clawctl") {
		t.Errorf("uninitialized no-args should show welcome: %s", out)
	}
	if !strings.Contains(out, "clawctl setup") {
		t.Errorf("should suggest setup command: %s", out)
	}
}

// 2c: First-run detection — initialized system with no agents
func TestIntegration_FirstRunInitializedNoAgents(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")

	out, _ := clawctl(t, root) // no args
	if !strings.Contains(out, "No agents running yet") {
		t.Errorf("initialized no-args with no agents should say so: %s", out)
	}
}

// 2c: First-run detection — initialized system with agents
func TestIntegration_FirstRunInitializedWithAgents(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")
	clawctl(t, root, "create", "alice")

	out, _ := clawctl(t, root) // no args
	if !strings.Contains(out, "1 agent(s) registered") {
		t.Errorf("should show agent count: %s", out)
	}
}

// 6d: Tiered help — topic routing
func TestIntegration_HelpTopicSetup(t *testing.T) {
	root := t.TempDir()
	out, err := clawctl(t, root, "help", "setup")
	if err != nil {
		t.Fatalf("help setup failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Getting Started") {
		t.Errorf("help setup should show getting started guide: %s", out)
	}
}

func TestIntegration_HelpTopicSecurity(t *testing.T) {
	root := t.TempDir()
	out, err := clawctl(t, root, "help", "security")
	if err != nil {
		t.Fatalf("help security failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Security Guide") {
		t.Errorf("help security should show security guide: %s", out)
	}
}

func TestIntegration_HelpTopicUnknown(t *testing.T) {
	root := t.TempDir()
	out, _ := clawctl(t, root, "help", "nonexistent")
	if !strings.Contains(out, "Unknown help topic") {
		t.Errorf("unknown topic should say so: %s", out)
	}
	if !strings.Contains(out, "Available topics") {
		t.Errorf("should list available topics: %s", out)
	}
}

func TestIntegration_HelpTopicFallsBackToSubcommand(t *testing.T) {
	root := t.TempDir()
	out, err := clawctl(t, root, "help", "create")
	if err != nil {
		t.Fatalf("help create failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Usage: clawctl create") {
		t.Errorf("help <command> should show subcommand help: %s", out)
	}
}

// 7b: Team create shortcut
func TestIntegration_TeamCreate(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")

	out, err := clawctl(t, root, "team", "create", "myteam")
	if err != nil {
		t.Fatalf("team create failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Group 'myteam' created") {
		t.Errorf("should create group: %s", out)
	}
	if !strings.Contains(out, "Shared resources enabled") {
		t.Errorf("should enable shared resources: %s", out)
	}

	// Verify shared dirs exist
	for _, sub := range []string{"shared/skills", "shared/workspace", "shared/hooks", "tasks"} {
		if _, err := os.Stat(filepath.Join(root, "myteam", sub)); err != nil {
			t.Errorf("team create should create %s: %v", sub, err)
		}
	}

	// Verify .group.json
	if _, err := os.Stat(filepath.Join(root, "myteam", ".group.json")); err != nil {
		t.Error("team create should create .group.json")
	}
}

// 5c: Unified status overview
func TestIntegration_StatusOverview(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")
	clawctl(t, root, "create", "alice")

	out, err := clawctl(t, root, "status")
	if err != nil {
		// status may return error if containers are down, that's fine
		_ = err
	}
	if !strings.Contains(out, "Instances (1)") {
		t.Errorf("status should show instance count: %s", out)
	}
	if !strings.Contains(out, "alice") {
		t.Errorf("status should list alice: %s", out)
	}
	if !strings.Contains(out, "Policy:") {
		t.Errorf("status should show policy section: %s", out)
	}
}

// 5c: Unified status with no instances
func TestIntegration_StatusEmpty(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")

	out, _ := clawctl(t, root, "status")
	if !strings.Contains(out, "No instances registered") {
		t.Errorf("status with no instances should say so: %s", out)
	}
}

// 4e: Inline create flags parsed (auth/channel would fail without containers, but flags should parse)
func TestIntegration_CreateInlineFlagsParsed(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")

	// --auth=codex should be accepted (will fail at auth step, but create should succeed)
	out, _ := clawctl(t, root, "create", "bob", "--auth=codex")
	if !strings.Contains(out, "Instance 'bob' created") {
		t.Errorf("create with --auth should still create instance: %s", out)
	}
}

func TestIntegration_CreateInlineTelegramParsed(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")

	// --telegram=TOKEN should be accepted as a flag (channel add will fail, but create should succeed)
	out, _ := clawctl(t, root, "create", "charlie", "--telegram=fake-token")
	if !strings.Contains(out, "Instance 'charlie' created") {
		t.Errorf("create with --telegram should still create instance: %s", out)
	}
}

// 8b: Default tool profile on create
func TestIntegration_CreateSetsToolProfile(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")
	clawctl(t, root, "create", "alpha")

	configPath := filepath.Join(root, "alpha", "openclaw.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("should have config file: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("invalid config JSON: %v", err)
	}

	tools, ok := cfg["tools"].(map[string]any)
	if !ok {
		t.Fatal("config should have tools section")
	}
	profile, ok := tools["profile"].(string)
	if !ok || profile != "coding" {
		t.Errorf("tools.profile should be 'coding', got: %v", tools["profile"])
	}
}

// 8b: Tool profile not overwritten if set by defaults.json
func TestIntegration_CreatePreservesExistingToolProfile(t *testing.T) {
	root := t.TempDir()
	clawctl(t, root, "init")

	// Set a custom tool profile in defaults
	defaults := `{"tools":{"profile":"research"}}`
	os.WriteFile(filepath.Join(root, "defaults.json"), []byte(defaults), 0644)

	clawctl(t, root, "create", "beta")
	configPath := filepath.Join(root, "beta", "openclaw.json")
	data, _ := os.ReadFile(configPath)

	var cfg map[string]any
	json.Unmarshal(data, &cfg)
	tools, _ := cfg["tools"].(map[string]any)
	if profile, _ := tools["profile"].(string); profile != "research" {
		t.Errorf("should preserve existing tool profile 'research', got: %v", profile)
	}
}

// 3n: Setup help text
func TestIntegration_SetupHelp(t *testing.T) {
	root := t.TempDir()
	out, err := clawctl(t, root, "setup", "--help")
	if err != nil {
		t.Fatalf("setup --help failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Guided interactive onboarding") {
		t.Errorf("setup help should describe the command: %s", out)
	}
	if !strings.Contains(out, "--non-interactive") {
		t.Errorf("setup help should mention non-interactive mode: %s", out)
	}
}
