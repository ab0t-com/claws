package main

import (
	"os"
	"path/filepath"
	"testing"
)

// CLAWS_ROOT has highest precedence.
func TestDefaultRoot_ClawsRootWins(t *testing.T) {
	t.Setenv("CLAWS_ROOT", "/tmp/claws-test-explicit")
	t.Setenv("OPENCLAW_ROOT", "/tmp/should-be-ignored")
	if got := defaultRoot(); got != "/tmp/claws-test-explicit" {
		t.Errorf("CLAWS_ROOT should win; got %q", got)
	}
}

// OPENCLAW_ROOT still respected when CLAWS_ROOT unset.
func TestDefaultRoot_OpenclawRootLegacy(t *testing.T) {
	t.Setenv("CLAWS_ROOT", "")
	t.Setenv("OPENCLAW_ROOT", "/tmp/legacy-explicit")
	if got := defaultRoot(); got != "/tmp/legacy-explicit" {
		t.Errorf("OPENCLAW_ROOT should be honored as legacy; got %q", got)
	}
}

// Fresh install (no env, no existing dirs) → ~/.claws-workspace.
func TestDefaultRoot_FreshInstallDefault(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("CLAWS_ROOT", "")
	t.Setenv("OPENCLAW_ROOT", "")
	want := filepath.Join(home, ".claws-workspace")
	if got := defaultRoot(); got != want {
		t.Errorf("fresh install default; want %q got %q", want, got)
	}
}

// If ~/.claws-workspace already exists, use it.
func TestDefaultRoot_NewDefaultIfExists(t *testing.T) {
	home := t.TempDir()
	newDir := filepath.Join(home, ".claws-workspace")
	if err := os.MkdirAll(newDir, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CLAWS_ROOT", "")
	t.Setenv("OPENCLAW_ROOT", "")
	if got := defaultRoot(); got != newDir {
		t.Errorf("existing ~/.claws-workspace should win; got %q", got)
	}
}

// Back-compat: legacy ~/.openclaw with .port-registry → use it.
func TestDefaultRoot_LegacyBackcompat(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".openclaw")
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, ".port-registry"), []byte("1:foo\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CLAWS_ROOT", "")
	t.Setenv("OPENCLAW_ROOT", "")
	if got := defaultRoot(); got != legacy {
		t.Errorf("populated ~/.openclaw should be used for back-compat; got %q want %q", got, legacy)
	}
}

// Empty ~/.openclaw (no .port-registry) → fresh install behavior.
func TestDefaultRoot_EmptyLegacyDirIgnored(t *testing.T) {
	home := t.TempDir()
	legacy := filepath.Join(home, ".openclaw")
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("CLAWS_ROOT", "")
	t.Setenv("OPENCLAW_ROOT", "")
	want := filepath.Join(home, ".claws-workspace")
	if got := defaultRoot(); got != want {
		t.Errorf("empty ~/.openclaw should be ignored; want %q got %q", want, got)
	}
}
