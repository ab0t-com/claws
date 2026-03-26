package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// Version is set at build time via -ldflags "-X main.Version=..."
var Version = "dev"

const (
	defaultBasePort    = 18789
	portStep           = 100
	maxInstances       = 8
	warnInstances      = 6
	credentialFileMode = 0600 // for files containing tokens/secrets (instance.env, .port-registry)
)

var (
	nameRegex = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// Paths resolves all paths from OPENCLAW_ROOT and SCRIPT_DIR.
type Paths struct {
	Root            string // ~/.openclaw
	PortRegistry    string // ~/.openclaw/.port-registry
	SharedDir       string // ~/.openclaw/shared
	ComposeTemplate string // docker-compose.yml location
}

func resolvePaths() Paths {
	root := os.Getenv("OPENCLAW_ROOT")
	if root == "" {
		home, _ := os.UserHomeDir()
		root = filepath.Join(home, ".openclaw")
	}

	// Compose template search order:
	// 1. OPENCLAW_ROOT/docker-compose.yml (installed by clawctl init)
	// 2. Next to the binary (development / co-located install)
	// 3. Current working directory (fallback)
	compose := ""
	candidates := []string{
		filepath.Join(root, "docker-compose.yml"),
	}

	exe, _ := os.Executable()
	if exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "docker-compose.yml"))
	}

	cwd, _ := os.Getwd()
	if cwd != "" {
		candidates = append(candidates, filepath.Join(cwd, "docker-compose.yml"))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			compose = c
			break
		}
	}

	// Last resort: assume OPENCLAW_ROOT (init will put it there)
	if compose == "" {
		compose = filepath.Join(root, "docker-compose.yml")
	}

	return Paths{
		Root:            root,
		PortRegistry:    filepath.Join(root, ".port-registry"),
		SharedDir:       filepath.Join(root, "shared"),
		ComposeTemplate: compose,
	}
}

func basePort() int {
	if v := os.Getenv("CLAWCTL_BASE_PORT"); v != "" {
		var p int
		fmt.Sscanf(v, "%d", &p)
		if p > 0 {
			return p
		}
	}
	return defaultBasePort
}

func instanceDir(paths Paths, name string) string {
	return filepath.Join(paths.Root, name)
}

func validateName(name string) error {
	if !nameRegex.MatchString(name) {
		return fmt.Errorf("invalid name '%s' — use lowercase letters, numbers, hyphens; must start with a letter", name)
	}
	if len(name) > 30 {
		return fmt.Errorf("name too long (max 30 chars)")
	}
	if name == "shared" {
		return fmt.Errorf("'shared' is reserved for shared resources")
	}
	return nil
}

func requireInstance(paths Paths, name string) error {
	ref, err := ParseRef(name)
	if err != nil {
		return err
	}
	envFile := filepath.Join(ref.Dir(paths), "instance.env")
	if _, err := os.Stat(envFile); err != nil {
		return fmt.Errorf("instance '%s' does not exist — run: clawctl create %s", name, name)
	}
	return nil
}

// hostBind converts a bind mode to a Docker port mapping host address.
func hostBind(bindMode string) string {
	switch bindMode {
	case "loopback":
		return "127.0.0.1"
	case "lan", "wan":
		return "0.0.0.0"
	default:
		return "127.0.0.1"
	}
}

// fixCredentialPermissions sets 0600 on all files in credentials/ under an instance dir.
func fixCredentialPermissions(instanceDir string) int {
	fixed := 0
	credsDir := filepath.Join(instanceDir, "credentials")
	filepath.Walk(credsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Mode().Perm() != credentialFileMode {
			os.Chmod(path, credentialFileMode)
			fixed++
		}
		return nil
	})
	return fixed
}

// fixAllPermissions fixes permissions across an entire OPENCLAW_ROOT.
// Returns counts of files fixed.
func fixAllPermissions(root string) (envFixed, credFixed, regFixed int) {
	// Fix instance.env files
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Name() == "instance.env" && info.Mode().Perm() != credentialFileMode {
			os.Chmod(path, credentialFileMode)
			envFixed++
		}
		return nil
	})

	// Fix credential files
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "credentials" || isUnderCredentials(root, path) {
			if info.Mode().Perm() != credentialFileMode {
				os.Chmod(path, credentialFileMode)
				credFixed++
			}
		}
		return nil
	})

	// Fix .port-registry
	regPath := filepath.Join(root, ".port-registry")
	if fi, err := os.Stat(regPath); err == nil && fi.Mode().Perm() != credentialFileMode {
		os.Chmod(regPath, credentialFileMode)
		regFixed = 1
	}

	return
}

func isUnderCredentials(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	parts := filepath.SplitList(rel)
	_ = parts
	// Check if any parent dir is "credentials"
	dir := filepath.Dir(path)
	for dir != root && dir != "/" && dir != "." {
		if filepath.Base(dir) == "credentials" {
			return true
		}
		dir = filepath.Dir(dir)
	}
	return false
}

// Color helpers
func info(msg string)  { fmt.Printf("\033[0;32m==> %s\033[0m\n", msg) }
func warn(msg string)  { fmt.Printf("\033[0;33m==> WARNING: %s\033[0m\n", msg) }
func errorf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

// hasFlag checks if a flag is present in args.
func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}
