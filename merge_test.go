package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDeepMerge(t *testing.T) {
	base := map[string]any{
		"a": "base",
		"b": map[string]any{"x": "1", "y": "2"},
		"c": "keep",
	}
	override := map[string]any{
		"a": "override",
		"b": map[string]any{"x": "overridden", "z": "new"},
		"d": "added",
	}

	result := deepMerge(base, override)

	if result["a"] != "override" {
		t.Errorf("a: expected 'override', got '%v'", result["a"])
	}
	if result["c"] != "keep" {
		t.Errorf("c: expected 'keep', got '%v'", result["c"])
	}
	if result["d"] != "added" {
		t.Errorf("d: expected 'added', got '%v'", result["d"])
	}
	b := result["b"].(map[string]any)
	if b["x"] != "overridden" {
		t.Errorf("b.x: expected 'overridden', got '%v'", b["x"])
	}
	if b["y"] != "2" {
		t.Errorf("b.y: expected '2', got '%v'", b["y"])
	}
	if b["z"] != "new" {
		t.Errorf("b.z: expected 'new', got '%v'", b["z"])
	}
}

func TestMergeConfigDefaults(t *testing.T) {
	tmp := t.TempDir()

	// defaults
	writeJSON(t, filepath.Join(tmp, "defaults.json"), map[string]any{
		"tools": map[string]any{"profile": "coding"},
		"agents": map[string]any{
			"defaults": map[string]any{"model": map[string]any{"primary": "test/v1"}},
		},
	})

	// skeleton
	writeJSON(t, filepath.Join(tmp, "skeleton.json"), map[string]any{
		"gateway": map[string]any{"port": float64(18789)},
	})

	out := filepath.Join(tmp, "output.json")
	err := mergeConfigLayers(
		filepath.Join(tmp, "defaults.json"),
		"", // no group defaults
		filepath.Join(tmp, "nonexistent.json"), // no template
		"",
		filepath.Join(tmp, "skeleton.json"),
		out,
	)
	if err != nil {
		t.Fatal(err)
	}

	result := readJSON(t, out)
	tools := result["tools"].(map[string]any)
	if tools["profile"] != "coding" {
		t.Errorf("tools.profile: expected 'coding', got '%v'", tools["profile"])
	}
	gw := result["gateway"].(map[string]any)
	if gw["port"] != float64(18789) {
		t.Errorf("gateway.port: expected 18789, got '%v'", gw["port"])
	}
}

func TestMergeConfigTemplate(t *testing.T) {
	tmp := t.TempDir()

	// template with channels that should be stripped
	writeJSON(t, filepath.Join(tmp, "template.json"), map[string]any{
		"tools": map[string]any{"profile": "coding", "alsoAllow": []any{"message"}},
		"gateway": map[string]any{"port": float64(99999), "token": "old-token"},
		"channels": map[string]any{
			"whatsapp": map[string]any{
				"enabled":        true,
				"allowFrom":      []any{"+1234"},
				"groups":         map[string]any{"abc@g.us": map[string]any{}},
				"groupAllowFrom": []any{"+1234"},
				"actions":        map[string]any{"sendMessage": true, "reactions": true},
			},
		},
	})

	// skeleton overrides gateway
	writeJSON(t, filepath.Join(tmp, "skeleton.json"), map[string]any{
		"gateway": map[string]any{"port": float64(18789), "auth": map[string]any{"token": "new-token"}},
	})

	out := filepath.Join(tmp, "output.json")
	err := mergeConfigLayers(
		filepath.Join(tmp, "nonexistent.json"), // no defaults
		"", // no group defaults
		filepath.Join(tmp, "template.json"),
		"source-instance",
		filepath.Join(tmp, "skeleton.json"),
		out,
	)
	if err != nil {
		t.Fatal(err)
	}

	result := readJSON(t, out)

	// tools should come from template
	tools := result["tools"].(map[string]any)
	if tools["profile"] != "coding" {
		t.Errorf("tools.profile: expected 'coding', got '%v'", tools["profile"])
	}

	// gateway should come from skeleton (overrides template)
	gw := result["gateway"].(map[string]any)
	if gw["port"] != float64(18789) {
		t.Errorf("gateway.port: expected 18789, got '%v'", gw["port"])
	}

	// channels should have allowFrom/groups stripped
	ch := result["channels"].(map[string]any)["whatsapp"].(map[string]any)
	if _, ok := ch["allowFrom"]; ok {
		t.Error("channels.whatsapp.allowFrom should be stripped")
	}
	if _, ok := ch["groups"]; ok {
		t.Error("channels.whatsapp.groups should be stripped")
	}
	if _, ok := ch["groupAllowFrom"]; ok {
		t.Error("channels.whatsapp.groupAllowFrom should be stripped")
	}
	if _, ok := ch["actions"]; ok {
		t.Error("channels.whatsapp.actions should be stripped from template")
	}
	if ch["enabled"] != true {
		t.Error("channels.whatsapp.enabled should be preserved")
	}
}

func writeJSON(t *testing.T, path string, data map[string]any) {
	t.Helper()
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}
