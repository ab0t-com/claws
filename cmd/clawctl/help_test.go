package main

import (
	"strings"
	"testing"
)

func TestIntegration_SubcommandHelp(t *testing.T) {
	root := t.TempDir()

	commands := []string{"create", "start", "stop", "list", "status", "remove",
		"health", "logs", "exec", "auth", "backup", "restore", "group", "task",
		"storage", "proxy", "init", "version", "doctor", "dashboard", "tunnel",
		"activity", "share", "unshare", "migrate", "stats"}

	for _, cmd := range commands {
		out, err := clawctl(t, root, cmd, "--help")
		if err != nil {
			t.Errorf("%s --help failed: %v", cmd, err)
			continue
		}
		if !strings.Contains(out, "Usage:") {
			t.Errorf("%s --help should contain 'Usage:', got:\n%s", cmd, out)
		}
	}
}

func TestIntegration_CreateHelp_HasExamples(t *testing.T) {
	root := t.TempDir()
	out, _ := clawctl(t, root, "create", "--help")
	if !strings.Contains(out, "--from=") {
		t.Errorf("create --help should document --from flag: %s", out)
	}
	if !strings.Contains(out, "--role=") {
		t.Errorf("create --help should document --role flag: %s", out)
	}
	if !strings.Contains(out, "Examples:") {
		t.Errorf("create --help should have examples: %s", out)
	}
}

func TestPrintSubcommandHelp_NoMatch(t *testing.T) {
	// Unknown command should not print help
	if printSubcommandHelp("nonexistent", []string{"--help"}) {
		t.Error("unknown command should return false")
	}
}

func TestPrintSubcommandHelp_NoFlag(t *testing.T) {
	// No --help flag should return false
	if printSubcommandHelp("create", []string{"my-instance"}) {
		t.Error("should return false without --help flag")
	}
}
