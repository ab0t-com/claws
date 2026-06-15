package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// minUsefulSwapBytes is the smallest swapfile that's worth bothering
// with. Below this the build will probably still OOM (the bundler
// peaks at ~3 GB beyond RAM), so it's better to fail fast than spend
// 30 seconds creating a 1 GB swap that won't save the build anyway.
const minUsefulSwapBytes = 2 * 1024 * 1024 * 1024 // 2 GiB

// recommendedSwapHeadroomBytes is the safety margin we add on top of
// "RAM + swap >= 4 GB" when sizing an auto-swap. The peak is 4 GB but
// the build does other things concurrently (e.g. compresses layers),
// and a too-small swap stresses the kernel into thrashing.
const recommendedSwapHeadroomBytes = 1 * 1024 * 1024 * 1024 // 1 GiB

// autoSwapCeilingBytes caps the auto-computed swap size. Even on a
// box with tons of disk, allocating more than this just wastes I/O.
// The build's working set doesn't grow beyond ~6 GB total budget.
const autoSwapCeilingBytes = 4 * 1024 * 1024 * 1024 // 4 GiB

// diskReserveBytes is the space we leave free on the chosen filesystem
// after allocating the swapfile, so the docker build itself + intermediate
// layers can write to disk without running it dry. Bumped from 1 GB to
// 3 GB after a client report — the openclaw image build's intermediate
// layers + final image need ~2.6 GB of disk, plus working scratch.
const diskReserveBytes = 3 * 1024 * 1024 * 1024 // 3 GiB

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

// totalMemoryBytes reads MemTotal from /proc/meminfo on Linux.
// MemTotal is what the box was provisioned with; it doesn't fluctuate
// with current cache pressure. Used for the auto-swap gating decision
// because an 8 GB box that's currently caching aggressively can show
// low MemAvailable but doesn't actually need swap — the kernel will
// reclaim cache when the build asks for RAM.
//
// Returns 0 on non-Linux or on read error.
func totalMemoryBytes() uint64 {
	return readMeminfoField("MemTotal:")
}

// availableMemoryBytes reads MemAvailable from /proc/meminfo on Linux.
// On other platforms returns 0 (= "unknown — caller should skip the
// check"). Reading MemAvailable directly is more accurate than
// `free -h`'s "available" column for our purposes (it factors in
// reclaimable cache).
func availableMemoryBytes() uint64 {
	return readMeminfoField("MemAvailable:")
}

// readMeminfoField returns the byte value of the named /proc/meminfo
// field (e.g. "MemTotal:" or "MemAvailable:"). Returns 0 on any failure.
func readMeminfoField(prefix string) uint64 {
	if runtime.GOOS != "linux" {
		return 0
	}
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, prefix) {
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
	path, free, err := pickSwapfilePath()
	if err != nil {
		return nil, err
	}
	// Clamp sizeBytes to what fits on the chosen filesystem, leaving
	// diskReserveBytes free for the build's own scratch.
	maxFit := uint64(0)
	if free > diskReserveBytes {
		maxFit = free - diskReserveBytes
	}
	if maxFit < minUsefulSwapBytes {
		return nil, fmt.Errorf("not enough free disk on any candidate path for a useful swapfile (need ≥ %s + %s reserve; best path %s has %s free)",
			formatBytes(minUsefulSwapBytes), formatBytes(diskReserveBytes), path, formatBytes(free))
	}
	if sizeBytes > maxFit {
		fmt.Printf("  %srequested swap %s is larger than disk free on %s — capping to %s%s\n",
			"\033[0;90m", formatBytes(sizeBytes), filepath.Dir(path), formatBytes(maxFit), "\033[0m")
		sizeBytes = maxFit
	}
	return &swapfileManager{
		path:      path,
		sizeBytes: sizeBytes,
		sudo:      sudo,
	}, nil
}

// pickSwapfilePath picks the best on-disk path for a swapfile.
//
// Tries candidates in order; picks the first one that:
//  1. ISN'T tmpfs (a swapfile on tmpfs is recursive — tmpfs IS backed
//     by RAM + swap)
//  2. Has at least minUsefulSwapBytes + diskReserveBytes free
//
// Returns the chosen path and the free-bytes count on its filesystem,
// or an error listing what was tried.
func pickSwapfilePath() (string, uint64, error) {
	type candidate struct{ dir, name string }
	candidates := []candidate{
		{"/var/cache/claws", "bootstrap.swap"},
		{"/var/tmp", "claws-bootstrap.swap"},
		{"/var", "claws-bootstrap.swap"},
	}
	// $HOME/.cache/claws — works for non-root setups where /var isn't
	// writable. Walk last so /var paths get tried first.
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, candidate{filepath.Join(home, ".cache", "claws"), "bootstrap.swap"})
	}
	// /tmp last, AND only if it's not tmpfs (most modern distros mount
	// /tmp as tmpfs — in which case a swapfile here is nonsense).
	candidates = append(candidates, candidate{"/tmp", "claws-bootstrap.swap"})

	type tried struct{ path, reason string }
	var rejected []tried

	const minNeeded = minUsefulSwapBytes + diskReserveBytes
	for _, c := range candidates {
		path := filepath.Join(c.dir, c.name)
		// Ensure dir exists (or can be created). If creation fails,
		// move on — most likely we don't have write access to /var/.
		if _, err := os.Stat(c.dir); err != nil {
			if mkErr := os.MkdirAll(c.dir, 0o755); mkErr != nil {
				rejected = append(rejected, tried{c.dir, fmt.Sprintf("can't create: %v", mkErr)})
				continue
			}
		}
		if isTmpfs(c.dir) {
			rejected = append(rejected, tried{c.dir, "is tmpfs (swap-on-tmpfs is recursive)"})
			continue
		}
		free := freeDiskBytes(c.dir)
		if free < minNeeded {
			rejected = append(rejected, tried{c.dir, fmt.Sprintf("only %s free (need ≥ %s)", formatBytes(free), formatBytes(minNeeded))})
			continue
		}
		return path, free, nil
	}

	// Nothing worked — surface what was tried, since the error message
	// is the only signal a non-technical operator has.
	var b strings.Builder
	b.WriteString("no candidate path is usable for a swapfile:")
	for _, r := range rejected {
		b.WriteString(fmt.Sprintf("\n  %s — %s", r.path, r.reason))
	}
	b.WriteString(fmt.Sprintf("\n\nFree up at least %s on /var or $HOME/.cache, or pass --no-swap to accept the OOM risk.", formatBytes(minNeeded)))
	return "", 0, fmt.Errorf("%s", b.String())
}

// freeDiskBytes returns the bytes available on the filesystem that
// contains the given directory. Returns 0 on statfs error.
func freeDiskBytes(dir string) uint64 {
	var s syscall.Statfs_t
	if err := syscall.Statfs(dir, &s); err != nil {
		return 0
	}
	// Bavail is blocks available to a non-superuser; multiply by block
	// size for byte count.
	return s.Bavail * uint64(s.Bsize)
}

// isTmpfs returns true if the filesystem that contains dir is tmpfs.
// Reads /proc/mounts and finds the longest-prefix mount point.
func isTmpfs(dir string) bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}
	return mountFstypeForPath(string(data), dir) == "tmpfs"
}

// mountFstypeForPath returns the fstype of the mount whose mountpoint
// is the longest prefix of `dir`. Pure-function variant for tests.
//
// /proc/mounts format: "device mountpoint fstype options dump pass".
// We care about fields 1 (mountpoint) and 2 (fstype).
func mountFstypeForPath(mountsContent, dir string) string {
	bestMount := ""
	bestType := ""
	for _, line := range strings.Split(mountsContent, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		mountpoint := fields[1]
		fstype := fields[2]
		// Match if dir is exactly mountpoint or under it. Slash boundary
		// guard so "/tmp" doesn't match "/tmp-foo". Root "/" is special-
		// cased because "/" + "/" = "//" which no path actually starts
		// with — every dir falls back to root.
		matches := dir == mountpoint ||
			mountpoint == "/" ||
			strings.HasPrefix(dir, mountpoint+"/")
		if matches && len(mountpoint) > len(bestMount) {
			bestMount = mountpoint
			bestType = fstype
		}
	}
	return bestType
}

// chooseAutoSwapSize picks the swap size to use when --add-swap is set
// without an explicit value. Goal: bring RAM + swap to (4 GB + 1 GB
// headroom). Bounded to [minUseful, autoSwapCeiling] so we don't
// allocate an absurd amount even on a tiny box with tons of disk —
// docker also needs disk for the build's own scratch.
//
// availMem is the current RAM-available figure (from /proc/meminfo).
// existingSwap is the currently-active swap from /proc/swaps.
func chooseAutoSwapSize(availMem, existingSwap uint64) uint64 {
	const target = 4*1024*1024*1024 + recommendedSwapHeadroomBytes // 5 GB
	have := availMem + existingSwap
	var need uint64
	if have >= target {
		need = minUsefulSwapBytes
	} else {
		need = target - have
	}
	if need < minUsefulSwapBytes {
		need = minUsefulSwapBytes
	}
	if need > autoSwapCeilingBytes {
		need = autoSwapCeilingBytes
	}
	return need
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
