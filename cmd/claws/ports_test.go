package main

import (
	"net"
	"os"
	"testing"
)

// TestPortInUse_DialBased verifies the TCP-dial port-probe both for a
// free port (no listener) and a held one (we spin up a listener).
func TestPortInUse_DialBased(t *testing.T) {
	// Ensure CLAWS_SKIP_VALIDATE is unset so portInUse actually probes.
	t.Setenv("CLAWS_SKIP_VALIDATE", "")
	if v := os.Getenv("CLAWS_SKIP_VALIDATE"); v != "" {
		t.Fatalf("CLAWS_SKIP_VALIDATE should be unset, got %q", v)
	}

	// 1. Bind a listener on an OS-assigned ephemeral port — must report in-use.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	if !portInUse(port) {
		t.Errorf("port %d has a live listener; portInUse should return true", port)
	}

	// 2. Close the listener, the same port should become free.
	l.Close()
	if portInUse(port) {
		t.Errorf("port %d listener closed; portInUse should return false", port)
	}
}

func TestPortInUse_HonorsSkipValidate(t *testing.T) {
	t.Setenv("CLAWS_SKIP_VALIDATE", "1")
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port
	if portInUse(port) {
		t.Errorf("with CLAWS_SKIP_VALIDATE=1, portInUse must always return false, even with a live listener on :%d", port)
	}
}

// TestNextIndex_SkipsHeldPort verifies that the smart allocation logic
// skips an index whose port is already bound by something external —
// the bug behind the team/sarah port collision.
func TestNextIndex_SkipsHeldPort(t *testing.T) {
	paths := testPaths(t)
	// Override the skip from testPaths — we WANT live port probing here.
	t.Setenv("CLAWS_SKIP_VALIDATE", "")

	// Hold port for index 0 by binding 127.0.0.1:<basePort+0>.
	// If this port happens to be held by something else on the test
	// machine (unlikely for 18789 in CI), the test would still produce
	// idx != 0, just for a different reason.
	port := portForIndex(0)
	l, err := net.Listen("tcp", "127.0.0.1:"+itoa(port))
	if err != nil {
		t.Skipf("can't bind 127.0.0.1:%d on this host: %v (test inconclusive)", port, err)
	}
	defer l.Close()

	idx, err := nextIndex(paths)
	if err != nil {
		t.Fatalf("nextIndex returned error: %v", err)
	}
	if idx == 0 {
		t.Errorf("nextIndex returned 0 despite port %d being held — smart-skip not working", port)
	}
}
