package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// defaultTarballURL is the pre-built openclaw:local image used by
// `claws image bootstrap --from-tarball` (no URL). Bumped whenever a
// new openclaw runtime snapshot is published. Override at runtime via
// --from-tarball=<URL> or the CLAWS_OPENCLAW_TARBALL_URL env var.
const defaultTarballURL = "https://github.com/ab0t-com/claws/releases/download/openclaw-image-2026.3.9-slim/openclaw-local-2026.3.9-slim-2026-03-09.tar.gz"

// tarballMinDiskFreeBytes is the disk-free pre-flight threshold. The
// 683 MB gzipped tarball decompresses to ~2.6 GB on docker load; we need
// headroom for both the temp file and the loaded image layers.
const tarballMinDiskFreeBytes = 3 * 1024 * 1024 * 1024 // 3 GB

// cmdImageBootstrap — `claws image bootstrap [--source=<url>] [--yes] [--no-build]`
//
// Single-command path from "fresh host" to "openclaw:local image present".
// Without this, every new user hits "Image openclaw:local not found" from
// `claws doctor` and has no path forward without reading the openclaw repo
// docs separately.
//
// Behavior:
//   1. If openclaw:local already exists → "already present", exit 0.
//   2. If OPENCLAW_IMAGE_SOURCE env or --source= flag points at a docker tag,
//      try `docker pull` first.
//   3. If still no image and --no-build is not set → offer to git clone the
//      source repo + docker build. Requires --yes to actually run.
//
// All steps print what they're about to do before running.
func cmdImageBootstrap(args []string) error {
	var source, sourceRepo, addSwapSize, tarballURL string
	var yes, noBuild, addSwap, noSwap, fromTarball bool
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Println(`Usage: claws image bootstrap [--from-tarball[=<URL>]] [--source=<image:tag>] [--source-repo=<git-url>] [--yes] [--no-build] [--add-swap[=SIZE] | --no-swap]

Ensure openclaw:local is present on this host. Order of operations:
  1. If openclaw:local already exists → no-op.
  2. If --from-tarball is set → download a pre-built image tarball and
     ` + "`" + `docker load` + "`" + ` it. Skips the build entirely. Best for low-RAM hosts.
  3. If --source= or OPENCLAW_IMAGE_SOURCE is set → docker pull from it.
  4. Else, clone + build from --source-repo (default: github.com/openclaw/openclaw).

Flags:
  --from-tarball[=<URL>]       Download pre-built openclaw:local tarball + docker load.
                                Without URL: uses the default for this claws version.
                                Override default: --from-tarball=<URL>  OR
                                                  CLAWS_OPENCLAW_TARBALL_URL=<URL>
                                The download is SHA256-verified (sibling .sha256 file).
                                Skips the build step entirely — no RAM gymnastics needed.
  --source=<image:tag>         Pull this image and tag it openclaw:local
  --source-repo=<git-url>      Git source to clone + build (default: github.com/openclaw/openclaw)
  --yes                        Skip the "about to run docker build" confirmation
  --no-build                   Skip the build fallback (pull only)
  --add-swap[=SIZE]            Force a temporary swapfile around the build (default 8g).
  --no-swap                    Opt OUT of the automatic low-RAM swapfile.

Auto-swap (default with --yes): if the host has < 4 GB free RAM AND --yes is set,
                                claws adds an 8 GB temporary swapfile for the build
                                and removes it afterwards. The openclaw build peaks at
                                ~3-4 GB RAM; without swap it OOM-kills on small VPS boxes.
                                Pass --no-swap to opt out (accepts the OOM risk).
                                The swapfile is /tmp/claws-bootstrap.swap, removed in
                                every exit path including Ctrl-C.

Examples:
  claws image bootstrap --from-tarball --yes                     # download pre-built image (FAST)
  claws image bootstrap --from-tarball=https://your-host/img.tar.gz --yes
  claws image bootstrap                                          # try repo build (interactive)
  claws image bootstrap --yes                                    # builds; auto-swaps on small boxes
  claws image bootstrap --source=openclaw/runtime:v2026.5        # pull from a registry
  claws image bootstrap --add-swap=4g --yes                      # explicit smaller swapfile
  claws image bootstrap --no-swap --yes                          # accept OOM risk; no swap
  OPENCLAW_IMAGE_SOURCE=openclaw/runtime:latest claws image bootstrap
  CLAWS_OPENCLAW_TARBALL_URL=https://… claws image bootstrap --from-tarball --yes`)
			return nil
		case a == "--from-tarball":
			fromTarball = true
		case strings.HasPrefix(a, "--from-tarball="):
			fromTarball = true
			tarballURL = strings.TrimPrefix(a, "--from-tarball=")
		case strings.HasPrefix(a, "--source="):
			source = strings.TrimPrefix(a, "--source=")
		case strings.HasPrefix(a, "--source-repo="):
			sourceRepo = strings.TrimPrefix(a, "--source-repo=")
		case a == "--yes" || a == "-y":
			yes = true
		case a == "--no-build":
			noBuild = true
		case a == "--add-swap":
			addSwap = true
		case strings.HasPrefix(a, "--add-swap="):
			addSwap = true
			addSwapSize = strings.TrimPrefix(a, "--add-swap=")
		case a == "--no-swap":
			noSwap = true
		}
	}

	// Tarball URL resolution: explicit flag value > env override > built-in default.
	if fromTarball {
		if tarballURL == "" {
			tarballURL = os.Getenv("CLAWS_OPENCLAW_TARBALL_URL")
		}
		if tarballURL == "" {
			tarballURL = defaultTarballURL
		}
	}

	const (
		bold  = "\033[1m"
		dim   = "\033[0;90m"
		green = "\033[0;32m"
		gold  = "\033[0;33m"
		red   = "\033[0;31m"
		nc    = "\033[0m"
	)

	fmt.Printf("%sclaws image bootstrap%s\n\n", bold, nc)

	// Step 1 — already present?
	if imageExists("openclaw:local") {
		fmt.Printf("  %s✓ openclaw:local already present%s — nothing to do.\n", green, nc)
		printPostBootstrapNextSteps()
		return nil
	}
	fmt.Printf("  %sopenclaw:local not present.%s\n\n", dim, nc)

	// Step 1.5 — tarball path. Skips git clone + docker build entirely.
	// Designed for low-RAM hosts where building OOMs even with swap.
	// Uses an SHA256-verified download from CLAWS_OPENCLAW_TARBALL_URL /
	// --from-tarball=<URL> / defaultTarballURL.
	if fromTarball {
		fmt.Printf("  %sStep 2 — fetching pre-built image (no build needed)%s\n", bold, nc)
		fmt.Printf("    %surl:   %s%s\n", dim, tarballURL, nc)
		if err := bootstrapFromTarball(tarballURL); err != nil {
			return errorf("from-tarball failed: %v", err)
		}
		if imageExists("openclaw:local") {
			fmt.Printf("\n  %s✓ openclaw:local loaded from tarball%s\n", green, nc)
			printPostBootstrapNextSteps()
			return nil
		}
		return errorf("tarball load completed but openclaw:local not present — investigate manually")
	}

	// Step 2 — try pull from --source / OPENCLAW_IMAGE_SOURCE.
	if source == "" {
		source = os.Getenv("OPENCLAW_IMAGE_SOURCE")
	}
	if source != "" {
		fmt.Printf("  %sStep 2 — pulling from %s%s\n", bold, source, nc)
		if err := runVerbose("docker", "pull", source); err == nil {
			if source != "openclaw:local" {
				if err := runVerbose("docker", "tag", source, "openclaw:local"); err != nil {
					return errorf("pull succeeded but tag failed: %v", err)
				}
			}
			if imageExists("openclaw:local") {
				fmt.Printf("\n  %s✓ openclaw:local installed via pull%s\n", green, nc)
				printPostBootstrapNextSteps()
				return nil
			}
		} else {
			fmt.Printf("  %s! pull failed — falling back to source build%s\n\n", gold, nc)
		}
	} else {
		fmt.Printf("  %sStep 2 — no --source= or OPENCLAW_IMAGE_SOURCE; skipping pull%s\n\n", dim, nc)
	}

	// Step 3 — source build.
	if noBuild {
		return errorf("openclaw:local not present, pull skipped/failed, --no-build set")
	}
	if sourceRepo == "" {
		sourceRepo = "https://github.com/openclaw/openclaw"
	}
	buildDir := filepath.Join(os.TempDir(), "claws-openclaw-build")
	fmt.Printf("  %sStep 3 — source build%s\n", bold, nc)
	fmt.Printf("    %srepo:%s     %s\n", dim, nc, sourceRepo)
	fmt.Printf("    %sclone to:%s %s\n", dim, nc, buildDir)
	fmt.Printf("    %sbuild:%s    docker build -t openclaw:local %s\n", dim, nc, buildDir)
	if !yes {
		fmt.Printf("\n  %sPass --yes to run the clone + build (~5-10 minutes the first time).%s\n", gold, nc)
		fmt.Printf("  %sOr install the image manually and re-run `claws doctor`.%s\n\n", dim, nc)
		return errorf("aborted (use --yes to proceed)")
	}

	// git: ensure available.
	if _, err := exec.LookPath("git"); err != nil {
		return errorf("git not found — install git first, or supply a pre-built image via --source=")
	}

	// Idempotent: if buildDir exists, git pull; else git clone.
	if info, err := os.Stat(buildDir); err == nil && info.IsDir() {
		fmt.Printf("\n  %srefreshing existing checkout at %s%s\n", dim, buildDir, nc)
		if err := runVerbose("git", "-C", buildDir, "pull", "--ff-only"); err != nil {
			fmt.Printf("  %s! git pull failed: %v (continuing with existing checkout)%s\n", gold, err, nc)
		}
	} else {
		fmt.Printf("\n  %scloning %s → %s%s\n", dim, sourceRepo, buildDir, nc)
		if err := runVerbose("git", "clone", "--depth=1", sourceRepo, buildDir); err != nil {
			return errorf("git clone failed: %v", err)
		}
	}

	// Memory check + auto-swap, before the heavy build.
	//
	// Goal: a user on a 1 GB VPS who runs `claws setup` and confirms
	// the bootstrap shouldn't have to know the word "swap" exists. The
	// build is going to fail without one, and they've already said yes
	// to "build now". So: detect low RAM, auto-enable swap, print what
	// we're doing, get on with it.
	//
	// Opt-outs:
	//   --no-swap     : never auto-add; accept OOM risk
	//   --add-swap[=] : explicit (legacy from v1.6.19; same effect as auto)
	//
	// Linux-only by construction (newSwapfileManager refuses on other
	// OSes). On macOS, RAM is configured via Docker Desktop's Resources
	// panel, not via swapfile.
	const recommendedRAM = 4 * 1024 * 1024 * 1024 // 4 GB
	// Use MemTotal for the gating decision (not MemAvailable). An 8 GB
	// box that's currently caching aggressively shows low MemAvailable
	// but doesn't actually need swap — the kernel reclaims cache when
	// the build asks for RAM. Auto-adding on those boxes wastes disk
	// and worried clients.
	//
	// MemAvailable is still used for sizing IF we decide to add (a more
	// accurate read of "how much headroom the build has right now").
	ramTotal := totalMemoryBytes()
	availMem := availableMemoryBytes()
	if ramTotal == 0 {
		// Couldn't read /proc/meminfo at all (non-Linux or unusual setup).
		// Fall back to the availMem path: if we can't tell either way,
		// trust whatever number we did manage to get.
		ramTotal = availMem
	}
	if ramTotal > 0 {
		fmt.Printf("\n  %sRAM total: %s   available now: %s   — openclaw build peaks at ~4 GB%s\n", dim, formatBytes(ramTotal), formatBytes(availMem), nc)
		existingSwap := currentSwapBytes()
		if existingSwap > 0 {
			fmt.Printf("  %sExisting swap: %s (total budget = %s)%s\n", dim, formatBytes(existingSwap), formatBytes(ramTotal+existingSwap), nc)
		}
		// Decision uses TOTAL RAM + existing swap. Never `swapoff`
		// anything that isn't ours — only /tmp/claws-bootstrap.swap (or
		// the /var/cache/claws fallback) ever gets touched.
		effectiveRAM := ramTotal + existingSwap
		needMoreRAM := effectiveRAM < recommendedRAM
		switch {
		case needMoreRAM && noSwap:
			fmt.Printf("  %s! Memory budget short + --no-swap — docker build may OOM-kill. Proceeding.%s\n", gold, nc)
		case needMoreRAM && !addSwap:
			// Auto-enable: total RAM+swap is genuinely below 4 GB.
			fmt.Printf("  %s[auto] adding temporary swap for the build (--no-swap to opt out)%s\n", gold, nc)
			addSwap = true
		case !needMoreRAM && !addSwap:
			// Box has enough total RAM (possibly with existing swap).
			// Don't touch anything.
			if existingSwap > 0 {
				fmt.Printf("  %s✓ RAM + existing swap covers the build budget — not adding any%s\n", dim, nc)
			} else {
				fmt.Printf("  %s✓ RAM covers the build budget — not adding swap%s\n", dim, nc)
			}
		case needMoreRAM && addSwap:
			// Explicit operator override; respect it.
		}
	}

	if addSwap {
		var sizeBytes uint64
		if addSwapSize != "" {
			// Operator specified explicit size — honour it (subject to
			// disk-space capping inside newSwapfileManager).
			sizeBytes = parseSwapSize(addSwapSize)
			if sizeBytes == 0 {
				return errorf("--add-swap=%q: invalid size (use e.g. 8g, 4G, 2048m)", addSwapSize)
			}
		} else {
			// Auto-pick: enough to bring RAM + swap to ~6 GB. Existing
			// swap already accounted for in the gating decision above
			// (we only get here if needMoreRAM); use 0 for existingSwap
			// in the size calc to ensure we add enough to actually
			// matter.
			sizeBytes = chooseAutoSwapSize(availMem, 0)
		}
		mgr, err := newSwapfileManager(sizeBytes)
		if err != nil {
			// Failures here include:
			//   - macOS ("configure Docker Desktop")
			//   - no candidate path has enough disk
			//   - sudo missing
			// All have actionable text already; propagate verbatim.
			return errorf("swap setup failed: %v", err)
		}
		defer mgr.disable()
		mgr.installSignalHandler()
		if err := mgr.enable(); err != nil {
			return errorf("failed to enable swap: %v", err)
		}
	}

	fmt.Printf("\n  %sbuilding openclaw:local (this can take several minutes)%s\n", dim, nc)
	if err := runVerbose("docker", "build", "-t", "openclaw:local", buildDir); err != nil {
		return errorf("docker build failed: %v", err)
	}
	if !imageExists("openclaw:local") {
		return errorf("build completed but openclaw:local not present — investigate manually")
	}
	fmt.Printf("\n  %s✓ openclaw:local built from source%s\n", green, nc)
	fmt.Printf("  %s(checkout kept at %s — safe to remove)%s\n\n", dim, buildDir, nc)
	printPostBootstrapNextSteps()
	return nil
}

// printPostBootstrapNextSteps prints a uniform "what to do next" hint after
// any successful image bootstrap path (already-present, tarball, pull, build).
// The output is context-aware: if no instances exist on this host, suggest
// `claws setup` (the interactive onboarding flow). If instances already
// exist, point at `claws start <name>` / `claws agent ping <name>`.
//
// The image is necessary but not sufficient — agents still need to be
// created, authed, and channel-wired. Non-technical users need this
// explicit next-step output so the success message isn't a dead-end.
func printPostBootstrapNextSteps() {
	const (
		bold  = "\033[1m"
		dim   = "\033[0;90m"
		green = "\033[0;32m"
		nc    = "\033[0m"
	)
	paths := resolvePaths()
	names, _ := listInstanceNames(paths)

	fmt.Println()
	fmt.Printf("  %sNext:%s\n", bold, nc)
	if len(names) == 0 {
		// Fresh box — guide the user into the wizard.
		fmt.Printf("    %s$%s claws setup                  %s# interactive onboarding (team, agent, auth, channel)%s\n", dim, nc, dim, nc)
		fmt.Printf("    %sor:%s claws create <team>/<agent> + auth + channel manually (see: claws --help)\n", dim, nc)
	} else {
		// Existing fleet — assume the operator was refreshing the image.
		fmt.Printf("    %s$%s claws list                    %s# show all instances%s\n", dim, nc, dim, nc)
		fmt.Printf("    %s$%s claws start-all               %s# start everything%s\n", dim, nc, dim, nc)
		fmt.Printf("    %s$%s claws agent ping <name>       %s# verify an agent end-to-end%s\n", dim, nc, dim, nc)
	}
	fmt.Println()
	fmt.Printf("  %sVerify:%s docker images openclaw:local\n", green, nc)
	fmt.Println()
}

// imageExists returns true if `docker image inspect <name>` succeeds.
func imageExists(name string) bool {
	cmd := exec.Command("docker", "image", "inspect", name)
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run() == nil
}

// runVerbose executes a command, streaming stdout/stderr to the user's TTY
// so they see exactly what's happening. Returns the command's error verbatim.
func runVerbose(name string, args ...string) error {
	const dim = "\033[0;90m"
	const nc = "\033[0m"
	fmt.Printf("    %s$ %s %s%s\n", dim, name, strings.Join(args, " "), nc)
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ---------------------------------------------------------------------------
// Tarball bootstrap path — `claws image bootstrap --from-tarball[=<URL>]`
//
// Downloads a pre-built `docker save` tarball of openclaw:local, verifies
// SHA256 against a sibling `.sha256` file, runs `docker load`, retags as
// openclaw:local. Skips the source build entirely — designed for hosts
// without the ~4 GB RAM the openclaw build needs.
// ---------------------------------------------------------------------------

// bootstrapFromTarball fetches the URL, verifies SHA256, docker-loads,
// and re-tags as openclaw:local. Self-contained: any intermediate file
// lands in a per-invocation tmpdir that's removed on every exit path.
func bootstrapFromTarball(url string) error {
	const dim = "\033[0;90m"
	const green = "\033[0;32m"
	const nc = "\033[0m"

	// Disk-free pre-flight. The tarball + the loaded image content + temp
	// docker layers need headroom. Fail fast if we're going to OOM mid-load.
	if err := requireDiskFree("/", tarballMinDiskFreeBytes); err != nil {
		return err
	}

	tmpdir, err := os.MkdirTemp("", "claws-tarball-*")
	if err != nil {
		return fmt.Errorf("create tmpdir: %w", err)
	}
	// Per feedback_no_rm_rf.md, mktemp-d-scoped removal is the right pattern —
	// we own the dir, only created files under it, never bulk-delete elsewhere.
	defer os.RemoveAll(tmpdir)

	tarPath := filepath.Join(tmpdir, "openclaw.tar.gz")
	fmt.Printf("    %s$ download → %s%s\n", dim, tarPath, nc)
	gotHash, err := fetchTarball(url, tarPath)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	expected, err := fetchExpectedSha256(url + ".sha256")
	if err != nil {
		return fmt.Errorf("sha256 sidecar: %w", err)
	}
	if !strings.EqualFold(expected, gotHash) {
		return fmt.Errorf("sha256 mismatch:\n    expected: %s\n    got:      %s\n  refusing to load tarball that doesn't match its sidecar", expected, gotHash)
	}
	fmt.Printf("    %s✓ sha256 verified: %s%s\n", green, gotHash[:16]+"...", nc)

	// docker load reads the tarball + adds layers. Parse the loaded tag
	// from stdout — usually "Loaded image: openclaw:local" but the tarball
	// may carry a different tag if someone built from a fork.
	fmt.Printf("    %s$ docker load -i %s%s\n", dim, tarPath, nc)
	loaded, err := dockerLoad(tarPath)
	if err != nil {
		return fmt.Errorf("docker load: %w", err)
	}
	fmt.Printf("    %s✓ loaded image: %s%s\n", green, loaded, nc)

	if loaded != "openclaw:local" {
		fmt.Printf("    %s$ docker tag %s openclaw:local%s\n", dim, loaded, nc)
		if err := exec.Command("docker", "tag", loaded, "openclaw:local").Run(); err != nil {
			return fmt.Errorf("docker tag: %w", err)
		}
	}
	return nil
}

// fetchTarball streams URL into dest, computing SHA256 in-flight via
// io.TeeReader so we don't need a second pass over the 683 MB file.
// Returns the hex-encoded SHA256 of what was downloaded.
func fetchTarball(url, dest string) (string, error) {
	// Total budget 30 min — slow VPS-to-GitHub links can take ages.
	client := &http.Client{Timeout: 30 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "claws image bootstrap")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d %s", resp.StatusCode, resp.Status)
	}

	f, err := os.Create(dest)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, label: filepath.Base(dest)}
	if _, err := io.Copy(f, io.TeeReader(pr, h)); err != nil {
		return "", err
	}
	pr.done()
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fetchExpectedSha256 fetches the small sidecar URL and parses the first
// whitespace-separated token as the expected hex digest. Format mirrors
// `sha256sum <file>` output: "<hex> <filename>".
func fetchExpectedSha256(url string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "claws image bootstrap")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d fetching %s — cannot verify download integrity", resp.StatusCode, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024))
	if err != nil {
		return "", err
	}
	tok := strings.Fields(string(body))
	if len(tok) == 0 || len(tok[0]) != 64 {
		return "", fmt.Errorf("sidecar at %s doesn't look like a sha256sum file (need 64-char hex)", url)
	}
	return tok[0], nil
}

// dockerLoad runs `docker load -i <tarPath>` and parses the loaded tag.
// Returns the first reasonable image reference found in stdout.
func dockerLoad(tarPath string) (string, error) {
	cmd := exec.Command("docker", "load", "-i", tarPath)
	out, err := cmd.Output()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return "", fmt.Errorf("%v: %s", err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", err
	}
	// Each "Loaded image:" line indicates a tag added. We take the first.
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		const prefix = "Loaded image: "
		if strings.HasPrefix(line, prefix) {
			return strings.TrimPrefix(line, prefix), nil
		}
		// Some docker versions print "Loaded image ID: sha256:..." when no
		// tag was attached — that's a degenerate tarball we can't usefully
		// retag without more work; surface it for the operator.
		const idPrefix = "Loaded image ID: "
		if strings.HasPrefix(line, idPrefix) {
			return "", fmt.Errorf("tarball loaded by ID only (no tag) — was it built with `docker save <hash>`? need `docker save openclaw:local`")
		}
	}
	return "", fmt.Errorf("docker load completed but no 'Loaded image:' line in output:\n%s", string(out))
}

// requireDiskFree returns an error if the filesystem holding `path` has
// fewer than `bytes` free. Pre-flight for the tarball download + load.
func requireDiskFree(path string, bytes uint64) error {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return nil // best-effort; don't fail the install on a non-Linux statfs
	}
	free := st.Bavail * uint64(st.Bsize)
	if free < bytes {
		return fmt.Errorf("only %s free on %s — need %s for tarball + image load", formatBytes(free), path, formatBytes(bytes))
	}
	return nil
}

// progressReader wraps an io.Reader and prints a progress line every ~1s.
// Quiet when stdout is not a TTY (cloud-init, cron, piped to file).
type progressReader struct {
	r           io.Reader
	read, total int64
	label       string
	lastPrint   time.Time
	isTTY       bool
	initOnce    bool
}

func (p *progressReader) Read(b []byte) (int, error) {
	if !p.initOnce {
		p.initOnce = true
		// Best-effort TTY detection — if stdout's mode lookup fails, assume not.
		if fi, err := os.Stdout.Stat(); err == nil {
			p.isTTY = (fi.Mode() & os.ModeCharDevice) != 0
		}
	}
	n, err := p.r.Read(b)
	p.read += int64(n)
	now := time.Now()
	if now.Sub(p.lastPrint) > 750*time.Millisecond {
		p.lastPrint = now
		if p.isTTY {
			if p.total > 0 {
				pct := 100 * p.read / p.total
				fmt.Fprintf(os.Stdout, "\r    \033[0;90m%s  %s / %s  (%d%%)\033[0m", p.label, formatBytes(uint64(p.read)), formatBytes(uint64(p.total)), pct)
			} else {
				fmt.Fprintf(os.Stdout, "\r    \033[0;90m%s  %s\033[0m", p.label, formatBytes(uint64(p.read)))
			}
		}
	}
	return n, err
}
func (p *progressReader) done() {
	if p.isTTY {
		// Clear line + newline. Otherwise the next print lands on top.
		fmt.Fprintf(os.Stdout, "\r\033[K")
	}
	if p.total > 0 {
		fmt.Printf("    \033[0;90m✓ downloaded %s%s\n", formatBytes(uint64(p.read)), "\033[0m")
	} else {
		fmt.Printf("    \033[0;90m✓ downloaded %s%s\n", formatBytes(uint64(p.read)), "\033[0m")
	}
}
