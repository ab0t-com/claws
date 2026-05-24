package main

import (
	"fmt"
	"os"
	"strings"
)

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

	team, agent := "default", "agent-1"
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
	if err := validateName(agent); err != nil {
		return errorf("invalid agent name: %v", err)
	}
	full := team + "/" + agent

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
	groupDir := paths.Root + "/" + team
	if _, err := os.Stat(groupDir); err != nil {
		// cmdGroup expects "create <name>"
		if err := cmdGroup([]string{"create", team}); err != nil {
			return errorf("group create %s failed: %v", team, err)
		}
	}

	fmt.Printf("  %s[4/4] agent             %s ", bold, nc)
	if instanceExists(paths, full) {
		fmt.Printf("%s✓ %s already exists (skipping)%s\n", dim, full, nc)
	} else {
		if err := cmdCreate([]string{full}); err != nil {
			return errorf("create failed: %v", err)
		}
		fmt.Printf("  %s✓ %s created%s\n", green, full, nc)
	}

	fmt.Println()
	fmt.Printf("%sYour first agent is ready.%s Two things to do next:\n\n", bold, nc)
	fmt.Println("  1. Authenticate (pick one):")
	fmt.Printf("     claws auth %s codex                # OpenAI Codex OAuth (opens browser)\n", full)
	fmt.Printf("     claws auth %s apikey openai sk-…   # or API key\n\n", full)
	fmt.Println("  2. Connect a channel (pick one):")
	fmt.Printf("     claws channel add %s telegram --token=<bot-token>\n", full)
	fmt.Printf("     claws channel add %s discord  --token=<bot-token>\n", full)
	fmt.Printf("     claws channel add %s slack    --bot-token=<t> --app-token=<t>\n\n", full)
	fmt.Printf("  Then: claws start %s\n\n", full)
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
