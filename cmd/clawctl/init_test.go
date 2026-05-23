package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_Init(t *testing.T) {
	root := t.TempDir()
	// Remove the root to test that init creates it
	os.RemoveAll(root)

	out, err := clawctl(t, root, "init")
	if err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Init complete") {
		t.Errorf("should show completion message: %s", out)
	}

	// Check dirs created
	for _, sub := range []string{"shared/skills", "shared/workspace"} {
		if _, err := os.Stat(filepath.Join(root, sub)); err != nil {
			t.Errorf("init should create %s", sub)
		}
	}

	// Check port registry
	if _, err := os.Stat(filepath.Join(root, ".port-registry")); err != nil {
		t.Error("init should create .port-registry")
	}

	// Check defaults.json
	if _, err := os.Stat(filepath.Join(root, "defaults.json")); err != nil {
		t.Error("init should create defaults.json")
	}
}

func TestIntegration_InitIdempotent(t *testing.T) {
	root := t.TempDir()

	// First init
	clawctl(t, root, "init")

	// Write a custom defaults to verify it's not overwritten
	customDefaults := `{"tools":{"profile":"custom"}}`
	os.WriteFile(filepath.Join(root, "defaults.json"), []byte(customDefaults), 0644)

	// Second init should not error
	out, err := clawctl(t, root, "init")
	if err != nil {
		t.Fatalf("second init failed: %v\n%s", err, out)
	}

	// Custom defaults should not be overwritten
	data, _ := os.ReadFile(filepath.Join(root, "defaults.json"))
	if !strings.Contains(string(data), "custom") {
		t.Error("init should not overwrite existing defaults.json")
	}
}

func TestIntegration_InitThenCreate(t *testing.T) {
	root := t.TempDir()
	os.RemoveAll(root)

	clawctl(t, root, "init")
	out, err := clawctl(t, root, "create", "alpha")
	if err != nil {
		t.Fatalf("create after init failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Instance 'alpha' created") {
		t.Errorf("should create instance successfully: %s", out)
	}
}

// 1d: Verify init creates policy.json and .access.json
func TestIntegration_InitCreatesPolicy(t *testing.T) {
	root := t.TempDir()
	os.RemoveAll(root)

	out, err := clawctl(t, root, "init")
	if err != nil {
		t.Fatalf("init failed: %v\n%s", err, out)
	}

	// policy.json should exist with secure defaults
	policyPath := filepath.Join(root, "policy.json")
	data, err := os.ReadFile(policyPath)
	if err != nil {
		t.Fatalf("init should create policy.json: %v", err)
	}
	if !strings.Contains(string(data), "loopback") {
		t.Error("policy should contain loopback bind mode")
	}
	if !strings.Contains(string(data), "\"auditLog\": true") {
		t.Error("policy should have auditLog enabled")
	}

	// .access.json should exist
	accessPath := filepath.Join(root, ".access.json")
	data, err = os.ReadFile(accessPath)
	if err != nil {
		t.Fatalf("init should create .access.json: %v", err)
	}
	if !strings.Contains(string(data), "admin") {
		t.Error(".access.json should contain admin role")
	}
	if !strings.Contains(string(data), "operator") {
		t.Error(".access.json should contain operator role")
	}
}

// 1d: Verify init doesn't overwrite existing policy/access
func TestIntegration_InitPreservesExistingPolicy(t *testing.T) {
	root := t.TempDir()
	os.RemoveAll(root)

	// First init
	clawctl(t, root, "init")

	// Modify policy
	custom := `{"maxInstances": 99}`
	os.WriteFile(filepath.Join(root, "policy.json"), []byte(custom), 0600)

	// Second init should not overwrite
	clawctl(t, root, "init")
	data, _ := os.ReadFile(filepath.Join(root, "policy.json"))
	if !strings.Contains(string(data), "99") {
		t.Error("init should not overwrite existing policy.json")
	}
}
