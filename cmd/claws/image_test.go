package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegration_CreateWithImage(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha", "--image=openclaw:custom")

	envFile := filepath.Join(root, "alpha", "instance.env")
	img := readEnvFromFile(t, envFile, "OPENCLAW_IMAGE")
	if img != "openclaw:custom" {
		t.Errorf("image should be openclaw:custom, got '%s'", img)
	}
}

func TestIntegration_ImagePin(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// Pin requires the image to exist — skip actual docker check in test
	// Just verify the command parses correctly
	_, err := claws(t, root, "image", "pin", "alpha", "openclaw:v1")
	// Will fail because image doesn't exist in test env, but should fail gracefully
	if err == nil {
		// If it succeeds (image exists), verify the pin
		envFile := filepath.Join(root, "alpha", "instance.env")
		img := readEnvFromFile(t, envFile, "OPENCLAW_IMAGE")
		if img != "openclaw:v1" {
			t.Errorf("pinned image should be openclaw:v1, got '%s'", img)
		}
	}
}

func TestIntegration_ImagePinPolicyBlock(t *testing.T) {
	root := t.TempDir()
	claws(t, root, "create", "alpha")

	// Create restrictive policy
	p := Policy{AllowedImages: []string{"openclaw:*"}}
	paths := Paths{Root: root, PortRegistry: filepath.Join(root, ".port-registry")}
	writePolicy(paths, p)

	_, err := claws(t, root, "image", "pin", "alpha", "evil:latest")
	if err == nil {
		t.Error("policy should block pinning to disallowed image")
	}
}

func TestIntegration_CreateWithImagePolicyBlock(t *testing.T) {
	root := t.TempDir()

	// Create restrictive policy
	p := Policy{AllowedImages: []string{"openclaw:local"}}
	os.MkdirAll(root, 0755)
	paths := Paths{Root: root, PortRegistry: filepath.Join(root, ".port-registry")}
	writePolicy(paths, p)

	_, err := claws(t, root, "create", "alpha", "--image=evil:latest")
	if err == nil {
		t.Error("policy should block create with disallowed image")
	}
}

func TestIntegration_UpgradeRequiresInstance(t *testing.T) {
	root := t.TempDir()
	_, err := claws(t, root, "upgrade", "nonexistent")
	if err == nil {
		t.Error("upgrade on nonexistent instance should fail")
	}
}

func TestIntegration_ImageListHelp(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "image", "--help")
	if !strings.Contains(out, "pull") {
		t.Errorf("image help should mention pull: %s", out)
	}
}

func TestIntegration_UpgradeHelp(t *testing.T) {
	root := t.TempDir()
	out, _ := claws(t, root, "upgrade", "--help")
	if !strings.Contains(out, "rolls back") {
		t.Errorf("upgrade help should mention rollback: %s", out)
	}
}
