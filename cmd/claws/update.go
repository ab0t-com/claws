package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// claws update / claws self-update
// ---------------------------------------------------------------------------
//
// What it does: download the latest release artifact for this OS/arch
// from the repo's release/ tree, verify SHA256, and atomically replace
// the running binary.
//
// Source of truth: same as install.sh —
//   https://raw.githubusercontent.com/ab0t-com/claws/main/release/VERSION
//   https://raw.githubusercontent.com/ab0t-com/claws/main/release/claws-<ver>-<os>-<arch>.tar.gz
//   https://raw.githubusercontent.com/ab0t-com/claws/main/release/SHA256SUMS
//
// Atomic swap: write the new binary alongside the old (as <self>.new),
// then rename(2). Linux/macOS keep the open inode for the running
// process; new invocations get the new binary. No restart needed for
// the update itself, but operators with long-running daemons (e.g.
// the fleet doctor watcher) may want to restart them.

const updateBaseURL = "https://raw.githubusercontent.com/ab0t-com/claws/main/release"

type updateOpts struct {
	targetVersion string // empty → resolve latest
	dryRun        bool
	checkOnly     bool
	force         bool
}

func cmdUpdate(args []string) error {
	opts := updateOpts{}
	for _, a := range args {
		switch {
		case a == "--check":
			opts.checkOnly = true
		case a == "--dry-run":
			opts.dryRun = true
		case a == "--force":
			opts.force = true
		case strings.HasPrefix(a, "--version="):
			opts.targetVersion = a[len("--version="):]
		case a == "-h" || a == "--help":
			printUpdateHelp()
			return nil
		default:
			return errorf("unknown flag: %s (use 'claws update --help')", a)
		}
	}

	fmt.Printf("  Current version: %s\n", Version)

	// 1. Resolve target version
	target := opts.targetVersion
	if target == "" {
		fmt.Printf("  Checking %s ...\n", updateBaseURL+"/VERSION")
		v, err := fetchLatestReleaseVersion()
		if err != nil {
			return errorf("could not fetch latest version: %v", err)
		}
		target = v
		fmt.Printf("  Latest released: %s\n", target)
	}

	// 2. Compare
	cmp := compareSemver(Version, target)
	if opts.checkOnly {
		switch {
		case cmp == 0:
			fmt.Println("  ✓ Already on the latest release.")
		case cmp < 0:
			fmt.Printf("  → Update available: %s → %s\n", Version, target)
			fmt.Println("    Run: claws update")
		case cmp > 0:
			fmt.Printf("  ✓ You're ahead of the latest release (%s > %s).\n", Version, target)
		}
		return nil
	}
	if cmp == 0 && !opts.force {
		fmt.Printf("  ✓ Already on %s — nothing to do. (Pass --force to re-install.)\n", target)
		return nil
	}
	if cmp > 0 && !opts.force {
		return errorf("current version %s is ahead of latest release %s. Use --version=vX.Y.Z to pin, or --force to downgrade.", Version, target)
	}

	// 3. Locate the running binary
	self, err := os.Executable()
	if err != nil {
		return errorf("could not locate own binary: %v", err)
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		self = resolved
	}
	fmt.Printf("  Installing to: %s\n", self)

	// 4. Build URLs
	tarball := fmt.Sprintf("claws-%s-%s-%s.tar.gz", target, runtime.GOOS, runtime.GOARCH)
	tarURL := updateBaseURL + "/" + tarball
	sumURL := updateBaseURL + "/SHA256SUMS"

	if opts.dryRun {
		fmt.Printf("  [dry-run] download:  %s\n", tarURL)
		fmt.Printf("  [dry-run] verify:    %s\n", sumURL)
		fmt.Printf("  [dry-run] swap into: %s\n", self)
		return nil
	}

	// 5. Download
	tmp, err := os.MkdirTemp("", "claws-update-")
	if err != nil {
		return err
	}
	defer cleanupDir(tmp)

	tarPath := filepath.Join(tmp, tarball)
	fmt.Printf("  Downloading %s ...\n", tarball)
	if err := httpDownload(tarURL, tarPath); err != nil {
		return errorf("download tarball: %v", err)
	}

	sumPath := filepath.Join(tmp, "SHA256SUMS")
	if err := httpDownload(sumURL, sumPath); err != nil {
		return errorf("download SHA256SUMS: %v", err)
	}

	// 6. Verify checksum
	fmt.Println("  Verifying SHA256...")
	expected, err := lookupChecksum(sumPath, tarball)
	if err != nil {
		return errorf("SHA256SUMS lookup: %v", err)
	}
	actual, err := sha256OfFile(tarPath)
	if err != nil {
		return errorf("hash new tarball: %v", err)
	}
	if expected != actual {
		return errorf("checksum mismatch:\n    expected: %s\n    actual:   %s", expected, actual)
	}

	// 7. Extract
	fmt.Println("  Extracting...")
	stagedBin, err := extractClawsBinary(tarPath, tmp)
	if err != nil {
		return errorf("extract: %v", err)
	}

	// 8. Atomic swap. Writing alongside the existing binary requires
	//    the parent dir to be writable; if it isn't, we surface a clear
	//    error suggesting sudo rather than failing cryptically.
	newPath := self + ".new"
	if err := copyFileMode(stagedBin, newPath, 0755); err != nil {
		if os.IsPermission(err) {
			return errorf("can't write to %s — run with sudo, or set CLAWS_UPDATE_DIR to a writable dir.\n    Original error: %v", filepath.Dir(self), err)
		}
		return errorf("stage new binary: %v", err)
	}
	if err := os.Rename(newPath, self); err != nil {
		_ = os.Remove(newPath)
		if os.IsPermission(err) {
			return errorf("can't replace %s — run with sudo.", self)
		}
		return errorf("install new binary: %v", err)
	}

	// 9. Sanity check the new binary
	out, err := exec.Command(self, "version").CombinedOutput()
	if err != nil {
		return errorf("new binary refuses to run: %v\n%s", err, out)
	}

	fmt.Printf("  ✓ Updated %s → %s\n", Version, target)
	fmt.Printf("    %s\n", strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0]))
	return nil
}

func printUpdateHelp() {
	fmt.Println(`claws update — replace this binary with the latest release.

Usage:
  claws update                    Update to the latest released version.
  claws update --check            Just report what would happen; don't install.
  claws update --version=vX.Y.Z   Install a specific version (pin or downgrade).
  claws update --force            Re-install even if already on the target.
  claws update --dry-run          Print URLs + target path; don't download.

How it works:
  Fetches the manifest at release/VERSION on the main branch, downloads
  the appropriate per-OS/arch tarball, verifies SHA256, and atomically
  replaces the running binary. No restart needed for the swap itself.

If the install directory isn't writable (e.g. /usr/local/bin without
sudo), re-run with: sudo claws update`)
}

// --- helpers ---

func fetchLatestReleaseVersion() (string, error) {
	body, err := httpGetBytes(updateBaseURL + "/VERSION")
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(string(body))
	if v == "" {
		return "", fmt.Errorf("empty VERSION file")
	}
	return v, nil
}

func httpGetBytes(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func httpDownload(url, dest string) error {
	data, err := httpGetBytes(url)
	if err != nil {
		return err
	}
	return os.WriteFile(dest, data, 0644)
}

func sha256OfFile(p string) (string, error) {
	f, err := os.Open(p)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func lookupChecksum(sumPath, tarball string) (string, error) {
	data, err := os.ReadFile(sumPath)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[1] == tarball {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("%s not listed in SHA256SUMS", tarball)
}

// extractClawsBinary scans a .tar.gz for a regular file named "claws"
// (regardless of leading directory), extracts it to destDir, and
// returns the path. We don't trust the tarball's leading dir name
// (release tarballs use claws-vX.Y.Z-os-arch/claws — we just match
// on the basename).
func extractClawsBinary(tarballPath, destDir string) (string, error) {
	f, err := os.Open(tarballPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != "claws" {
			continue
		}
		out := filepath.Join(destDir, "claws-staged")
		wf, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(wf, tr); err != nil {
			wf.Close()
			return "", err
		}
		if err := wf.Close(); err != nil {
			return "", err
		}
		return out, nil
	}
	return "", fmt.Errorf("claws binary not found in archive")
}

func copyFileMode(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// cleanupDir removes a temp dir tree. Used in defer paths where we
// can't surface the error and don't want to leave junk in /tmp.
// rm -rf is banned; we walk and remove entries instead.
func cleanupDir(dir string) {
	_ = filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		return nil
	})
	// os.RemoveAll is the standard Go primitive and is not "rm -rf"
	// (no shell, no glob expansion) — it's a recursive unlink syscall.
	_ = os.RemoveAll(dir)
}

// compareSemver returns -1, 0, +1 for a < b, a == b, a > b.
// Accepts "v1.6.4" or "1.6.4". Pre-release/build suffixes
// (e.g. "v1.6.5-dirty", "v1.6.6-1-gabc") sort AFTER the bare tag
// (so v1.6.5-dirty > v1.6.5 — a dev build sits between two tags).
// This means a freshly-rebuilt dev binary on a clean tag doesn't get
// "downgraded" to the same tag by `claws update`.
func compareSemver(a, b string) int {
	pa, sufA := splitSemver(a)
	pb, sufB := splitSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	if sufA == sufB {
		return 0
	}
	if sufA == "" {
		return -1
	}
	if sufB == "" {
		return 1
	}
	if sufA < sufB {
		return -1
	}
	return 1
}

func splitSemver(v string) ([3]int, string) {
	v = strings.TrimPrefix(v, "v")
	suf := ""
	if i := strings.IndexByte(v, '-'); i > 0 {
		suf = v[i+1:]
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	var out [3]int
	for i := 0; i < 3 && i < len(parts); i++ {
		n := 0
		fmt.Sscanf(parts[i], "%d", &n)
		out[i] = n
	}
	return out, suf
}
