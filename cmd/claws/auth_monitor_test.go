package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLooksLikeAPIKey covers the sanity check that gates the fallback-key
// path. Reject empties, reject too-short, reject things with whitespace or
// non-alphanum garbage; accept sk-*, generic alphanum tokens.
func TestLooksLikeAPIKey(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty", "", false},
		{"too short", "sk-abc", false},
		{"minimum length valid", strings.Repeat("a", 20), true},
		{"sk- happy path", "sk-1234567890abcdefghijklmnop", true},
		{"sk-ant- (anthropic)", "sk-ant-api03-AAAAbbbbCCCC1234567890", true},
		{"sk-or- (openrouter)", "sk-or-v1-AAAAbbbbCCCCdddd1234567890", true},
		{"generic alphanum-token (e.g. service account)", "AKIAIOSFODNN7EXAMPLE_abcdef", true},
		{"contains whitespace", "sk-aaaa bbbb ccccdddd", false},
		{"contains newline", "sk-1234567890abcdefghij\n", false},
		{"contains tab", "sk-1234567890abcdefghij\t", false},
		{"contains slash", "sk-aaaa/bbbb/cccc/dddd", false},
		{"contains @ (e.g. pasted email)", "user@example.com", false},
		{"too long (>512)", strings.Repeat("a", 600), false},
		{"max length valid (512)", strings.Repeat("a", 512), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := looksLikeAPIKey(c.in)
			if got != c.want {
				t.Errorf("looksLikeAPIKey(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// TestWriteMonitorAuditFormat — the JSONL entries the monitor writes must
// parse back to the expected shape. Tests both the happy `recover` case
// and the `stalled` case so downstream tooling can rely on the format.
func TestWriteMonitorAuditFormat(t *testing.T) {
	tmpdir := t.TempDir()
	paths := Paths{Root: tmpdir}

	cases := []struct {
		name   string
		kind   string
		agents []string
		result string
		detail string
	}{
		{"recover happy", "auth.monitor.recover", []string{"team/sarah"}, "recovered=1 still_broken=0", "openai"},
		{"recover partial", "auth.monitor.recover", []string{"team/sarah", "team/john"}, "recovered=1 still_broken=1", "openai"},
		{"stalled no key", "auth.monitor.stalled", []string{"team/sarah"}, "no-fallback-key", "/var/lib/claws/.recovery-api-key"},
		{"empty agents (no broken)", "auth.monitor.recover", nil, "recovered=0 still_broken=0", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			writeMonitorAudit(paths, c.kind, c.agents, c.result, c.detail)
		})
	}

	// Read back the audit log and parse each line.
	logPath := filepath.Join(paths.Root, auditLogFile)
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("audit log not written: %v", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != len(cases) {
		t.Fatalf("expected %d audit lines, got %d", len(cases), len(lines))
	}
	for i, line := range lines {
		var entry map[string]any
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Errorf("line %d not valid JSON: %v\n  raw: %s", i, err, line)
			continue
		}
		// Required keys.
		for _, k := range []string{"ts", "kind", "agents", "result", "detail"} {
			if _, ok := entry[k]; !ok {
				t.Errorf("line %d missing key %q", i, k)
			}
		}
		if entry["kind"] != cases[i].kind {
			t.Errorf("line %d kind: got %v, want %s", i, entry["kind"], cases[i].kind)
		}
		if entry["result"] != cases[i].result {
			t.Errorf("line %d result: got %v, want %s", i, entry["result"], cases[i].result)
		}
		// Spot-check the timestamp parses as RFC3339 (we don't pin a
		// specific value — Date.now()-style nondeterminism is fine — but
		// the format must be parseable so jq + downstream tools work).
		ts, ok := entry["ts"].(string)
		if !ok || len(ts) < 20 || !strings.Contains(ts, "T") {
			t.Errorf("line %d ts doesn't look like RFC3339: %v", i, entry["ts"])
		}
	}
}

// TestAuditLogIsAppendOnly — calling writeMonitorAudit multiple times
// across separate "sweeps" must accumulate; never truncate.
func TestAuditLogIsAppendOnly(t *testing.T) {
	tmpdir := t.TempDir()
	paths := Paths{Root: tmpdir}

	for i := 0; i < 5; i++ {
		writeMonitorAudit(paths, "auth.monitor.recover", []string{"team/sarah"}, "ok", "")
	}

	data, err := os.ReadFile(filepath.Join(paths.Root, auditLogFile))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 5 {
		t.Errorf("expected 5 append-only lines, got %d", len(lines))
	}
}

// TestAuditLogResilientToReadOnlyDir — when the audit dir can't be written
// (e.g., readonly mount, full disk, permission denied), the monitor must
// NOT panic; it silently degrades. The runtime expectation is that a
// human reviewing logs notices the absence, not a process crash.
func TestAuditLogResilientToReadOnlyDir(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("writeMonitorAudit panicked on unwritable dir: %v", r)
		}
	}()
	paths := Paths{Root: "/proc"} // /proc is readonly; OpenFile O_CREATE there fails cleanly
	writeMonitorAudit(paths, "auth.monitor.stalled", []string{"any"}, "test", "test")
	// No assertion needed — surviving without panic is the contract.
}

// TestDefaultMonitorIntervalIsSane — the default polling interval needs
// to be tight enough to catch a 401 within one human-noticeable window
// but loose enough not to hammer docker. Guard against accidental
// constant changes.
func TestDefaultMonitorIntervalIsSane(t *testing.T) {
	mins := defaultMonitorInterval.Minutes()
	if mins < 1 || mins > 30 {
		t.Errorf("defaultMonitorInterval = %v; expected 1-30 min range for systemd timer use", defaultMonitorInterval)
	}
}

// TestDefaultFallbackKeyNameIsHidden — the recovery key file lives in
// ~/.openclaw/ alongside fleet state. It MUST start with a dot so it's
// not picked up by `claws list` glob scans of agent dirs (which would
// surface the credential to ANY operator who runs the command).
func TestDefaultFallbackKeyNameIsHidden(t *testing.T) {
	if !strings.HasPrefix(defaultFallbackKeyName, ".") {
		t.Errorf("defaultFallbackKeyName = %q; expected leading dot so it's not glob-listed alongside agents", defaultFallbackKeyName)
	}
}
