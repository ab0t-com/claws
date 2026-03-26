package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ---------------------------------------------------------------------------
// clawctl init — first-run setup
// ---------------------------------------------------------------------------

func cmdInit(args []string) error {
	paths := resolvePaths()

	bold := "\033[1m"
	nc := "\033[0m"

	fmt.Printf("%sclawctl init%s — setting up environment\n\n", bold, nc)

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
		// Find compose template: next to binary, then CWD
		var composeSource string
		exe, _ := os.Executable()
		candidates := []string{}
		if exe != "" {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), "docker-compose.yml"))
		}
		cwd, _ := os.Getwd()
		if cwd != "" {
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
			warn("docker-compose.yml not found next to clawctl binary or in CWD")
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
	fmt.Println()

	// 6. Check Docker
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

	// 7. Check image
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
	info("Init complete.")
	fmt.Println()
	fmt.Println("  Next steps:")
	fmt.Println("    clawctl create <name>       # create your first instance")
	fmt.Println("    clawctl doctor              # diagnose any remaining issues")
	return nil
}
