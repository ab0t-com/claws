package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
//
// When --probe is requested, the Probe field is populated with the result of
// verifyOneInstance for this agent. Without --probe it stays nil — the JSON
// shape becomes "probe optional", which is the right contract because
// callers paying for `--probe` opt into the cost.
type authStatusRecord struct {
	Name            string             `json:"name"`
	Model           string             `json:"model"`           // from openclaw.json, or "" if unset
	GatewayTokenSet bool               `json:"gatewayTokenSet"` // OPENCLAW_GATEWAY_TOKEN present and non-empty
	ChannelCreds    []string           `json:"channelCreds"`    // filenames in credentials/, no contents
	LastAuthAt      string             `json:"lastAuthAt"`      // RFC3339 from audit log, "" if never
	LastAuthCmd     string             `json:"lastAuthCmd"`     // raw audit args, e.g. "alpha codex"
	LastAuthResult  string             `json:"lastAuthResult"`  // "ok" / "error"
	Probe           *authVerifyResult  `json:"probe,omitempty"` // populated only when --probe is set
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
	probe := hasFlag(args, "--probe")
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
		rec := gatherAuthStatus(paths, n)
		if probe {
			pr := verifyOneInstance(paths, n)
			rec.Probe = &pr
		}
		records = append(records, rec)
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
	const wVerified = 18

	fmt.Print(bold)
	fmt.Print(padVisible("NAME", wName))
	fmt.Print(" " + padVisible("MODEL", wModel))
	fmt.Print(" " + padVisible("TOKEN", wToken))
	fmt.Print(" " + padVisible("CHANNEL CREDS", wCreds))
	fmt.Print(" " + padVisible("LAST AUTH", wLast))
	if probe {
		fmt.Print(" " + padVisible("VERIFIED", wVerified))
	}
	fmt.Println(nc)

	fmt.Print(padVisible(strings.Repeat("─", wName), wName))
	fmt.Print(" " + padVisible(strings.Repeat("─", wModel), wModel))
	fmt.Print(" " + padVisible(strings.Repeat("─", wToken), wToken))
	fmt.Print(" " + padVisible(strings.Repeat("─", wCreds), wCreds))
	fmt.Print(" " + padVisible(strings.Repeat("─", wLast), wLast))
	if probe {
		fmt.Print(" " + padVisible(strings.Repeat("─", wVerified), wVerified))
	}
	fmt.Println()

	// Track the per-row fix suggestions when --probe surfaces failures so
	// we can summarise them in a trailer block at the bottom of the table.
	type fixRow struct {
		name string
		cmd  string
	}
	var needsAttention []fixRow

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
		if probe && r.Probe != nil {
			// Verified column values are a bounded short set; pad to visible
			// width without truncating — the byte-counting truncate would
			// cut into ANSI escape sequences and produce garbled output.
			var verifStr string
			switch {
			case r.Probe.Verified && r.Probe.Strategy == "logs":
				verifStr = green + "✓ no errors 5m" + nc
			case r.Probe.Verified:
				verifStr = green + "✓ " + r.Probe.Strategy + nc
			case r.Probe.Strategy == "skipped":
				verifStr = yellow + "? inconclusive" + nc
			default:
				verifStr = red + "✗ failing" + nc
				if r.Probe.FixCommand != "" {
					needsAttention = append(needsAttention, fixRow{name: r.Name, cmd: r.Probe.FixCommand})
				}
			}
			fmt.Print(" " + padVisible(verifStr, wVerified))
		}
		fmt.Println()
	}
	fmt.Println()
	fmt.Println("  Model shows what's pinned in openclaw.json; \"—\" means the runtime default applies.")
	fmt.Println("  Channel creds is the count + sample filenames from credentials/. No secrets shown.")
	fmt.Println("  Last auth is the most recent `clawctl auth <name> ...` invocation from the audit log.")
	if probe {
		fmt.Println("  Verified column: log-scan checks last 5m for upstream auth errors.")
	}
	if len(needsAttention) > 0 {
		fmt.Println()
		fmt.Printf("  %s%d instance(s) need attention:%s\n", bold, len(needsAttention), nc)
		for _, f := range needsAttention {
			fmt.Printf("    %s\n", f.cmd)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// auth verify <name> — the per-instance primitive
//
// Tries verification strategies in order, returning the first conclusive
// answer. Strategies are deliberately ordered cheapest-and-most-reliable
// first; strategy C (log scan) is the pragmatic v1 because it works without
// any OpenClaw runtime changes and answers the question operators actually
// ask during incidents: "is auth currently 401-ing?"
//
// Strategy A:  GET /__openclaw__/auth-check  → JSON {verified, ...}
//              (Preferred; not exposed by the OpenClaw runtime today. Reserved.)
// Strategy B:  GET /readyz  → JSON {ready, failing[]}; if "failing" contains
//              any auth-related subsystem, that's a verify failure.
// Strategy C:  Scan last 5 minutes of gateway logs for known auth-error
//              patterns. Most pragmatic; no runtime cooperation needed.
// ---------------------------------------------------------------------------

type authVerifyResult struct {
	Name       string `json:"name"`
	Provider   string `json:"provider,omitempty"`
	Model      string `json:"model,omitempty"`
	Verified   bool   `json:"verified"`
	Strategy   string `json:"strategy"` // "endpoint" | "readyz" | "logs" | "skipped"
	LatencyMs  int    `json:"latency_ms,omitempty"`
	Error      string `json:"error,omitempty"`
	FixCommand string `json:"fix_command,omitempty"`
}

// authErrorPatterns matches log lines that prove the agent's *model* auth is
// failing (as opposed to channel-layer auth like Baileys WhatsApp 401s,
// which use different shapes). The provider names + standard OpenAI/
// Anthropic/etc. error shapes give us high-confidence detection. Compiled
// once per process via the package-level var.
var authErrorPatterns = regexp.MustCompile(
	`(?i)(invalid_api_key|incorrect_api_key|insufficient_quota|insufficient.quota|` +
		`authentication_error|model_not_found|access_denied|` +
		`401\s+(unauthor|forbidden)|403\s+(unauthor|forbidden)|` +
		`(openai|codex|anthropic|claude)[^\n]*?(401|403|unauthor|forbidden|invalid|expired))`)

// verifyOneInstance runs the verification chain for a single instance.
// Always returns a populated result; Verified=true iff at least one
// strategy reported success.
func verifyOneInstance(paths Paths, name string) authVerifyResult {
	res := authVerifyResult{Name: name, Strategy: "skipped"}

	ref, _ := ParseRef(name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	rt := mustResolveRuntime(paths, name)

	// Best-effort fill of provider/model so the renderer has context.
	if cfg, err := readInstanceConfig(rt.ConfigPath(dir)); err == nil {
		if v := getNestedConfig(cfg, "agents.defaults.model.primary"); v != nil {
			if s, ok := v.(string); ok {
				res.Model = s
				// Provider is typically the "<provider>/<model>" prefix.
				if idx := strings.Index(s, "/"); idx > 0 {
					res.Provider = s[:idx]
				}
			}
		}
	}

	port := readEnvValue(envFile, "OPENCLAW_GATEWAY_PORT")
	if port == "" {
		res.Error = "instance has no gateway port (not yet started?)"
		res.FixCommand = fmt.Sprintf("clawctl start %s", name)
		return res
	}

	// Strategy A — auth-check endpoint (reserved; OpenClaw doesn't expose it yet).
	if ok, latency, err := tryAuthCheckEndpoint(port, rt); err == nil {
		res.Strategy = "endpoint"
		res.LatencyMs = latency
		res.Verified = ok
		if !ok {
			res.Error = "auth-check endpoint reported failure"
			res.FixCommand = suggestReauthCommand(name, res.Provider)
		}
		return res
	}
	// Strategy B — /readyz with failing[] containing auth-related entries.
	if conclusion := tryReadyzAuth(port, rt); conclusion != nil {
		res.Strategy = "readyz"
		res.LatencyMs = conclusion.LatencyMs
		res.Verified = conclusion.Verified
		if !conclusion.Verified {
			res.Error = conclusion.Error
			res.FixCommand = suggestReauthCommand(name, res.Provider)
		}
		return res
	}
	// Strategy C — log scan over last 5 minutes.
	if conclusion := tryLogScanAuth(paths, name); conclusion != nil {
		res.Strategy = "logs"
		res.Verified = conclusion.Verified
		if !conclusion.Verified {
			res.Error = conclusion.Error
			res.FixCommand = suggestReauthCommand(name, res.Provider)
		}
		return res
	}

	// No strategy reached a conclusion. Surface honestly with a fix message
	// that matches the actual state — don't tell an operator to `start` an
	// already-running agent.
	res.Strategy = "skipped"
	if containerIsRunning(paths, name) {
		res.Error = "gateway is running but had no auth activity in the last 5m to check against"
		res.FixCommand = fmt.Sprintf("send a test message to '%s' via its channel, then re-run verify", name)
	} else {
		res.Error = "gateway not running"
		res.FixCommand = fmt.Sprintf("clawctl start %s", name)
	}
	return res
}

// tryAuthCheckEndpoint hits the (reserved) /__openclaw__/auth-check endpoint.
// Returns (verified, latency_ms, nil) on a successful 200; (false, 0, err)
// if the endpoint doesn't exist or any transport error occurs.
func tryAuthCheckEndpoint(port string, rt Runtime) (bool, int, error) {
	url := fmt.Sprintf("http://127.0.0.1:%s/__openclaw__/auth-check", port)
	client := &http.Client{Timeout: 3 * time.Second}
	start := time.Now()
	resp, err := client.Get(url)
	latency := int(time.Since(start) / time.Millisecond)
	if err != nil {
		return false, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 404 {
		// Endpoint not implemented by this runtime. Treat as "no signal";
		// caller should fall through.
		return false, 0, fmt.Errorf("auth-check endpoint not implemented")
	}
	if resp.StatusCode != 200 {
		// Non-200 from an existing endpoint = inconclusive; fall through.
		return false, 0, fmt.Errorf("auth-check returned %d", resp.StatusCode)
	}
	var body struct {
		Verified bool `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, 0, err
	}
	return body.Verified, latency, nil
}

// tryReadyzAuth probes /readyz and inspects the failing[] array for auth-
// related subsystem names. Returns nil if the result is inconclusive
// (couldn't reach /readyz, or failing[] is empty, or the failing entries
// aren't recognisably auth-related).
func tryReadyzAuth(port string, rt Runtime) *authVerifyResult {
	if rt.ReadyEndpoint == "" {
		return nil
	}
	url := fmt.Sprintf("http://127.0.0.1:%s%s", port, rt.ReadyEndpoint)
	client := &http.Client{Timeout: 3 * time.Second}
	start := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	latency := int(time.Since(start) / time.Millisecond)
	var body struct {
		Ready   bool     `json:"ready"`
		Failing []string `json:"failing"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil
	}
	if body.Ready {
		return &authVerifyResult{Verified: true, LatencyMs: latency}
	}
	// Only treat /readyz failure as an *auth* failure if the failing[]
	// entries name an auth-related subsystem. Otherwise it's another
	// problem and we should fall through to log scan.
	authSubsystems := []string{"model", "auth", "openai", "codex", "anthropic", "claude", "credentials"}
	for _, f := range body.Failing {
		fl := strings.ToLower(f)
		for _, s := range authSubsystems {
			if strings.Contains(fl, s) {
				return &authVerifyResult{
					Verified:  false,
					LatencyMs: latency,
					Error:     fmt.Sprintf("/readyz reports failing subsystem: %s", f),
				}
			}
		}
	}
	return nil // inconclusive; fall through
}

// containerIsRunning reports whether docker thinks the gateway container
// for an instance is currently up. Used to distinguish "container down"
// from "container up but quiet" in the inconclusive path so the directive
// fix message is accurate (start vs trigger-a-test).
func containerIsRunning(paths Paths, name string) bool {
	cs := containerStatus(paths, name)
	return strings.Contains(cs, "Up")
}

// tryLogScanAuth shells docker compose logs --since 5m and matches against
// known auth-error patterns. Returns nil if the container isn't running
// (no log signal); a populated result with Verified=false if any matching
// line is found; Verified=true if no matches AND there's recent activity
// (the container has logged *something* in the window, but no auth error).
func tryLogScanAuth(paths Paths, name string) *authVerifyResult {
	ref, _ := ParseRef(name)
	rt := mustResolveRuntime(paths, name)
	dir := ref.Dir(paths)
	envFile := filepath.Join(dir, "instance.env")
	override := rt.OverridePath(dir)
	composeTemplate := rt.ComposeTemplatePath(paths)

	allArgs := []string{"compose", "-f", composeTemplate}
	if _, err := os.Stat(override); err == nil {
		allArgs = append(allArgs, "-f", override)
	}
	allArgs = append(allArgs, "--env-file", envFile, "-p", rt.MakeProjectName(ref))
	allArgs = append(allArgs, "logs", "--since=5m", "--no-color", rt.GatewayService)

	out, err := exec.Command("docker", allArgs...).CombinedOutput()
	if err != nil {
		// Compose error usually means the container isn't running. No signal.
		return nil
	}
	logs := string(out)
	if strings.TrimSpace(logs) == "" {
		// No recent log activity. Inconclusive — the agent might be alive
		// and silent, or it might not be invoking the model. We can't claim
		// verified.
		return nil
	}
	if loc := authErrorPatterns.FindStringIndex(logs); loc != nil {
		// Take the matched line for the operator-facing message.
		snippet := matchedLine(logs, loc[0])
		return &authVerifyResult{
			Verified: false,
			Error:    fmt.Sprintf("auth error in last 5m: %s", truncate(snippet, 120)),
		}
	}
	// Recent activity, no auth errors. Highest confidence we can offer
	// without an upstream probe.
	return &authVerifyResult{Verified: true}
}

// matchedLine returns the full log line containing offset.
func matchedLine(logs string, offset int) string {
	start := strings.LastIndexByte(logs[:offset], '\n') + 1
	end := strings.IndexByte(logs[offset:], '\n')
	if end < 0 {
		return strings.TrimSpace(logs[start:])
	}
	return strings.TrimSpace(logs[start : offset+end])
}

// suggestReauthCommand produces the per-provider directive fix string.
// For codex we know the exact verb; for unknown providers we surface
// the apikey form with placeholders.
func suggestReauthCommand(name, provider string) string {
	switch provider {
	case "openai-codex", "codex", "openai":
		return fmt.Sprintf("clawctl auth %s codex", name)
	case "anthropic", "claude":
		return fmt.Sprintf("clawctl auth %s apikey anthropic <key>", name)
	default:
		return fmt.Sprintf("clawctl auth %s codex  (or: clawctl auth %s apikey <provider> <key>)", name, name)
	}
}

func cmdAuthVerify(args []string) error {
	paths := resolvePaths()
	name := firstPositional(args)
	if name == "" {
		return errorf("usage: clawctl auth verify <name> [--json]")
	}
	if err := requireInstance(paths, name); err != nil {
		return err
	}
	jsonMode := hasFlag(args, "--json")
	res := verifyOneInstance(paths, name)

	if jsonMode {
		data, _ := json.MarshalIndent(res, "", "  ")
		fmt.Println(string(data))
		if !res.Verified {
			// Non-zero exit so scripts can `|| alert`.
			os.Exit(1)
		}
		return nil
	}

	bold := "\033[1m"
	nc := "\033[0m"
	green := "\033[0;32m"
	red := "\033[0;31m"
	yellow := "\033[0;33m"

	fmt.Printf("%sVerifying model auth for '%s'...%s\n", bold, res.Name, nc)
	if res.Provider != "" {
		fmt.Printf("  provider:  %s\n", res.Provider)
	}
	if res.Model != "" {
		fmt.Printf("  model:     %s\n", res.Model)
	}
	fmt.Printf("  strategy:  %s", res.Strategy)
	if res.LatencyMs > 0 {
		fmt.Printf(" (%dms)", res.LatencyMs)
	}
	fmt.Println()

	switch {
	case res.Verified:
		// Communicate confidence in the success message so operators don't
		// over-trust a strategy=logs pass (which is "no failures seen" not
		// "we made a successful round-trip just now").
		switch res.Strategy {
		case "endpoint":
			fmt.Printf("  %s✓ auth verified via upstream check%s\n", green, nc)
		case "readyz":
			fmt.Printf("  %s✓ /readyz reports auth subsystem ready%s\n", green, nc)
		case "logs":
			fmt.Printf("  %s✓ no auth errors observed in last 5m%s\n", green, nc)
			fmt.Printf("  %s(lower confidence — log scan can't prove the next call will succeed)%s\n", "\033[0;90m", nc)
		default:
			fmt.Printf("  %s✓ auth ok%s\n", green, nc)
		}
		return nil
	case res.Strategy == "skipped":
		fmt.Printf("  %s? inconclusive%s — %s\n", yellow, nc, res.Error)
		if res.FixCommand != "" {
			fmt.Printf("  Try:  %s\n", res.FixCommand)
		}
		return errorf("inconclusive")
	default:
		fmt.Printf("  %s✗ %s%s\n", red, res.Error, nc)
		if res.FixCommand != "" {
			fmt.Printf("  Fix:  %s\n", res.FixCommand)
		}
		return errorf("auth verification failed for '%s'", name)
	}
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
