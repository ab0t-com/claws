package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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
	Peers    []string               `json:"peers,omitempty"`           // v1.5 — explicit peer references
	Image    string                 `json:"image,omitempty"`           // pin to specific runtime image
	Sandbox  *bool                  `json:"sandbox,omitempty"`         // agents.defaults.sandbox (nil = inherit)
	Tools    *ProfileTools          `json:"tools,omitempty"`
	Skills   []SkillRef             `json:"skills,omitempty"`          // strings OR ResourceRefs (custom unmarshal)
	Hooks    map[string]HookRef     `json:"hooks,omitempty"`           // event → string OR ResourceRef
	Cron     []ProfileCronJob       `json:"cron,omitempty"`            // v1.5 — periodic jobs
	Events   *ProfileEvents         `json:"events,omitempty"`          // v1.5 — event injection config
	Sidecars []ProfileSidecar       `json:"sidecars,omitempty"`        // v1.5 — per-agent helper sidecars
	Config   map[string]interface{} `json:"config,omitempty"`          // arbitrary openclaw.json patches
	Auth     *ProfileAuth           `json:"auth,omitempty"`
	Channels []ProfileChannel       `json:"channels,omitempty"`
}

// ProfileCronJob — one periodic action declared in agents[].cron[].
// Schedule formats accepted:
//   - 5-field crontab:        "0 9 * * 1"          (9am every Monday)
//   - @-aliases:              "@hourly", "@daily", "@weekly", "@monthly", "@reboot"
//   - duration:               "every 30m"          (Go duration syntax after the word "every")
// Exactly one of Prompt / Command / Hook / Exec must be set.
//
// Action semantics depend on the runtime. The OpenClaw runtime treats every
// cron event as a "send this prompt to the agent" — so `prompt` is the
// natural fit. `command/hook/exec` are wrapped as best-effort text payloads
// for runtimes that interpret literally.
type ProfileCronJob struct {
	Name     string   `json:"name"`                  // unique within the agent
	Schedule string   `json:"schedule"`              // see above
	Prompt   string   `json:"prompt,omitempty"`      // v1.6 — natural-language system-event text
	Command  string   `json:"command,omitempty"`     // inline shell command (wrapped as prompt)
	Hook     string   `json:"hook,omitempty"`        // reference an event in agents[].hooks (wrapped)
	Exec     []string `json:"exec,omitempty"`        // exec form (wrapped)
	Timezone string   `json:"timezone,omitempty"`    // e.g. "UTC", "Pacific/Auckland"
	Enabled  *bool    `json:"enabled,omitempty"`     // default true
}

// ProfileEvents — declares the agent accepts external event injection.
// Maps to openclaw.json events.* via cmdConfig set. Runtime decides
// whether/how to expose the endpoint based on its Capabilities.Events.
type ProfileEvents struct {
	Enabled       bool     `json:"enabled,omitempty"`
	DigestMode    bool     `json:"digestMode,omitempty"`    // batch events into periodic digests
	Endpoint      string   `json:"endpoint,omitempty"`      // relative path, e.g. "/events/sarah"
	AllowFromIPs  []string `json:"allowFromIps,omitempty"`  // CIDR allowlist, empty = any
}

// ProfileSidecar — declares a helper CLI that should be configured for
// this agent. The operator installs the sidecar binary separately;
// claws only writes the integration config.
type ProfileSidecar struct {
	Name   string                 `json:"name"`             // local identifier within the template
	Kind   string                 `json:"kind"`             // "sharedwatch" | "intent-gateway" | "custom"
	Config map[string]interface{} `json:"config,omitempty"` // kind-specific configuration
}

// SkillRef accepts either a bare string ("calendar") or a full object form
// ({"name": "...", "from": "...", "fromUrl": "...", "sha256": "..."}).
type SkillRef struct {
	Name    string `json:"name,omitempty"`
	From    string `json:"from,omitempty"`
	FromURL string `json:"fromUrl,omitempty"`
	Sha256  string `json:"sha256,omitempty"`
}

// UnmarshalJSON lets SkillRef accept a bare string for back-compat.
func (s *SkillRef) UnmarshalJSON(data []byte) error {
	// Try bare string first.
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		s.Name = str
		return nil
	}
	// Object form — use a dummy type to avoid infinite recursion.
	type alias SkillRef
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*s = SkillRef(a)
	return nil
}

// HookRef accepts either a bare string command OR an object with
// command/from/fromUrl/sha256.
type HookRef struct {
	Command string `json:"command,omitempty"`
	From    string `json:"from,omitempty"`
	FromURL string `json:"fromUrl,omitempty"`
	Sha256  string `json:"sha256,omitempty"`
}

func (h *HookRef) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		h.Command = str
		return nil
	}
	type alias HookRef
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*h = HookRef(a)
	return nil
}

// resolveContent returns the raw body for a SkillRef or HookRef, fetching
// from URL or reading from disk as needed. Returns the value, a label
// describing the source, and any error.
func resolveSkillContent(s SkillRef) ([]byte, string, error) {
	switch {
	case s.FromURL != "":
		body, err := fetchResource(s.FromURL, s.Sha256)
		return body, "url:" + s.FromURL, err
	case s.From != "":
		body, err := os.ReadFile(s.From)
		return body, "file:" + s.From, err
	case s.Name != "":
		// Bare name → inline reference, no body needed (manifest only).
		return nil, "name:" + s.Name, nil
	}
	return nil, "", errorf("skill must have one of: name, from, fromUrl")
}

func resolveHookContent(h HookRef) ([]byte, string, error) {
	switch {
	case h.FromURL != "":
		body, err := fetchResource(h.FromURL, h.Sha256)
		return body, "url:" + h.FromURL, err
	case h.From != "":
		body, err := os.ReadFile(h.From)
		return body, "file:" + h.From, err
	case h.Command != "":
		return []byte(h.Command), "inline", nil
	}
	return nil, "", errorf("hook must have one of: command, from, fromUrl")
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
	var yes, dryRun, skipAudit, allowMissing bool
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
		case a == "--allow-missing":
			allowMissing = true
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
		// v1.5 — validate cron schedules at parse-time (fail loud, not silent)
		for _, cj := range ag.Cron {
			if cj.Name == "" {
				return errorf("agent %q: cron job missing name", ag.Name)
			}
			if err := validateCronSchedule(cj.Schedule); err != nil {
				return errorf("agent %q cron %q: %v", ag.Name, cj.Name, err)
			}
			n := 0
			if cj.Prompt != "" {
				n++
			}
			if cj.Command != "" {
				n++
			}
			if cj.Hook != "" {
				n++
			}
			if len(cj.Exec) > 0 {
				n++
			}
			if n != 1 {
				return errorf("agent %q cron %q: exactly one of prompt/command/hook/exec required", ag.Name, cj.Name)
			}
			if cj.Hook != "" {
				if _, ok := ag.Hooks[cj.Hook]; !ok {
					return errorf("agent %q cron %q: references unknown hook %q", ag.Name, cj.Name, cj.Hook)
				}
			}
		}
	}
	// v1.5 — topology validation: cycle detection on manager chains, peers reference existing agents
	if err := validateTopology(p.Agents); err != nil {
		return err
	}
	// v1.6.1 — pre-check every secret reference. Fail loud at parse-time if
	// any required env/file is missing, listing exactly what to set. The old
	// behavior (silently skip the step that needed the missing secret) was
	// the #1 day-one footgun. Opt back in via --allow-missing.
	if !dryRun && !allowMissing {
		if err := checkSecretsResolvable(p); err != nil {
			return err
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
				var sNames []string
				for _, sk := range ag.Skills {
					if sk.Name != "" {
						sNames = append(sNames, sk.Name)
					} else if sk.FromURL != "" {
						sNames = append(sNames, "url:"+filepath.Base(sk.FromURL))
					} else if sk.From != "" {
						sNames = append(sNames, "file:"+filepath.Base(sk.From))
					}
				}
				fmt.Printf("  %s✓ skills enabled: %s%s\n", green, strings.Join(sNames, ", "), nc)
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

		// v1.5 Phase A — cron jobs
		if len(ag.Cron) > 0 {
			if err := applyCron(paths, full, ag); err != nil {
				fmt.Printf("  %s! cron: %v%s\n", gold, err, nc)
			} else {
				fmt.Printf("  %s✓ cron written: %d job(s)%s\n", green, len(ag.Cron), nc)
			}
		}

		// v1.5 Phase B — event injection config
		if ag.Events != nil && ag.Events.Enabled {
			if err := applyEventsConfig(full, ag.Events); err != nil {
				fmt.Printf("  %s! events: %v%s\n", gold, err, nc)
			} else {
				mode := "single"
				if ag.Events.DigestMode {
					mode = "digest"
				}
				fmt.Printf("  %s✓ events injection enabled (%s mode)%s\n", green, mode, nc)
			}
		}

		// v1.5 Phase C — per-agent sidecars
		if len(ag.Sidecars) > 0 {
			for _, sc := range ag.Sidecars {
				if err := applySidecar(paths, full, sc); err != nil {
					fmt.Printf("  %s! sidecar %s: %v%s\n", gold, sc.Kind, err, nc)
				} else {
					fmt.Printf("  %s✓ sidecar configured: %s (%s)%s\n", green, sc.Name, sc.Kind, nc)
				}
			}
		}

		// v1.5 Phase D — topology (manager/peers/workers materialised per-agent)
		if err := applyTopology(paths, p.Agents, full); err != nil {
			fmt.Printf("  %s! topology: %v%s\n", gold, err, nc)
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
// checkSecretsResolvable — v1.6.1: pre-flight for every Env/File secret ref
// in the profile. Fails loud at parse-time rather than silently skipping
// the step at apply-time (the #1 day-one silent failure of v1.5/v1.6.0).
//
// Skips `command:` refs — those are dynamic and run at apply-time.
// ---------------------------------------------------------------------------
func checkSecretsResolvable(p Profile) error {
	type missing struct {
		field    string // human-readable "where" (e.g. "agents[0].channels[0].telegram tokenFrom")
		source   string // "env:OPENAI_API_KEY" or "file:/etc/claws/..."
		hint     string // provider URL or remediation
	}
	var miss []missing

	check := func(field string, ref *SecretRef, hint string) {
		if ref == nil {
			return
		}
		switch {
		case ref.Env != "":
			if os.Getenv(ref.Env) == "" {
				miss = append(miss, missing{field, "env:" + ref.Env, hint})
			}
		case ref.File != "":
			if _, err := os.Stat(ref.File); err != nil {
				miss = append(miss, missing{field, "file:" + ref.File, hint})
			}
		}
	}

	for ai, ag := range p.Agents {
		// auth.fallbackApiKey.fromEnv / fromFile
		if ag.Auth != nil && ag.Auth.FallbackAPIKey != nil {
			provider := ag.Auth.FallbackAPIKey.Provider
			ref := &SecretRef{Env: ag.Auth.FallbackAPIKey.FromEnv, File: ag.Auth.FallbackAPIKey.FromFile}
			check(fmt.Sprintf("agents[%d].auth.fallbackApiKey (%s)", ai, provider),
				ref, providerHint(provider))
		}
		// channels[].tokenFrom / botTokenFrom / appTokenFrom
		for ci, ch := range ag.Channels {
			check(fmt.Sprintf("agents[%d].channels[%d].%s tokenFrom", ai, ci, ch.Type),
				ch.TokenFrom, channelHint(ch.Type))
			check(fmt.Sprintf("agents[%d].channels[%d].%s botTokenFrom", ai, ci, ch.Type),
				ch.BotToken, channelHint(ch.Type))
			check(fmt.Sprintf("agents[%d].channels[%d].%s appTokenFrom", ai, ci, ch.Type),
				ch.AppToken, channelHint(ch.Type))
		}
		// skills[].from (file only — URL pre-fetched separately)
		for si, sk := range ag.Skills {
			if sk.From != "" {
				ref := &SecretRef{File: sk.From}
				check(fmt.Sprintf("agents[%d].skills[%d] from", ai, si), ref, "")
			}
		}
		// hooks[].from
		for ev, hk := range ag.Hooks {
			if hk.From != "" {
				ref := &SecretRef{File: hk.From}
				check(fmt.Sprintf("agents[%d].hooks.%s from", ai, ev), ref, "")
			}
		}
	}

	if len(miss) == 0 {
		return nil
	}

	const (
		bold = "\033[1m"
		red  = "\033[0;31m"
		dim  = "\033[0;90m"
		gold = "\033[0;33m"
		nc   = "\033[0m"
	)
	var b strings.Builder
	fmt.Fprintf(&b, "%scannot apply profile %q: %d secret(s) not resolvable%s\n\n",
		red, p.Metadata.Name, len(miss), nc)
	fmt.Fprintf(&b, "  %s%-50s %-30s %s%s\n", bold, "WHERE", "SOURCE", "GET ONE AT", nc)
	for _, m := range miss {
		hint := m.hint
		if hint == "" {
			hint = "(set this env var or create this file)"
		}
		fmt.Fprintf(&b, "  %-50s %-30s %s\n", m.field, m.source, hint)
	}
	fmt.Fprintf(&b, "\n%sFix one of these ways:%s\n", bold, nc)
	for _, m := range miss {
		if strings.HasPrefix(m.source, "env:") {
			fmt.Fprintf(&b, "  %sexport %s=<value>%s\n", dim, strings.TrimPrefix(m.source, "env:"), nc)
		} else {
			fmt.Fprintf(&b, "  %secho <value> > %s && chmod 600 %s%s\n",
				dim, strings.TrimPrefix(m.source, "file:"), strings.TrimPrefix(m.source, "file:"), nc)
		}
	}
	fmt.Fprintf(&b, "\nOr re-run with %s--allow-missing%s to skip steps with unresolved secrets.\n", gold, nc)
	return errorf("%s", b.String())
}

// providerHint returns the canonical "where do I get this key" URL for
// each known auth provider. Empty if we don't know.
func providerHint(provider string) string {
	switch strings.ToLower(provider) {
	case "openai":
		return "https://platform.openai.com/api-keys"
	case "anthropic":
		return "https://console.anthropic.com/settings/keys"
	case "google", "gemini":
		return "https://aistudio.google.com/app/apikey"
	case "groq":
		return "https://console.groq.com/keys"
	case "openrouter":
		return "https://openrouter.ai/keys"
	}
	return ""
}

// channelHint returns where to create a bot for each known channel type.
func channelHint(channel string) string {
	switch strings.ToLower(channel) {
	case "telegram":
		return "https://t.me/BotFather (/newbot)"
	case "discord":
		return "https://discord.com/developers/applications"
	case "slack":
		return "https://api.slack.com/apps"
	case "whatsapp":
		return "no token — QR scan via `claws channel add … whatsapp`"
	case "signal":
		return "signal-cli — see channels-guide.html"
	}
	return ""
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
// applySkills — B2 + Phase 3 + v1.6 scope fix: skills are mounted by the
// runtime from <team>/shared/skills RO at /home/node/.openclaw/bundled-skills.
// Per-agent workspace/skills (v1.5 default) was silently ignored.
//
// SkillsScope on the runtime declares the destination:
//   "team"  → <team>/shared/skills/                  (v1.6 default for openclaw)
//   "agent" → <instance>/workspace/skills/           (v1.5 legacy)
//   "both"  → both paths
// ---------------------------------------------------------------------------
func applySkills(paths Paths, full string, skills []SkillRef) error {
	rt, err := resolveRuntime(paths, full)
	if err != nil {
		rt = openclawRuntime()
	}
	scope := rt.SkillsScope
	if scope == "" {
		scope = "team"
	}
	team, _ := splitFull(full)
	var skillsDirs []string
	if scope == "team" || scope == "both" {
		skillsDirs = append(skillsDirs, filepath.Join(paths.Root, team, "shared", "skills"))
	}
	if scope == "agent" || scope == "both" {
		skillsDirs = append(skillsDirs, filepath.Join(paths.Root, full, "workspace", "skills"))
	}
	for _, d := range skillsDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	// Use the first dir for the "primary" location messages; write to all.
	skillsDir := skillsDirs[0]

	var names []string
	for _, s := range skills {
		name := s.Name
		if name == "" {
			// Derive from URL or path if not set.
			switch {
			case s.FromURL != "":
				name = filepath.Base(s.FromURL)
			case s.From != "":
				name = filepath.Base(s.From)
			default:
				continue
			}
			name = strings.TrimSuffix(name, filepath.Ext(name))
		}
		names = append(names, name)

		// Materialise the content if there is any (URL or local file).
		if s.FromURL != "" || s.From != "" {
			if s.FromURL != "" && s.Sha256 == "" {
				fmt.Printf("    \033[0;33m! skill %q: no sha256 declared for %s — fetch without integrity check\033[0m\n", name, s.FromURL)
			}
			body, src, err := resolveSkillContent(s)
			if err != nil {
				fmt.Printf("    \033[0;33m! skill %q (%s): %v\033[0m\n", name, src, err)
				continue
			}
			ext := filepath.Ext(s.From)
			if ext == "" {
				ext = ".md"
			}
			for _, dir := range skillsDirs {
				out := filepath.Join(dir, name+ext)
				if existing, err := os.ReadFile(out); err == nil && string(existing) == string(body) {
					continue
				}
				if err := os.WriteFile(out, body, 0644); err != nil {
					return err
				}
			}
		}
	}

	// Manifest of declared skill names — written to all scope dirs.
	body := strings.Join(names, "\n") + "\n"
	for _, dir := range skillsDirs {
		manifest := filepath.Join(dir, "MANIFEST.txt")
		if existing, err := os.ReadFile(manifest); err == nil && string(existing) == body {
			continue
		}
		if err := os.WriteFile(manifest, []byte(body), 0644); err != nil {
			return err
		}
	}
	_ = skillsDir // used in messages
	return nil
}

// ---------------------------------------------------------------------------
// applyHooks — B3 + Phase 3 + v1.6 scope fix: write hook scripts where the
// runtime actually reads them. OpenClaw runtime mounts <team>/shared/hooks
// RO at /home/node/.openclaw/shared-hooks, so per-agent workspace/hooks
// (v1.5 default) was silently ignored.
//
// HooksScope on the runtime declares the intended destination:
//   "team"  → <team>/shared/hooks/<event>.sh       (v1.6 default for openclaw)
//   "agent" → <instance>/workspace/<HooksDir>/<event>.sh   (v1.5 legacy)
//   "both"  → write to both paths
// ---------------------------------------------------------------------------
func applyHooks(paths Paths, full string, hooks map[string]HookRef) error {
	rt, err := resolveRuntime(paths, full)
	if err != nil {
		rt = openclawRuntime()
	}
	if rt.HooksDir == "" {
		return errorf("runtime %q does not declare a HooksDir — hooks not supported", rt.Name)
	}
	ext := rt.HookFileExt
	if ext == "" {
		ext = ".sh"
	}
	allowed := map[string]bool{}
	for _, e := range rt.SupportedHookEvents {
		allowed[e] = true
	}
	scope := rt.HooksScope
	if scope == "" {
		scope = "team" // v1.6 default — matches openclaw runtime contract
	}

	team, _ := splitFull(full)
	var destDirs []string
	if scope == "team" || scope == "both" {
		// Team-shared dir mounted into the container as /shared-hooks
		destDirs = append(destDirs, filepath.Join(paths.Root, team, "shared", "hooks"))
	}
	if scope == "agent" || scope == "both" {
		destDirs = append(destDirs, filepath.Join(paths.Root, full, "workspace", rt.HooksDir))
	}
	for _, d := range destDirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	writeHook := func(name string, body []byte) error {
		var firstErr error
		for _, d := range destDirs {
			out := filepath.Join(d, name+ext)
			if existing, err := os.ReadFile(out); err == nil && string(existing) == string(body) {
				continue
			}
			if err := os.WriteFile(out, body, 0755); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		return firstErr
	}

	_ = writeHook // silence lint until the next loop uses it

	// We rebuild the per-event loop below to call writeHook.
	hooksDir := destDirs[0] // legacy compat for inline error messages
	_ = hooksDir // referenced only in legacy error paths

	for event, ref := range hooks {
		if strings.ContainsAny(event, "/\\.") {
			continue
		}
		if len(allowed) > 0 && !allowed[event] {
			fmt.Printf("    \033[0;33m! hook event %q not in runtime %q SupportedHookEvents — writing anyway, may be ignored at runtime\033[0m\n", event, rt.Name)
		}
		if ref.FromURL != "" && ref.Sha256 == "" {
			fmt.Printf("    \033[0;33m! hook %q: no sha256 declared for %s — fetch without integrity check\033[0m\n", event, ref.FromURL)
		}
		raw, src, err := resolveHookContent(ref)
		if err != nil {
			fmt.Printf("    \033[0;33m! hook %q (%s): %v\033[0m\n", event, src, err)
			continue
		}
		var body []byte
		if ref.Command != "" {
			body = []byte("#!/bin/sh\n# claws-managed hook for event: " + event + " (runtime: " + rt.Name + ", source: " + src + ", scope: " + scope + ")\n" + string(raw) + "\n")
		} else {
			body = raw
		}
		if err := writeHook(event, body); err != nil {
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

// ===========================================================================
// v1.5 — cron, events, sidecars, topology
// ===========================================================================

// validateCronSchedule accepts:
//   - 5-field crontab ("0 9 * * 1")
//   - @-aliases (@hourly, @daily, @weekly, @monthly, @yearly, @reboot)
//   - "every <duration>" (Go duration syntax, e.g. "every 30m")
func validateCronSchedule(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errorf("empty schedule")
	}
	if strings.HasPrefix(s, "@") {
		switch s {
		case "@hourly", "@daily", "@weekly", "@monthly", "@yearly", "@annually", "@reboot":
			return nil
		}
		return errorf("unknown @-alias %q", s)
	}
	if strings.HasPrefix(s, "every ") {
		dur := strings.TrimSpace(strings.TrimPrefix(s, "every "))
		if _, err := time.ParseDuration(dur); err != nil {
			return errorf("invalid duration after 'every': %v", err)
		}
		return nil
	}
	// Plain crontab: must have exactly 5 whitespace-separated fields.
	fields := strings.Fields(s)
	if len(fields) != 5 {
		return errorf("crontab must have 5 fields (got %d): %q", len(fields), s)
	}
	return nil
}

// applyCron dispatches to the right writer based on the runtime's CronFormat.
// v1.6: "claws-jobs.json" is the runtime's actual contract; "crontab" is the
// v1.5 legacy format kept for runtimes that expect it.
func applyCron(paths Paths, full string, ag ProfileAgent) error {
	rt, err := resolveRuntime(paths, full)
	if err != nil {
		rt = openclawRuntime()
	}
	if !rt.Capabilities.Cron {
		return errorf("runtime %q does not declare Cron capability", rt.Name)
	}
	switch rt.CronFormat {
	case "", "claws-jobs.json":
		return applyCronJobsJSON(paths, full, ag)
	case "crontab":
		return applyCronCrontab(paths, full, ag, rt)
	default:
		return errorf("runtime %q declares unknown CronFormat %q", rt.Name, rt.CronFormat)
	}
}

// applyCronCrontab — v1.5 legacy format. Kept for non-openclaw runtimes that
// expect crontab files in <instance>/workspace/<CronDir>/claws.crontab.
func applyCronCrontab(paths Paths, full string, ag ProfileAgent, rt Runtime) error {
	dir := rt.CronDir
	if dir == "" {
		dir = "cron"
	}
	cronDir := filepath.Join(paths.Root, full, "workspace", dir)
	if err := os.MkdirAll(cronDir, 0755); err != nil {
		return err
	}
	var lines []string
	lines = append(lines, "# claws-managed cron — agent: "+full)
	lines = append(lines, "# Edit by editing the template profile and re-running `claws apply`.")
	lines = append(lines, "")
	for _, cj := range ag.Cron {
		if cj.Enabled != nil && !*cj.Enabled {
			lines = append(lines, "# DISABLED: "+cj.Name)
			continue
		}
		var cmd string
		switch {
		case cj.Prompt != "":
			cmd = "echo " + strconv.Quote(cj.Prompt)
		case cj.Command != "":
			cmd = cj.Command
		case cj.Hook != "":
			if rt.HooksDir != "" {
				cmd = "sh /workspace/" + rt.HooksDir + "/" + cj.Hook + rt.HookFileExt
			} else {
				cmd = "# hook " + cj.Hook + " (runtime has no HooksDir)"
			}
		case len(cj.Exec) > 0:
			cmd = strings.Join(cj.Exec, " ")
		}
		tz := ""
		if cj.Timezone != "" {
			tz = "CRON_TZ=" + cj.Timezone + " "
		}
		lines = append(lines, "# job: "+cj.Name)
		lines = append(lines, tz+cj.Schedule+" "+cmd)
	}
	body := strings.Join(lines, "\n") + "\n"
	out := filepath.Join(cronDir, "claws.crontab")
	if existing, err := os.ReadFile(out); err == nil && string(existing) == body {
		return nil
	}
	return os.WriteFile(out, []byte(body), 0644)
}

// applyEventsConfig writes events.* config keys via cmdConfig set so the
// runtime can pick them up. Runtime decides whether to expose an actual
// HTTP endpoint based on its Capabilities.Events flag.
func applyEventsConfig(full string, ev *ProfileEvents) error {
	if ev == nil {
		return nil
	}
	set := func(k string, v interface{}) error {
		j, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return cmdConfig([]string{"set", full, k, string(j)})
	}
	if err := set("events.enabled", ev.Enabled); err != nil {
		return err
	}
	if err := set("events.digestMode", ev.DigestMode); err != nil {
		return err
	}
	if ev.Endpoint != "" {
		if err := set("events.endpoint", ev.Endpoint); err != nil {
			return err
		}
	}
	if len(ev.AllowFromIPs) > 0 {
		if err := set("events.allowFromIps", ev.AllowFromIPs); err != nil {
			return err
		}
	}
	return nil
}

// applySidecar writes a sidecar declaration to the agent's workspace.
// claws does NOT install or run the sidecar binary itself — that's the
// operator's job. We just write the integration config the sidecar can
// pick up at runtime, plus a hint file for the operator.
func applySidecar(paths Paths, full string, sc ProfileSidecar) error {
	if sc.Kind == "" {
		return errorf("sidecar missing kind")
	}
	// Validate kind against built-in registry.
	switch sc.Kind {
	case "sharedwatch", "intent-gateway", "custom":
		// ok
	default:
		return errorf("unknown sidecar kind %q (built-ins: sharedwatch, intent-gateway, custom)", sc.Kind)
	}
	sidecarDir := filepath.Join(paths.Root, full, "workspace", "sidecars")
	if err := os.MkdirAll(sidecarDir, 0755); err != nil {
		return err
	}
	name := sc.Name
	if name == "" {
		name = sc.Kind
	}
	out := filepath.Join(sidecarDir, name+".json")
	body := map[string]interface{}{
		"name":     name,
		"kind":     sc.Kind,
		"agent":    full,
		"config":   sc.Config,
		"_managed": "claws",
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if existing, err := os.ReadFile(out); err == nil && string(existing) == string(data) {
		return nil
	}
	return os.WriteFile(out, data, 0644)
}

// validateTopology checks manager chains for cycles and peer references for existence.
func validateTopology(agents []ProfileAgent) error {
	names := map[string]bool{}
	for _, a := range agents {
		names[a.Name] = true
	}
	// Manager refs must point to existing agents.
	for _, a := range agents {
		if a.Manager != "" && !names[a.Manager] {
			return errorf("agent %q: manager %q is not in the team", a.Name, a.Manager)
		}
		if a.Manager == a.Name {
			return errorf("agent %q: cannot be its own manager", a.Name)
		}
		for _, p := range a.Peers {
			if !names[p] {
				return errorf("agent %q: peer %q is not in the team", a.Name, p)
			}
			if p == a.Name {
				return errorf("agent %q: cannot be its own peer", a.Name)
			}
		}
	}
	// Cycle detection: walk manager chain from each agent, fail if we revisit.
	for _, a := range agents {
		seen := map[string]bool{a.Name: true}
		cur := a.Manager
		for cur != "" {
			if seen[cur] {
				return errorf("manager cycle detected involving agent %q", cur)
			}
			seen[cur] = true
			var next string
			for _, b := range agents {
				if b.Name == cur {
					next = b.Manager
					break
				}
			}
			cur = next
		}
	}
	return nil
}

// applyTopology writes <instance>/workspace/topology.json describing this
// agent's manager, peers, and workers (derived from who declares this
// agent as their manager).
func applyTopology(paths Paths, agents []ProfileAgent, full string) error {
	parts := strings.SplitN(full, "/", 2)
	if len(parts) != 2 {
		return nil
	}
	team, name := parts[0], parts[1]

	var self *ProfileAgent
	for i := range agents {
		if agents[i].Name == name {
			self = &agents[i]
			break
		}
	}
	if self == nil {
		return nil
	}
	var workers []string
	for _, a := range agents {
		if a.Manager == name {
			workers = append(workers, a.Name)
		}
	}
	topo := map[string]interface{}{
		"team":    team,
		"name":    name,
		"role":    self.Role,
		"manager": self.Manager,
		"peers":   self.Peers,
		"workers": workers,
	}
	data, err := json.MarshalIndent(topo, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	out := filepath.Join(paths.Root, full, "workspace", "topology.json")
	if err := os.MkdirAll(filepath.Dir(out), 0755); err != nil {
		return err
	}
	if existing, err := os.ReadFile(out); err == nil && string(existing) == string(data) {
		return nil
	}
	return os.WriteFile(out, data, 0644)
}
