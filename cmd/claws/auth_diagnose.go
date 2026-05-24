package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// claws auth diagnose [name|fleet] — read-only diagnostic that aggregates
// existing state (audit log, verifyOneInstance, instance.env) and renders
// one screen with the operator's actual next moves.
//
// No new on-disk state. No new file formats. Pure read.
//
// Surfaces the failure mode behind the refresh_token_reused incident:
// multiple agents authed against the same upstream provider account
// within a tight window. Heuristic, not authoritative — but cheap.
// ---------------------------------------------------------------------------

type authDiagRow struct {
	Name         string
	Provider     string
	VerifyState  string // "verified" | "failed" | "inconclusive" | "no-grant"
	VerifyDetail string
	LastAuthAt   time.Time
	LastAuthMode string // "codex" | "apikey" | ""
	Remediation  string
}

func cmdAuthDiagnose(args []string) error {
	paths := resolvePaths()
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}

	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	entries = filterEntriesByGroup(entries, filterGroup)
	if len(entries) == 0 {
		fmt.Println("No agents to diagnose.")
		return nil
	}

	// Pull the audit log once and index by (agent, mode) → last timestamp.
	// We re-use the same audit reader pattern the existing `auth status`
	// command uses. The log is JSON-lines under <root>/.audit.log.
	lastAuth := readAuditAuthIndex(paths)

	rows := make([]authDiagRow, 0, len(entries))
	for _, e := range entries {
		row := authDiagRow{Name: e.Name}

		// Provider from openclaw.json (same lookup verify uses)
		rt := mustResolveRuntime(paths, e.Name)
		ref, _ := ParseRef(e.Name)
		if cfg, err := readInstanceConfig(rt.ConfigPath(ref.Dir(paths))); err == nil {
			if v := getNestedConfig(cfg, "agents.defaults.model.primary"); v != nil {
				if s, ok := v.(string); ok {
					if i := strings.Index(s, "/"); i > 0 {
						row.Provider = s[:i]
					} else {
						row.Provider = s
					}
				}
			}
		}
		if row.Provider == "" {
			row.Provider = "?"
		}

		// Last auth from the audit log.
		if rec, ok := lastAuth[e.Name]; ok {
			row.LastAuthAt = rec.ts
			row.LastAuthMode = rec.mode
		}

		// Verify (the expensive bit — same call cmdStart uses since v1.6.13).
		v := verifyOneInstance(paths, e.Name)
		switch {
		case v.Verified:
			row.VerifyState = "verified"
			row.VerifyDetail = v.Strategy
		case v.Strategy == "skipped":
			row.VerifyState = "inconclusive"
			row.VerifyDetail = "no recent activity"
			row.Remediation = "claws agent ping " + e.Name
		default:
			row.VerifyState = "failed"
			row.VerifyDetail = compact(v.Error, 50)
			row.Remediation = v.FixCommand
		}
		if row.LastAuthMode == "" && row.VerifyState == "failed" {
			row.VerifyState = "no-grant"
			row.VerifyDetail = "no auth event in log"
			row.Remediation = "claws auth " + e.Name + " codex"
		}
		rows = append(rows, row)
	}

	// Render table.
	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"
	dim := "\033[0;90m"

	fmt.Printf("%s%-18s %-14s %-22s %-14s %s%s\n",
		bold, "NAME", "PROVIDER", "VERIFY", "LAST AUTH", "REMEDIATION", nc)
	fmt.Printf("%-18s %-14s %-22s %-14s %s\n",
		"──────────────────", "──────────────", "──────────────────────", "──────────────", "──────────────────────────")

	for _, r := range rows {
		// Colorise verify state.
		var state string
		switch r.VerifyState {
		case "verified":
			state = green + "✓ " + r.VerifyDetail + nc
		case "failed":
			state = red + "✗ " + r.VerifyDetail + nc
		case "inconclusive":
			state = yellow + "? " + r.VerifyDetail + nc
		case "no-grant":
			state = red + "✗ " + r.VerifyDetail + nc
		}
		lastAuth := "—"
		if !r.LastAuthAt.IsZero() {
			lastAuth = humanAgo(time.Since(r.LastAuthAt))
		}
		rem := r.Remediation
		if rem == "" {
			rem = dim + "—" + nc
		}
		fmt.Printf("%-18s %-14s %-31s %-14s %s\n", r.Name, r.Provider, state, lastAuth, rem)
	}

	// Risk heuristics.
	risks := detectRisks(rows)
	if len(risks) > 0 {
		fmt.Printf("\n%sRisk signals:%s\n", bold, nc)
		for _, line := range risks {
			fmt.Printf("  %s\n", line)
		}
	}

	// Next-step hints.
	hintsAgentName := ""
	for _, r := range rows {
		if r.VerifyState == "failed" || r.VerifyState == "no-grant" {
			hintsAgentName = r.Name
			break
		}
	}
	ctx := hintsCtxCheap(paths)
	if hintsAgentName != "" {
		ctx.AgentName = hintsAgentName
	}
	hintsRender("auth diagnose", ctx)
	return nil
}

// detectRisks runs the cheap heuristics described in the ticket:
//   - bunched auth events for the same provider in a tight window
//     → operator probably shares an upstream account; warn about
//       refresh_token_reused
//   - all agents failed verify with the same error class → confirms
//     the shared-grant hypothesis
func detectRisks(rows []authDiagRow) []string {
	var out []string

	// Group by provider; flag if N>=2 agents authed within 15 minutes.
	byProv := map[string][]authDiagRow{}
	for _, r := range rows {
		if !r.LastAuthAt.IsZero() {
			byProv[r.Provider] = append(byProv[r.Provider], r)
		}
	}
	for prov, list := range byProv {
		if len(list) < 2 {
			continue
		}
		sort.Slice(list, func(i, j int) bool { return list[i].LastAuthAt.Before(list[j].LastAuthAt) })
		span := list[len(list)-1].LastAuthAt.Sub(list[0].LastAuthAt)
		if span <= 15*time.Minute {
			names := make([]string, 0, len(list))
			for _, r := range list {
				names = append(names, r.Name)
			}
			out = append(out,
				fmt.Sprintf("⚠ %d agents authed within %s for %s (%s).",
					len(list), span.Round(time.Second), prov, strings.Join(names, ", ")),
				fmt.Sprintf("  If they share an upstream account, refresh_token_reused will recur."),
				fmt.Sprintf("  Each agent should have its own OAuth grant:"),
				fmt.Sprintf("    claws auth fleet codex --missing-only"))
		}
	}

	// Multiple verify failures with reuse-style errors? Confirm the
	// shared-grant cause.
	reuseAgents := []string{}
	for _, r := range rows {
		if r.VerifyState == "failed" && (strings.Contains(strings.ToLower(r.VerifyDetail), "reuse") ||
			strings.Contains(strings.ToLower(r.VerifyDetail), "refresh")) {
			reuseAgents = append(reuseAgents, r.Name)
		}
	}
	if len(reuseAgents) >= 2 {
		out = append(out,
			fmt.Sprintf("⚠ %d agents failing with refresh-token-reuse: %s.",
				len(reuseAgents), strings.Join(reuseAgents, ", ")),
			fmt.Sprintf("  Confirmed shared-grant collision. Re-auth each independently:"),
			fmt.Sprintf("    claws auth fleet codex"))
	}

	return out
}

// readAuditAuthIndex reads the host audit log and returns the last
// `auth <name> <mode>` event per agent. Best-effort: missing file or
// parse errors return an empty map rather than failing.
type auditAuthRec struct {
	ts   time.Time
	mode string
}

func readAuditAuthIndex(paths Paths) map[string]auditAuthRec {
	out := map[string]auditAuthRec{}
	// Same format + same path lastAuthEvent uses in observability.go.
	// JSON-lines: {ts, user, cmd, args, result}. We only care about
	// cmd=="auth" and args=[<name>, <mode>, ...].
	data, err := os.ReadFile(filepath.Join(paths.Root, auditLogFile))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var entry map[string]any
		if json.Unmarshal([]byte(line), &entry) != nil {
			continue
		}
		cmd, _ := entry["cmd"].(string)
		if cmd != "auth" {
			continue
		}
		argsAny, _ := entry["args"].([]any)
		if len(argsAny) < 2 {
			continue
		}
		name, _ := argsAny[0].(string)
		mode, _ := argsAny[1].(string)
		// Skip non-mutating auth verbs (status, verify).
		if mode == "status" || mode == "verify" {
			continue
		}
		tsStr, _ := entry["ts"].(string)
		ts, err := time.Parse(time.RFC3339, tsStr)
		if err != nil {
			continue
		}
		if existing, ok := out[name]; !ok || ts.After(existing.ts) {
			out[name] = auditAuthRec{ts: ts, mode: mode}
		}
	}
	return out
}

func humanAgo(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func compact(s string, max int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > max {
		return s[:max-1] + "…"
	}
	return s
}
