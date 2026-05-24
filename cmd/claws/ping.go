package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// cmdAgentPing — single-screen "is this agent responding" check.
//
// Combines existing helpers (verifyOneInstance, /healthz + /readyz probes,
// last-30s log scan) into one read-only command with a clear summary.
// Exits non-zero on any check failure so it composes with shell pipelines.
//
// Why this exists: after `claws start`, the only way to know the agent
// is alive was to message it on Telegram. If silent: auth? channel?
// image? network? `claws agent ping` answers in one screen.
func cmdAgentPing(args []string) error {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(`Usage: claws agent ping <name>

Single-screen "is my agent responding?" check. Reads the gateway
endpoints, runs the auth-verify chain, scans the last 30s of logs.
Exits non-zero on any failure.

Examples:
  claws agent ping default/sarah
  claws agent ping team/sarah && echo "alive"`)
		return nil
	}
	full := args[0]
	paths := resolvePaths()

	// 1. instance.env — read gateway port + token so we know where to look.
	envPath := filepath.Join(paths.Root, full, "instance.env")
	env, err := readEnvFile(envPath)
	if err != nil {
		return errorf("agent %q: %v", full, err)
	}
	port := env["OPENCLAW_GATEWAY_PORT"]
	if port == "" {
		return errorf("agent %q has no OPENCLAW_GATEWAY_PORT in instance.env", full)
	}

	const (
		bold  = "\033[1m"
		dim   = "\033[0;90m"
		green = "\033[0;32m"
		gold  = "\033[0;33m"
		red   = "\033[0;31m"
		nc    = "\033[0m"
	)
	type check struct {
		label string
		ok    bool
		note  string
	}
	var checks []check
	failed := 0
	mark := func(label string, ok bool, note string) {
		checks = append(checks, check{label, ok, note})
		if !ok {
			failed++
		}
	}

	fmt.Printf("%sclaws agent ping%s %s\n\n", bold, nc, full)

	// 2. /healthz — use Docker's HEALTHCHECK verdict (the gateway binds
	// to container-internal 127.0.0.1, so host probes get connection
	// reset). Falls back to in-container probe when the image declares
	// no HEALTHCHECK.
	switch containerHealth(paths, full) {
	case "healthy":
		mark("gateway", true, "container reports healthy on :"+port)
	case "starting":
		mark("gateway", false, "container HEALTHCHECK still in start-period — wait or claws logs "+full)
	case "unhealthy":
		mark("gateway", false, "container HEALTHCHECK failing — claws logs "+full)
	case "none":
		// No HEALTHCHECK in image → fall through to in-container probe.
		pr := containerProbe(paths, full, "/healthz")
		switch {
		case !pr.Reachable:
			mark("gateway", false, fmt.Sprintf("couldn't probe /healthz — container down? (claws start %s)", full))
		case pr.Status == 200:
			mark("gateway", true, "/healthz 200 OK (in-container probe)")
		default:
			mark("gateway", false, fmt.Sprintf("/healthz %d (in-container probe)", pr.Status))
		}
	default:
		// "" — container not running, or docker not reachable.
		mark("gateway", false, fmt.Sprintf("container not running — claws start %s", full))
	}

	// 3. /readyz — also in-container probe. No way to reach the gateway
	// from the host with the current runtime bind config.
	pr := containerProbe(paths, full, "/readyz")
	switch {
	case !pr.Reachable:
		mark("readyz", false, "couldn't probe /readyz (container not running)")
	case pr.Status == 200:
		mark("readyz", true, "/readyz 200 — agent ready to receive")
	case pr.Status == 404:
		// /readyz isn't a required endpoint — some runtime versions
		// don't ship one. Treat absent as "no signal" rather than fail.
		mark("readyz", true, "/readyz not implemented in this runtime (skipped)")
	default:
		mark("readyz", false, fmt.Sprintf("/readyz %d — agent up but not ready", pr.Status))
	}

	// 4. Auth verify — reuse the 3-strategy chain.
	authRes := verifyOneInstance(paths, full)
	if authRes.Verified {
		mark("auth", true, fmt.Sprintf("verified via %s strategy", authRes.Strategy))
	} else {
		note := "no auth configured — run: claws auth " + full + " codex   (or apikey)"
		switch {
		case authRes.FixCommand != "":
			note = authRes.FixCommand
		case authRes.Error != "":
			note = authRes.Error
		}
		mark("auth", false, note)
	}

	// 5. Channels — read openclaw.json to count enabled channels.
	channelCount, channelNames := readEnabledChannels(paths, full)
	if channelCount > 0 {
		mark("channels", true, fmt.Sprintf("%d configured: %s", channelCount, strings.Join(channelNames, ", ")))
	} else {
		mark("channels", false, "no channels — run: claws channel add "+full+" telegram --token=<bot-token>")
	}

	// 6. Last 30s of logs (best-effort — surfaced only if container is named).
	logTail := readRecentLogLines(full, 5)

	// Render.
	for _, c := range checks {
		dot := green + "✓" + nc
		if !c.ok {
			dot = red + "✗" + nc
		}
		fmt.Printf("  %s %-10s %s%s%s\n", dot, c.label+":", dim, c.note, nc)
	}
	if len(logTail) > 0 {
		fmt.Printf("\n%slast log lines%s\n", dim, nc)
		for _, l := range logTail {
			fmt.Printf("    %s\n", l)
		}
	}
	fmt.Println()
	if failed == 0 {
		fmt.Printf("%s✓ %s looks healthy%s\n\n", green, full, nc)
		return nil
	}
	fmt.Printf("%s✗ %s has %d failing check(s)%s — see notes above for the fix command.\n\n",
		red, full, failed, nc)
	return errorf("%d check(s) failed", failed)
}

// readEnvFile parses a simple KEY=VALUE env file. No quoting/escaping
// support beyond what claws writes itself.
func readEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "="); i > 0 {
			out[line[:i]] = strings.TrimSpace(line[i+1:])
		}
	}
	return out, nil
}

// readEnabledChannels parses openclaw.json and returns the count + names
// of channels with enabled=true.
func readEnabledChannels(paths Paths, full string) (int, []string) {
	cfgPath := filepath.Join(paths.Root, full, "openclaw.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return 0, nil
	}
	// Light parse — channels.<name>.enabled true.
	// Avoid pulling json package overhead; just look for the pattern.
	enabled := []string{}
	for _, channel := range []string{"telegram", "discord", "slack", "whatsapp", "signal"} {
		needle := `"` + channel + `":`
		if i := strings.Index(string(data), needle); i >= 0 {
			rest := string(data[i:])
			// Quick-and-dirty: look for "enabled": true within the next ~200 bytes.
			end := len(rest)
			if end > 400 {
				end = 400
			}
			if strings.Contains(rest[:end], `"enabled": true`) || strings.Contains(rest[:end], `"enabled":true`) {
				enabled = append(enabled, channel)
			}
		}
	}
	return len(enabled), enabled
}

// readRecentLogLines tails the agent's container logs via docker. Best-effort:
// returns empty slice if docker isn't available or the container isn't named
// in the expected way.
func readRecentLogLines(full string, n int) []string {
	// Compose project name follows openclaw-<group>-<name> by default.
	containerName := "openclaw-" + strings.ReplaceAll(full, "/", "-") + "-openclaw-gateway-1"
	cmd := exec.Command("docker", "logs", "--tail", fmt.Sprintf("%d", n), "--since", "30s", containerName)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil
	}
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if l == "" {
			continue
		}
		// Truncate long lines for screen rendering.
		if len(l) > 120 {
			l = l[:117] + "…"
		}
		lines = append(lines, l)
	}
	return lines
}
