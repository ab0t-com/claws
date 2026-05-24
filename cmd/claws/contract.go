package main

import (
	"fmt"
)

// cmdContract prints the runtime adapter's declared contract — paths, formats,
// capabilities. Useful to verify "this runtime supports X" before writing a
// template that uses X.
func cmdContract(args []string) error {
	if len(args) < 1 || args[0] == "-h" || args[0] == "--help" {
		fmt.Println(`Usage: claws contract show [<runtime>]
       claws contract list

Print the declared contract for a runtime adapter — where claws writes
hooks/skills/cron/sidecars, which lifecycle hook events the runtime
recognises, what capabilities it supports. Useful before writing a
template that depends on a specific feature.

Subcommands:
  show <name>   Print the named runtime's contract (default: openclaw)
  list          List all registered runtimes`)
		return nil
	}
	paths := resolvePaths()
	switch args[0] {
	case "show":
		name := "openclaw"
		if len(args) >= 2 {
			name = args[1]
		}
		rt, ok := getRuntimeByName(paths, name)
		if !ok {
			return errorf("runtime %q not registered (claws contract list)", name)
		}
		return printContract(rt)
	case "list":
		all := listRuntimes(paths)
		const (
			bold = "\033[1m"
			nc   = "\033[0m"
		)
		fmt.Printf("%s%-16s %-12s %s%s\n", bold, "NAME", "VERSION", "DESCRIPTION", nc)
		for name, rt := range all {
			fmt.Printf("%-16s %-12s %s\n", name, rt.Version, rt.Description)
		}
		return nil
	default:
		return errorf("unknown contract subcommand %q (use show, list)", args[0])
	}
}

func printContract(rt Runtime) error {
	const (
		bold  = "\033[1m"
		dim   = "\033[0;90m"
		green = "\033[0;32m"
		gold  = "\033[0;33m"
		nc    = "\033[0m"
	)
	mark := func(b bool) string {
		if b {
			return green + "✓" + nc
		}
		return dim + "—" + nc
	}

	fmt.Printf("%s%s runtime%s — %s\n", bold, rt.Name, nc, rt.Description)
	if rt.Version != "" {
		fmt.Printf("  %sversion%s:    %s\n", dim, nc, rt.Version)
	}
	fmt.Printf("  %sdefault image%s: %s\n", dim, nc, rt.DefaultImage)
	fmt.Println()

	fmt.Printf("%sCapabilities%s\n", bold, nc)
	fmt.Printf("  %s channels   — messaging integration\n", mark(rt.Capabilities.Channels))
	fmt.Printf("  %s pairing    — DM pairing / approval flow\n", mark(rt.Capabilities.Pairing))
	fmt.Printf("  %s auth       — Codex OAuth + API keys\n", mark(rt.Capabilities.Auth))
	fmt.Printf("  %s config     — JSON config merging\n", mark(rt.Capabilities.Config))
	fmt.Printf("  %s tasks      — manager/worker task queue\n", mark(rt.Capabilities.Tasks))
	fmt.Printf("  %s shared     — team-shared workspace/skills/hooks\n", mark(rt.Capabilities.Shared))
	fmt.Printf("  %s bridge     — bridge port for sidecar protocol\n", mark(rt.Capabilities.Bridge))
	fmt.Printf("  %s cron       — periodic jobs\n", mark(rt.Capabilities.Cron))
	fmt.Printf("  %s events     — external event injection\n", mark(rt.Capabilities.Events))
	fmt.Printf("  %s sidecars   — first-class helper integration\n", mark(rt.Capabilities.Sidecars))
	fmt.Println()

	fmt.Printf("%sHook contract%s\n", bold, nc)
	if rt.HooksDir == "" {
		fmt.Printf("  %s (not supported)%s\n", dim, nc)
	} else {
		scope := rt.HooksScope
		if scope == "" {
			scope = "team"
		}
		fmt.Printf("  scope:        %s\n", scope)
		fmt.Printf("  dir:          %s\n", rt.HooksDir)
		fmt.Printf("  file ext:     %s\n", rt.HookFileExt)
		fmt.Printf("  events:       %v\n", rt.SupportedHookEvents)
	}
	fmt.Println()

	fmt.Printf("%sCron contract%s\n", bold, nc)
	if !rt.Capabilities.Cron {
		fmt.Printf("  %s (not supported)%s\n", dim, nc)
	} else {
		fmt.Printf("  format:       %s\n", rt.CronFormat)
		fmt.Printf("  dir:          %s\n", rt.CronDir)
		fmt.Printf("  payload kind: systemEvent (runtime sends a prompt to the agent on each fire)\n")
	}
	fmt.Println()

	fmt.Printf("%sSkills contract%s\n", bold, nc)
	skillsScope := rt.SkillsScope
	if skillsScope == "" {
		skillsScope = "team"
	}
	fmt.Printf("  scope:        %s\n", skillsScope)
	fmt.Println()

	fmt.Printf("%sMount points (host → container)%s\n", bold, nc)
	mounts := []struct{ k, v string }{
		{"workspace home", rt.ContainerHome},
		{"config dir",     rt.ContainerConfigDir},
		{"workspace",      rt.ContainerWorkspace},
		{"skills",         rt.MountSkills},
		{"shared ws",      rt.MountWorkspace},
		{"shared hooks",   rt.MountHooks},
		{"tasks",          rt.MountTasks},
		{"output",         rt.MountOutput},
		{"manager",        rt.MountManager},
	}
	for _, m := range mounts {
		if m.v == "" {
			continue
		}
		fmt.Printf("  %-14s %s\n", m.k+":", m.v)
	}
	if rt.GatewayService != "" {
		fmt.Println()
		fmt.Printf("%sDocker compose%s\n", bold, nc)
		fmt.Printf("  gateway service: %s\n", rt.GatewayService)
		if rt.CLIService != "" {
			fmt.Printf("  CLI service:     %s\n", rt.CLIService)
		}
		fmt.Printf("  internal port:   %d\n", rt.InternalPort)
		if rt.BridgePort > 0 {
			fmt.Printf("  bridge port:     +%d\n", rt.BridgePort)
		}
	}
	fmt.Println()

	// Hint: warn on declared-but-uncertain features.
	if rt.Capabilities.Events {
		fmt.Printf("%s! Events capability declared, but endpoint contract is unverified.%s\n", gold, nc)
		fmt.Printf("  See docs/runtimes.md or `claws agent show <name>` to check what got configured.\n\n")
	}
	return nil
}
