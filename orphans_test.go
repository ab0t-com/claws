package main

import "testing"

func TestContainerProject(t *testing.T) {
	cases := []struct {
		container string
		want      string
	}{
		// ungrouped instance, gateway
		{"openclaw-alpha-openclaw-gateway-1", "openclaw-alpha"},
		// grouped instance, gateway
		{"openclaw-team-sarah-openclaw-gateway-1", "openclaw-team-sarah"},
		// CLI sidecar (when running as a long-lived service)
		{"openclaw-alpha-openclaw-cli-1", "openclaw-alpha"},
		// the canonical "bob" orphan from the live host
		{"openclaw-bob-openclaw-gateway-1", "openclaw-bob"},

		// Not a clawctl-managed container shape.
		{"random-container", ""},
		{"openclaw-something-weird", ""},     // missing service suffix
		{"openclaw-gateway-1", ""},           // missing project name
		{"openclaw-alpha-openclaw-gateway", ""}, // missing replica
		{"", ""},
	}
	for _, tc := range cases {
		if got := containerProject(tc.container); got != tc.want {
			t.Errorf("containerProject(%q) = %q, want %q", tc.container, got, tc.want)
		}
	}
}

func TestColorStatus(t *testing.T) {
	// We only care that colors are applied for the expected states; the
	// exact escape codes are passed through unchanged.
	red := "[red]"
	yellow := "[yellow]"
	nc := "[/]"

	cases := []struct {
		status string
		want   string
	}{
		{"running", "[yellow]running[/]"},
		{"restarting", "[yellow]restarting[/]"},
		{"exited", "[red]exited[/]"},
		{"dead", "[red]dead[/]"},
		{"removing", "[red]removing[/]"},
		{"created", "created"}, // no color
		{"", ""},
	}
	for _, tc := range cases {
		if got := colorStatus(tc.status, yellow, red, nc); got != tc.want {
			t.Errorf("colorStatus(%q) = %q, want %q", tc.status, got, tc.want)
		}
	}
}
