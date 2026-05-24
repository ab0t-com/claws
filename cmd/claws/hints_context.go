package main

import (
	"os"
	"strings"

	"github.com/ab0t-com/claws/cmd/claws/hints"
)

// ---------------------------------------------------------------------------
// hints — Context population
// ---------------------------------------------------------------------------
//
// Lives next to the (main) cmd/claws package so it can read claws-internal
// state (registry, paths) without polluting the hints package — which by
// design has zero claws imports.
//
// Two populators:
//   - hintsCtxCheap(): registry-only. No docker calls. Sub-millisecond.
//     Used everywhere fleet-status isn't relevant (top-level, post-create,
//     post-auth, etc.).
//   - hintsCtxWithStatus(): same + per-agent status via the existing
//     status path. Cost: 1 `docker compose ps` per agent. Use when the
//     hints actually need live status (cmdList already pays this cost).
//
// Profile resolution honors CLAWS_HINTS=off (or =terse, =agent) so an
// operator who finds the Next: blocks noisy can silence them.

func hintsResolveProfile() hints.Profile {
	return hints.ResolveProfile(
		"", // no global --hints flag yet; per-cmd flag could populate this later
		os.Getenv("CLAWS_HINTS"),
		false, // JSON detection lives at the caller
	)
}

// hintsCtxCheap returns a Context populated only from the port registry
// — no Docker calls, no docker compose ps. Safe to call from any handler.
func hintsCtxCheap(paths Paths) hints.Context {
	ctx := hints.Context{Profile: hintsResolveProfile()}
	entries, err := readRegistry(paths)
	if err != nil {
		return ctx
	}
	ctx.AgentTotal = len(entries)
	teamSet := map[string]struct{}{}
	for _, e := range entries {
		ctx.Agents = append(ctx.Agents, hints.AgentRef{Name: e.Name})
		if i := strings.IndexByte(e.Name, '/'); i > 0 {
			teamSet[e.Name[:i]] = struct{}{}
		}
	}
	for t := range teamSet {
		ctx.ExistingTeams = append(ctx.ExistingTeams, t)
	}
	return ctx
}

// hintsAttachStatus enriches ctx.Agents with status values and updates
// the aggregate counts (AgentHealthy / AgentNeverStarted / AgentStopped /
// AgentError). Caller supplies the statuses (typically computed during
// the command's existing render path).
func hintsAttachStatus(ctx *hints.Context, statuses map[string]string) {
	if ctx == nil || len(statuses) == 0 {
		return
	}
	ctx.AgentHealthy = 0
	ctx.AgentNeverStarted = 0
	ctx.AgentStopped = 0
	ctx.AgentError = 0
	for i := range ctx.Agents {
		s := statuses[ctx.Agents[i].Name]
		ctx.Agents[i].Status = s
		switch s {
		case "healthy":
			ctx.AgentHealthy++
		case "created", "":
			ctx.AgentNeverStarted++
		case "stopped":
			ctx.AgentStopped++
		default:
			// Anything else (unhealthy / exited / error etc.)
			ctx.AgentError++
		}
	}
}

// hintsRender writes the Next: block to stdout, honoring the resolved
// profile (no-op when profile is off or there are no hints). Centralised
// so handlers don't import the hints package directly for output.
func hintsRender(command string, ctx hints.Context) {
	hints.RenderText(os.Stdout, hints.For(command, ctx))
}
