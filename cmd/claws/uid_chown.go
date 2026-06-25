package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// ---------------------------------------------------------------------------
// uid_chown.go — handle the root-vs-uid-1000 mismatch
//
// Problem: openclaw's container runs as uid 1000 (Dockerfile: `USER node`).
// When claws runs as root (uid 0) — common on fresh EC2/RHEL/Amazon Linux
// 2023 boxes — every file claws writes is owned by root:root. The container
// mounts those files via bind mount and immediately EACCESses on its own
// config. Auth, exec, start all fail.
//
// Fix: at instance-create time, after we've written all the agent's files,
// if we're running as root, chown the instance dir + everything under it
// to the runtime's container UID. Pre-flight chown helpers also surface
// the issue for ALREADY-broken agents (cmdAuth + cmdExec call them).
//
// Non-root non-1000 case: out of scope for v1.6.30 — the user has to
// either run with sudo or pass OPENCLAW_USER_UID through to docker compose.
// That's a separate ticket.
// ---------------------------------------------------------------------------

// runtimeContainerUID returns the uid the openclaw runtime container runs
// as. Today hardcoded to 1000 (openclaw Dockerfile: USER node, uid 1000).
// Future: read from runtime.json's optional ContainerUID field; if absent,
// fall back to 1000.
func runtimeContainerUID(rt Runtime) int {
	// Future: rt.ContainerUID if non-zero
	return 1000
}

// runningAsRoot reports whether the current process is running as uid 0.
// Cheap; the gate for all of this module's logic.
func runningAsRoot() bool {
	return os.Geteuid() == 0
}

// uidMismatchActive returns true iff there's anything to fix: we're running
// as root AND the runtime expects a non-root uid. If we're already running
// as the right uid (or some other uid that owns its own files), no chown
// is needed.
func uidMismatchActive(rt Runtime) bool {
	if !runningAsRoot() {
		return false
	}
	return runtimeContainerUID(rt) != 0
}

// chownInstanceDir walks `dir` and chowns every file + subdir to the
// runtime's container uid (and gid set to the same value — openclaw's
// node user is uid 1000 / gid 1000). Idempotent; safe to call repeatedly.
//
// Returns nil on best-effort success. Failures (e.g. a read-only mount,
// permission denied for a sub-file we don't own) are surfaced as
// non-fatal warnings via the returned error; caller decides whether to
// hard-fail. The chown applies to the dir itself + ALL contents recursively.
func chownInstanceDir(dir string, uid int) error {
	if uid < 0 {
		return nil
	}
	var firstErr error
	err := filepath.Walk(dir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			if firstErr == nil {
				firstErr = walkErr
			}
			return nil
		}
		// Skip symlinks — Lchown would change link itself, not target;
		// in practice we don't create symlinks but be defensive.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if err := os.Chown(path, uid, uid); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return firstErr
}

// ensureRuntimeReadable is the pre-flight: confirm the runtime container's
// uid can read the instance's `openclaw.json`. If we're running as root and
// the file is unreadable to that uid, fix it. Otherwise surface a clear
// directive so the operator can chown by hand.
//
// Called from cmdAuth, cmdExec, cmdStart, cmdRestart — anywhere claws is
// about to invoke a docker compose run / exec against the runtime that
// would fail with EACCES if file ownership is wrong.
func ensureRuntimeReadable(paths Paths, name string) error {
	rt := mustResolveRuntime(paths, name)
	if !uidMismatchActive(rt) {
		return nil
	}
	ref, _ := ParseRef(name)
	dir := ref.Dir(paths)
	configPath := filepath.Join(dir, rt.ConfigFileName)
	st, err := os.Stat(configPath)
	if err != nil {
		// File doesn't exist — caller will hit its own error.
		return nil
	}
	containerUID := runtimeContainerUID(rt)
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		if int(sys.Uid) == containerUID {
			// Already owned correctly; nothing to do.
			return nil
		}
	}
	// Mismatch detected. Auto-fix if we're root, else surface.
	if runningAsRoot() {
		fmt.Printf("    \033[0;90mauto-fixing file ownership for runtime container (uid %d) on %s\033[0m\n", containerUID, name)
		if err := chownInstanceDir(dir, containerUID); err != nil {
			return errorf("chown %s to uid %d failed: %v\n  fix manually: sudo chown -R %d:%d %s",
				dir, containerUID, err, containerUID, containerUID, dir)
		}
		return nil
	}
	// Not root — can't fix. Tell operator exactly what to run.
	return errorf(`%s files are owned by uid %d but openclaw container needs uid %d.
  fix: sudo chown -R %d:%d %s`,
		name, int(stUid(st)), containerUID, containerUID, containerUID, dir)
}

// stUid extracts the uid from a FileInfo's Sys() (Linux only). Returns 0
// on non-Linux / unknown — caller treats that as "no info, can't decide",
// which is fine because uidMismatchActive() already gated on Linux/root.
func stUid(info os.FileInfo) int {
	if sys, ok := info.Sys().(*syscall.Stat_t); ok {
		return int(sys.Uid)
	}
	return 0
}

// cmdRepairOwnership — `claws repair-ownership [--dry-run]`
//
// Walks every instance dir under the fleet and chowns it to the runtime's
// container uid. Idempotent; does nothing if files already match. Designed
// for clients who had agents created BEFORE v1.6.30's auto-chown landed
// (so their existing fleet is root-owned and broken), or who manually
// moved files around and lost ownership.
//
// Refuses if not running as root (chown of non-owned files needs root).
func cmdRepairOwnership(args []string) error {
	dryRun := false
	for _, a := range args {
		switch a {
		case "--dry-run", "-n":
			dryRun = true
		case "-h", "--help":
			fmt.Println(`Usage: claws repair-ownership [--dry-run]

Walks every instance under the fleet and chowns files to the runtime
container's uid (1000 for openclaw). Idempotent. Designed for fleets
created BEFORE v1.6.30's auto-chown landed.

Flags:
  --dry-run, -n    Show what would change; don't actually chown.

Examples:
  sudo claws repair-ownership            # fix every instance
  sudo claws repair-ownership --dry-run  # preview only`)
			return nil
		}
	}
	if !runningAsRoot() {
		return errorf("claws repair-ownership must be run as root (need to chown files we don't own)\n  sudo claws repair-ownership")
	}
	paths := resolvePaths()
	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("No instances to repair.")
		return nil
	}
	const (
		bold  = "\033[1m"
		dim   = "\033[0;90m"
		green = "\033[0;32m"
		nc    = "\033[0m"
	)
	fmt.Printf("%sclaws repair-ownership%s\n", bold, nc)
	if dryRun {
		fmt.Printf("  %s(dry-run — no files modified)%s\n", dim, nc)
	}
	fmt.Println()

	var fixed, alreadyOK, failed int
	for _, e := range entries {
		rt := mustResolveRuntime(paths, e.Name)
		containerUID := runtimeContainerUID(rt)
		ref, _ := ParseRef(e.Name)
		dir := ref.Dir(paths)
		// Quick check: is openclaw.json already the right uid?
		configPath := filepath.Join(dir, rt.ConfigFileName)
		if st, err := os.Stat(configPath); err == nil {
			if stUid(st) == containerUID {
				fmt.Printf("  %s✓%s %s already uid %d\n", green, nc, e.Name, containerUID)
				alreadyOK++
				continue
			}
		}
		if dryRun {
			fmt.Printf("  %s→%s would chown %s to uid %d\n", dim, nc, e.Name, containerUID)
			fixed++
			continue
		}
		if err := chownInstanceDir(dir, containerUID); err != nil {
			fmt.Printf("  \033[0;31m✗\033[0m %s chown failed: %v\n", e.Name, err)
			failed++
			continue
		}
		fmt.Printf("  %s✓%s %s chowned to uid %d\n", green, nc, e.Name, containerUID)
		fixed++
	}
	fmt.Println()
	fmt.Printf("  %d ok, %d %s, %d failed.\n",
		alreadyOK,
		fixed,
		func() string {
			if dryRun {
				return "would-fix"
			}
			return "fixed"
		}(),
		failed)
	if failed > 0 {
		return errorf("%d instance(s) failed to chown — see output above", failed)
	}
	return nil
}

// printRootRunningBanner emits a one-line banner during `claws setup`
// when running as root. Tells the operator what's about to happen so the
// "files getting chowned to uid 1000" isn't a surprise. Non-blocking —
// claws proceeds either way.
func printRootRunningBanner() {
	if !runningAsRoot() {
		return
	}
	const (
		yellow = "\033[0;33m"
		dim    = "\033[0;90m"
		nc     = "\033[0m"
	)
	fmt.Printf("    %s! Running as root.%s Files will be chowned to uid 1000 so the openclaw\n", yellow, nc)
	fmt.Printf("      container can read them. %sFor production, prefer a non-root user:%s\n", dim, nc)
	fmt.Printf("        %suseradd -m -G docker claws && su - claws && claws setup%s\n", dim, nc)
	fmt.Println()
}
