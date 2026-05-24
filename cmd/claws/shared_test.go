package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestYamlVolume_Normal(t *testing.T) {
	got := yamlVolume(6, "/home/user/.openclaw/skills", "/home/node/.openclaw/bundled-skills", "ro")
	expected := `      - "/home/user/.openclaw/skills:/home/node/.openclaw/bundled-skills:ro"`
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestYamlVolume_Spaces(t *testing.T) {
	got := yamlVolume(6, "/home/my user/path with spaces", "/container/path", "rw")
	if !strings.Contains(got, `"/home/my user/path with spaces:/container/path:rw"`) {
		t.Errorf("paths with spaces should be quoted: %s", got)
	}
}

func TestYamlVolume_NoMode(t *testing.T) {
	got := yamlVolume(6, "/host", "/container", "")
	expected := `      - "/host:/container"`
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestParseSharedFlags(t *testing.T) {
	tests := []struct {
		args     []string
		skills   bool
		workspace bool
		hooks    bool
	}{
		{[]string{"--shared-skills"}, true, false, false},
		{[]string{"--shared-workspace"}, false, true, false},
		{[]string{"--shared-hooks"}, false, false, true},
		{[]string{"--shared"}, true, true, false},
		{[]string{"--skills"}, true, false, false},
		{[]string{"--workspace"}, false, true, false},
		{[]string{"--all"}, true, true, false},
		{[]string{"--skills", "--workspace", "--hooks"}, true, true, true},
		{[]string{}, false, false, false},
	}

	for _, tc := range tests {
		f := parseSharedFlags(tc.args)
		if f.Skills != tc.skills || f.Workspace != tc.workspace || f.Hooks != tc.hooks {
			t.Errorf("parseSharedFlags(%v) = skills=%v,ws=%v,hooks=%v; want %v,%v,%v",
				tc.args, f.Skills, f.Workspace, f.Hooks, tc.skills, tc.workspace, tc.hooks)
		}
	}
}

func TestRebuildOverride(t *testing.T) {
	paths := testPaths(t)
	name := "test-instance"
	dir := filepath.Join(paths.Root, name)
	os.MkdirAll(dir, 0755)
	envFile := filepath.Join(dir, "instance.env")

	// No shared flags — no override file
	os.WriteFile(envFile, []byte("INSTANCE_NAME=test-instance\n"), 0644)
	rebuildOverride(paths, name)
	if _, err := os.Stat(filepath.Join(dir, "docker-compose.override.yml")); err == nil {
		t.Error("override should not exist with no shared flags")
	}

	// Add skills flag
	setSharedFlag(envFile, "SHARED_SKILLS", true)
	rebuildOverride(paths, name)

	override, err := os.ReadFile(filepath.Join(dir, "docker-compose.override.yml"))
	if err != nil {
		t.Fatal("override file should exist after setting SHARED_SKILLS")
	}
	content := string(override)
	if !strings.Contains(content, "bundled-skills") {
		t.Error("override should contain bundled-skills mount")
	}
	if !strings.Contains(content, "OPENCLAW_BUNDLED_SKILLS_DIR") {
		t.Error("override should contain OPENCLAW_BUNDLED_SKILLS_DIR env")
	}
	if _, err := os.Stat(filepath.Join(paths.SharedDir, "skills")); err != nil {
		t.Error("shared/skills dir should be created")
	}

	// Add workspace flag
	setSharedFlag(envFile, "SHARED_WORKSPACE", true)
	rebuildOverride(paths, name)

	override, _ = os.ReadFile(filepath.Join(dir, "docker-compose.override.yml"))
	content = string(override)
	if !strings.Contains(content, "/shared:rw") {
		t.Error("override should contain shared workspace mount")
	}

	// Remove skills, keep workspace
	setSharedFlag(envFile, "SHARED_SKILLS", false)
	rebuildOverride(paths, name)

	override, _ = os.ReadFile(filepath.Join(dir, "docker-compose.override.yml"))
	content = string(override)
	if strings.Contains(content, "bundled-skills") {
		t.Error("override should NOT contain bundled-skills after unshare")
	}
	if !strings.Contains(content, "/shared:rw") {
		t.Error("override should still contain shared workspace")
	}

	// Remove all — override file should be deleted
	setSharedFlag(envFile, "SHARED_WORKSPACE", false)
	rebuildOverride(paths, name)
	if _, err := os.Stat(filepath.Join(dir, "docker-compose.override.yml")); err == nil {
		t.Error("override should be deleted when no shared flags")
	}
}

func TestSetSharedFlag(t *testing.T) {
	tmp := t.TempDir()
	envFile := filepath.Join(tmp, "instance.env")
	os.WriteFile(envFile, []byte("INSTANCE_NAME=test\nOPENCLAW_GATEWAY_PORT=18789\n"), 0644)

	// Set
	setSharedFlag(envFile, "SHARED_SKILLS", true)
	data, _ := os.ReadFile(envFile)
	if !strings.Contains(string(data), "SHARED_SKILLS=true") {
		t.Error("should contain SHARED_SKILLS=true")
	}
	// Original lines preserved
	if !strings.Contains(string(data), "INSTANCE_NAME=test") {
		t.Error("should preserve existing lines")
	}

	// Unset
	setSharedFlag(envFile, "SHARED_SKILLS", false)
	data, _ = os.ReadFile(envFile)
	if strings.Contains(string(data), "SHARED_SKILLS") {
		t.Error("should not contain SHARED_SKILLS after unset")
	}
}
