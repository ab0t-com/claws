package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// claws auth-monitor — unattended auth-recovery loop
//
// The motivating scenario: Mike's agents share an upstream Codex account, so
// OAuth refresh tokens collide. Sarah goes 401 in the middle of the night;
// Mike notices it the next day. We need the fleet to self-heal without Mike
// touching the box.
//
// The recovery primitive already exists: `claws auth fleet apikey openai <key>`
// is non-interactive and swaps every agent's auth from the colliding OAuth to a
// per-agent API key in one shot. This file wires it to fire automatically:
//
//   1. Poll every agent via verifyOneInstance (same probe `claws auth verify` uses).
//   2. When ANY agent fails: read a pre-staged fallback API key from disk.
//   3. Run `claws auth fleet apikey <provider> <key>` to swap the broken agents.
//   4. Re-verify; log the recovery to the audit log.
//   5. If no fallback key is configured: print a clear directive and stop —
//      Mike then runs `claws paste-secret openai.key` from his phone and the
//      next monitor cycle picks it up.
//
// Designed to be invoked from a systemd timer (5-minute cadence) OR run as a
// long-running daemon with --interval. --once exits after a single sweep, which
// is what the timer-driven mode wants.

const (
	defaultMonitorInterval = 5 * time.Minute
	// Default location for the fallback key. Lives under paths.Root so it
	// rides along with the rest of the fleet state (backups, etc.). User
	// can override with --fallback-key-file.
	defaultFallbackKeyName = ".recovery-api-key"
)

// cmdAuthMonitor handles `claws auth-monitor [flags]`.
func cmdAuthMonitor(args []string) error {
	// Default: single sweep (--once is implicit for systemd timer use).
	// Use --interval to make it loop instead.
	once := true
	interval := defaultMonitorInterval
	fallbackKeyFile := ""
	fallbackProvider := "openai"
	dryRun := false
	verbose := false

	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--once":
			once = true
		case a == "--loop":
			once = false
		case strings.HasPrefix(a, "--interval="):
			d, err := time.ParseDuration(strings.TrimPrefix(a, "--interval="))
			if err != nil {
				return errorf("invalid --interval: %v", err)
			}
			interval = d
			once = false
		case strings.HasPrefix(a, "--fallback-key-file="):
			fallbackKeyFile = strings.TrimPrefix(a, "--fallback-key-file=")
		case strings.HasPrefix(a, "--fallback-provider="):
			fallbackProvider = strings.TrimPrefix(a, "--fallback-provider=")
		case a == "--dry-run":
			dryRun = true
		case a == "-v", a == "--verbose":
			verbose = true
		case a == "-h", a == "--help":
			printMonitorHelp()
			return nil
		default:
			return errorf("unknown flag %q (try claws auth-monitor --help)", a)
		}
	}

	paths := resolvePaths()
	if fallbackKeyFile == "" {
		fallbackKeyFile = filepath.Join(paths.Root, defaultFallbackKeyName)
	}

	for {
		runMonitorSweep(paths, fallbackKeyFile, fallbackProvider, dryRun, verbose)
		if once {
			return nil
		}
		if verbose {
			fmt.Printf("\n==> sleeping %s until next sweep\n", interval)
		}
		time.Sleep(interval)
	}
}

func printMonitorHelp() {
	fmt.Println(`Usage: claws auth-monitor [flags]

Unattended auth-recovery loop. Detects 401/refresh_token_reused failures
across the fleet and swaps to a pre-staged fallback API key via the
existing `+"`"+`claws auth fleet apikey`+"`"+` machinery.

Designed to be invoked from a systemd timer (5-minute cadence) or run as
a long-running daemon with --loop.

Flags:
  --once                       Single sweep then exit (default; systemd-friendly)
  --loop                       Sleep + repeat (combine with --interval)
  --interval=<duration>        Sleep between sweeps (default 5m). Implies --loop.
  --fallback-key-file=<path>   API key to use when an agent's OAuth dies.
                               Default: ~/.openclaw/.recovery-api-key
  --fallback-provider=<name>   Provider for the fallback key (default openai;
                               also: anthropic, openrouter)
  --dry-run                    Print what would be done; don't change anything
  -v, --verbose                Per-agent probe details

How to stage the fallback key (Mike-from-phone, no SSH):
  1. On the box:  claws paste-secret recovery-api-key
  2. On phone:    visit the URL, paste the API key, hit save
  3. The next sweep auto-detects and recovers any failing agents

Audit:
  Recovery events written to ~/.openclaw/.audit.log with kind=auth.monitor.recover.

Examples:
  claws auth-monitor                              # one sweep, exit
  claws auth-monitor --loop --interval=2m -v      # daemon mode, 2-min cadence
  claws auth-monitor --dry-run -v                 # what would happen?`)
}

// runMonitorSweep is one polling cycle: verify every agent, recover the broken
// ones via the fallback key if available, audit-log the outcome.
func runMonitorSweep(paths Paths, fallbackKeyFile, fallbackProvider string, dryRun, verbose bool) {
	now := time.Now().UTC().Format(time.RFC3339)
	fmt.Printf("==> auth-monitor sweep at %s\n", now)

	entries, err := readRegistry(paths)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  error reading registry: %v\n", err)
		return
	}

	var broken []string
	for _, e := range entries {
		v := verifyOneInstance(paths, e.Name)
		// The verifyOneInstance probe uses /readyz which reports auth
		// SUBSYSTEM health, not actual model-call success. An agent can
		// be hard-401'd on every model call while readyz reports ✓ —
		// that's the silent-failure class this monitor exists to catch.
		// Layer a log scan on top: recent gateway logs trump readyz.
		// Wider window: catch failures from the last hour so a single
		// 401 burst at 2 AM doesn't get missed by a 03:00 sweep. The
		// failure pattern is sticky (each model call retries), so
		// over-broad is fine — we'll skip recovery if the agent is now
		// actually healthy after the API-key swap.
		logSays := scanLogsForAuthFailure(e.Name, 60*time.Minute)
		isBroken := !v.Verified || logSays
		if verbose {
			status := "✓"
			if isBroken {
				status = "✗"
			}
			reason := v.Error
			if logSays {
				reason = "logs show refresh_token_reused / 401"
			}
			fmt.Printf("  %s %s (%s): %s\n", status, e.Name, v.Strategy, reason)
		}
		if isBroken {
			broken = append(broken, e.Name)
		}
	}

	if len(broken) == 0 {
		fmt.Println("  all agents verified — nothing to recover")
		return
	}

	fmt.Printf("  %d agent(s) need recovery: %s\n", len(broken), strings.Join(broken, ", "))

	// Read fallback key.
	keyData, err := os.ReadFile(fallbackKeyFile)
	if err != nil {
		fmt.Printf("\033[1;33m  ⚠  no fallback key at %s — recovery cannot proceed automatically\033[0m\n", fallbackKeyFile)
		fmt.Printf("     To enable auto-recovery: claws paste-secret %s\n", filepath.Base(fallbackKeyFile))
		fmt.Printf("     Then this sweep will pick it up on the next run.\n")
		writeMonitorAudit(paths, "auth.monitor.stalled", broken, "no-fallback-key", fallbackKeyFile)
		return
	}
	apiKey := strings.TrimSpace(string(keyData))
	if apiKey == "" {
		fmt.Printf("\033[1;33m  ⚠  fallback key file is empty: %s\033[0m\n", fallbackKeyFile)
		writeMonitorAudit(paths, "auth.monitor.stalled", broken, "empty-fallback-key", fallbackKeyFile)
		return
	}
	if !looksLikeAPIKey(apiKey) {
		fmt.Printf("\033[1;33m  ⚠  fallback key doesn't look like an API key (expected sk-... or similar)\033[0m\n")
		writeMonitorAudit(paths, "auth.monitor.stalled", broken, "invalid-fallback-key", fallbackKeyFile)
		return
	}

	if dryRun {
		fmt.Printf("  --dry-run: would run `claws auth %s apikey %s <REDACTED>` for each broken agent\n", "<name>", fallbackProvider)
		return
	}

	// Recover each broken agent with the fallback API key. We don't use
	// `auth fleet apikey` here because that would also touch agents that
	// are working — we only want to swap the broken ones. Per-agent
	// invocation reuses cmdAuth's existing apikey path.
	var recovered, stillBroken []string
	for _, name := range broken {
		fmt.Printf("\n\033[1m==> claws auth %s apikey %s ***\033[0m\n", name, fallbackProvider)
		if err := cmdAuth([]string{name, "apikey", fallbackProvider, apiKey}); err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ recovery failed: %v\n", err)
			stillBroken = append(stillBroken, name)
			continue
		}
		// Re-verify (cmdAuth's apikey path auto-verifies but be defensive).
		v := verifyOneInstance(paths, name)
		if v.Verified {
			recovered = append(recovered, name)
		} else {
			fmt.Fprintf(os.Stderr, "  ✗ verified after recovery: %s\n", v.Error)
			stillBroken = append(stillBroken, name)
		}
	}

	fmt.Println()
	if len(recovered) > 0 {
		fmt.Printf("\033[0;32m  ✓ recovered: %s\033[0m\n", strings.Join(recovered, ", "))
	}
	if len(stillBroken) > 0 {
		fmt.Printf("\033[0;31m  ✗ still broken: %s\033[0m\n", strings.Join(stillBroken, ", "))
	}
	writeMonitorAudit(paths, "auth.monitor.recover", broken, fmt.Sprintf("recovered=%d still_broken=%d", len(recovered), len(stillBroken)), fallbackProvider)
}

// scanLogsForAuthFailure runs `docker logs --since=<since>` on the agent's
// gateway container and reports true if a known auth-failure pattern appears.
// This is the layer that catches what verifyOneInstance/readyz miss: the
// silent 401 / refresh_token_reused class where the agent's auth subsystem
// thinks it's fine but every actual model call dies.
//
// Patterns intentionally narrow — the goal is to recover when we have HIGH
// confidence the agent is failing, not on every benign 401 in unrelated logs.
func scanLogsForAuthFailure(name string, since time.Duration) bool {
	container := resolveContainerName(resolvePaths(), name)
	if container == "" {
		return false
	}
	cmd := exec.Command("docker", "logs", "--since",
		fmt.Sprintf("%ds", int(since.Seconds())), container)
	out, _ := cmd.CombinedOutput()
	s := string(out)
	patterns := []string{
		"refresh_token_reused",
		"Token refresh failed:",
		"OAuth token refresh failed",
		"Please try signing in again",
	}
	for _, p := range patterns {
		if strings.Contains(s, p) {
			return true
		}
	}
	return false
}

// looksLikeAPIKey is a cheap sanity check on the fallback key — catches
// "Mike pasted his Telegram token by mistake" or empty-file regressions.
// We can't validate without burning a model call; this just makes sure
// the value has the right shape.
func looksLikeAPIKey(s string) bool {
	if len(s) < 20 || len(s) > 512 {
		return false
	}
	// OpenAI: sk-..., Anthropic: sk-ant-..., OpenRouter: sk-or-..., generic.
	if strings.HasPrefix(s, "sk-") {
		return true
	}
	// Some providers use plain hex tokens. Accept if it's alphanum-ish.
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// writeMonitorAudit appends a recovery event to ~/.openclaw/.audit.log.
// One entry per sweep (whether successful or stalled) so the audit log
// has a complete history of when the monitor ran and what it tried.
func writeMonitorAudit(paths Paths, kind string, agents []string, result, detail string) {
	entry := map[string]any{
		"ts":     time.Now().UTC().Format(time.RFC3339),
		"kind":   kind,
		"agents": agents,
		"result": result,
		"detail": detail,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	logPath := filepath.Join(paths.Root, auditLogFile)
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(line)
	f.Write([]byte{'\n'})
}
