package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// ---------------------------------------------------------------------------
// swap.go — temporary swapfile lifecycle for `claws image bootstrap --add-swap`
//
// Ticket: tickets/openclaw-image-build-ram-2026-06-15/ticket.md (option 1)
//
// The openclaw docker build peaks at ~3-4 GB RAM during the node bundle
// stage. Hosts under ~4 GB OOM-kill the bundle and have no path forward.
// This file lets the operator add a temporary swapfile *just for the
// build* — created before docker build, removed after (success, failure,
// or Ctrl-C). The swapfile is NEVER left behind: signal handler covers
// the interrupt case, deferred disable() covers the normal-return case.
//
// Linux-only. On macOS, Docker Desktop manages its own RAM allocation —
// the bootstrap caller should print the "configure RAM in Docker
// Desktop Settings → Resources" guidance instead of trying to add swap.
// ---------------------------------------------------------------------------

// currentSwapBytes returns the total currently-active swap on the
// system, in bytes. Reads /proc/swaps; returns 0 on read error or on
// non-Linux platforms. Used by the auto-swap decision so we don't add
// more swap if the operator already has enough configured via fstab,
// cloud-init, or any other means.
func currentSwapBytes() uint64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	data, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return 0
	}
	return parseProcSwapsTotal(string(data))
}

// parseProcSwapsTotal sums the "Size" column of /proc/swaps content
// (excluding the header line) and returns the total in bytes. The
// /proc/swaps Size column is in KB. Exposed as a pure function so it
// can be unit-tested with fixture content.
func parseProcSwapsTotal(content string) uint64 {
	var total uint64
	for i, line := range strings.Split(content, "\n") {
		if i == 0 || line == "" { // skip header + blank lines
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		kb, err := strconv.ParseUint(fields[2], 10, 64)
		if err != nil {
			continue
		}
		total += kb * 1024
	}
	return total
}

// swapfileActive returns true if /proc/swaps lists the given path as
// currently-active swap. Used to detect "our swap from a previous run
// is still on" so we can reuse it rather than fail or double-swapon.
func swapfileActive(path string) bool {
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile("/proc/swaps")
	if err != nil {
		return false
	}
	return swapfileActiveIn(string(data), path)
}

// swapfileActiveIn is the pure-function variant of swapfileActive, for
// unit tests.
func swapfileActiveIn(content, path string) bool {
	for i, line := range strings.Split(content, "\n") {
		if i == 0 || line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 1 && fields[0] == path {
			return true
		}
	}
	return false
}

// availableMemoryBytes reads MemAvailable from /proc/meminfo on Linux.
// On other platforms returns 0 (= "unknown — caller should skip the
// check"). Reading MemAvailable directly is more accurate than
// `free -h`'s "available" column for our purposes (it factors in
// reclaimable cache).
func availableMemoryBytes() uint64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "MemAvailable:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return 0
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0
		}
		return kb * 1024
	}
	return 0
}

// formatBytes renders an integer byte count as "1.2 GB" / "512 MB".
// Decimal units (GB / MB), not binary, so the numbers match what
// operators see in `free -h` and cloud provider dashboards.
func formatBytes(b uint64) string {
	const (
		mb = 1000 * 1000
		gb = 1000 * 1000 * 1000
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%d MB", b/mb)
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// parseSwapSize parses "8g" / "4G" / "2048m" / "2048M" into bytes.
// Empty input → 8 GiB default (matches the ticket's recommended size).
// Returns 0 on parse error, which the caller treats as invalid input.
func parseSwapSize(s string) uint64 {
	if s == "" {
		return 8 * 1024 * 1024 * 1024
	}
	s = strings.TrimSpace(strings.ToLower(s))
	var mult uint64 = 1
	switch {
	case strings.HasSuffix(s, "g"):
		mult = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "g")
	case strings.HasSuffix(s, "m"):
		mult = 1024 * 1024
		s = strings.TrimSuffix(s, "m")
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0
	}
	return n * mult
}

// swapfileManager creates a temporary swapfile for the duration of an
// operation and removes it afterwards. The lifecycle is:
//
//	mgr, err := newSwapfileManager(sizeBytes)
//	if err != nil { return err }
//	defer mgr.disable()  // covers success + non-signal failure
//	mgr.installSignalHandler()  // covers Ctrl-C
//	if err := mgr.enable(); err != nil { return err }
//	// ... do the heavy work that needs swap ...
//
// disable() is idempotent and safe to call multiple times.
type swapfileManager struct {
	path      string
	sizeBytes uint64
	sudo      string
	active    bool // true between successful enable() and disable()
	sigCh     chan os.Signal
}

func newSwapfileManager(sizeBytes uint64) (*swapfileManager, error) {
	if runtime.GOOS != "linux" {
		return nil, fmt.Errorf("--add-swap is Linux-only (on macOS, configure RAM in Docker Desktop Settings → Resources)")
	}
	sudo, err := resolveSudoPrefix()
	if err != nil {
		return nil, err
	}
	return &swapfileManager{
		path:      "/tmp/claws-bootstrap.swap",
		sizeBytes: sizeBytes,
		sudo:      sudo,
	}, nil
}

// enable creates the swapfile and turns it on. After this returns nil,
// the caller MUST eventually call disable() (best done via defer).
//
// Three safety guards run first:
//
//  1. If our path is ALREADY active in /proc/swaps, we reuse it —
//     don't double-swapon, don't overwrite. This handles the case
//     where a previous run got SIGKILLed and the cleanup didn't fire.
//     disable() will still swapoff + remove on exit, so we're not
//     leaving state behind on the user's behalf.
//  2. If a file exists at our path but ISN'T active swap, we refuse —
//     could be a stale file from a corrupted previous run, could be
//     something the user put there. Either way, blindly overwriting is
//     wrong. Operator removes it manually then re-runs.
//  3. The build path (cmdImageBootstrap) only reaches enable() after
//     deciding existing swap isn't enough; so by the time we're here,
//     we know we need to add more.
func (s *swapfileManager) enable() error {
	const dim = "\033[0;90m"
	const nc = "\033[0m"

	// Guard 1: our path already active in /proc/swaps → reuse.
	if swapfileActive(s.path) {
		fmt.Printf("\n  ✓ swap already active at %s (reusing from previous run)\n", s.path)
		s.active = true
		return nil
	}
	// Guard 2: stale file at our path → refuse.
	if _, err := os.Stat(s.path); err == nil {
		return fmt.Errorf("%s exists but isn't active swap — refusing to overwrite. Inspect or remove manually:\n  sudo rm %s\nThen re-run the build", s.path, s.path)
	}

	fmt.Printf("\n  Adding %s temporary swapfile at %s\n", formatBytes(s.sizeBytes), s.path)
	fmt.Printf("  %s(temporary — removed when the build finishes or you Ctrl-C)%s\n", dim, nc)

	// fallocate is fast on ext4/xfs/btrfs but not supported on all
	// filesystems (e.g. tmpfs, some network mounts). Fall back to dd
	// when fallocate fails; dd is portable but writes zeros to every
	// byte, so it's much slower for big files.
	sizeStr := strconv.FormatUint(s.sizeBytes, 10)
	if err := s.run("fallocate", "-l", sizeStr, s.path); err != nil {
		fmt.Printf("  %sfallocate failed (filesystem may not support it); falling back to dd%s\n", dim, nc)
		sizeMB := s.sizeBytes / (1024 * 1024)
		if err := s.run("dd", "if=/dev/zero", "of="+s.path, "bs=1M", "count="+strconv.FormatUint(sizeMB, 10), "status=progress"); err != nil {
			s.removeFileBestEffort()
			return fmt.Errorf("dd fallback failed: %v", err)
		}
	}
	if err := s.run("chmod", "600", s.path); err != nil {
		s.removeFileBestEffort()
		return fmt.Errorf("chmod failed: %v", err)
	}
	if err := s.run("mkswap", s.path); err != nil {
		s.removeFileBestEffort()
		return fmt.Errorf("mkswap failed: %v", err)
	}
	if err := s.run("swapon", s.path); err != nil {
		s.removeFileBestEffort()
		return fmt.Errorf("swapon failed: %v", err)
	}
	s.active = true
	fmt.Printf("  ✓ swap active (%s)\n\n", formatBytes(s.sizeBytes))
	return nil
}

// disable turns the swapfile off and removes it. Idempotent: safe to
// call when enable() never succeeded. Logs but does not return errors
// — once we get to disable(), we want to clean up as much as we can,
// not error out partway and leave state behind.
func (s *swapfileManager) disable() {
	if s == nil {
		return
	}
	if s.sigCh != nil {
		signal.Stop(s.sigCh)
		s.sigCh = nil
	}
	if s.active {
		fmt.Printf("\n  Cleaning up swap: swapoff + rm %s\n", s.path)
		_ = s.run("swapoff", s.path)
		s.active = false
	}
	s.removeFileBestEffort()
}

func (s *swapfileManager) removeFileBestEffort() {
	if _, err := os.Stat(s.path); err == nil {
		_ = s.run("rm", "-f", s.path)
	}
}

// installSignalHandler wires SIGINT + SIGTERM to disable() so a Ctrl-C
// during the docker build doesn't leave the swapfile turned on or
// undeleted. The handler exits 130 (the conventional shell "killed by
// SIGINT" code) after cleanup.
func (s *swapfileManager) installSignalHandler() {
	s.sigCh = make(chan os.Signal, 2)
	signal.Notify(s.sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-s.sigCh
		fmt.Printf("\n  caught %s during build — cleaning up swap before exiting\n", sig)
		s.disable()
		os.Exit(130)
	}()
}

func (s *swapfileManager) run(name string, args ...string) error {
	const dim = "\033[0;90m"
	const nc = "\033[0m"
	if s.sudo != "" {
		args = append([]string{name}, args...)
		name = s.sudo
	}
	fmt.Printf("    %s$ %s %s%s\n", dim, name, strings.Join(args, " "), nc)
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// resolveSudoPrefix returns "sudo" when we're not root and sudo is
// available, "" when we're root, and an error when we need sudo but
// it's missing.
func resolveSudoPrefix() (string, error) {
	if os.Geteuid() == 0 {
		return "", nil
	}
	if _, err := exec.LookPath("sudo"); err != nil {
		return "", fmt.Errorf("not root and sudo not installed; re-run as root")
	}
	return "sudo", nil
}
