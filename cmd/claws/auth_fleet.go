package main

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// claws auth fleet <method> [...] — run auth across every (or every-missing)
// agent in sequence, with a 3-second countdown so a typo isn't irreversible.
//
// Sibling of `claws auth <name> <method>`. Solves the operational hump where
// an operator has to type the same auth command N times for N agents on the
// same host. Each agent gets its own independent OAuth grant → no shared
// refresh chain → no refresh_token_reused collisions.
//
// Designed against the "simple" branch of the credential-management story
// (see tickets/auth-fleet-helpers-2026-05-24/). The broker daemon is the
// long-term answer; this is the everyday answer for one operator with
// a handful of agents.
// ---------------------------------------------------------------------------

func cmdAuthFleet(args []string) error {
	if len(args) < 1 {
		return errorf(`usage: claws auth fleet <method> [args...] [--group=<team>] [--missing-only]

  Runs 'claws auth <name> <method> [args...]' for every agent.

  Methods:
    codex                                Codex OAuth flow per agent
    apikey <provider> <key>              same API key applied to every agent
                                         (provider: anthropic|openai|openrouter)

  Flags:
    --group=<team>      limit to one team (e.g. --group=team)
    --missing-only      skip agents that already verify cleanly
    --yes               skip the 3-second countdown

Examples:
  claws auth fleet codex                       # all agents, run OAuth per agent
  claws auth fleet codex --missing-only        # only agents that need it
  claws auth fleet apikey openai sk-…          # set same OpenAI key fleet-wide
  claws auth fleet codex --group=team --yes    # team agents, no countdown`)
	}

	paths := resolvePaths()
	filterGroup := flagValue(args, "--group=")
	missingOnly := hasFlag(args, "--missing-only")
	skipCountdown := hasFlag(args, "--yes")
	if err := requireGroup(paths, filterGroup); err != nil {
		return err
	}

	// Strip our own flags before re-passing args to cmdAuth.
	passThrough := make([]string, 0, len(args))
	for _, a := range args {
		switch {
		case a == "--missing-only", a == "--yes":
			continue
		case strings.HasPrefix(a, "--group="):
			continue
		default:
			passThrough = append(passThrough, a)
		}
	}
	method := passThrough[0]

	entries, err := readRegistry(paths)
	if err != nil {
		return err
	}
	entries = filterEntriesByGroup(entries, filterGroup)
	if len(entries) == 0 {
		return errorf("no agents to operate on")
	}

	// If --missing-only, drop agents that already verify cleanly. Saves
	// time AND avoids re-running the OAuth flow against an already-good
	// grant (which itself burns the refresh chain).
	if missingOnly {
		kept := entries[:0]
		for _, e := range entries {
			v := verifyOneInstance(paths, e.Name)
			if !v.Verified {
				kept = append(kept, e)
			}
		}
		entries = kept
		if len(entries) == 0 {
			info("All agents already have verified auth — nothing to do.")
			return nil
		}
	}

	scope := "all agents"
	if filterGroup != "" {
		scope = "team '" + filterGroup + "'"
	}
	if missingOnly {
		scope += " (missing only)"
	}
	fmt.Printf("This will run 'claws auth <name> %s' for %d agent(s) in %s:\n",
		method, len(entries), scope)
	for _, e := range entries {
		fmt.Printf("  • %s\n", e.Name)
	}

	if !skipCountdown {
		fmt.Printf("\n  Starting in 3 seconds — Ctrl-C to cancel.\n")
		countdown(3)
	}

	var failed []string
	for _, e := range entries {
		fmt.Printf("\n\033[1m==> claws auth %s %s\033[0m\n", e.Name, strings.Join(passThrough, " "))
		perAgentArgs := append([]string{e.Name}, passThrough...)
		if err := cmdAuth(perAgentArgs); err != nil {
			warn(fmt.Sprintf("auth failed for '%s': %v", e.Name, err))
			failed = append(failed, e.Name)
			// Keep going — one agent's failure shouldn't block the rest.
		}
	}

	fmt.Println()
	if len(failed) == 0 {
		info(fmt.Sprintf("All %d agent(s) authed successfully.", len(entries)))
		return nil
	}
	warn(fmt.Sprintf("%d of %d agent(s) failed: %s", len(failed), len(entries), strings.Join(failed, ", ")))
	return errorf("auth fleet completed with %d failures", len(failed))
}
