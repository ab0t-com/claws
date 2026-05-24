package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
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
	nameRegex   = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
	quietCreate bool // when true, cmdCreate suppresses standalone output (used by setup flow)
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
	// 1. OPENCLAW_ROOT/docker-compose.yml (installed by `claws init`)
	// 2. Next to the binary (dev / co-located install)
	// 3. XDG data dir — $XDG_DATA_HOME/claws or ~/.local/share/claws (install.sh puts it here)
	// 4. Current working directory (fallback)
	compose := ""
	candidates := []string{
		filepath.Join(root, "docker-compose.yml"),
	}

	exe, _ := os.Executable()
	if exe != "" {
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
	if v := os.Getenv("CLAWS_BASE_PORT"); v != "" {
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
		return fmt.Errorf("instance '%s' does not exist — run: claws create %s", name, name)
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

// padVisible returns s left-padded with spaces so that its *visible* width
// (excluding ANSI SGR escape sequences) is exactly width. If s is already
// wider than width, it is returned unchanged. Use this for fixed-width
// column rendering when a value may or may not contain color codes — Go's
// printf %-Ns counts bytes, which causes columns to skew based on whether
// a row has color or not.
func padVisible(s string, width int) string {
	visible := 0
	inEscape := false
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		visible++
	}
	if visible >= width {
		return s
	}
	pad := width - visible
	// strings.Repeat with a non-negative count is safe; the visible<width
	// guard above ensures positivity here.
	out := make([]byte, len(s)+pad)
	copy(out, s)
	for i := 0; i < pad; i++ {
		out[len(s)+i] = ' '
	}
	return string(out)
}

// firstPositional returns the first arg that does not begin with '-'. It is
// the symmetric companion of flagValue for commands that take both a single
// positional name and one or more flags, where the name's position in args
// is not guaranteed (e.g., `claws restart --hard alpha` vs
// `claws restart alpha --hard`). Returns "" if every arg is a flag.
func firstPositional(args []string) string {
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			return a
		}
	}
	return ""
}

// flagValue returns the value of the first --foo=bar style flag matching prefix
// ("--group=", "--since=", etc.), or "" if no such flag is present. The prefix
// must include the trailing '=' so the caller can disambiguate similarly-named
// flags ("--group" vs "--group-defaults"). This wraps the manual
// strings.HasPrefix + slice-offset idiom used across the codebase so call sites
// stay self-documenting and a typoed offset can never silently return the
// wrong value.
func flagValue(args []string, prefix string) string {
	for _, a := range args {
		if strings.HasPrefix(a, prefix) {
			return a[len(prefix):]
		}
	}
	return ""
}

// filterEntriesByGroup returns the subset of entries belonging to the named
// group. An empty group returns entries unchanged so callers can pass through
// the flag value without conditional logic. Names that fail to parse (which
// should never happen for entries from readRegistry, but defensive) are
// dropped from the filtered output.
func filterEntriesByGroup(entries []RegistryEntry, group string) []RegistryEntry {
	if group == "" {
		return entries
	}
	out := make([]RegistryEntry, 0, len(entries))
	for _, e := range entries {
		ref, err := ParseRef(e.Name)
		if err != nil {
			continue
		}
		if ref.Group == group {
			out = append(out, e)
		}
	}
	return out
}

// requireGroup is the standard precondition for any command that accepts
// --group=<name>. It returns nil when group is empty (i.e., no filter
// requested), or when the group exists on disk. It returns a directive error
// message otherwise. Callers can rely on this single check rather than each
// re-implementing the existence test.
func requireGroup(paths Paths, group string) error {
	if group == "" {
		return nil
	}
	groupDir := filepath.Join(paths.Root, group)
	if !IsGroup(groupDir) {
		return fmt.Errorf("group '%s' does not exist — see: claws group list", group)
	}
	return nil
}
