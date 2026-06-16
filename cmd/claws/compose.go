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
	security := securityComposePath(rt, dir)

	composeTemplate := rt.ComposeTemplatePath(paths)

	composeArgs := []string{"-f", composeTemplate}
	if _, err := os.Stat(override); err == nil {
		composeArgs = append(composeArgs, "-f", override)
	}
	// Security overlay applies LAST so cap_drop / security_opt overrides
	// from the tier setting take precedence over both base and override.
	if _, err := os.Stat(security); err == nil {
		composeArgs = append(composeArgs, "-f", security)
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
	security := securityComposePath(rt, dir)

	composeTemplate := rt.ComposeTemplatePath(paths)

	composeArgs := []string{"-f", composeTemplate}
	if _, err := os.Stat(override); err == nil {
		composeArgs = append(composeArgs, "-f", override)
	}
	if _, err := os.Stat(security); err == nil {
		composeArgs = append(composeArgs, "-f", security)
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

// itoaPort renders a port int as decimal string. Local helper to keep
// this file free of strconv imports — port is always non-negative.
func itoaPort(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [6]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// probeResult is the outcome of a containerProbe call.
//
// Reachable=false means the probe couldn't run at all (container missing,
// docker unreachable, node missing inside the image, etc). The caller
// should treat this as inconclusive — distinct from "got a response
// indicating failure".
type probeResult struct {
	Reachable bool
	Status    int    // HTTP status code (0 if !Reachable or probe errored)
	Body      []byte // response body, truncated to 64 KiB
}

// containerProbe runs an HTTP GET against the gateway INSIDE the
// container via `docker exec`. Required because the openclaw runtime
// binds to container-internal 127.0.0.1, which the host's port mapping
// can't reach. The probe uses node's built-in fetch (always present in
// the runtime image — it's what the Docker HEALTHCHECK uses too), so
// no extra binary install in the container.
//
// Endpoints are looked up against the runtime's declared gateway port
// from instance.env. A nil/empty endpoint returns Reachable=false.
func containerProbe(paths Paths, name, endpoint string) probeResult {
	if endpoint == "" {
		return probeResult{}
	}
	container := resolveContainerName(paths, name)
	if container == "" {
		return probeResult{}
	}
	// IMPORTANT: use the runtime's container-internal port, NOT
	// OPENCLAW_GATEWAY_PORT (which is the host-side port mapping).
	// Each agent's host port differs (18789, 18889, 19089, …) but the
	// runtime always binds 18789 inside the container. Probing the
	// host-port number from inside the container would ECONNREFUSED for
	// every agent except the one whose host port happens to equal the
	// internal port (typically index 0).
	rt := mustResolveRuntime(paths, name)
	internalPort := rt.InternalPort
	if internalPort == 0 {
		internalPort = 18789 // safety fallback matching the runtime default
	}
	// Single-line node script: fetch, print "<status>\n<body>" to stdout,
	// exit 0 regardless of HTTP status (we encode the result in stdout).
	// catch() exits non-zero so a transport-level failure surfaces.
	//
	// Uses console.log (which adds its own newline) instead of an
	// explicit '\n' escape — node's TypeScript-mode -e evaluator
	// mis-parses backslash-n inside single-quoted JS strings,
	// breaking the literal across two lines and producing a SyntaxError.
	script := `fetch('http://127.0.0.1:` + itoaPort(internalPort) + endpoint +
		`').then(async r=>{const t=await r.text();console.log(r.status);process.stdout.write(t);process.exit(0);})` +
		`.catch(e=>{process.stderr.write(String(e&&e.message||e));process.exit(1);})`
	cmd := exec.Command("docker", "exec", container, "node", "-e", script)
	out, err := cmd.Output()
	if err != nil {
		// docker exec failed (container down / node missing / network error)
		return probeResult{}
	}
	// Parse "STATUS\n<body>" — split on first newline.
	nl := -1
	for i, b := range out {
		if b == '\n' {
			nl = i
			break
		}
	}
	if nl < 0 {
		return probeResult{Reachable: true}
	}
	status := 0
	for _, b := range out[:nl] {
		if b < '0' || b > '9' {
			break
		}
		status = status*10 + int(b-'0')
	}
	body := out[nl+1:]
	if len(body) > 64*1024 {
		body = body[:64*1024]
	}
	return probeResult{Reachable: true, Status: status, Body: body}
}

// containerHealth returns Docker's healthcheck verdict for an instance's
// gateway container, as reported by `docker inspect`. One of:
//
//   "healthy"    — HEALTHCHECK succeeded and the container is up.
//   "starting"   — HEALTHCHECK still in start-period or not yet run.
//   "unhealthy"  — HEALTHCHECK has failed enough times to flip.
//   "none"       — no HEALTHCHECK defined for this container.
//   ""           — couldn't inspect (container missing, docker unreachable, etc.).
//
// We use this rather than probing the gateway from the host because
// modern openclaw runtimes bind to container-internal 127.0.0.1, which
// the host port mapping can't reach. The HEALTHCHECK runs INSIDE the
// container where the loopback is real, so it's authoritative.
func containerHealth(paths Paths, name string) string {
	container := resolveContainerName(paths, name)
	if container == "" {
		return ""
	}
	cmd := exec.Command("docker", "inspect",
		"--format", "{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}",
		container)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return trimSpace(string(out))
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
