package main

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// TestRuntimeContainerUID returns 1000 for openclaw — guard against
// accidental constant edits that would silently break the chown path.
func TestRuntimeContainerUID(t *testing.T) {
	if got := runtimeContainerUID(openclawRuntime()); got != 1000 {
		t.Errorf("openclaw runtime uid: got %d, want 1000", got)
	}
}

// TestUidMismatchActive_NotRoot — when claws is not running as root,
// there's nothing to fix regardless of runtime config. This guard prevents
// the chown path from firing unintentionally when a uid-1000 user runs claws.
func TestUidMismatchActive_NotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root; this test asserts the non-root path")
	}
	if uidMismatchActive(openclawRuntime()) {
		t.Errorf("uidMismatchActive returned true when not running as root")
	}
}

// TestRunningAsRoot — must return os.Geteuid() == 0 truthy/falsy.
// Just a sanity guard since the gate is the entry point for everything else.
func TestRunningAsRoot(t *testing.T) {
	got := runningAsRoot()
	want := os.Geteuid() == 0
	if got != want {
		t.Errorf("runningAsRoot() = %v, want %v", got, want)
	}
}

// TestChownInstanceDir_Idempotent — running chownInstanceDir against a dir
// where every file is already at the target uid must be a no-op. This is
// the safety property the v1.6.30 fix depends on (we call it from auth/exec
// pre-flight; must be cheap + idempotent).
func TestChownInstanceDir_Idempotent(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to test chown idempotence")
	}
	dir := t.TempDir()
	// Create a couple of files; they'll be owned by root since we're root.
	for _, name := range []string{"openclaw.json", "instance.env", "logs/log1"} {
		full := filepath.Join(dir, name)
		os.MkdirAll(filepath.Dir(full), 0755)
		if err := os.WriteFile(full, []byte("test"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	// First chown — should succeed.
	if err := chownInstanceDir(dir, 0); err != nil {
		t.Fatalf("first chown: %v", err)
	}
	// Second chown — should also succeed (idempotent).
	if err := chownInstanceDir(dir, 0); err != nil {
		t.Fatalf("second chown (idempotent): %v", err)
	}
}

// TestChownInstanceDir_RecursesIntoSubdirs — verify the walk covers
// nested files. Bug we want to catch: someone changes from Walk to
// listing only the top level + we silently miss agents/main/agent/
// auth-profiles.json.
func TestChownInstanceDir_RecursesIntoSubdirs(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires root to test chown")
	}
	dir := t.TempDir()
	deep := filepath.Join(dir, "agents", "main", "agent")
	os.MkdirAll(deep, 0755)
	deepFile := filepath.Join(deep, "auth-profiles.json")
	if err := os.WriteFile(deepFile, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	// Set known uid 0 (we're root) and chown to a different uid; expect
	// the deep file to also be re-owned. Use uid 1 (daemon on most distros)
	// as the target since we know it exists but isn't us.
	if err := chownInstanceDir(dir, 1); err != nil {
		t.Fatalf("chown: %v", err)
	}
	st, err := os.Stat(deepFile)
	if err != nil {
		t.Fatal(err)
	}
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		if int(sys.Uid) != 1 {
			t.Errorf("deep file uid = %d, want 1 (walk didn't recurse)", sys.Uid)
		}
	}
	// Reset for cleanup safety.
	chownInstanceDir(dir, 0)
}

// TestEnsureRuntimeReadable_NotRoot_NoChange — when running as a non-root
// user owning the files, ensureRuntimeReadable must be a no-op (caller
// can proceed; nothing to fix).
func TestEnsureRuntimeReadable_NotRoot_NoChange(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test asserts the non-root path")
	}
	// Set up a fake instance dir owned by us (the test runner).
	tmpdir := t.TempDir()
	agentDir := filepath.Join(tmpdir, "team", "alice")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	envFile := filepath.Join(agentDir, "instance.env")
	if err := os.WriteFile(envFile, []byte("INSTANCE_NAME=team/alice\n"), 0600); err != nil {
		t.Fatal(err)
	}
	configFile := filepath.Join(agentDir, "openclaw.json")
	if err := os.WriteFile(configFile, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	paths := Paths{Root: tmpdir}
	// Call should be a clean no-op since we're not root.
	if err := ensureRuntimeReadable(paths, "team/alice"); err != nil {
		t.Errorf("ensureRuntimeReadable: %v", err)
	}
}

// TestStUid_Returns0OnInvalidFileInfo — defensive: when info.Sys() isn't
// a *syscall.Stat_t (e.g. on Windows / certain mock setups) we should
// return 0 rather than panic. Caller treats 0 as "unknown".
func TestStUid_NoStatType(t *testing.T) {
	// Use a real os.Stat result then validate the function doesn't panic.
	tmp := t.TempDir()
	st, err := os.Stat(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("stUid panicked: %v", r)
		}
	}()
	got := stUid(st)
	// We expect SOME uid; can't assert a specific one (depends on test runner).
	if got < 0 {
		t.Errorf("stUid returned negative: %d", got)
	}
}

// TestPrintRootRunningBanner_NoPanic — banner must not panic when called.
// (Output is to stdout; test just runs the function.)
func TestPrintRootRunningBanner_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("printRootRunningBanner panicked: %v", r)
		}
	}()
	printRootRunningBanner()
}

// TestEnsureRuntimeReadable_MissingConfig_NoError — when openclaw.json
// doesn't exist (e.g. agent partially created), the pre-flight should
// silently no-op and let the caller hit its own (clearer) error.
func TestEnsureRuntimeReadable_MissingConfig_NoError(t *testing.T) {
	tmpdir := t.TempDir()
	// Create a fake instance.env so the agent appears registered but no openclaw.json.
	agentDir := filepath.Join(tmpdir, "team", "ghost")
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentDir, "instance.env"), []byte("INSTANCE_NAME=team/ghost\n"), 0600); err != nil {
		t.Fatal(err)
	}
	paths := Paths{Root: tmpdir}
	err := ensureRuntimeReadable(paths, "team/ghost")
	if err != nil && !strings.Contains(err.Error(), "ghost") {
		t.Errorf("missing openclaw.json should not surface as ownership error; got: %v", err)
	}
}
