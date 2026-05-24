package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Namespaced lookup: "ns/name" form resolves only to that exact path.
func TestResolveTemplate_Namespaced(t *testing.T) {
	root := t.TempDir()
	tplDir := filepath.Join(root, "templates", "solo")
	_ = os.MkdirAll(tplDir, 0755)
	_ = os.WriteFile(filepath.Join(tplDir, "tg.json"), []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "tg"}, "team": {"name": "x"}, "agents": [{"name": "a"}]
}`), 0644)
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(root)

	path, err := resolveTemplate("solo/tg")
	if err != nil {
		t.Fatalf("namespaced resolve failed: %v", err)
	}
	if !strings.HasSuffix(path, "solo/tg.json") {
		t.Errorf("expected solo/tg.json, got %s", path)
	}
}

// Bare name resolves through namespace dirs when unambiguous.
func TestResolveTemplate_BareNameUnambiguous(t *testing.T) {
	root := t.TempDir()
	tplDir := filepath.Join(root, "templates", "specialty")
	_ = os.MkdirAll(tplDir, 0755)
	_ = os.WriteFile(filepath.Join(tplDir, "unique-name.json"), []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "u"}, "team": {"name": "x"}, "agents": [{"name": "a"}]
}`), 0644)
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(root)

	path, err := resolveTemplate("unique-name")
	if err != nil {
		t.Fatalf("bare-name resolve failed: %v", err)
	}
	if !strings.HasSuffix(path, "specialty/unique-name.json") {
		t.Errorf("expected specialty/unique-name.json, got %s", path)
	}
}

// Bare name errors clearly on ambiguity.
func TestResolveTemplate_BareNameAmbiguous(t *testing.T) {
	root := t.TempDir()
	for _, ns := range []string{"solo", "teams"} {
		dir := filepath.Join(root, "templates", ns)
		_ = os.MkdirAll(dir, 0755)
		_ = os.WriteFile(filepath.Join(dir, "shared.json"), []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "s"}, "team": {"name": "x"}, "agents": [{"name": "a"}]
}`), 0644)
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(root)

	_, err := resolveTemplate("shared")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error should mention 'ambiguous', got: %v", err)
	}
}

// Flat layout back-compat: a top-level templates/foo.json still resolves
// by bare name.
func TestResolveTemplate_FlatBackcompat(t *testing.T) {
	root := t.TempDir()
	tplDir := filepath.Join(root, "templates")
	_ = os.MkdirAll(tplDir, 0755)
	_ = os.WriteFile(filepath.Join(tplDir, "flat.json"), []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "f"}, "team": {"name": "x"}, "agents": [{"name": "a"}]
}`), 0644)
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(root)

	path, err := resolveTemplate("flat")
	if err != nil {
		t.Fatalf("flat-layout resolve failed: %v", err)
	}
	if !strings.HasSuffix(path, "templates/flat.json") {
		t.Errorf("expected templates/flat.json, got %s", path)
	}
}

// Path-separator and traversal attempts are rejected.
func TestResolveTemplate_RejectsTraversal(t *testing.T) {
	for _, name := range []string{
		"../../etc/passwd",
		"/absolute/path",
		"backslash\\path",
		"name/with/../traversal",
	} {
		if _, err := resolveTemplate(name); err == nil {
			t.Errorf("expected rejection for %q", name)
		}
	}
}

// listTemplates groups by namespace and sorts.
func TestListTemplates_Groups(t *testing.T) {
	root := t.TempDir()
	// Isolate XDG data dir + HOME so we don't pick up host's installed templates.
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, ".local", "share"))
	t.Setenv("HOME", root)

	for _, p := range []string{
		"templates/solo/alpha.json",
		"templates/teams/beta.json",
		"templates/flat.json",
	} {
		full := filepath.Join(root, p)
		_ = os.MkdirAll(filepath.Dir(full), 0755)
		_ = os.WriteFile(full, []byte(`{
  "apiVersion": "claws.ab0t.com/v1", "kind": "Profile",
  "metadata": {"name": "x"}, "team": {"name": "t"}, "agents": [{"name": "a"}]
}`), 0644)
	}
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	_ = os.Chdir(root)

	out := listTemplates()
	// Filter to templates we actually wrote (defensive vs binary-adjacent ones).
	wanted := map[string]bool{"alpha": true, "beta": true, "flat": true}
	var ours []templateInfo
	for _, t := range out {
		if wanted[t.Name] {
			ours = append(ours, t)
		}
	}
	if len(ours) != 3 {
		t.Fatalf("expected 3 known templates, got %d (full list: %v)", len(ours), out)
	}
	// First should be flat ("" namespace sorts first), then solo, then teams.
	if ours[0].Namespace != "" || ours[0].Name != "flat" {
		t.Errorf("expected first = flat (empty ns), got %s/%s", ours[0].Namespace, ours[0].Name)
	}
	if ours[1].Namespace != "solo" || ours[1].Name != "alpha" {
		t.Errorf("expected second = solo/alpha, got %s/%s", ours[1].Namespace, ours[1].Name)
	}
	if ours[2].Namespace != "teams" || ours[2].Name != "beta" {
		t.Errorf("expected third = teams/beta, got %s/%s", ours[2].Namespace, ours[2].Name)
	}
}
