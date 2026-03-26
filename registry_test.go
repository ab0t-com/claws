package main

import (
	"os"
	"path/filepath"
	"testing"
)

func testPaths(t *testing.T) Paths {
	t.Helper()
	tmp := t.TempDir()
	return Paths{
		Root:            tmp,
		PortRegistry:    filepath.Join(tmp, ".port-registry"),
		SharedDir:       filepath.Join(tmp, "shared"),
		ComposeTemplate: filepath.Join(tmp, "docker-compose.yml"), // won't be used in unit tests
	}
}

func TestNextIndex(t *testing.T) {
	paths := testPaths(t)

	idx, err := nextIndex(paths)
	if err != nil {
		t.Fatal(err)
	}
	if idx != 0 {
		t.Errorf("first index should be 0, got %d", idx)
	}

	registerPort(paths, 0, "alpha")
	idx, _ = nextIndex(paths)
	if idx != 1 {
		t.Errorf("second index should be 1, got %d", idx)
	}

	registerPort(paths, 1, "bravo")
	idx, _ = nextIndex(paths)
	if idx != 2 {
		t.Errorf("third index should be 2, got %d", idx)
	}
}

func TestPortReuse(t *testing.T) {
	paths := testPaths(t)

	registerPort(paths, 0, "alpha")
	registerPort(paths, 1, "bravo")
	unregisterPort(paths, "alpha")

	idx, _ := nextIndex(paths)
	if idx != 0 {
		t.Errorf("should reuse freed index 0, got %d", idx)
	}
}

func TestRegistryReadWrite(t *testing.T) {
	paths := testPaths(t)

	registerPort(paths, 0, "alpha")
	registerPort(paths, 1, "bravo")
	registerPort(paths, 2, "charlie")

	entries, err := readRegistry(paths)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Name != "alpha" || entries[0].Index != 0 {
		t.Errorf("entry 0: expected 0:alpha, got %d:%s", entries[0].Index, entries[0].Name)
	}
}

func TestUnregister(t *testing.T) {
	paths := testPaths(t)

	registerPort(paths, 0, "alpha")
	registerPort(paths, 1, "bravo")
	unregisterPort(paths, "alpha")

	entries, _ := readRegistry(paths)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after unregister, got %d", len(entries))
	}
	if entries[0].Name != "bravo" {
		t.Errorf("remaining entry should be bravo, got %s", entries[0].Name)
	}
}

func TestInstanceCount(t *testing.T) {
	paths := testPaths(t)

	if c := instanceCount(paths); c != 0 {
		t.Errorf("empty registry count should be 0, got %d", c)
	}

	registerPort(paths, 0, "alpha")
	registerPort(paths, 1, "bravo")

	if c := instanceCount(paths); c != 2 {
		t.Errorf("count should be 2, got %d", c)
	}
}

func TestPortForIndex(t *testing.T) {
	// With default base port
	os.Setenv("CLAWCTL_BASE_PORT", "18789")
	defer os.Unsetenv("CLAWCTL_BASE_PORT")

	if p := portForIndex(0); p != 18789 {
		t.Errorf("index 0: expected 18789, got %d", p)
	}
	if p := portForIndex(1); p != 18889 {
		t.Errorf("index 1: expected 18889, got %d", p)
	}
	if p := portForIndex(2); p != 18989 {
		t.Errorf("index 2: expected 18989, got %d", p)
	}
}
