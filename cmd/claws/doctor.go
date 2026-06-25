package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// ---------------------------------------------------------------------------
// claws version
// ---------------------------------------------------------------------------

func cmdVersion(args []string) error {
	fmt.Printf("claws %s\n", Version)
	fmt.Printf("  go:     %s\n", runtime.Version())
	fmt.Printf("  os:     %s/%s\n", runtime.GOOS, runtime.GOARCH)

	// Docker version
	if out, err := exec.Command("docker", "version", "--format", "{{.Server.Version}}").Output(); err == nil {
		fmt.Printf("  docker: %s\n", trimSpace(string(out)))
	} else {
		fmt.Printf("  docker: not found\n")
	}

	// Docker Compose version
	if out, err := exec.Command("docker", "compose", "version", "--short").Output(); err == nil {
		fmt.Printf("  compose: %s\n", trimSpace(string(out)))
	} else {
		fmt.Printf("  compose: not found\n")
	}

	// Image
	image := os.Getenv("OPENCLAW_IMAGE")
	if image == "" {
		image = "openclaw:local"
	}
	fmt.Printf("  image:  %s\n", image)

	return nil
}

// ---------------------------------------------------------------------------
// claws doctor — diagnose environment
// ---------------------------------------------------------------------------

type checkResult struct {
	Name    string
	OK      bool
	Message string
}

func cmdDoctor(args []string) error {
	paths := resolvePaths()
	fix := hasFlag(args, "--fix")
	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"

	fmt.Printf("%sclaws doctor%s — checking environment\n\n", bold, nc)

	// --fix: remediate file permissions before running checks
	if fix {
		info("Fixing file permissions...")
		envFixed, credFixed, regFixed := fixAllPermissions(paths.Root)
		if envFixed+credFixed+regFixed > 0 {
			fmt.Printf("  Fixed: %d instance.env, %d credential files, %d registry\n\n", envFixed, credFixed, regFixed)
		} else {
			fmt.Println("  All permissions already correct.")
		fmt.Println()
		}
	}

	var checks []checkResult

	// 1. Docker
	_, err := exec.LookPath("docker")
	if err == nil {
		if out, err := exec.Command("docker", "info", "--format", "{{.ServerVersion}}").Output(); err == nil {
			checks = append(checks, checkResult{"Docker", true, fmt.Sprintf("running (v%s)", trimSpace(string(out)))})
		} else {
			checks = append(checks, checkResult{"Docker", false, "installed but not running — start Docker daemon"})
		}
	} else {
		checks = append(checks, checkResult{"Docker", false, "not installed — https://docs.docker.com/get-docker/"})
	}

	// 2. Docker Compose v2
	if out, err := exec.Command("docker", "compose", "version", "--short").Output(); err == nil {
		checks = append(checks, checkResult{"Docker Compose", true, fmt.Sprintf("v%s", trimSpace(string(out)))})
	} else {
		checks = append(checks, checkResult{"Docker Compose", false, "not found — install Docker Compose v2"})
	}

	// 3. OPENCLAW_ROOT exists and writable
	if fi, err := os.Stat(paths.Root); err == nil && fi.IsDir() {
		// Check writable
		testFile := paths.Root + "/.doctor-test"
		if err := os.WriteFile(testFile, []byte("test"), 0600); err == nil {
			os.Remove(testFile)
			checks = append(checks, checkResult{"OPENCLAW_ROOT", true, paths.Root})
		} else {
			checks = append(checks, checkResult{"OPENCLAW_ROOT", false, fmt.Sprintf("%s exists but not writable", paths.Root)})
		}
	} else {
		checks = append(checks, checkResult{"OPENCLAW_ROOT", false, fmt.Sprintf("%s does not exist — run: claws init", paths.Root)})
	}

	// 4. Compose template findable
	if _, err := os.Stat(paths.ComposeTemplate); err == nil {
		checks = append(checks, checkResult{"Compose template", true, paths.ComposeTemplate})
	} else {
		checks = append(checks, checkResult{"Compose template", false, "docker-compose.yml not found next to claws binary"})
	}

	// 5. OpenClaw image
	image := os.Getenv("OPENCLAW_IMAGE")
	if image == "" {
		image = "openclaw:local"
	}
	// v1.6.30 — surface the root/uid-1000 mismatch as a doctor check.
	// Triggered any time claws is run as root AND there's at least one
	// instance on disk owned by a uid other than the runtime container's uid.
	if runningAsRoot() {
		entries, _ := readRegistry(paths)
		var misOwned []string
		for _, e := range entries {
			rt := mustResolveRuntime(paths, e.Name)
			containerUID := runtimeContainerUID(rt)
			ref, _ := ParseRef(e.Name)
			configPath := filepath.Join(ref.Dir(paths), rt.ConfigFileName)
			if st, err := os.Stat(configPath); err == nil {
				if u := stUid(st); u != containerUID {
					misOwned = append(misOwned, e.Name)
				}
			}
		}
		if len(misOwned) == 0 {
			checks = append(checks, checkResult{"File ownership", true, "all instance files match runtime container uid"})
		} else {
			checks = append(checks, checkResult{
				"File ownership",
				false,
				fmt.Sprintf("%d instance(s) own files as wrong uid — run: claws repair-ownership", len(misOwned)),
			})
		}
	}

	if out, err := exec.Command("docker", "image", "inspect", image, "--format", "{{.Id}}").Output(); err == nil {
		short := trimSpace(string(out))
		if len(short) > 19 {
			short = short[:19]
		}
		checks = append(checks, checkResult{"OpenClaw image", true, fmt.Sprintf("%s (%s)", image, short)})
	} else {
		// v1.6.27: on low-RAM hosts surface the --from-tarball path
		// because the source build will OOM. Operators on small VPSes
		// otherwise spend 20 minutes wondering why the build dies.
		hint := "build or pull the image — run: claws image bootstrap --yes"
		if ramTotalBytes := totalMemoryBytes(); ramTotalBytes > 0 && ramTotalBytes < 4*1024*1024*1024 {
			hint = "build needs ~4 GB RAM (you have " + formatBytes(ramTotalBytes) + ") — run: claws image bootstrap --from-tarball --yes"
		}
		checks = append(checks, checkResult{"OpenClaw image", false, fmt.Sprintf("%s not found — %s", image, hint)})
	}

	// 6. Disk space
	var stat syscall.Statfs_t
	if err := syscall.Statfs(paths.Root, &stat); err == nil {
		freeGB := float64(stat.Bavail*uint64(stat.Bsize)) / (1024 * 1024 * 1024)
		if freeGB >= 2 {
			checks = append(checks, checkResult{"Disk space", true, fmt.Sprintf("%.1f GB free", freeGB)})
		} else {
			checks = append(checks, checkResult{"Disk space", false, fmt.Sprintf("%.1f GB free — recommend at least 2 GB", freeGB)})
		}
	}

	// 7. Instance count
	count := instanceCount(paths)
	checks = append(checks, checkResult{"Instances", true, fmt.Sprintf("%d / %d (max)", count, maxInstances)})

	// 8. Optional tools
	optionalTools := []struct {
		name string
		hint string
	}{
		{"rclone", "for S3 storage sync"},
		{"aws", "for S3 storage setup"},
		{"mount-s3", "for S3 FUSE mount"},
		{"caddy", "for reverse proxy"},
	}
	for _, tool := range optionalTools {
		if _, err := exec.LookPath(tool.name); err == nil {
			checks = append(checks, checkResult{tool.name, true, "installed (optional)"})
		} else {
			checks = append(checks, checkResult{tool.name, true, fmt.Sprintf("not installed — %s (optional)", tool.hint)})
		}
	}

	// Print results
	passed, warnings, errors := 0, 0, 0
	for _, c := range checks {
		marker := green + "OK" + nc
		if !c.OK {
			// Distinguish hard errors from warnings
			if c.Name == "Docker" || c.Name == "Docker Compose" || c.Name == "OPENCLAW_ROOT" {
				marker = red + "FAIL" + nc
				errors++
			} else {
				marker = yellow + "WARN" + nc
				warnings++
			}
		} else {
			passed++
		}
		fmt.Printf("  [%s] %-18s %s\n", marker, c.Name, c.Message)
	}

	fmt.Println()
	fmt.Printf("%s%d passed, %d warnings, %d errors%s\n", bold, passed, warnings, errors, nc)

	if errors > 0 {
		return errorf("doctor found %d error(s) — fix them before using claws", errors)
	}
	return nil
}
