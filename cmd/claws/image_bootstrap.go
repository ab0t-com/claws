package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

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
	var source, sourceRepo string
	var yes, noBuild bool
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Println(`Usage: claws image bootstrap [--source=<image:tag>] [--source-repo=<git-url>] [--yes] [--no-build]

Ensure openclaw:local is present on this host. Order of operations:
  1. If openclaw:local already exists → no-op.
  2. If --source= or OPENCLAW_IMAGE_SOURCE is set → docker pull from it.
  3. Else, clone + build from --source-repo (default: github.com/openclaw/openclaw).

Flags:
  --source=<image:tag>         Pull this image and tag it openclaw:local
  --source-repo=<git-url>      Git source to clone + build (default: github.com/openclaw/openclaw)
  --yes                        Skip the "about to run docker build" confirmation
  --no-build                   Skip the build fallback (pull only)

Examples:
  claws image bootstrap                                          # try repo build (default)
  claws image bootstrap --source=openclaw/runtime:v2026.5        # pull from a registry
  OPENCLAW_IMAGE_SOURCE=openclaw/runtime:latest claws image bootstrap`)
			return nil
		case strings.HasPrefix(a, "--source="):
			source = strings.TrimPrefix(a, "--source=")
		case strings.HasPrefix(a, "--source-repo="):
			sourceRepo = strings.TrimPrefix(a, "--source-repo=")
		case a == "--yes" || a == "-y":
			yes = true
		case a == "--no-build":
			noBuild = true
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
		return nil
	}
	fmt.Printf("  %sopenclaw:local not present.%s\n\n", dim, nc)

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

	fmt.Printf("\n  %sbuilding openclaw:local (this can take several minutes)%s\n", dim, nc)
	if err := runVerbose("docker", "build", "-t", "openclaw:local", buildDir); err != nil {
		return errorf("docker build failed: %v", err)
	}
	if !imageExists("openclaw:local") {
		return errorf("build completed but openclaw:local not present — investigate manually")
	}
	fmt.Printf("\n  %s✓ openclaw:local built from source%s\n", green, nc)
	fmt.Printf("  %s(checkout kept at %s — safe to remove)%s\n\n", dim, buildDir, nc)
	return nil
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
