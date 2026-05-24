package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ---------------------------------------------------------------------------
// claws setup — pre-flight auth detection
// ---------------------------------------------------------------------------
//
// Goal: a non-technical user running `claws setup` for the second agent on
// the same box shouldn't have to re-find/re-paste credentials they already
// pasted somewhere. We scan three reservoirs:
//
//   1. Process environment — ANTHROPIC_API_KEY, OPENAI_API_KEY, etc.
//      Cheapest signal, no I/O.
//   2. CLI auth dirs from sibling tools — ~/.codex/auth.json and
//      ~/.claude/.credentials.json. These are OAuth/session token files,
//      not raw API keys; we surface them as "found" but with a caveat,
//      since copying OAuth tokens between identities is generally wrong.
//   3. Existing claws / openclaw agents on this host —
//      ~/.claws-workspace/*/*/instance.env and ~/.openclaw/*/*/instance.env.
//      We only READ the file to enumerate WHICH KEYS are present (not the
//      values) — surfacing those agents lets the operator know "you've
//      already done this for agent X" rather than us silently copying
//      session-keyed credentials between agents (which can break the source).
//
// READ-ONLY. We never mutate detected sources.

// AuthDetection is the report returned to the setup wizard.
type AuthDetection struct {
	// API keys in environment (values present so wizard can pipe them
	// straight into `claws auth <name> apikey <provider> <key>`).
	EnvAnthropicKey   string
	EnvOpenAIKey      string
	EnvOpenRouterKey  string

	// OAuth/session token files from sibling CLIs. We record the path
	// only; the wizard surfaces these as informational hints because
	// safely re-using an OAuth token across identities is not a thing
	// we want to encourage.
	ClaudeCodeOAuth string // ~/.claude/.credentials.json (with claudeAiOauth block)
	CodexOAuth      string // ~/.codex/auth.json (or .codex/config.json)

	// Existing instances on this host. The wizard surfaces these so the
	// user knows "you already authenticated agent X — you may want to
	// keep using that one instead of making a new one."
	ExistingAgents []DetectedAgent
}

// DetectedAgent describes a previously-set-up claws/openclaw instance.
type DetectedAgent struct {
	Workspace string   // root (~/.claws-workspace or ~/.openclaw)
	Team      string   // first path component under workspace
	Name      string   // second path component
	EnvPath   string   // full path to instance.env
	HasKeys   []string // names of relevant *_KEY / *_TOKEN / *_SESSION_* vars present
}

// Any returns true if any detection produced a result.
func (d *AuthDetection) Any() bool {
	if d == nil {
		return false
	}
	return d.EnvAnthropicKey != "" || d.EnvOpenAIKey != "" || d.EnvOpenRouterKey != "" ||
		d.ClaudeCodeOAuth != "" || d.CodexOAuth != "" || len(d.ExistingAgents) > 0
}

// detectExistingAuth runs all detectors and returns what was found.
// Safe to call multiple times; never writes.
func detectExistingAuth() *AuthDetection {
	d := &AuthDetection{
		EnvAnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
		EnvOpenAIKey:     os.Getenv("OPENAI_API_KEY"),
		EnvOpenRouterKey: os.Getenv("OPENROUTER_API_KEY"),
	}

	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return d
	}

	// Codex CLI auth — try the documented locations in order.
	for _, p := range []string{
		filepath.Join(home, ".codex", "auth.json"),
		filepath.Join(home, ".codex", "config.json"),
	} {
		if fileExistsNonEmpty(p) {
			d.CodexOAuth = p
			break
		}
	}

	// Claude Code OAuth — only count it if the file actually parses as
	// a credentials JSON with the expected block. The mere existence of
	// the file doesn't prove it's a credential store.
	if p := filepath.Join(home, ".claude", ".credentials.json"); fileExistsNonEmpty(p) {
		if hasClaudeOAuthBlock(p) {
			d.ClaudeCodeOAuth = p
		}
	}

	// Existing claws / openclaw agents. Both workspaces have the same
	// shape: <root>/<team>/<agent>/instance.env.
	for _, ws := range []string{
		filepath.Join(home, ".claws-workspace"),
		filepath.Join(home, ".openclaw"),
	} {
		matches, _ := filepath.Glob(filepath.Join(ws, "*", "*", "instance.env"))
		for _, m := range matches {
			keys := readRelevantKeys(m)
			if len(keys) == 0 {
				continue
			}
			rel, _ := filepath.Rel(ws, m)
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) < 2 {
				continue
			}
			d.ExistingAgents = append(d.ExistingAgents, DetectedAgent{
				Workspace: ws,
				Team:      parts[0],
				Name:      parts[1],
				EnvPath:   m,
				HasKeys:   keys,
			})
		}
	}
	// Stable order for deterministic UX.
	sort.Slice(d.ExistingAgents, func(i, j int) bool {
		a, b := d.ExistingAgents[i], d.ExistingAgents[j]
		if a.Workspace != b.Workspace {
			return a.Workspace < b.Workspace
		}
		if a.Team != b.Team {
			return a.Team < b.Team
		}
		return a.Name < b.Name
	})

	return d
}

// fileExistsNonEmpty returns true if the path is a non-empty regular file.
func fileExistsNonEmpty(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.Mode().IsRegular() && fi.Size() > 0
}

// hasClaudeOAuthBlock checks for the documented credentials.json shape
// (a "claudeAiOauth" object). We don't pull token values — just confirm
// the file is what we think it is.
func hasClaudeOAuthBlock(p string) bool {
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(data, &doc); err != nil {
		return false
	}
	_, ok := doc["claudeAiOauth"]
	return ok
}

// readRelevantKeys returns the names of *_KEY / *_TOKEN / *_SESSION_* /
// COOKIE-style variables present in an instance.env file. Values are
// never returned — only the key names — because the wizard's job is to
// SAY what's already set up, not to copy credentials.
func readRelevantKeys(envPath string) []string {
	f, err := os.Open(envPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(scan.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue
		}
		k := line[:eq]
		v := strings.TrimSpace(line[eq+1:])
		if v == "" {
			continue
		}
		if isAuthLikeName(k) {
			out = append(out, k)
		}
	}
	return out
}

// shortenHome rewrites a $HOME-prefixed path to use "~/" for display.
func shortenHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return p
	}
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

// isAuthLikeName matches env var names that look like credentials.
// Conservative on purpose — we'd rather miss a weird custom name than
// surface non-secret config (paths, ports, etc.).
func isAuthLikeName(k string) bool {
	u := strings.ToUpper(k)
	// Things we explicitly skip even though they contain "KEY" etc.
	switch u {
	case "OPENCLAW_GATEWAY_TOKEN":
		// This one IS a credential, but it's a per-instance shared
		// secret between gateway and host — not something to re-use.
		return false
	}
	if strings.Contains(u, "API_KEY") ||
		strings.Contains(u, "_TOKEN") ||
		strings.Contains(u, "_SECRET") ||
		strings.Contains(u, "_COOKIE") ||
		strings.Contains(u, "SESSION_KEY") {
		return true
	}
	return false
}
