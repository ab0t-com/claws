package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	Name     string             `json:"name"`
	Role     string             `json:"role,omitempty"`
	Manager  string             `json:"manager,omitempty"`
	Auth     *ProfileAuth       `json:"auth,omitempty"`
	Channels []ProfileChannel   `json:"channels,omitempty"`
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
	var file string
	var yes, dryRun bool
	for _, a := range args {
		switch {
		case a == "-h" || a == "--help":
			fmt.Print(applyHelp)
			return nil
		case strings.HasPrefix(a, "--file="):
			file = strings.TrimPrefix(a, "--file=")
		case a == "--yes" || a == "-y":
			yes = true
		case a == "--dry-run":
			dryRun = true
		}
	}
	if file == "" {
		return errorf("--file=<profile.json> required")
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
			if err := cmdCreate(createArgs); err != nil {
				return errorf("create %s: %v", full, err)
			}
			fmt.Printf("  %s✓ created%s\n", green, nc)
		}

		// channels — only attempt if token resolves
		for _, ch := range ag.Channels {
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
			if err := cmdChannel(chArgs); err != nil {
				fmt.Printf("  %s! channel %s add failed: %v%s\n", gold, ch.Type, err, nc)
				continue
			}
			fmt.Printf("  %s✓ channel %s connected%s\n", green, ch.Type, nc)
		}

		// auth — apikey we can do non-interactively if it resolves
		if ag.Auth != nil && ag.Auth.FallbackAPIKey != nil {
			ref := &SecretRef{Env: ag.Auth.FallbackAPIKey.FromEnv, File: ag.Auth.FallbackAPIKey.FromFile}
			key := ref.resolve()
			if key != "" {
				if err := cmdAuth([]string{full, "apikey", ag.Auth.FallbackAPIKey.Provider, key}); err != nil {
					fmt.Printf("  %s! auth apikey %s failed: %v%s\n", gold, ag.Auth.FallbackAPIKey.Provider, err, nc)
				} else {
					fmt.Printf("  %s✓ auth (apikey %s) configured%s\n", green, ag.Auth.FallbackAPIKey.Provider, nc)
				}
			} else if ag.Auth.Preferred == "codex" {
				fmt.Printf("  %s→ next: claws auth %s codex   (OAuth, opens browser)%s\n", dim, full, nc)
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

	fmt.Printf("\n%s✓ apply complete%s — %d agent(s)\n\n", green, nc, len(p.Agents))
	return nil
}
