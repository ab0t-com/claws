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
// clawctl channels (no args) — fleet-wide channel matrix
// clawctl auth status [name]   — per-agent auth provider inventory
//
// Both are read-only views over instance.env + openclaw.json + credentials/
// + the audit log. They consolidate what `channel status`, `config get`,
// and `access audit` would tell you per-instance into one screen.
// ---------------------------------------------------------------------------

// knownChannelTypes is the set of channel types the matrix renders columns
// for. Sourced from `channelProfiles` in channel.go so the matrix stays in
// sync with the channels clawctl actually knows how to provision. Any
// channel configured on an agent outside this set still shows up via
// `clawctl channel status <name>`, just not in the fleet matrix.
func knownChannelTypes() []string {
	keys := make([]string, 0, len(channelProfiles))
	for k := range channelProfiles {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// channelMatrixRow captures one agent's row in the matrix. Cells maps each
// channel type to its dmPolicy when enabled, or "" when not configured.
type channelMatrixRow struct {
	Name  string            `json:"name"`
	Cells map[string]string `json:"cells"`
}

func gatherChannelMatrix(paths Paths, filterGroup string) ([]channelMatrixRow, []string, error) {
	entries, err := readRegistry(paths)
	if err != nil {
		return nil, nil, err
	}
	entries = filterEntriesByGroup(entries, filterGroup)

	cols := knownChannelTypes()
	rows := make([]channelMatrixRow, 0, len(entries))

	for _, e := range entries {
		row := channelMatrixRow{Name: e.Name, Cells: map[string]string{}}
		ref, _ := ParseRef(e.Name)
		dir := ref.Dir(paths)
		rt := mustResolveRuntime(paths, e.Name)
		cfg, err := readInstanceConfig(rt.ConfigPath(dir))
		if err != nil {
			rows = append(rows, row)
			continue
		}
		channels, _ := cfg["channels"].(map[string]any)
		for _, c := range cols {
			cm, ok := channels[c].(map[string]any)
			if !ok {
				continue
			}
			enabled, _ := cm["enabled"].(bool)
			if !enabled {
				continue
			}
			policy, _ := cm["dmPolicy"].(string)
			if policy == "" {
				policy = "—"
			}
			row.Cells[c] = policy
		}
		rows = append(rows, row)
	}
	return rows, cols, nil
}

func cmdChannelsMatrix(args []string) error {
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}

	rows, cols, err := gatherChannelMatrix(paths, filterGroup)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		if jsonMode {
			fmt.Println("[]")
		} else if filterGroup != "" {
			fmt.Printf("No instances in group '%s'.\n", filterGroup)
		} else {
			fmt.Println("No instances registered.")
		}
		return nil
	}

	if jsonMode {
		data, _ := json.MarshalIndent(map[string]any{
			"columns": cols,
			"rows":    rows,
		}, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"

	const wName = 18
	const wCell = 12

	// Header.
	fmt.Print(bold)
	fmt.Print(padVisible("NAME", wName))
	for _, c := range cols {
		fmt.Print(" " + padVisible(c, wCell))
	}
	fmt.Println(nc)

	// Separator.
	fmt.Print(padVisible(strings.Repeat("─", wName), wName))
	for range cols {
		fmt.Print(" " + padVisible(strings.Repeat("─", wCell), wCell))
	}
	fmt.Println()

	// Rows.
	for _, r := range rows {
		fmt.Print(padVisible(r.Name, wName))
		for _, c := range cols {
			policy := r.Cells[c]
			if policy == "" {
				fmt.Print(" " + padVisible("—", wCell))
			} else {
				fmt.Print(" " + padVisible(green+"✓ "+truncate(policy, wCell-2)+nc, wCell))
			}
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("  ✓ = enabled with dmPolicy shown; — = not configured")
	fmt.Println("  Drill in: clawctl channel status <name>  |  clawctl channel security <name>")
	return nil
}

// ---------------------------------------------------------------------------
// auth status
// ---------------------------------------------------------------------------

// authStatusRecord summarises the auth state for one instance. Fields are
// intentionally narrow: we only surface what we can verify from disk + audit
// log, never anything that would require talking to the model.
type authStatusRecord struct {
	Name             string   `json:"name"`
	Model            string   `json:"model"`              // from openclaw.json, or "" if unset
	GatewayTokenSet  bool     `json:"gatewayTokenSet"`    // OPENCLAW_GATEWAY_TOKEN present and non-empty
	ChannelCreds     []string `json:"channelCreds"`       // filenames in credentials/, no contents
	LastAuthAt       string   `json:"lastAuthAt"`         // RFC3339 from audit log, "" if never
	LastAuthCmd      string   `json:"lastAuthCmd"`        // raw audit args, e.g. "alpha codex"
	LastAuthResult   string   `json:"lastAuthResult"`     // "ok" / "error"
}

func gatherAuthStatus(paths Paths, name string) authStatusRecord {
	ref, _ := ParseRef(name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	rt := mustResolveRuntime(paths, name)

	rec := authStatusRecord{Name: name}
	if token := readEnvValue(envFile, "OPENCLAW_GATEWAY_TOKEN"); token != "" {
		rec.GatewayTokenSet = true
	}
	if cfg, err := readInstanceConfig(rt.ConfigPath(dir)); err == nil {
		if v := getNestedConfig(cfg, "agents.defaults.model.primary"); v != nil {
			if s, ok := v.(string); ok {
				rec.Model = s
			}
		}
	}
	if entries, err := os.ReadDir(filepath.Join(dir, "credentials")); err == nil {
		for _, e := range entries {
			rec.ChannelCreds = append(rec.ChannelCreds, e.Name())
		}
		sort.Strings(rec.ChannelCreds)
	}
	rec.LastAuthAt, rec.LastAuthCmd, rec.LastAuthResult = lastAuthEvent(paths, name)
	return rec
}

// lastAuthEvent scans the audit log backwards for the most recent `cmd=auth`
// entry whose first positional arg equals name. Returns ts, "argv-joined",
// result — or empty strings if no such entry exists.
func lastAuthEvent(paths Paths, name string) (string, string, string) {
	data, err := os.ReadFile(filepath.Join(paths.Root, auditLogFile))
	if err != nil {
		return "", "", ""
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	// Walk backwards.
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
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
		argStrs := make([]string, len(argsAny))
		for j, a := range argsAny {
			argStrs[j], _ = a.(string)
		}
		if !auditEntryInGroupOrName(argStrs, name) {
			continue
		}
		ts, _ := entry["ts"].(string)
		result, _ := entry["result"].(string)
		return ts, strings.Join(argStrs, " "), result
	}
	return "", "", ""
}

func cmdAuthStatus(args []string) error {
	paths := resolvePaths()
	jsonMode := hasFlag(args, "--json")
	filterGroup := flagValue(args, "--group=")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}
	name := firstPositional(args)

	var names []string
	switch {
	case name != "":
		if err := requireInstance(paths, name); err != nil {
			return err
		}
		names = []string{name}
	default:
		entries, err := readRegistry(paths)
		if err != nil {
			return err
		}
		entries = filterEntriesByGroup(entries, filterGroup)
		for _, e := range entries {
			names = append(names, e.Name)
		}
	}

	var records []authStatusRecord
	for _, n := range names {
		records = append(records, gatherAuthStatus(paths, n))
	}

	if jsonMode {
		data, _ := json.MarshalIndent(records, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if len(records) == 0 {
		if filterGroup != "" {
			fmt.Printf("No instances in group '%s'.\n", filterGroup)
		} else {
			fmt.Println("No instances registered.")
		}
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	yellow := "\033[0;33m"
	red := "\033[0;31m"

	const wName = 18
	const wModel = 30
	const wToken = 8
	const wCreds = 30
	const wLast = 22

	fmt.Print(bold)
	fmt.Print(padVisible("NAME", wName))
	fmt.Print(" " + padVisible("MODEL", wModel))
	fmt.Print(" " + padVisible("TOKEN", wToken))
	fmt.Print(" " + padVisible("CHANNEL CREDS", wCreds))
	fmt.Print(" " + padVisible("LAST AUTH", wLast))
	fmt.Println(nc)

	fmt.Print(padVisible(strings.Repeat("─", wName), wName))
	fmt.Print(" " + padVisible(strings.Repeat("─", wModel), wModel))
	fmt.Print(" " + padVisible(strings.Repeat("─", wToken), wToken))
	fmt.Print(" " + padVisible(strings.Repeat("─", wCreds), wCreds))
	fmt.Print(" " + padVisible(strings.Repeat("─", wLast), wLast))
	fmt.Println()

	for _, r := range records {
		// Token column: yes (green) / no (red).
		token := red + "no" + nc
		if r.GatewayTokenSet {
			token = green + "yes" + nc
		}
		// Channel creds: count + first one or two filenames.
		credsStr := "—"
		if len(r.ChannelCreds) > 0 {
			summary := fmt.Sprintf("%d (", len(r.ChannelCreds))
			max := 2
			if len(r.ChannelCreds) < max {
				max = len(r.ChannelCreds)
			}
			summary += strings.Join(r.ChannelCreds[:max], ",")
			if len(r.ChannelCreds) > max {
				summary += ",…"
			}
			summary += ")"
			credsStr = summary
		}
		// Last auth: relative time + result. Show "—" if never authed.
		lastStr := "—"
		if r.LastAuthAt != "" {
			when := r.LastAuthAt
			if t, err := time.Parse(time.RFC3339, r.LastAuthAt); err == nil {
				when = relativeAge(time.Since(t))
			}
			marker := green + "✓" + nc
			if r.LastAuthResult == "error" {
				marker = yellow + "!" + nc
			}
			lastStr = marker + " " + when
		}

		fmt.Print(padVisible(r.Name, wName))
		fmt.Print(" " + padVisible(truncate(orDash(r.Model), wModel), wModel))
		fmt.Print(" " + padVisible(token, wToken))
		fmt.Print(" " + padVisible(truncate(credsStr, wCreds), wCreds))
		fmt.Print(" " + padVisible(truncate(lastStr, wLast), wLast))
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("  Model shows what's pinned in openclaw.json; \"—\" means the runtime default applies.")
	fmt.Println("  Channel creds is the count + sample filenames from credentials/. No secrets shown.")
	fmt.Println("  Last auth is the most recent `clawctl auth <name> ...` invocation from the audit log.")
	return nil
}

// relativeAge renders a duration as a compact human string ("2h", "3d", "12s").
func relativeAge(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
