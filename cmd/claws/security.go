package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SecurityTier represents the container-level privilege tier of an agent.
// It lives at meta.securityTier in openclaw.json and drives the emitted
// docker-compose.security.yml overlay (cap_add/drop, security_opt, etc).
//
// Default = TierStandard for backwards compatibility; agents created before
// this feature existed will continue to run at standard with no on-disk change.
type SecurityTier string

const (
	TierUntrusted  SecurityTier = "untrusted"
	TierStandard   SecurityTier = "standard"
	TierPrivileged SecurityTier = "privileged"
	TierHostReach  SecurityTier = "host-reach"
)

// securityComposeFile is the per-instance overlay filename emitted from the tier.
const securityComposeFile = "docker-compose.security.yml"

// validTiers in canonical promotion order.
var validTiers = []SecurityTier{TierUntrusted, TierStandard, TierPrivileged, TierHostReach}

func parseTier(s string) (SecurityTier, bool) {
	for _, t := range validTiers {
		if string(t) == s {
			return t, true
		}
	}
	return "", false
}

// requiresAcceptRisk reports whether moving INTO this tier needs --accept-risk.
// Demotions (to standard or untrusted) never need the flag — security improvements
// shouldn't be gated.
func (t SecurityTier) requiresAcceptRisk() bool {
	return t == TierPrivileged || t == TierHostReach
}

// requiresHostReachAck reports whether the host-reach pwn-warning flag is needed.
func (t SecurityTier) requiresHostReachAck() bool {
	return t == TierHostReach
}

// describe is used in CLI warnings.
func (t SecurityTier) describe() string {
	switch t {
	case TierUntrusted:
		return "untrusted (read-only rootfs, full hardening, audit every call)"
	case TierStandard:
		return "standard (cap_drop=ALL + no-new-privileges — default fleet posture)"
	case TierPrivileged:
		return "privileged (sudo + apt + /etc write INSIDE the container; host is still unreachable)"
	case TierHostReach:
		return "host-reach (docker.sock + pid=host — the agent can reach the host machine)"
	default:
		return string(t)
	}
}

// securityComposePath returns the per-instance overlay path.
func securityComposePath(rt Runtime, instanceDir string) string {
	return filepath.Join(instanceDir, securityComposeFile)
}

// ---------------------------------------------------------------------------
// Tier storage — instance.env (claws-side metadata, NOT openclaw.json)
//
// openclaw.json is the RUNTIME's config and has strict schema validation —
// unknown keys cause "Config invalid" + container restart loop. The tier
// is purely a claws-side concept (it drives the compose overlay; the
// runtime doesn't need to know about it), so it lives in instance.env
// under the CLAWS_SECURITY_TIER key alongside other claws-internal state.
// ---------------------------------------------------------------------------

const securityTierEnvKey = "CLAWS_SECURITY_TIER"

// readSecurityTier returns the current tier for an instance.
// Missing/invalid → TierStandard (today's default).
func readSecurityTier(paths Paths, name string) SecurityTier {
	envFile := instanceEnvPath(paths, name)
	raw := readEnvValue(envFile, securityTierEnvKey)
	if t, ok := parseTier(raw); ok {
		return t
	}
	return TierStandard
}

// writeSecurityTier persists the tier to instance.env. Standard is the
// default and stored as the absence of the key (smaller diff for the
// common case).
func writeSecurityTier(paths Paths, name string, tier SecurityTier) error {
	envFile := instanceEnvPath(paths, name)
	data, err := os.ReadFile(envFile)
	if err != nil {
		return fmt.Errorf("read %s: %w", envFile, err)
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, securityTierEnvKey+"=") {
			lines = append(lines, line)
		}
	}
	// Trim trailing blanks introduced by the split.
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if tier != TierStandard {
		lines = append(lines, securityTierEnvKey+"="+string(tier))
	}
	return os.WriteFile(envFile, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

// instanceEnvPath returns the path to instance.env for an instance.
func instanceEnvPath(paths Paths, name string) string {
	return filepath.Join(instanceDir(paths, name), "instance.env")
}

// ---------------------------------------------------------------------------
// Compose overlay emission
// ---------------------------------------------------------------------------

// writeSecurityCompose generates the overlay yml from the current tier in
// openclaw.json. Called on create, on tier change, and on upgrade.
func writeSecurityCompose(paths Paths, name string) error {
	rt := mustResolveRuntime(paths, name)
	dir := instanceDir(paths, name)
	tier := readSecurityTier(paths, name)
	yml := securityComposeYAML(rt, tier)
	if yml == "" {
		// Standard tier — no overlay needed (matches the base file exactly).
		// Remove a stale file if it exists.
		os.Remove(securityComposePath(rt, dir))
		return nil
	}
	return os.WriteFile(securityComposePath(rt, dir), []byte(yml), 0644)
}

// securityComposeYAML is the pure overlay generator. Returns empty string for
// the standard tier (= match base, no overlay needed). For non-standard tiers,
// returns the docker compose snippet that overrides cap_drop/cap_add/security_opt
// for BOTH the gateway AND CLI services.
//
// COMPOSE MERGE GOTCHA: docker compose merges LISTS by APPENDING items from
// later -f files, not REPLACING. A naked `security_opt: []` in this overlay
// would NOT remove the base file's `no-new-privileges:true` — it would just
// append nothing, leaving the restriction in place (which then blocks sudo
// even when we have SETUID cap).
//
// Compose 2.20+ ships the `!reset` tag exactly for this case: `!reset []` /
// `!reset null` truly REPLACE the inherited value rather than appending.
// We require compose 2.20+ for the privilege-tier feature; older compose
// will yaml-parse the !reset tag as a literal string and the merge will
// regress to "appended". Host check: `docker compose version` >= 2.20.
func securityComposeYAML(rt Runtime, tier SecurityTier) string {
	if tier == TierStandard {
		return ""
	}
	var b strings.Builder
	b.WriteString("# AUTO-GENERATED by claws — do not edit manually.\n")
	b.WriteString("# Derived from CLAWS_SECURITY_TIER in instance.env.\n")
	b.WriteString("# To change: claws security tier <agent> --set <level>\n")
	b.WriteString("# Tier: " + string(tier) + "\n")
	b.WriteString("# NOTE: !reset tags require docker compose 2.20+.\n")
	b.WriteString("services:\n")

	write := func(service string) {
		if service == "" {
			return
		}
		b.WriteString("  " + service + ":\n")
		switch tier {
		case TierUntrusted:
			// Already as locked as the base; just enforce read_only rootfs.
			b.WriteString("    read_only: true\n")
			b.WriteString("    tmpfs:\n")
			b.WriteString("      - /tmp\n")
			b.WriteString("      - /run\n")
		case TierPrivileged:
			// Reset the base's [ALL] drop + no-new-privileges, then add
			// the minimum-viable cap set so sudo + apt work inside.
			b.WriteString("    cap_drop: !reset []\n")
			b.WriteString("    cap_add:\n")
			b.WriteString("      - SETUID\n")
			b.WriteString("      - SETGID\n")
			b.WriteString("      - CHOWN\n")
			b.WriteString("      - DAC_OVERRIDE\n")
			b.WriteString("      - DAC_READ_SEARCH\n")
			b.WriteString("      - FOWNER\n")
			b.WriteString("      - FSETID\n")
			b.WriteString("      - NET_ADMIN\n")
			b.WriteString("      - NET_BIND_SERVICE\n")
			b.WriteString("      - NET_RAW\n")
			b.WriteString("      - KILL\n")
			b.WriteString("      - AUDIT_WRITE\n")
			b.WriteString("    security_opt: !reset []\n")
		case TierHostReach:
			b.WriteString("    cap_drop: !reset []\n")
			b.WriteString("    cap_add:\n")
			b.WriteString("      - ALL\n")
			b.WriteString("    security_opt: !reset []\n")
			b.WriteString("    pid: \"host\"\n")
			b.WriteString("    volumes:\n")
			b.WriteString("      - /var/run/docker.sock:/var/run/docker.sock\n")
		}
	}
	write(rt.GatewayService)
	if rt.HasCLI() {
		write(rt.CLIService)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Audit log entries — security.tier.change
// ---------------------------------------------------------------------------

type tierChangeEntry struct {
	TS         string `json:"ts"`
	Kind       string `json:"kind"`
	Agent      string `json:"agent"`
	From       string `json:"from"`
	To         string `json:"to"`
	Operator   string `json:"operator"`
	AcceptRisk bool   `json:"accept_risk,omitempty"`
}

func writeSecurityTierAudit(paths Paths, name string, from, to SecurityTier, acceptRisk bool) {
	entry := tierChangeEntry{
		TS:         time.Now().UTC().Format(time.RFC3339),
		Kind:       "security.tier.change",
		Agent:      name,
		From:       string(from),
		To:         string(to),
		Operator:   currentOperator(),
		AcceptRisk: acceptRisk,
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

func currentOperator() string {
	if u := os.Getenv("USER"); u != "" {
		host, _ := os.Hostname()
		if host != "" {
			return u + "@" + host
		}
		return u
	}
	return "unknown"
}

// ---------------------------------------------------------------------------
// Privileged-overlay install — apt install sudo + NOPASSWD sudoers
// ---------------------------------------------------------------------------

// installPrivilegedOverlay runs apt install sudo + writes /etc/sudoers.d/node
// inside the agent's container, AFTER the container has been recreated with
// the privileged tier's docker security config. Then also seeds the openclaw
// runtime's exec-approvals allowlist so the LLM can actually invoke sudo+apt
// through its bash tool (the runtime gates exec calls by glob-pattern; with
// an empty allowlist, even a present binary is unreachable).
//
// Idempotent: probes for the sudo binary + working `sudo -n true` and seeds
// the allowlist on every call, returning success if it ends in the right
// state regardless of whether anything was changed.
func installPrivilegedOverlay(paths Paths, name string) error {
	container := resolveContainerName(paths, name)
	if container == "" {
		return fmt.Errorf("container for %s not found — start the agent first", name)
	}
	// Seed allowlist unconditionally — it's safe to re-add (idempotent) and
	// covers the case where the allowlist was lost / agent was recreated.
	if err := seedPrivilegedAllowlist(container); err != nil {
		// Non-fatal — the runtime may not yet be ready; we'll print a hint.
		fmt.Fprintf(os.Stderr, "  warning: exec allowlist seed failed: %v\n", err)
	}
	// Sudo binary path: short-circuit if already working.
	if probeNodeSudo(container) {
		return nil
	}
	install := exec.Command("docker", "exec", "-u", "root", container, "sh", "-c",
		`apt-get update -qq && DEBIAN_FRONTEND=noninteractive apt-get install -y -qq sudo`)
	install.Stdout = os.Stdout
	install.Stderr = os.Stderr
	if err := install.Run(); err != nil {
		return fmt.Errorf("apt-get install sudo failed: %w", err)
	}
	sudoers := exec.Command("docker", "exec", "-u", "root", container, "sh", "-c",
		`install -d /etc/sudoers.d && echo 'node ALL=(ALL) NOPASSWD: ALL' > /etc/sudoers.d/node && chmod 440 /etc/sudoers.d/node`)
	sudoers.Stdout = os.Stdout
	sudoers.Stderr = os.Stderr
	if err := sudoers.Run(); err != nil {
		return fmt.Errorf("sudoers write failed: %w", err)
	}
	if !probeNodeSudo(container) {
		return fmt.Errorf("sudo installed but `sudo -n true` still fails — check caps in docker-compose.security.yml")
	}
	return nil
}

// privilegedAllowlistPatterns is the set of openclaw exec-approvals patterns
// granted on entry to the privileged tier. These are what the LLM-side tool
// surface needs to actually invoke privileged commands; without them, the
// binary exists but the runtime returns "not permitted" on every bash call.
//
// Kept tight: shells, sudo, package managers, and curl/wget. Anything more
// exotic the operator can add via `openclaw approvals allowlist add` directly.
var privilegedAllowlistPatterns = []string{
	"/usr/bin/sudo",
	"/usr/bin/apt",
	"/usr/bin/apt-get",
	"/usr/bin/dpkg",
	"/usr/bin/curl",
	"/usr/bin/wget",
	"/usr/bin/pip",
	"/usr/bin/pip3",
	"/usr/bin/python3",
	"/usr/bin/bash",
	"/usr/bin/sh",
	"/bin/bash",
	"/bin/sh",
}

// seedPrivilegedAllowlist runs `openclaw approvals allowlist add` for every
// pattern in privilegedAllowlistPatterns. Idempotent — duplicate adds collapse.
// Patterns target the runtime's "main" agent (the default openclaw agent name
// for a single-tenant instance; this is what every claws agent uses today).
func seedPrivilegedAllowlist(container string) error {
	for _, pattern := range privilegedAllowlistPatterns {
		cmd := exec.Command("docker", "exec", container, "openclaw", "approvals",
			"allowlist", "add", "--agent", "main", pattern)
		// Discard stdout — the openclaw CLI dumps a colorful table per add;
		// we don't want to spam the operator. Errors still go through.
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("seed %q: %w", pattern, err)
		}
	}
	return nil
}

// probeNodeSudo returns true if the node user can sudo without password.
func probeNodeSudo(container string) bool {
	cmd := exec.Command("docker", "exec", "-u", "node", container, "sudo", "-n", "true")
	return cmd.Run() == nil
}

// ---------------------------------------------------------------------------
// CLI: claws security ...
// ---------------------------------------------------------------------------

func cmdSecurity(args []string) error {
	if len(args) == 0 {
		return errorf("usage: claws security <tier> [args]\n  claws security tier <agent>\n  claws security tier <agent> --set <level> [--accept-risk]\n  claws security tier --all")
	}
	switch args[0] {
	case "tier":
		return cmdSecurityTier(args[1:])
	default:
		return errorf("unknown security subcommand: %q (try: tier)", args[0])
	}
}

// cmdSecurityTier handles:
//   claws security tier <agent>                                 -- print current tier
//   claws security tier --all                                   -- print all tiers in fleet
//   claws security tier <agent> --set <level> [--accept-risk]   -- set new tier
//
// --accept-risk is required to enter privileged.
// --accept-risk plus --i-understand-this-can-pwn-the-host is required for host-reach.
func cmdSecurityTier(args []string) error {
	paths := resolvePaths()
	if len(args) == 0 {
		return errorf("usage: claws security tier <agent> [--set <level>] [--accept-risk]")
	}
	if args[0] == "--all" {
		return printAllTiers(paths)
	}
	name := args[0]
	rest := args[1:]

	// Parse flags.
	var setLevel string
	var acceptRisk bool
	var hostReachAck bool
	i := 0
	for i < len(rest) {
		a := rest[i]
		switch {
		case a == "--set" && i+1 < len(rest):
			setLevel = rest[i+1]
			i += 2
		case strings.HasPrefix(a, "--set="):
			setLevel = strings.TrimPrefix(a, "--set=")
			i++
		case a == "--accept-risk":
			acceptRisk = true
			i++
		case a == "--i-understand-this-can-pwn-the-host":
			hostReachAck = true
			i++
		default:
			return errorf("unknown flag for security tier: %q", a)
		}
	}

	if setLevel == "" {
		// Get mode.
		current := readSecurityTier(paths, name)
		fmt.Printf("%s: %s\n", name, current)
		return nil
	}

	// Set mode.
	to, ok := parseTier(setLevel)
	if !ok {
		return errorf("unknown tier %q — valid: untrusted, standard, privileged, host-reach", setLevel)
	}
	from := readSecurityTier(paths, name)
	if from == to {
		fmt.Printf("%s is already at tier %s — no change.\n", name, to)
		return nil
	}

	// Risk gates — only check when PROMOTING.
	promoting := tierRank(to) > tierRank(from)
	if promoting {
		if to.requiresAcceptRisk() && !acceptRisk {
			return errorf("promoting %s → %s requires --accept-risk\n  %s", from, to, to.describe())
		}
		if to.requiresHostReachAck() && !hostReachAck {
			return errorf("promoting to host-reach also requires --i-understand-this-can-pwn-the-host\n  A compromised host-reach agent can read the host filesystem, control other containers, and load kernel modules.")
		}
	}

	// Warn loudly.
	if promoting {
		fmt.Printf("\033[1;33m⚠  Promoting %s from %s → %s\033[0m\n", name, from, to)
		fmt.Printf("    %s\n", to.describe())
		fmt.Printf("    Container will be RECREATED to apply new caps. Channels reconnect briefly.\n")
		fmt.Printf("    Audit entry will be written as %s.\n", currentOperator())
	} else {
		fmt.Printf("\033[0;32m==> Demoting %s from %s → %s (security improvement)\033[0m\n", name, from, to)
	}

	// 1. Persist the tier.
	if err := writeSecurityTier(paths, name, to); err != nil {
		return err
	}
	// 2. Regenerate the overlay file.
	if err := writeSecurityCompose(paths, name); err != nil {
		return fmt.Errorf("write security overlay: %w", err)
	}
	// 3. Write audit entry.
	writeSecurityTierAudit(paths, name, from, to, acceptRisk)
	// 4. Recreate the container so docker picks up the new security_opt / cap_drop.
	fmt.Println("==> Recreating container to apply security changes...")
	if err := dcRun(paths, name, "up", "-d", "--force-recreate"); err != nil {
		return fmt.Errorf("recreate failed (security yml may be invalid): %w", err)
	}
	// 5. If transitioning INTO privileged or host-reach, install sudo overlay.
	if to == TierPrivileged || to == TierHostReach {
		fmt.Println("==> Installing privileged overlay (apt install sudo + NOPASSWD sudoers)...")
		// Brief wait so the container is up before exec.
		time.Sleep(2 * time.Second)
		if err := installPrivilegedOverlay(paths, name); err != nil {
			return fmt.Errorf("overlay install failed: %w", err)
		}
	}
	fmt.Printf("\033[0;32m==> %s is now at tier %s.\033[0m\n", name, to)
	if to == TierPrivileged {
		fmt.Println("    sudo + apt + /etc write available INSIDE the container.")
		fmt.Println("    Host is still unreachable (no docker.sock, no host-pid).")
	}
	return nil
}

// tierRank returns a stable ordering for promotion checks.
//
//	untrusted (0) < standard (1) < privileged (2) < host-reach (3)
func tierRank(t SecurityTier) int {
	switch t {
	case TierUntrusted:
		return 0
	case TierStandard:
		return 1
	case TierPrivileged:
		return 2
	case TierHostReach:
		return 3
	}
	return 1
}

// printAllTiers prints the tier for every instance in the fleet.
// Used by `claws security tier --all`.
func printAllTiers(paths Paths) error {
	instances, err := listInstanceNames(paths)
	if err != nil {
		return err
	}
	sort.Strings(instances)
	maxLen := 0
	for _, n := range instances {
		if len(n) > maxLen {
			maxLen = len(n)
		}
	}
	for _, n := range instances {
		t := readSecurityTier(paths, n)
		color := tierColor(t)
		fmt.Printf("%-*s  %s%s%s\n", maxLen, n, color, t, "\033[0m")
	}
	return nil
}

// tierColor returns the ANSI color code for a tier — used in list rendering.
func tierColor(t SecurityTier) string {
	switch t {
	case TierUntrusted:
		return "\033[0;36m" // cyan
	case TierStandard:
		return "\033[0;90m" // dim grey — common case, low signal
	case TierPrivileged:
		return "\033[0;33m" // yellow
	case TierHostReach:
		return "\033[0;31m" // red
	}
	return ""
}

// fleetHasElevatedTier returns true if any instance is above standard.
// Used by `claws list` to decide whether to show the TIER column at all.
func fleetHasElevatedTier(paths Paths) bool {
	names, err := listInstanceNames(paths)
	if err != nil {
		return false
	}
	for _, n := range names {
		t := readSecurityTier(paths, n)
		if t != TierStandard {
			return true
		}
	}
	return false
}

// listInstanceNames returns "team/name" entries for every instance on disk.
// Walks paths.Root, looking for directories that contain instance.env.
func listInstanceNames(paths Paths) ([]string, error) {
	var out []string
	// Top-level: groupless instances at paths.Root/<name>
	// Grouped:   paths.Root/<group>/<name>
	rootEntries, err := os.ReadDir(paths.Root)
	if err != nil {
		return nil, err
	}
	for _, e := range rootEntries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(paths.Root, e.Name())
		// Direct instance?
		if _, err := os.Stat(filepath.Join(full, "instance.env")); err == nil {
			out = append(out, e.Name())
			continue
		}
		// Otherwise: treat as a group, scan one level deeper.
		subs, err := os.ReadDir(full)
		if err != nil {
			continue
		}
		for _, s := range subs {
			if !s.IsDir() {
				continue
			}
			subFull := filepath.Join(full, s.Name())
			if _, err := os.Stat(filepath.Join(subFull, "instance.env")); err == nil {
				out = append(out, e.Name()+"/"+s.Name())
			}
		}
	}
	return out, nil
}
