package main

import (
	"testing"
)

func TestIndexOfStr(t *testing.T) {
	tests := []struct {
		s, sub string
		want   int
	}{
		{`"Name":"container-1"`, `"Name":"`, 0},
		{`{"ID":"abc","Name":"my-container"}`, `"Name":"`, 12},
		{`no match here`, `"Name":"`, -1},
		{"", `"Name":"`, -1},
	}
	for _, tc := range tests {
		got := indexOfStr(tc.s, tc.sub)
		if got != tc.want {
			t.Errorf("indexOfStr(%q, %q) = %d, want %d", tc.s, tc.sub, got, tc.want)
		}
	}
}

func TestResolveContainerName_Fallback(t *testing.T) {
	paths := testPaths(t)
	// No running container, should fall back to conventional name
	name := resolveContainerName(paths, "alpha")
	if name != "openclaw-alpha-openclaw-gateway-1" {
		t.Errorf("fallback name should be conventional, got %s", name)
	}
}

func TestResolveContainerName_Grouped_Fallback(t *testing.T) {
	paths := testPaths(t)
	name := resolveContainerName(paths, "backend/sarah")
	if name != "openclaw-backend-sarah-openclaw-gateway-1" {
		t.Errorf("grouped fallback name should be conventional, got %s", name)
	}
}

func TestContainerRAM_NoDocker(t *testing.T) {
	paths := testPaths(t)
	ram := containerRAM(paths, "nonexistent")
	if ram != "—" {
		t.Errorf("should return dash for nonexistent container, got %s", ram)
	}
}
