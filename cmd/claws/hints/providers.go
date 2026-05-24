package hints

import "fmt"

// init wires the built-in providers. Adding hints for a new command is
// one Register call here + a new func — handlers don't need to know about
// other providers.
func init() {
	Register("", providerToplevel)         // `claws` with no args
	Register("list", providerList)
	Register("setup", providerSetup)
	Register("create", providerCreate)
	Register("start", providerStart)
	Register("stop", providerStop)
	Register("remove", providerRemove)
	Register("auth", providerAuth)
	Register("channel add", providerChannelAdd)
	Register("update", providerUpdate)
	Register("image bootstrap", providerImageBootstrap)
	Register("agent ping", providerAgentPing)
}

// providerToplevel — `claws` with no args. Lead with the most likely
// action given fleet state.
func providerToplevel(ctx Context) []Hint {
	if ctx.AgentTotal == 0 {
		return []Hint{
			{Name: "guided_setup", Command: "claws setup",
				Reason: "guided wizard — create your first team + agent in 6 steps"},
			{Name: "see_help", Command: "claws help",
				Reason: "browse every command"},
		}
	}
	var out []Hint
	if ctx.AgentNeverStarted > 0 {
		out = append(out, Hint{
			Name:    "start_all",
			Command: "claws start-all",
			Reason:  fmt.Sprintf("%d agent(s) never started — bring the whole fleet up", ctx.AgentNeverStarted),
		})
	}
	if ctx.AgentHealthy > 0 {
		out = append(out, Hint{
			Name:    "live_status",
			Command: "claws dashboard",
			Reason:  fmt.Sprintf("%d agent(s) healthy — open the live status view", ctx.AgentHealthy),
		})
	}
	out = append(out, Hint{
		Name:    "list_agents",
		Command: "claws list",
		Reason:  "see every agent with status and a per-row next step",
	})
	if ctx.AgentError > 0 {
		out = append(out, Hint{
			Name:    "triage_errors",
			Command: "claws errors",
			Reason:  fmt.Sprintf("%d agent(s) in error state — show recent failures", ctx.AgentError),
		})
	}
	if ctx.NewerExists {
		out = append(out, Hint{
			Name:    "update_cli",
			Command: "sudo claws update",
			Reason:  fmt.Sprintf("a newer release (%s) is available", ctx.Latest),
		})
	}
	return out
}

// providerList — after printing the table, suggest aggregated next steps.
// Per-row next-step is rendered inline by cmdList's NEXT column, so this
// provider focuses on the fleet-level moves.
func providerList(ctx Context) []Hint {
	var out []Hint
	if ctx.AgentTotal == 0 {
		return []Hint{{
			Name: "create_first", Command: "claws setup",
			Reason: "no agents yet — start the wizard",
		}}
	}
	if ctx.AgentNeverStarted > 0 {
		out = append(out, Hint{
			Name:    "start_all",
			Command: "claws start-all",
			Reason:  fmt.Sprintf("%d agent(s) never started", ctx.AgentNeverStarted),
		})
	}
	// Suggest pinging one healthy agent specifically (concrete > generic).
	for _, a := range ctx.Agents {
		if a.Status == "healthy" {
			out = append(out, Hint{
				Name:    "ping_one_healthy",
				Command: fmt.Sprintf("claws agent ping %s", a.Name),
				Reason:  fmt.Sprintf("verify %s end-to-end (gateway + auth + channel)", a.Name),
			})
			break
		}
	}
	if ctx.AgentHealthy > 0 {
		out = append(out, Hint{
			Name: "tail_logs", Command: "claws fleet logs -f",
			Reason: "tail every running agent's logs in one stream",
		})
	}
	if ctx.AgentError > 0 {
		out = append(out, Hint{
			Name: "triage", Command: "claws errors",
			Reason: fmt.Sprintf("%d agent(s) in error state — show recent failures", ctx.AgentError),
		})
	}
	return out
}

// providerSetup — wizard step suggestions. Triggered from inside cmdSetup
// at decision points, not from the top of the wizard.
func providerSetup(ctx Context) []Hint {
	// Surface existing teams + agents so a user re-running `claws setup`
	// can jump to an existing one.
	var out []Hint
	if len(ctx.ExistingTeams) > 0 {
		out = append(out, Hint{
			Name:    "use_existing_team",
			Command: "claws list",
			Reason:  fmt.Sprintf("%d team(s) already exist on this host — review before adding more", len(ctx.ExistingTeams)),
		})
	}
	if ctx.AgentTotal > 0 {
		out = append(out, Hint{
			Name: "see_existing_agents", Command: "claws list",
			Reason: fmt.Sprintf("%d agent(s) already on this host", ctx.AgentTotal),
		})
	}
	return out
}

// providerCreate — after a successful `claws create <name>`, the agent
// needs auth + channel before it can do anything.
func providerCreate(ctx Context) []Hint {
	name := ctx.CreatedNameOrAgentName()
	if name == "" {
		return nil
	}
	return []Hint{
		{
			Name:    "configure_auth",
			Command: fmt.Sprintf("claws auth %s codex", name),
			Reason:  "agent needs an auth method before it can run a model",
		},
		{
			Name:    "add_channel",
			Command: fmt.Sprintf("claws channel add %s telegram", name),
			Reason:  "give the agent a way to receive messages",
		},
		{
			Name:    "start_agent",
			Command: fmt.Sprintf("claws start %s", name),
			Reason:  "bring the container up",
		},
	}
}

// providerStart — after `claws start <name>` succeeds, suggest verifying.
func providerStart(ctx Context) []Hint {
	if ctx.AgentName == "" {
		return nil
	}
	return []Hint{
		{
			Name:    "ping",
			Command: fmt.Sprintf("claws agent ping %s", ctx.AgentName),
			Reason:  "verify gateway + auth + channel end-to-end",
		},
		{
			Name:    "tail_logs",
			Command: fmt.Sprintf("claws logs %s -f", ctx.AgentName),
			Reason:  "watch the agent boot and respond",
		},
	}
}

// providerStop — after stop, suggest the obvious follow-ups.
func providerStop(ctx Context) []Hint {
	if ctx.AgentName == "" {
		return nil
	}
	return []Hint{
		{
			Name:    "restart",
			Command: fmt.Sprintf("claws start %s", ctx.AgentName),
			Reason:  "bring it back up",
		},
		{
			Name:    "remove",
			Command: fmt.Sprintf("claws remove %s --purge", ctx.AgentName),
			Reason:  "if you're done with this agent, delete it",
		},
	}
}

// providerRemove — after remove, see what's left.
func providerRemove(ctx Context) []Hint {
	return []Hint{{
		Name: "see_remaining", Command: "claws list",
		Reason: "see what agents are still on this host",
	}}
}

// providerAuth — after auth, suggest channel setup + ping to verify.
func providerAuth(ctx Context) []Hint {
	if ctx.AgentName == "" {
		return nil
	}
	out := []Hint{{
		Name:    "verify",
		Command: fmt.Sprintf("claws agent ping %s", ctx.AgentName),
		Reason:  "confirm auth actually works",
	}}
	if !ctx.AgentHasChan {
		out = append(out, Hint{
			Name:    "add_channel",
			Command: fmt.Sprintf("claws channel add %s telegram", ctx.AgentName),
			Reason:  "give the agent a way to receive messages",
		})
	}
	return out
}

// providerChannelAdd — after channel add, suggest start + DM.
func providerChannelAdd(ctx Context) []Hint {
	if ctx.AgentName == "" {
		return nil
	}
	return []Hint{
		{
			Name:    "start_agent",
			Command: fmt.Sprintf("claws start %s", ctx.AgentName),
			Reason:  "bring the container up so the channel can connect",
		},
		{
			Name:    "ping",
			Command: fmt.Sprintf("claws agent ping %s", ctx.AgentName),
			Reason:  "verify the channel connection",
		},
	}
}

// providerUpdate — after a successful update, run fleet diagnostics.
func providerUpdate(ctx Context) []Hint {
	out := []Hint{{
		Name: "version", Command: "claws version",
		Reason: "confirm the new build",
	}}
	if ctx.AgentTotal > 0 {
		out = append(out, Hint{
			Name: "fleet_doctor", Command: "claws fleet doctor",
			Reason: "verify all agents still operate cleanly under the new binary",
		})
	}
	return out
}

// providerImageBootstrap — image is built; the obvious next step is
// `claws setup` (which would have been blocked).
func providerImageBootstrap(ctx Context) []Hint {
	if ctx.AgentTotal == 0 {
		return []Hint{{
			Name: "run_setup", Command: "claws setup",
			Reason: "image is ready; run the wizard for your first agent",
		}}
	}
	return []Hint{{
		Name: "start_all", Command: "claws start-all",
		Reason: "image is ready; bring the fleet up",
	}}
}

// providerAgentPing — `claws agent ping` outcome → suggested follow-up.
func providerAgentPing(ctx Context) []Hint {
	if ctx.AgentName == "" {
		return nil
	}
	// If the ping just succeeded, the most useful next thing is logs.
	if ctx.AgentStatus == "healthy" {
		return []Hint{{
			Name:    "tail_logs",
			Command: fmt.Sprintf("claws logs %s -f", ctx.AgentName),
			Reason:  "watch the agent in action",
		}}
	}
	// If the agent isn't healthy, suggest logs (to see why) and auth verify.
	return []Hint{
		{
			Name:    "see_logs",
			Command: fmt.Sprintf("claws logs %s", ctx.AgentName),
			Reason:  "ping failed — show recent log lines",
		},
		{
			Name:    "verify_auth",
			Command: fmt.Sprintf("claws auth verify %s", ctx.AgentName),
			Reason:  "check the credential separately",
		},
	}
}

// CreatedNameOrAgentName centralises the AgentName / CreatedName check.
// Some callers populate AgentName (operations on an existing agent),
// others CreatedName (the agent just created). Providers should not care
// which one was used.
func (c Context) CreatedNameOrAgentName() string {
	if c.CreatedName != "" {
		return c.CreatedName
	}
	return c.AgentName
}
