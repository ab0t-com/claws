package main

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
)

// personalAssistantNames is a curated set of short, gender-neutral,
// easy-to-type names. Used as the default agent name in `claws quickstart`
// when no explicit name is given. Anyone on a fresh install gets a
// "personal assistant" by default rather than `agent-1`.
var personalAssistantNames = []string{
	"ada", "ari", "ava", "avery", "bo", "charlie", "ellis",
	"finn", "grace", "jamie", "jules", "kit", "lex", "max",
	"milo", "nia", "nova", "pax", "piper", "quinn", "river",
	"sage", "sky", "tess", "val", "wren", "zane", "zoe",
}

func pickAssistantName() string {
	return personalAssistantNames[rand.IntN(len(personalAssistantNames))]
}

// existingAgentsInGroup returns the agent names registered under a group.
// Used for quickstart idempotence: if the team already has agents, we
// reuse the first one for next-step hints rather than creating another.
func existingAgentsInGroup(paths Paths, team string) []string {
	entries, err := readRegistry(paths)
	if err != nil {
		return nil
	}
	prefix := team + "/"
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name, prefix) {
			names = append(names, strings.TrimPrefix(e.Name, prefix))
		}
	}
	return names
}

const quickstartHelp = `Usage: claws quickstart [team] [agent]

One-click first agent. Idempotent — safe to re-run.

Runs: init → policy init → access init → group create → agent create
All steps are skip-if-exists. Auth and channels are NOT run automatically
(they need data only you have: a browser for OAuth, a bot token for the
channel) — printed as next-step commands instead.

Defaults:
  team:  default
  agent: agent-1

Examples:
  claws quickstart                       # default/agent-1
  claws quickstart research              # research/agent-1
  claws quickstart research sarah        # research/sarah
`

func cmdQuickstart(args []string) error {
	for _, a := range args {
		if a == "-h" || a == "--help" {
			fmt.Print(quickstartHelp)
			return nil
		}
	}

	team := "default"
	agent := "" // resolved below
	pos := []string{}
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			pos = append(pos, a)
		}
	}
	if len(pos) >= 1 {
		team = pos[0]
	}
	if len(pos) >= 2 {
		agent = pos[1]
	}
	if err := validateName(team); err != nil {
		return errorf("invalid team name: %v", err)
	}
	if agent != "" {
		if err := validateName(agent); err != nil {
			return errorf("invalid agent name: %v", err)
		}
	}
	// agent name + idempotence is resolved at step 4 below, after we know
	// whether the team already has members.

	const (
		bold  = "\033[1m"
		green = "\033[0;32m"
		dim   = "\033[0;90m"
		nc    = "\033[0m"
	)

	fmt.Printf("%s==> claws quickstart — one-click first agent%s\n\n", bold, nc)
	paths := resolvePaths()

	// 1/4 — init
	fmt.Printf("  %s[1/4] init              %s ", bold, nc)
	if isInitialized(paths) {
		fmt.Printf("%s✓ already initialized (skipping)%s\n", dim, nc)
	} else {
		fmt.Println()
		if err := cmdInit(nil); err != nil {
			return errorf("init failed: %v", err)
		}
	}

	// 2/4 — policy
	fmt.Printf("  %s[2/4] policy            %s ", bold, nc)
	if _, err := os.Stat(paths.Root + "/policy.json"); err == nil {
		fmt.Printf("%s✓ already configured (skipping)%s\n", dim, nc)
	} else {
		if err := cmdPolicy([]string{"init"}); err != nil {
			fmt.Printf("%s! policy init failed: %v (continuing)%s\n", dim, err, nc)
		} else {
			fmt.Printf("%s✓ created (secure defaults)%s\n", green, nc)
		}
	}

	// 3/4 — access
	fmt.Printf("  %s[3/4] access            %s ", bold, nc)
	if _, err := os.Stat(paths.Root + "/.access.json"); err == nil {
		fmt.Printf("%s✓ already configured (skipping)%s\n", dim, nc)
	} else {
		if err := cmdAccess([]string{"init"}); err != nil {
			fmt.Printf("%s! access init failed: %v (continuing)%s\n", dim, err, nc)
		} else {
			fmt.Printf("%s✓ created (you are admin)%s\n", green, nc)
		}
	}

	// 4/4 — group + agent
	groupDir := filepath.Join(paths.Root, team)
	if _, err := os.Stat(groupDir); err != nil {
		if err := cmdGroup([]string{"create", team}); err != nil {
			return errorf("group create %s failed: %v", team, err)
		}
	}

	// Idempotence: if the team already has agents (any), use the first one
	// for the next-step hints instead of creating a new one. Means re-running
	// `claws quickstart` is always a no-op rather than spawning a new random
	// agent each time.
	existing := existingAgentsInGroup(paths, team)
	var full string
	fmt.Printf("  %s[4/4] agent             %s ", bold, nc)
	switch {
	case agent != "" && instanceExists(paths, team+"/"+agent):
		full = team + "/" + agent
		fmt.Printf("%s✓ %s already exists (skipping)%s\n", dim, full, nc)
	case agent == "" && len(existing) > 0:
		// Reuse the first existing agent in the team.
		full = team + "/" + existing[0]
		fmt.Printf("%s✓ %s already exists (skipping)%s\n", dim, full, nc)
		if len(existing) > 1 {
			fmt.Printf("  %s  (team also has: %s)%s\n", dim, strings.Join(existing[1:], ", "), nc)
		}
	default:
		if agent == "" {
			agent = pickAssistantName()
		}
		full = team + "/" + agent
		fmt.Printf("%s→ creating personal assistant: %s%s\n", green, full, nc)
		if err := cmdCreate([]string{full}); err != nil {
			return errorf("create failed: %v", err)
		}
		fmt.Printf("  %s✓ %s created%s\n", green, full, nc)
	}

	// Environment check — surface any docker/image/disk issues before the
	// user goes hunting for bot tokens.
	fmt.Println()
	fmt.Printf("%s==> Environment check%s\n", bold, nc)
	doctorErr := cmdDoctor(nil)
	if doctorErr != nil {
		fmt.Printf("\n  %s⚠ Some environment checks failed — fix them above before proceeding.%s\n", "\033[0;33m", nc)
	}

	fmt.Println()
	fmt.Printf("%sYour first agent is ready.%s Three things to do next:\n\n", bold, nc)

	fmt.Println("  1. Authenticate (pick one):")
	fmt.Printf("     claws auth %s codex                # OpenAI Codex OAuth (opens browser)\n", full)
	fmt.Printf("     claws auth %s apikey openai sk-…   # or your API key\n", full)
	fmt.Printf("     claws auth %s apikey anthropic sk-ant-…\n\n", full)

	fmt.Println("  2. Connect a channel (pick one):")
	fmt.Printf("     claws channel add %s telegram --token=<bot-token>\n", full)
	fmt.Printf("     claws channel add %s discord  --token=<bot-token>\n", full)
	fmt.Printf("     claws channel add %s slack    --bot-token=<t> --app-token=<t>\n\n", full)

	fmt.Println("  3. Verify + start:")
	fmt.Printf("     claws audit                          # security audit (after auth + channel)\n")
	fmt.Printf("     claws start %s\n\n", full)

	// Token sources — close the "where do I get these?" gap.
	fmt.Printf("  %sNeed a bot token?%s\n", bold, nc)
	fmt.Println("    Telegram:  t.me/BotFather             → /newbot → copy token         (~2 min)")
	fmt.Println("    Discord:   discord.com/developers/applications → New Application → Bot  (~5 min)")
	fmt.Println("    Slack:     api.slack.com/apps         → Create New App → OAuth & Permissions  (~10 min)")
	fmt.Println("    WhatsApp:  no token needed (QR scan via `claws channel add … whatsapp`)")
	tokenGuide := ""
	if home, _ := os.UserHomeDir(); home != "" {
		tokenGuide = filepath.Join(home, ".local", "share", "claws", "html", "channels-guide.html")
		if _, err := os.Stat(tokenGuide); err == nil {
			fmt.Printf("    Full guide: %s\n", tokenGuide)
		}
	}
	fmt.Println()

	fmt.Printf("  %sRe-run safe.%s `claws quickstart` again is a no-op.\n\n", dim, nc)
	return nil
}

// instanceExists checks the port registry for an instance by full name (group/agent).
func instanceExists(paths Paths, fullName string) bool {
	entries, err := readRegistry(paths)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.Name == fullName {
			return true
		}
	}
	return false
}

// isInitialized reports whether `claws init` has run.
func isInitialized(paths Paths) bool {
	if _, err := os.Stat(paths.PortRegistry); err != nil {
		return false
	}
	if _, err := os.Stat(paths.ComposeTemplate); err != nil {
		return false
	}
	return true
}
