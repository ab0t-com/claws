package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const applyHelp = `Usage: claws apply --file=<profile.json> [--yes] [--dry-run]

Apply a declarative JSON profile. Reconciles your host to match.
Idempotent — re-running converges to declared state.

Profile schema (claws.ab0t.com/v1) — minimum example:

  {
    "apiVersion": "claws.ab0t.com/v1",
    "kind": "Profile",
    "metadata": { "name": "my-bot", "version": "1.0.0" },
    "team":     { "name": "default" },
    "agents": [
      {
        "name": "agent-1",
        "channels": [
          { "type": "telegram", "tokenFrom": { "env": "TELEGRAM_BOT_TOKEN" } }
        ]
      }
    ]
  }

Secret references: tokenFrom.env, tokenFrom.file, or tokenFrom.command.
Profiles contain no secrets.

Options:
  --file=<path>     Profile JSON file (required)
  --yes             Skip confirmation for elevated permissions
  --dry-run         Print what would change, don't mutate state
`

type Profile struct {
	APIVersion string           `json:"apiVersion"`
	Kind       string           `json:"kind"`
	Metadata   ProfileMetadata  `json:"metadata"`
	Runtime    *ProfileRuntime  `json:"runtime,omitempty"`
	Policy     *ProfilePolicy   `json:"policy,omitempty"`
	Team       ProfileTeam      `json:"team"`
	Agents     []ProfileAgent   `json:"agents"`
	PostSetup  []string         `json:"postSetup,omitempty"`
	Warnings   []string         `json:"warnings,omitempty"`
}

type ProfileMetadata struct {
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	License     string   `json:"license,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

type ProfileRuntime struct {
	Name  string `json:"name,omitempty"`
	Image string `json:"image,omitempty"`
}

type ProfilePolicy struct {
	LoopbackOnly    *bool  `json:"loopbackOnly,omitempty"`
	DMDefault       string `json:"dmDefault,omitempty"`
	OutboundDefault string `json:"outboundDefault,omitempty"`
}

type ProfileTeam struct {
	Name   string `json:"name"`
	Shared bool   `json:"shared,omitempty"`
}

type ProfileAgent struct {
	Name     string                 `json:"name"`
	Role     string                 `json:"role,omitempty"`
	Manager  string                 `json:"manager,omitempty"`
	Image    string                 `json:"image,omitempty"`           // pin to specific runtime image
	Sandbox  *bool                  `json:"sandbox,omitempty"`         // agents.defaults.sandbox (nil = inherit)
	Tools    *ProfileTools          `json:"tools,omitempty"`
	Skills   []string               `json:"skills,omitempty"`          // skill names to enable
	Hooks    map[string]string      `json:"hooks,omitempty"`           // event → command/script
	Config   map[string]interface{} `json:"config,omitempty"`          // arbitrary openclaw.json patches
	Auth     *ProfileAuth           `json:"auth,omitempty"`
	Channels []ProfileChannel       `json:"channels,omitempty"`
}

type ProfileTools struct {
	Profile string   `json:"profile,omitempty"`
	Allow   []string `json:"allow,omitempty"`
	Deny    []string `json:"deny,omitempty"`
}

type ProfileAuth struct {
	Preferred      string         `json:"preferred,omitempty"`
	FallbackAPIKey *ProfileAPIKey `json:"fallbackApiKey,omitempty"`
}

type ProfileAPIKey struct {
	Provider string `json:"provider"`
	FromEnv  string `json:"fromEnv,omitempty"`
	FromFile string `json:"fromFile,omitempty"`
}

type ProfileChannel struct {
	Type      string     `json:"type"`
	TokenFrom *SecretRef `json:"tokenFrom,omitempty"`
	BotToken  *SecretRef `json:"botTokenFrom,omitempty"`
	AppToken  *SecretRef `json:"appTokenFrom,omitempty"`
	DMPolicy  string     `json:"dmPolicy,omitempty"`
}

type SecretRef struct {
	Env     string   `json:"env,omitempty"`
	File    string   `json:"file,omitempty"`
	Command []string `json:"command,omitempty"`
}

// resolve returns the secret value or "" if unresolvable (caller decides how to handle).
func (s *SecretRef) resolve() string {
	if s == nil {
		return ""
	}
	if s.Env != "" {
		return os.Getenv(s.Env)
	}
	if s.File != "" {
		data, err := os.ReadFile(s.File)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	}
	if len(s.Command) > 0 {
		out, err := exec.Command(s.Command[0], s.Command[1:]...).Output()
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(out))
	}
	return ""
}

func (s *SecretRef) describe() string {
	if s == nil {
		return "(none)"
	}
	switch {
	case s.Env != "":
		return "env:" + s.Env
	case s.File != "":
		return "file:" + s.File
	case len(s.Command) > 0:
		return "command:" + s.Command[0]
	}
	return "(unset)"
}

func cmdApply(args []string) error {
	var file, templateName string
	var yes, dryRun, skipAudit bool
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Print(applyHelp)
			return nil
		case strings.HasPrefix(a, "--file="):
			file = strings.TrimPrefix(a, "--file=")
		case strings.HasPrefix(a, "--template="):
			templateName = strings.TrimPrefix(a, "--template=")
		case a == "--yes" || a == "-y":
			yes = true
		case a == "--dry-run":
			dryRun = true
		case a == "--skip-audit":
			skipAudit = true
		}
	}
	_ = skipAudit // wired in E2 below
	if file == "" && templateName == "" {
		return errorf("either --file=<profile.json> or --template=<name> required")
	}
	if file != "" && templateName != "" {
		return errorf("--file and --template are mutually exclusive")
	}
	if templateName != "" {
		resolved, err := resolveTemplate(templateName)
		if err != nil {
			return err
		}
		file = resolved
	}

	data, err := os.ReadFile(file)
	if err != nil {
		return errorf("read profile: %v", err)
	}
	var p Profile
	if err := json.Unmarshal(data, &p); err != nil {
		return errorf("parse profile JSON: %v", err)
	}
	if p.APIVersion == "" || p.Kind == "" {
		return errorf("profile missing apiVersion or kind")
	}
	if p.APIVersion != "claws.ab0t.com/v1" {
		return errorf("unsupported apiVersion: %q (want claws.ab0t.com/v1)", p.APIVersion)
	}
	if p.Kind != "Profile" {
		return errorf("unsupported kind: %q (want Profile)", p.Kind)
	}
	if p.Team.Name == "" {
		return errorf("profile missing team.name")
	}
	if len(p.Agents) == 0 {
		return errorf("profile has no agents")
	}
	if err := validateName(p.Team.Name); err != nil {
		return errorf("invalid team name %q: %v", p.Team.Name, err)
	}
	for _, ag := range p.Agents {
		if err := validateName(ag.Name); err != nil {
			return errorf("invalid agent name %q: %v", ag.Name, err)
		}
	}

	const (
		bold  = "\033[1m"
		green = "\033[0;32m"
		dim   = "\033[0;90m"
		gold  = "\033[0;33m"
		nc    = "\033[0m"
	)

	fmt.Printf("%s==> claws apply%s %s\n", bold, nc, file)
	fmt.Printf("  profile:  %s@%s\n", p.Metadata.Name, p.Metadata.Version)
	if p.Metadata.Description != "" {
		fmt.Printf("  what:     %s\n", p.Metadata.Description)
	}
	if p.Runtime != nil && p.Runtime.Name != "" {
		fmt.Printf("  runtime:  %s\n", p.Runtime.Name)
	}
	fmt.Printf("  team:     %s\n", p.Team.Name)
	fmt.Printf("  agents:   %d\n", len(p.Agents))
	if len(p.Warnings) > 0 {
		fmt.Printf("\n%s⚠ This profile declares warnings:%s\n", gold, nc)
		for _, w := range p.Warnings {
			fmt.Printf("  • %s\n", w)
		}
		if !yes && !dryRun {
			fmt.Printf("\nPass --yes to acknowledge and continue.\n")
			return errorf("aborted (warnings unacknowledged)")
		}
	}

	if dryRun {
		fmt.Printf("\n%s[dry-run] Would do:%s\n", gold, nc)
		fmt.Printf("  • init / policy init / access init (if missing)\n")
		fmt.Printf("  • create group %s\n", p.Team.Name)
		for _, ag := range p.Agents {
			full := p.Team.Name + "/" + ag.Name
			fmt.Printf("  • create agent %s\n", full)
			if ag.Auth != nil {
				fmt.Printf("    auth: %s\n", ag.Auth.Preferred)
			}
			for _, ch := range ag.Channels {
				tokRef := ch.TokenFrom
				if tokRef == nil {
					tokRef = ch.BotToken
				}
				fmt.Printf("    channel: %s (token %s)\n", ch.Type, tokRef.describe())
			}
		}
		for _, ps := range p.PostSetup {
			fmt.Printf("  • postSetup: %s\n", ps)
		}
		fmt.Println()
		return nil
	}

	paths := resolvePaths()

	// init / policy / access (idempotent)
	if !isInitialized(paths) {
		fmt.Printf("\n%s==> init%s\n", bold, nc)
		if err := cmdInit(nil); err != nil {
			return errorf("init failed: %v", err)
		}
	}
	if _, err := os.Stat(paths.Root + "/policy.json"); err != nil {
		fmt.Printf("\n%s==> policy init%s\n", bold, nc)
		if err := cmdPolicy([]string{"init"}); err != nil {
			fmt.Printf("  %s! policy init failed: %v (continuing)%s\n", dim, err, nc)
		}
	}
	if _, err := os.Stat(paths.Root + "/.access.json"); err != nil {
		fmt.Printf("\n%s==> access init%s\n", bold, nc)
		if err := cmdAccess([]string{"init"}); err != nil {
			fmt.Printf("  %s! access init failed: %v (continuing)%s\n", dim, err, nc)
		}
	}

	// A1 — apply policy block (after init has written defaults, before agents created)
	if p.Policy != nil {
		fmt.Printf("\n%s==> applying policy%s\n", bold, nc)
		if err := applyProfilePolicy(paths, p.Policy); err != nil {
			fmt.Printf("  %s! policy apply failed: %v (continuing)%s\n", gold, err, nc)
		} else {
			fmt.Printf("  %s✓ policy.json updated%s\n", green, nc)
		}
	}

	// group (created once per profile)
	groupDir := paths.Root + "/" + p.Team.Name
	if _, err := os.Stat(groupDir); err != nil {
		fmt.Printf("\n%s==> group %s%s\n", bold, p.Team.Name, nc)
		if err := cmdGroup([]string{"create", p.Team.Name}); err != nil {
			return errorf("group create %s: %v", p.Team.Name, err)
		}
	}

	// agents
	for _, ag := range p.Agents {
		full := p.Team.Name + "/" + ag.Name
		fmt.Printf("\n%s==> agent %s%s\n", bold, full, nc)

		if instanceExists(paths, full) {
			fmt.Printf("  %s✓ already exists (skipping create)%s\n", dim, nc)
		} else {
			createArgs := []string{full}
			if ag.Role != "" {
				createArgs = append(createArgs, "--role="+ag.Role)
			}
			if ag.Manager != "" {
				createArgs = append(createArgs, "--manager="+ag.Manager)
			}
			// A2 — runtime.image (per-agent override of profile runtime) → --image flag
			img := ag.Image
			if img == "" && p.Runtime != nil {
				img = p.Runtime.Image
			}
			if img != "" {
				createArgs = append(createArgs, "--image="+img)
			}
			if err := cmdCreate(createArgs); err != nil {
				return errorf("create %s: %v", full, err)
			}
			fmt.Printf("  %s✓ created%s\n", green, nc)
		}

		// A4+B1+B5 — sandbox + tools.profile + tools.allow/deny via cmdConfig set
		// B4 — arbitrary config catch-all
		applyAgentConfig(full, &ag, dim, gold, green, nc)

		// B2 — skills
		if len(ag.Skills) > 0 {
			if err := applySkills(paths, full, ag.Skills); err != nil {
				fmt.Printf("  %s! skills: %v%s\n", gold, err, nc)
			} else {
				fmt.Printf("  %s✓ skills enabled: %s%s\n", green, strings.Join(ag.Skills, ", "), nc)
			}
		}

		// B3 — hooks
		if len(ag.Hooks) > 0 {
			if err := applyHooks(paths, full, ag.Hooks); err != nil {
				fmt.Printf("  %s! hooks: %v%s\n", gold, err, nc)
			} else {
				fmt.Printf("  %s✓ hooks written: %d event(s)%s\n", green, len(ag.Hooks), nc)
			}
		}

		// channels — D1 idempotence: skip if already enabled
		for _, ch := range ag.Channels {
			if channelEnabled(paths, full, ch.Type) {
				fmt.Printf("  %s✓ channel %s already configured (skipping)%s\n", dim, ch.Type, nc)
				continue
			}
			tokRef := ch.TokenFrom
			if tokRef == nil {
				tokRef = ch.BotToken
			}
			token := tokRef.resolve()
			if token == "" {
				fmt.Printf("  %s! channel %s skipped — secret %s did not resolve%s\n",
					gold, ch.Type, tokRef.describe(), nc)
				continue
			}
			chArgs := []string{"add", full, ch.Type, "--token=" + token}
			if ch.Type == "slack" && ch.AppToken != nil {
				appTok := ch.AppToken.resolve()
				if appTok != "" {
					chArgs = []string{"add", full, ch.Type, "--bot-token=" + token, "--app-token=" + appTok}
				}
			}
			// A3 — dmPolicy flag pass-through
			if ch.DMPolicy != "" {
				chArgs = append(chArgs, "--dmPolicy="+ch.DMPolicy)
			}
			if err := cmdChannel(chArgs); err != nil {
				fmt.Printf("  %s! channel %s add failed: %v%s\n", gold, ch.Type, err, nc)
				continue
			}
			fmt.Printf("  %s✓ channel %s connected%s\n", green, ch.Type, nc)
		}

		// auth — D2 idempotence: skip if apikey already configured for provider
		if ag.Auth != nil && ag.Auth.FallbackAPIKey != nil {
			provider := ag.Auth.FallbackAPIKey.Provider
			if apikeyConfigured(paths, full, provider) {
				fmt.Printf("  %s✓ auth (apikey %s) already set (skipping)%s\n", dim, provider, nc)
			} else {
				ref := &SecretRef{Env: ag.Auth.FallbackAPIKey.FromEnv, File: ag.Auth.FallbackAPIKey.FromFile}
				key := ref.resolve()
				if key != "" {
					if err := cmdAuth([]string{full, "apikey", provider, key}); err != nil {
						fmt.Printf("  %s! auth apikey %s failed: %v%s\n", gold, provider, err, nc)
					} else {
						fmt.Printf("  %s✓ auth (apikey %s) configured%s\n", green, provider, nc)
					}
				} else if ag.Auth.Preferred == "codex" {
					fmt.Printf("  %s→ next: claws auth %s codex   (OAuth, opens browser)%s\n", dim, full, nc)
				}
			}
		} else if ag.Auth != nil && ag.Auth.Preferred == "codex" {
			fmt.Printf("  %s→ next: claws auth %s codex   (OAuth, opens browser)%s\n", dim, full, nc)
		}
	}

	// postSetup — best effort
	for _, step := range p.PostSetup {
		fmt.Printf("\n%s==> postSetup: %s%s\n", bold, step, nc)
		fields := strings.Fields(step)
		if len(fields) == 0 {
			continue
		}
		var perr error
		switch fields[0] {
		case "audit":
			perr = cmdAudit(fields[1:])
		case "verify":
			perr = cmdAuthVerify(fields[1:])
		case "start":
			perr = cmdStart(fields[1:])
		default:
			fmt.Printf("  %s! unknown postSetup verb %q (skipping)%s\n", dim, fields[0], nc)
		}
		if perr != nil {
			fmt.Printf("  %s! %s failed: %v (continuing)%s\n", dim, step, perr, nc)
		}
	}

	// E2 — auto-audit unless explicitly skipped
	if !skipAudit {
		fmt.Printf("\n%s==> Security audit%s\n", bold, nc)
		if err := cmdAudit(nil); err != nil {
			fmt.Printf("  %s! audit reported issues (see above) — fix before going live%s\n", gold, nc)
			// Don't fail the apply — surface but continue.
		}
	}

	fmt.Printf("\n%s✓ apply complete%s — %d agent(s)\n\n", green, nc, len(p.Agents))
	return nil
}

// ---------------------------------------------------------------------------
// applyProfilePolicy — A1: map ProfilePolicy → Policy + writePolicy.
// ---------------------------------------------------------------------------
func applyProfilePolicy(paths Paths, pp *ProfilePolicy) error {
	if pp == nil {
		return nil
	}
	pol := readPolicy(paths)
	if pp.LoopbackOnly != nil {
		if *pp.LoopbackOnly {
			pol.AllowedBindModes = []string{"loopback"}
		} else {
			pol.AllowedBindModes = nil // empty = any
		}
	}
	switch pp.DMDefault {
	case "pairing", "allowlist":
		pol.RequireDmPairing = true
	case "open":
		pol.RequireDmPairing = false
	}
	switch pp.OutboundDefault {
	case "off":
		pol.RequireOutboundAllowlist = true
	case "allowlist":
		pol.RequireOutboundAllowlist = true
	case "open":
		pol.RequireOutboundAllowlist = false
	}
	// Always keep audit on for templates that don't say otherwise.
	pol.AuditLog = true
	return writePolicy(paths, pol)
}

// ---------------------------------------------------------------------------
// applyAgentConfig — A4 + B1 + B4 + B5: per-agent config-set patches.
// ---------------------------------------------------------------------------
func applyAgentConfig(full string, ag *ProfileAgent, dim, gold, green, nc string) {
	// Collect all config keys we want to set into one ordered list for clean output.
	type kv struct{ k, v string }
	var sets []kv

	// B1 — sandbox
	if ag.Sandbox != nil {
		sets = append(sets, kv{"agents.defaults.sandbox", fmt.Sprintf("%t", *ag.Sandbox)})
	}
	// A4 — tools.profile
	if ag.Tools != nil && ag.Tools.Profile != "" {
		sets = append(sets, kv{"tools.profile", strconv.Quote(ag.Tools.Profile)})
	}
	// B5 — tools.allow / tools.deny
	if ag.Tools != nil && len(ag.Tools.Allow) > 0 {
		j, _ := json.Marshal(ag.Tools.Allow)
		sets = append(sets, kv{"tools.allow", string(j)})
	}
	if ag.Tools != nil && len(ag.Tools.Deny) > 0 {
		j, _ := json.Marshal(ag.Tools.Deny)
		sets = append(sets, kv{"tools.deny", string(j)})
	}
	// B4 — arbitrary config catch-all
	for k, v := range ag.Config {
		j, err := json.Marshal(v)
		if err != nil {
			fmt.Printf("  %s! config %s: marshal failed: %v%s\n", gold, k, err, nc)
			continue
		}
		sets = append(sets, kv{k, string(j)})
	}

	for _, s := range sets {
		if err := cmdConfig([]string{"set", full, s.k, s.v}); err != nil {
			fmt.Printf("  %s! config set %s: %v%s\n", gold, s.k, err, nc)
		} else {
			fmt.Printf("  %s✓ config %s = %s%s\n", green, s.k, s.v, nc)
		}
	}
}

// ---------------------------------------------------------------------------
// applySkills — B2: write a skills manifest under the agent's workspace.
// ---------------------------------------------------------------------------
func applySkills(paths Paths, full string, skills []string) error {
	instDir := filepath.Join(paths.Root, full)
	skillsDir := filepath.Join(instDir, "workspace", "skills")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return err
	}
	manifest := filepath.Join(skillsDir, "MANIFEST.txt")
	body := strings.Join(skills, "\n") + "\n"
	// Idempotence: only write if content differs
	if existing, err := os.ReadFile(manifest); err == nil && string(existing) == body {
		return nil
	}
	return os.WriteFile(manifest, []byte(body), 0644)
}

// ---------------------------------------------------------------------------
// applyHooks — B3: write hook scripts under the agent's workspace.
// ---------------------------------------------------------------------------
func applyHooks(paths Paths, full string, hooks map[string]string) error {
	instDir := filepath.Join(paths.Root, full)
	hooksDir := filepath.Join(instDir, "workspace", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return err
	}
	for event, cmd := range hooks {
		// Validate event name (alphanum + dash, no slashes)
		if strings.ContainsAny(event, "/\\.") {
			continue
		}
		hookFile := filepath.Join(hooksDir, event+".sh")
		body := "#!/bin/sh\n# claws-managed hook for event: " + event + "\n" + cmd + "\n"
		// Idempotence: only write if content differs
		if existing, err := os.ReadFile(hookFile); err == nil && string(existing) == body {
			continue
		}
		if err := os.WriteFile(hookFile, []byte(body), 0755); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// channelEnabled — D1: pre-check before cmdChannel add.
// Returns true if the channel is already configured + enabled for this agent.
// ---------------------------------------------------------------------------
func channelEnabled(paths Paths, full, channelType string) bool {
	cfgPath := filepath.Join(paths.Root, full, "openclaw.json")
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		return false
	}
	var cfg map[string]interface{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return false
	}
	channels, _ := cfg["channels"].(map[string]interface{})
	ch, _ := channels[channelType].(map[string]interface{})
	enabled, _ := ch["enabled"].(bool)
	return enabled
}

// ---------------------------------------------------------------------------
// apikeyConfigured — D2: pre-check before cmdAuth apikey.
// Returns true if the credentials/<provider>.key file exists and is non-empty.
// ---------------------------------------------------------------------------
func apikeyConfigured(paths Paths, full, provider string) bool {
	keyFile := filepath.Join(paths.Root, full, "credentials", provider+".key")
	if info, err := os.Stat(keyFile); err == nil && info.Size() > 0 {
		return true
	}
	return false
}
