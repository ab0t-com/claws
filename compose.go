package main

import (
	"os"
	"os/exec"
	"path/filepath"
)

// dc runs docker compose with the right flags for an instance.
// It resolves the runtime from instance.env to determine compose template,
// project name, and override file.
func dc(paths Paths, name string, args ...string) *exec.Cmd {
	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	override := rt.OverridePath(dir)

	composeTemplate := rt.ComposeTemplatePath(paths)

	composeArgs := []string{"-f", composeTemplate}
	if _, err := os.Stat(override); err == nil {
		composeArgs = append(composeArgs, "-f", override)
	}
	composeArgs = append(composeArgs, "--env-file", envFile, "-p", rt.MakeProjectName(ref))
	composeArgs = append(composeArgs, args...)

	cmd := exec.Command("docker", append([]string{"compose"}, composeArgs...)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

// dcOutput runs docker compose and captures stdout.
func dcOutput(paths Paths, name string, args ...string) (string, error) {
	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	override := rt.OverridePath(dir)

	composeTemplate := rt.ComposeTemplatePath(paths)

	composeArgs := []string{"-f", composeTemplate}
	if _, err := os.Stat(override); err == nil {
		composeArgs = append(composeArgs, "-f", override)
	}
	composeArgs = append(composeArgs, "--env-file", envFile, "-p", rt.MakeProjectName(ref))
	composeArgs = append(composeArgs, args...)

	cmd := exec.Command("docker", append([]string{"compose"}, composeArgs...)...)
	out, err := cmd.Output()
	return string(out), err
}

// dcRun runs and waits, returning error if nonzero exit.
func dcRun(paths Paths, name string, args ...string) error {
	return dc(paths, name, args...).Run()
}

// readEnvValue reads a key=value from an instance.env file.
func readEnvValue(envFile, key string) string {
	data, err := os.ReadFile(envFile)
	if err != nil {
		return ""
	}
	for _, line := range splitLines(string(data)) {
		if len(line) > 0 && line[0] != '#' {
			parts := splitFirst(line, '=')
			if len(parts) == 2 && parts[0] == key {
				return parts[1]
			}
		}
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func splitFirst(s string, sep byte) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == sep {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s}
}

// containerStatus returns the status string from docker compose ps.
func containerStatus(paths Paths, name string) string {
	rt := mustResolveRuntime(paths, name)
	out, err := dcOutput(paths, name, "ps", "--format", "{{.Status}}", rt.GatewayService)
	if err != nil {
		return ""
	}
	return trimSpace(out)
}

func trimSpace(s string) string {
	i, j := 0, len(s)
	for i < j && (s[i] == ' ' || s[i] == '\n' || s[i] == '\r' || s[i] == '\t') {
		i++
	}
	for j > i && (s[j-1] == ' ' || s[j-1] == '\n' || s[j-1] == '\r' || s[j-1] == '\t') {
		j--
	}
	return s[i:j]
}

// resolveContainerName finds the actual container name for an instance's gateway service.
func resolveContainerName(paths Paths, name string) string {
	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)

	out, err := dcOutput(paths, name, "ps", "--format", "json", rt.GatewayService)
	if err == nil && len(trimSpace(out)) > 0 {
		for _, line := range splitLines(out) {
			line = trimSpace(line)
			if line == "" || line[0] != '{' {
				continue
			}
			if idx := indexOf(line, '"'); idx >= 0 {
				nameKey := `"Name":"`
				if pos := indexOfStr(line, nameKey); pos >= 0 {
					start := pos + len(nameKey)
					end := indexOf(line[start:], '"')
					if end >= 0 {
						return line[start : start+end]
					}
				}
			}
		}
	}
	// Fallback to conventional name using runtime
	return rt.DefaultContainerName(ref)
}

func indexOfStr(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// containerRAM returns RAM usage string.
func containerRAM(paths Paths, name string) string {
	containerName := resolveContainerName(paths, name)
	cmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{.MemUsage}}", containerName)
	out, err := cmd.Output()
	if err != nil {
		return "—"
	}
	s := trimSpace(string(out))
	if idx := indexOf(s, '/'); idx >= 0 {
		return trimSpace(s[:idx])
	}
	return s
}

func indexOf(s string, c byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}
