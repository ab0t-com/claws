package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateName(t *testing.T) {
	valid := []string{"sarah", "my-bot", "a", "test-bot-1", "abcdefghijklmnopqrstuvwxyz1234"}
	for _, name := range valid {
		if err := validateName(name); err != nil {
			t.Errorf("'%s' should be valid, got: %v", name, err)
		}
	}

	invalid := []struct {
		name   string
		reason string
	}{
		{"MyBot", "uppercase"},
		{"1bot", "starts with number"},
		{"-bot", "starts with hyphen"},
		{"my bot", "space"},
		{"my_bot", "underscore"},
		{"my.bot", "dot"},
		{"shared", "reserved"},
		{"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "too long (33 chars)"},
	}
	for _, tc := range invalid {
		if err := validateName(tc.name); err == nil {
			t.Errorf("'%s' should be invalid (%s)", tc.name, tc.reason)
		}
	}
}

func TestBasePort(t *testing.T) {
	// Default
	t.Setenv("CLAWCTL_BASE_PORT", "")
	if p := basePort(); p != 18789 {
		t.Errorf("default base port should be 18789, got %d", p)
	}

	// Override
	t.Setenv("CLAWCTL_BASE_PORT", "28789")
	if p := basePort(); p != 28789 {
		t.Errorf("overridden base port should be 28789, got %d", p)
	}
}

func TestFlagValue(t *testing.T) {
	cases := []struct {
		args   []string
		prefix string
		want   string
	}{
		{[]string{"--group=team", "alpha"}, "--group=", "team"},
		{[]string{"alpha", "--group=team"}, "--group=", "team"},
		{[]string{"alpha"}, "--group=", ""},
		{[]string{}, "--group=", ""},
		{[]string{"--group="}, "--group=", ""},          // empty value
		{[]string{"--group=a", "--group=b"}, "--group=", "a"}, // first wins
		{[]string{"--since=2h"}, "--group=", ""},        // unrelated flag
		{[]string{"--group=multi=word"}, "--group=", "multi=word"}, // = in value
	}
	for _, tc := range cases {
		if got := flagValue(tc.args, tc.prefix); got != tc.want {
			t.Errorf("flagValue(%v, %q) = %q, want %q", tc.args, tc.prefix, got, tc.want)
		}
	}
}

func TestFirstPositional(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"alpha"}, "alpha"},
		{[]string{"--group=team", "alpha"}, "alpha"},
		{[]string{"alpha", "--hard"}, "alpha"},
		{[]string{"--hard", "alpha"}, "alpha"},
		{[]string{}, ""},
		{[]string{"--hard", "--group=team"}, ""},
	}
	for _, tc := range cases {
		if got := firstPositional(tc.args); got != tc.want {
			t.Errorf("firstPositional(%v) = %q, want %q", tc.args, got, tc.want)
		}
	}
}

func TestFilterEntriesByGroup(t *testing.T) {
	entries := []RegistryEntry{
		{Index: 0, Name: "alpha"},
		{Index: 1, Name: "team/sarah"},
		{Index: 2, Name: "team/john"},
		{Index: 3, Name: "other/ben"},
	}

	// Empty group filter returns input unchanged.
	if got := filterEntriesByGroup(entries, ""); len(got) != 4 {
		t.Errorf("empty filter should be passthrough, got %d entries", len(got))
	}

	// Filter to "team".
	got := filterEntriesByGroup(entries, "team")
	if len(got) != 2 {
		t.Fatalf("team filter expected 2 entries, got %d", len(got))
	}
	for _, e := range got {
		if e.Name != "team/sarah" && e.Name != "team/john" {
			t.Errorf("unexpected entry in team filter: %s", e.Name)
		}
	}

	// Filter that matches nothing.
	if got := filterEntriesByGroup(entries, "ghost"); len(got) != 0 {
		t.Errorf("nonexistent group should yield 0 entries, got %d", len(got))
	}

	// Standalone instances (no group) are excluded by any non-empty filter,
	// even one matching their name — only group/name parses to a group.
	if got := filterEntriesByGroup(entries, "alpha"); len(got) != 0 {
		t.Errorf("group filter 'alpha' should not match standalone instance 'alpha', got %d", len(got))
	}
}

func TestRequireGroup(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Root: root}

	// Empty group is always ok.
	if err := requireGroup(paths, ""); err != nil {
		t.Errorf("empty group should be ok, got %v", err)
	}

	// Nonexistent group is rejected.
	if err := requireGroup(paths, "ghost"); err == nil {
		t.Errorf("nonexistent group should error")
	}

	// Create a real group dir with .group.json and verify it's accepted.
	groupDir := filepath.Join(root, "team")
	if err := WriteGroupConfig(mkdirOrFatal(t, groupDir), GroupConfig{Name: "team"}); err != nil {
		t.Fatal(err)
	}
	if err := requireGroup(paths, "team"); err != nil {
		t.Errorf("existing group should be ok, got %v", err)
	}
}

func TestPadVisible(t *testing.T) {
	// Plain text: byte length == visible width.
	if got := padVisible("abc", 5); got != "abc  " {
		t.Errorf("plain pad: %q, want %q", got, "abc  ")
	}

	// ANSI-colored text: pad to visible width, ignoring escape sequences.
	colored := "\033[0;32mok\033[0m" // visible "ok" (2 chars), 11 invisible
	got := padVisible(colored, 5)
	if got != colored+"   " {
		t.Errorf("colored pad: %q, want %q", got, colored+"   ")
	}

	// Already wider than width: returned unchanged.
	if got := padVisible("toolong", 3); got != "toolong" {
		t.Errorf("wider value: %q, want unchanged", got)
	}

	// Exact width: no padding added.
	if got := padVisible("abc", 3); got != "abc" {
		t.Errorf("exact width: %q, want %q", got, "abc")
	}

	// Multiple ANSI sequences in one value.
	mixed := "\033[1m\033[0;31mfail\033[0m" // visible "fail" (4)
	if got := padVisible(mixed, 6); got != mixed+"  " {
		t.Errorf("mixed ANSI: %q, want %q", got, mixed+"  ")
	}

	// Empty input.
	if got := padVisible("", 3); got != "   " {
		t.Errorf("empty pad: %q, want %q", got, "   ")
	}
}

func TestAuditEntryInGroup(t *testing.T) {
	cases := []struct {
		args  []string
		group string
		want  bool
	}{
		{[]string{"team/sarah"}, "team", true},
		{[]string{"team/sarah", "codex"}, "team", true},
		{[]string{"alpha"}, "team", false},      // standalone
		{[]string{"other/sarah"}, "team", false}, // different group
		{[]string{}, "team", false},              // no args
		{[]string{"--json"}, "team", false},      // only flags
		{[]string{"--since=1h", "team/sarah"}, "team", true}, // flag then ref
	}
	for _, tc := range cases {
		if got := auditEntryInGroup(tc.args, tc.group); got != tc.want {
			t.Errorf("auditEntryInGroup(%v, %q) = %v, want %v", tc.args, tc.group, got, tc.want)
		}
	}
}

// mkdirOrFatal is a tiny test helper: makes dir and returns the path, failing
// the test on error. Used where the caller wants to use the dir immediately.
func mkdirOrFatal(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}
