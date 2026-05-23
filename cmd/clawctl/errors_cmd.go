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

// ---------------------------------------------------------------------------
// clawctl errors — incident-triage umbrella view.
//
// Composes four existing read paths into one screen:
//   1. Container state (running/restarting/exited, restart counts)
//   2. Recent log errors per instance (reuses activity.recentLogErrors)
//   3. Recent clawctl operations that returned error (filtered audit log)
//   4. Orphan Docker containers (reuses orphans.discoverOrphans)
// Plus a "Fix paths" trailer with the exact command to address each finding.
//
// Read-only. Composition layer. Adds no new state.
// ---------------------------------------------------------------------------

type errorsContainerRow struct {
	Name         string `json:"name"`
	State        string `json:"state"`         // "running" | "restarting" | "exited" | "missing"
	Uptime       string `json:"uptime,omitempty"`
	RestartCount int    `json:"restartCount,omitempty"`
}

type errorsAuditRow struct {
	TS     string   `json:"ts"`
	User   string   `json:"user"`
	Cmd    string   `json:"cmd"`
	Args   []string `json:"args,omitempty"`
}

type errorsReport struct {
	Containers []errorsContainerRow `json:"containers"`
	LogErrors  []ActivityEntry      `json:"logErrors"`
	AuditErrors []errorsAuditRow    `json:"auditErrors"`
	Orphans    []orphanInfo         `json:"orphans"`
	FixPaths   []string             `json:"fixPaths,omitempty"`
}

func cmdErrors(args []string) error {
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}

	since := 2 * time.Hour
	if v := flagValue(args, "--since="); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			since = d
		}
	}

	report := gatherErrorsReport(paths, filterGroup, since)

	if jsonMode {
		data, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	renderErrorsReport(report, filterGroup, since)
	return nil
}

// gatherErrorsReport assembles the four sections by calling into existing
// read paths. Each section is independent — a failure to gather one doesn't
// poison the others. Filters by group when set.
func gatherErrorsReport(paths Paths, filterGroup string, since time.Duration) errorsReport {
	rep := errorsReport{}

	// --- Container state ---
	entries, _ := readRegistry(paths)
	entries = filterEntriesByGroup(entries, filterGroup)
	for _, e := range entries {
		row := errorsContainerRow{Name: e.Name, State: "missing"}
		row.State, row.Uptime, row.RestartCount = inspectContainerState(paths, e.Name)
		rep.Containers = append(rep.Containers, row)
	}

	// --- Recent log errors per instance ---
	cutoff := time.Now().Add(-since)
	for _, e := range entries {
		for _, le := range recentLogErrors(paths, e.Name, cutoff, 10) {
			rep.LogErrors = append(rep.LogErrors, ActivityEntry{
				Time:     le.Time,
				Instance: e.Name,
				Type:     "error",
				Detail:   le.Detail,
			})
		}
	}
	// Newest first; cap to 20 to keep the screen readable.
	sort.Slice(rep.LogErrors, func(i, j int) bool {
		return rep.LogErrors[i].Time.After(rep.LogErrors[j].Time)
	})
	if len(rep.LogErrors) > 20 {
		rep.LogErrors = rep.LogErrors[:20]
	}

	// --- Audit errors ---
	rep.AuditErrors = gatherAuditErrors(paths, filterGroup, since)

	// --- Orphans ---
	if orphs, err := discoverOrphans(paths); err == nil {
		rep.Orphans = orphs
	}

	// --- Fix paths ---
	rep.FixPaths = computeFixPaths(rep)

	return rep
}

// inspectContainerState shells `docker compose ps --format json` for one
// instance and extracts state + uptime + restart count. Returns ("missing",
// "", 0) when no container exists for the instance.
func inspectContainerState(paths Paths, name string) (state, uptime string, restartCount int) {
	cs := containerStatus(paths, name)
	if cs == "" {
		return "missing", "", 0
	}
	switch {
	case strings.Contains(cs, "Restarting"):
		state = "restarting"
	case strings.Contains(cs, "Up"):
		state = "running"
		uptime = strings.Replace(cs, "Up ", "", 1)
		if idx := strings.Index(uptime, " ("); idx >= 0 {
			uptime = uptime[:idx]
		}
	case strings.Contains(cs, "Exited"):
		state = "exited"
	default:
		state = strings.ToLower(strings.Fields(cs)[0])
	}

	// Restart count via docker inspect — only useful when the container
	// actually exists.
	containerName := resolveContainerName(paths, name)
	if containerName != "" {
		out, err := exec.Command("docker", "inspect", "--format", "{{.RestartCount}}", containerName).Output()
		if err == nil {
			fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &restartCount)
		}
	}
	return state, uptime, restartCount
}

// gatherAuditErrors reads the audit log and returns entries with result=error
// within the time window, optionally filtered to instances in the named group.
func gatherAuditErrors(paths Paths, filterGroup string, since time.Duration) []errorsAuditRow {
	data, err := readAuditLog(paths)
	if err != nil {
		return nil
	}
	cutoff := time.Now().Add(-since)
	var out []errorsAuditRow
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		result, _ := entry["result"].(string)
		if result != "error" {
			continue
		}
		ts, _ := entry["ts"].(string)
		if t, err := time.Parse(time.RFC3339, ts); err == nil {
			if t.Before(cutoff) {
				continue
			}
		}
		argsAny, _ := entry["args"].([]any)
		argStrs := make([]string, len(argsAny))
		for i, a := range argsAny {
			argStrs[i], _ = a.(string)
		}
		if filterGroup != "" && !auditEntryInGroup(argStrs, filterGroup) {
			continue
		}
		user, _ := entry["user"].(string)
		cmd, _ := entry["cmd"].(string)
		out = append(out, errorsAuditRow{TS: ts, User: user, Cmd: cmd, Args: argStrs})
	}
	return out
}

// readAuditLog is a tiny wrapper that lets gatherAuditErrors stay
// testable without duplicating the audit-log path resolution.
func readAuditLog(paths Paths) ([]byte, error) {
	return os.ReadFile(filepath.Join(paths.Root, auditLogFile))
}

// computeFixPaths inspects the report and emits a deduplicated list of
// directive commands that, run in order, address each surfaced finding.
// Operators copy-paste; we never run anything ourselves.
//
// Deliberately narrow: we only emit *commands*, never running commentary.
// Audit errors are visible in their own section above; re-emitting them as
// "# audit had: ..." comments in the fix list adds noise without action.
func computeFixPaths(rep errorsReport) []string {
	seen := map[string]bool{}
	var fixes []string
	add := func(cmd string) {
		if cmd == "" || seen[cmd] {
			return
		}
		seen[cmd] = true
		fixes = append(fixes, cmd)
	}

	// Restart-loop containers usually want a hard restart or a config look.
	for _, c := range rep.Containers {
		if c.State == "restarting" {
			add(fmt.Sprintf("clawctl logs %s --tail=50   # diagnose restart loop", c.Name))
		}
	}

	// Per-instance log errors → suggest the targeted log-grep.
	loggedInstances := map[string]bool{}
	for _, le := range rep.LogErrors {
		loggedInstances[le.Instance] = true
	}
	for name := range loggedInstances {
		add(fmt.Sprintf("clawctl logs %s --grep=error --since=2h", name))
	}

	// Orphans get a one-shot cleanup command (only suggest once).
	if len(rep.Orphans) > 0 {
		add("clawctl orphans clean --all --yes")
	}

	sort.Strings(fixes)
	return fixes
}

// stripANSI removes ANSI SGR escape sequences from s. Used to render log-
// error details where the upstream container may have emitted color codes
// that our truncate-by-bytes pass would cut mid-escape, leaving the
// terminal in a stuck color state.
func stripANSI(s string) string {
	var b strings.Builder
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
		b.WriteRune(r)
	}
	return b.String()
}

// stripLeadingDockerTimestamp drops the RFC3339-ish timestamp that
// `docker compose logs --timestamps` prepends to every line. Format we
// see in practice:  "2026-05-23T06:10:19.648549645Z <rest of line>"
// We keep the rest. Defensive — if the prefix doesn't match, return s
// unchanged.
func stripLeadingDockerTimestamp(s string) string {
	// Format: digit{4}-digit{2}-digit{2}T... up to the first space.
	if len(s) < 20 || s[4] != '-' || s[7] != '-' || s[10] != 'T' {
		return s
	}
	sp := strings.IndexByte(s, ' ')
	if sp < 0 {
		return s
	}
	return s[sp+1:]
}

// renderErrorsReport prints the human-readable umbrella view.
func renderErrorsReport(rep errorsReport, filterGroup string, since time.Duration) {
	bold := "\033[1m"
	nc := "\033[0m"
	red := "\033[0;31m"
	yellow := "\033[0;33m"
	green := "\033[0;32m"

	scope := ""
	if filterGroup != "" {
		scope = fmt.Sprintf(" (group: %s)", filterGroup)
	}
	fmt.Printf("%sclawctl errors%s%s — last %s\n\n", bold, scope, nc, since)

	// Section 1: container state
	fmt.Printf("%sContainer state (%d)%s\n", bold, len(rep.Containers), nc)
	if len(rep.Containers) == 0 {
		fmt.Println("  (no instances)")
	}
	for _, c := range rep.Containers {
		stateColored := green + c.State + nc
		switch c.State {
		case "restarting":
			stateColored = yellow + c.state() + nc
		case "exited", "missing":
			stateColored = red + c.State + nc
		}
		extra := ""
		if c.Uptime != "" {
			extra = fmt.Sprintf("%s uptime", c.Uptime)
		}
		if c.RestartCount > 0 {
			if extra != "" {
				extra += "   "
			}
			extra += fmt.Sprintf("%s%d restarts%s", yellow, c.RestartCount, nc)
		}
		fmt.Printf("  %-20s %-12s %s\n", c.Name, stateColored, extra)
	}
	fmt.Println()

	// Section 2: log errors
	fmt.Printf("%sRecent log errors (%d)%s\n", bold, len(rep.LogErrors), nc)
	if len(rep.LogErrors) == 0 {
		fmt.Println("  (none in window)")
	}
	for _, le := range rep.LogErrors {
		// Strip ANSI and any leading docker-compose timestamp duplicate;
		// the parsed le.Time already provides the canonical timestamp.
		// What's left is the agent's own log line — the part operators
		// actually want to read during triage.
		detail := stripANSI(le.Detail)
		detail = stripLeadingDockerTimestamp(detail)
		fmt.Printf("  %s  %-15s  %s\n", le.Time.Format("15:04:05"), le.Instance, truncate(detail, 100))
	}
	fmt.Println()

	// Section 3: clawctl audit errors
	fmt.Printf("%sRecent clawctl operations that returned error (%d)%s\n", bold, len(rep.AuditErrors), nc)
	if len(rep.AuditErrors) == 0 {
		fmt.Println("  (none in window)")
	}
	for _, ae := range rep.AuditErrors {
		fmt.Printf("  %s  %-10s  %-12s  %s\n", ae.TS, ae.User, ae.Cmd, strings.Join(ae.Args, " "))
	}
	fmt.Println()

	// Section 4: orphans
	fmt.Printf("%sOrphan containers (%d)%s\n", bold, len(rep.Orphans), nc)
	if len(rep.Orphans) == 0 {
		fmt.Println("  (none)")
	}
	for _, o := range rep.Orphans {
		marker := ""
		if len(o.MountsBad) > 0 {
			marker = fmt.Sprintf("   %s%d mounts ✗%s", yellow, len(o.MountsBad), nc)
		}
		fmt.Printf("  %-45s %-12s%s\n", o.Container, o.Status, marker)
	}
	fmt.Println()

	// Trailer: fix paths
	if len(rep.FixPaths) > 0 {
		fmt.Printf("%sFix paths%s\n", bold, nc)
		for _, f := range rep.FixPaths {
			fmt.Printf("  %s\n", f)
		}
	} else {
		fmt.Printf("%sNothing to fix.%s\n", green, nc)
	}
}

// errorsContainerRow.state accessor avoids accidental mutation in render
// switch statements. (Defensive — Go has no consts on struct fields.)
func (c errorsContainerRow) state() string { return c.State }
