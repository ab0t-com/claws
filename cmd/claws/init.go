package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// claws init — first-run setup
// ---------------------------------------------------------------------------

func cmdInit(args []string) error {
	paths := resolvePaths()

	bold := "\033[1m"
	nc := "\033[0m"

	fmt.Printf("%sclaws init%s — setting up environment\n\n", bold, nc)

	// 1. Create OPENCLAW_ROOT
	info(fmt.Sprintf("Creating %s...", paths.Root))
	if err := os.MkdirAll(paths.Root, 0755); err != nil {
		return errorf("failed to create OPENCLAW_ROOT: %v", err)
	}
	fmt.Printf("  %s\n", paths.Root)

	// 2. Create standard subdirs
	dirs := []string{
		filepath.Join(paths.Root, "shared", "skills"),
		filepath.Join(paths.Root, "shared", "workspace"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return errorf("failed to create %s: %v", d, err)
		}
	}
	fmt.Println("  shared/skills/")
	fmt.Println("  shared/workspace/")

	// 3. Ensure port registry exists
	if _, err := os.Stat(paths.PortRegistry); os.IsNotExist(err) {
		if err := os.WriteFile(paths.PortRegistry, nil, credentialFileMode); err != nil {
			return errorf("failed to create port registry: %v", err)
		}
	}
	fmt.Println("  .port-registry")
	fmt.Println()

	// 4. Copy compose template if not present in OPENCLAW_ROOT
	composeDest := filepath.Join(paths.Root, "docker-compose.yml")
	if _, err := os.Stat(composeDest); os.IsNotExist(err) {
		// Find compose template — same lookup order as resolvePaths:
		// next to the binary → XDG data dir → CWD
		var composeSource string
		candidates := []string{}
		if exe, _ := os.Executable(); exe != "" {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "docker-compose.yml"))
		}
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			if home, _ := os.UserHomeDir(); home != "" {
				dataHome = filepath.Join(home, ".local", "share")
			}
		}
		if dataHome != "" {
			candidates = append(candidates, filepath.Join(dataHome, "claws", "docker-compose.yml"))
		}
		if cwd, _ := os.Getwd(); cwd != "" {
			candidates = append(candidates, filepath.Join(cwd, "docker-compose.yml"))
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				composeSource = c
				break
			}
		}

		if composeSource != "" {
			data, err := os.ReadFile(composeSource)
			if err == nil {
				os.WriteFile(composeDest, data, 0644)
				info(fmt.Sprintf("Copied docker-compose.yml from %s", composeSource))
			}
		} else {
			warn("docker-compose.yml not found next to claws binary, in XDG data dir, or in CWD")
			fmt.Println("  Copy it manually to: " + composeDest)
		}
	} else {
		fmt.Println("  docker-compose.yml already present")
	}

	// 5. Write defaults.json skeleton if not present
	defaultsPath := filepath.Join(paths.Root, "defaults.json")
	if _, err := os.Stat(defaultsPath); os.IsNotExist(err) {
		defaultsSkeleton := `{
  "tools": {},
  "agents": {
    "defaults": {}
  }
}
`
		os.WriteFile(defaultsPath, []byte(defaultsSkeleton), 0644)
		info("Created defaults.json skeleton")
	}

	// 6. Create policy.json with secure defaults (if not present)
	if !policyExists(paths) {
		p := Policy{
			AllowedBindModes:         []string{"loopback"},
			MaxInstances:             8,
			MemoryLimitMB:            2048,
			CPULimit:                 2.0,
			AllowDockerSocket:        false,
			RequireSandbox:           false,
			RequireDmPairing:         true,
			RequireOutboundAllowlist: true,
			AllowedImages:            []string{"openclaw:*"},
			AuditLog:                 true,
		}
		if err := writePolicy(paths, p); err != nil {
			return errorf("failed to create policy: %v", err)
		}
		info("Created policy.json (secure defaults: loopback-only, DM pairing, audit on)")
	} else {
		fmt.Println("  policy.json already present")
	}

	// 7. Create .access.json with current user as admin (if not present)
	if !accessExists(paths) {
		username := os.Getenv("USER")
		if username == "" {
			username = "ubuntu"
		}
		ac := AccessConfig{
			Roles: map[string]Role{
				"admin": {
					Users:    []string{username},
					Commands: []string{"*"},
				},
				"operator": {
					Users: []string{},
					Commands: []string{
						"start", "stop", "restart", "logs", "exec", "health",
						"status", "list", "dashboard", "activity", "stats",
						"config show", "channel status", "tunnel", "backup",
					},
				},
				"user": {
					Users:    []string{},
					Commands: []string{"status", "health", "logs", "list"},
				},
			},
		}
		if err := writeAccessConfig(paths, ac); err != nil {
			return errorf("failed to create access config: %v", err)
		}
		info(fmt.Sprintf("Created .access.json (admin: %s)", username))
	} else {
		fmt.Println("  .access.json already present")
	}
	fmt.Println()

	// 8. Check Docker
	info("Checking Docker...")
	if _, err := exec.LookPath("docker"); err != nil {
		warn("Docker not found — install: https://docs.docker.com/get-docker/")
	} else {
		if out, err := exec.Command("docker", "info", "--format", "{{.ServerVersion}}").Output(); err == nil {
			fmt.Printf("  Docker v%s — running\n", trimSpace(string(out)))
		} else {
			warn("Docker installed but not running — start the Docker daemon")
		}
	}

	// 9. Check image
	image := os.Getenv("OPENCLAW_IMAGE")
	if image == "" {
		image = "openclaw:local"
	}
	if _, err := exec.Command("docker", "image", "inspect", image).Output(); err == nil {
		fmt.Printf("  Image %s — found\n", image)
	} else {
		warn(fmt.Sprintf("Image '%s' not found — build or pull it", image))
	}

	fmt.Println()
	info("Init complete. Security policy and access control are configured.")
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    claws setup               # guided team setup (recommended)")
	fmt.Println("    claws create <name>       # create an instance manually")
	fmt.Println("    claws doctor              # diagnose any remaining issues")
	fmt.Println("    claws policy show          # review security policy")
	return nil
}
