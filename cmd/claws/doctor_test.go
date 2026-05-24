package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_Version(t *testing.T) {
	root := t.TempDir()
	out, err := claws(t, root, "version")
	if err != nil {
		t.Fatalf("version failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "claws") {
		t.Errorf("version should contain 'claws': %s", out)
	}
	if !strings.Contains(out, "go:") {
		t.Errorf("version should show go version: %s", out)
	}
}

func TestIntegration_Doctor(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "doctor")
	// Doctor may fail (Docker not running in test env) but should not crash
	if !strings.Contains(out, "doctor") {
		t.Errorf("doctor should show header: %s", out)
	}
	if !strings.Contains(out, "passed") {
		t.Errorf("doctor should show summary: %s", out)
	}
}

func TestIntegration_DoctorShowsRoot(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "doctor")
	if !strings.Contains(out, "OPENCLAW_ROOT") {
		t.Errorf("doctor should check OPENCLAW_ROOT: %s", out)
	}
}

func TestIntegration_DoctorFix(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// Break permissions intentionally
	envFile := filepath.Join(root, "alpha", "instance.env")
	os.Chmod(envFile, 0644)

	credDir := filepath.Join(root, "alpha", "credentials")
	os.WriteFile(filepath.Join(credDir, "test.json"), []byte("{}"), 0644)

	regFile := filepath.Join(root, ".port-registry")
	os.Chmod(regFile, 0644)

	// Run doctor --fix
	out, _ := claws(t, root, "doctor", "--fix")
	if !strings.Contains(out, "Fixing") {
		t.Errorf("doctor --fix should report fixing: %s", out)
	}

	// Verify permissions are now 0600
	fi, _ := os.Stat(envFile)
	if fi.Mode().Perm() != 0600 {
		t.Errorf("instance.env should be 0600 after fix, got %04o", fi.Mode().Perm())
	}

	fi, _ = os.Stat(filepath.Join(credDir, "test.json"))
	if fi.Mode().Perm() != 0600 {
		t.Errorf("credential file should be 0600 after fix, got %04o", fi.Mode().Perm())
	}

	fi, _ = os.Stat(regFile)
	if fi.Mode().Perm() != 0600 {
		t.Errorf(".port-registry should be 0600 after fix, got %04o", fi.Mode().Perm())
	}
}

func TestIntegration_CreateDefaultsToLoopback(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	envFile := filepath.Join(root, "alpha", "instance.env")
	bind := readEnvFromFile(t, envFile, "OPENCLAW_GATEWAY_BIND")
	if bind != "loopback" {
		t.Errorf("default bind should be loopback, got '%s'", bind)
	}
}

func TestIntegration_CreateWithBindLan(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha", "--bind=lan")

	envFile := filepath.Join(root, "alpha", "instance.env")
	bind := readEnvFromFile(t, envFile, "OPENCLAW_GATEWAY_BIND")
	if bind != "lan" {
		t.Errorf("explicit --bind=lan should set lan, got '%s'", bind)
	}
}
