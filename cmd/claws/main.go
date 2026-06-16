package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printFirstRun()
		os.Exit(0)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Per-subcommand help: claws <cmd> --help
	if printSubcommandHelp(cmd, args) {
		os.Exit(0)
	}

	// Access control enforcement
	paths := resolvePaths()
	if err := enforceAccess(paths, cmd, args); err != nil {
		fmt.Fprintf(os.Stderr, "\033[0;31m==> %s\033[0m\n", err)
		os.Exit(1)
	}

	// Prereq guard: commands that need docker get a friendly error if
	// docker is missing or the daemon isn't reachable, instead of the
	// opaque exec.LookPath failure that a non-technical user can't act on.
	// Commands in commandsNotNeedingDocker (version, help, update, doctor,
	// init, paste-secret, no-args) skip this check.
	if err := validatePrereqsForCommand(cmd); err != nil {
		fmt.Fprintf(os.Stderr, "\033[0;31m==> %s\033[0m\n", err)
		os.Exit(1)
	}

	var err error
	switch cmd {
	case "create":
		err = cmdCreate(args)
	case "start":
		err = cmdStart(args)
	case "stop":
		err = cmdStop(args)
	case "restart":
		err = cmdRestart(args)
	case "list", "ls":
		err = cmdList(args)
	case "status":
		err = cmdStatus(args)
	case "info":
		err = cmdInfo(args)
	case "remove", "rm":
		err = cmdRemove(args)
	case "logs":
		err = cmdLogs(args)
	case "exec":
		err = cmdExec(args)
	case "auth":
		err = cmdAuth(args)
	case "channel":
		err = cmdChannel(args)
	case "approve":
		err = cmdApprove(args)
	case "tunnel":
		err = cmdTunnel(args)
	case "stats":
		err = cmdStats(args)
	case "health":
		err = cmdHealth(args)
	case "dashboard":
		err = cmdDashboard(args)
	case "backup":
		err = cmdBackup(args)
	case "restore":
		err = cmdRestore(args)
	case "group":
		err = cmdGroup(args)
	case "team":
		// v1.6: intercept `team tree` for the new topology renderer.
		// Existing `team show` keeps its table-style listing.
		if len(args) > 0 && args[0] == "tree" {
			err = cmdTeamTree(args[1:])
		} else {
			err = cmdTeam(args)
		}
	case "cron":
		err = cmdCron(args)
	case "fleet":
		err = cmdFleet(args)
	case "id":
		err = cmdID(args)
	case "by-id":
		err = cmdByID(args)
	case "contract":
		err = cmdContract(args)
	case "agent":
		// v1.6.1: `claws agent ping <name>` is the only subcommand today.
		// Reserved namespace for future per-agent operator commands.
		if len(args) > 0 && args[0] == "ping" {
			err = cmdAgentPing(args[1:])
		} else {
			err = errorf("usage: claws agent <ping> <name>")
		}
	case "paste-secret":
		err = cmdPasteSecret(args)
	case "storage":
		err = cmdStorage(args)
	case "migrate":
		err = cmdMigrate(args)
	case "task":
		err = cmdTask(args)
	case "activity":
		err = cmdActivity(args)
	case "proxy":
		err = cmdProxy(args)
	case "share":
		err = cmdShare(args)
	case "unshare":
		err = cmdUnshare(args)
	case "start-all":
		err = cmdStartAll(args)
	case "stop-all":
		err = cmdStopAll(args)
	case "runtime":
		err = cmdRuntime(args)
	case "access":
		err = cmdAccess(args)
	case "token":
		err = cmdToken(args)
	case "image":
		// v1.6.1: intercept `image bootstrap` for the new day-one image setup.
		// All other `image` subcommands (list, pull, pin) stay on cmdImage.
		if len(args) > 0 && args[0] == "bootstrap" {
			err = cmdImageBootstrap(args[1:])
		} else {
			err = cmdImage(args)
		}
	case "upgrade":
		err = cmdUpgrade(args)
	case "update", "self-update":
		err = cmdUpdate(args)
	case "policy":
		err = cmdPolicy(args)
	case "audit":
		err = cmdAudit(args)
	case "security":
		err = cmdSecurity(args)
	case "auth-monitor":
		err = cmdAuthMonitor(args)
	case "config":
		err = cmdConfig(args)
	case "setup":
		err = cmdSetup(args)
	case "init":
		err = cmdInit(args)
	case "quickstart":
		err = cmdQuickstart(args)
	case "apply":
		err = cmdApply(args)
	case "template", "templates":
		err = cmdTemplate(args)
	case "version", "--version", "-v":
		err = cmdVersion(args)
	case "doctor":
		err = cmdDoctor(args)
	case "orphans":
		err = cmdOrphans(args)
	case "channels":
		err = cmdChannelsMatrix(args)
	case "errors":
		err = cmdErrors(args)
	case "drift":
		err = cmdDrift(args)
	case "help", "--help", "-h":
		if len(args) > 0 {
			printTopicHelp(args[0])
		} else {
			printHelp()
		}
	default:
		printHelp()
		os.Exit(1)
	}

	// Audit log
	if err != nil {
		writeAuditLog(paths, cmd, args, "error")
		fmt.Fprintf(os.Stderr, "\033[0;31m==> ERROR: %s\033[0m\n", err)
		os.Exit(1)
	}
	writeAuditLog(paths, cmd, args, "ok")
}

func printHelp() {
	sections := []struct {
		title string
		lines []string
	}{
		{"Getting Started", []string{
			"setup                      Guided interactive onboarding",
			"init                       First-run setup (creates dirs, security, deps)",
			"doctor                     Check Docker, image, disk, tools",
		}},
		{"Lifecycle", []string{
			"create <name>              Create a new instance",
			"create <group>/<name>      Create instance in a group",
			"create <name> --from=<src> Create from template (copies config)",
			"create <g>/<n> --role=manager|worker  Assign role",
			"start <name>               Start an instance",
			"stop <name>                Stop an instance",
			"restart <name>             Restart an instance",
			"remove <name> [--purge]    Remove (--purge deletes all data)",
		}},
		{"Info", []string{
			"list [--rich] [--group=<g>]  List instances (--rich adds model, role, channels)",
			"status [--group=<g>]         Unified system overview (health + policy)",
			"status <name>                Show instance details",
			"info <name>                  Deep info: identity, channels, creds, recent activity",
			"health [name...] [--group=<g>] Deep health probe (live + ready)",
			"stats                        Docker stats for all instances",
			"dashboard [--interval=5s]    Live refreshing status view",
		}},
		{"Auth & Channels", []string{
			"auth <name> codex              OAuth flow for OpenAI Codex",
			"auth <name> apikey <p> <k>     Register API key",
			"auth status [name]             Fleet auth inventory (no secrets)",
			"channels                       Fleet channel matrix (rows=agents, cols=channels)",
			"channel add <n> telegram --token=<t>  Quick-add Telegram",
			"channel add <n> discord --token=<t>   Quick-add Discord",
			"channel add <n> slack --bot-token=<t> --app-token=<t>",
			"channel add <n> whatsapp       Add WhatsApp (QR login)",
			"channel status <name>          Show configured channels",
			"channel security <name> [<ch>] Show channel security posture",
			"channel send <n> <ch> --enable Toggle outbound messaging",
			"channel allow <n> <ch> <id>    Add approved contact",
			"channel deny <n> <ch> <id>     Remove approved contact",
			"channel remove <name> <ch>     Disable a channel",
			"approve <name> <ch> <code>     Approve DM pairing code",
		}},
		{"Backup", []string{
			"backup <name> [<file>]     Tarball instance to file",
			"restore <name> <file>      Restore instance from backup",
		}},
		{"Groups & Teams", []string{
			"team create <name>         Create team (group + shared + tasks)",
			"team list                  List all teams/groups",
			"group create <name>        Create a group (manual)",
			"group list                 List all groups",
			"group add <group> <inst>   Move instance into group",
			"group remove <name>        Remove group",
			"group shared <name> --all  Enable group-level sharing",
		}},
		{"Tasks (Manager/Worker)", []string{
			"task create <group> <title>  Dispatch a task to the group queue",
			"task list <group>            List tasks (--status=pending|claimed|done)",
			"task claim <g> <id> --by=<i> Claim a pending task for a worker",
			"task complete <group> <id>   Mark a claimed task as done",
			"task status <group> <id>     Show task details",
		}},
		{"Shared Resources", []string{
			"share <name> --skills|--workspace|--hooks|--all",
			"unshare <name> --skills|--workspace|--hooks|--all",
		}},
		{"Storage (S3)", []string{
			"migrate <name> --to s3     Move workspace to S3 mount",
			"storage setup --bucket=<n>  Configure S3 bucket + rclone",
			"storage sync [--dry-run]    Copy to S3 (additive, excludes creds)",
			"storage sync --mirror       Mirror to S3 (deletes dest-only files)",
			"storage cron --enable       Auto-sync every hour",
			"storage mount               FUSE mount shared workspace",
			"storage unmount             Unmount FUSE",
			"storage status              Show storage config and state",
		}},
		{"Observability", []string{
			"activity [--since=2h]      Recent actions across instances",
			"proxy setup --domain=<d>   Set up Caddy reverse proxy",
			"proxy status               Show proxy state",
			"proxy reload               Reload Caddy config",
		}},
		{"Runtime", []string{
			"runtime list                   List available runtimes",
			"runtime show <name>            Show runtime config",
			"runtime add <n> --from=openclaw --image=<i>  Quick fork",
			"runtime init <name>            Scaffold custom runtime",
			"runtime test <name>            Validate runtime works",
			"runtime detect <image>         Auto-detect from image",
			"runtime export <name>          Export definition to JSON",
			"runtime import <file>          Import shared definition",
			"runtime remove <name>          Remove custom runtime",
		}},
		{"Image & Upgrade", []string{
			"image list                     List local images",
			"image pull <image:tag>         Pull from registry",
			"image pin <name> <image:tag>   Pin instance to image",
			"upgrade <name> [--image=<i>]   Upgrade with health-check rollback",
			"upgrade --all                  Upgrade all instances",
		}},
		{"Admin", []string{
			"policy init                    Create secure default policy",
			"policy show                    Show current policy",
			"policy validate                Check all instances against policy",
			"policy enforce [--restart]     Fix all violations automatically",
			"access init                    Set up role-based access control",
			"access show                    Show access config",
			"access grant <user> <role>     Grant admin/operator/user role",
			"access revoke <user>           Revoke access",
			"access audit [--since=24h]     Show command audit log",
			"token rotate <name>            Rotate gateway token",
			"token show <name> [--full]     Show gateway token",
			"audit                          Run security audit",
		}},
		{"Config", []string{
			"config show <name>             Show full config (--no-secrets)",
			"config get <name> <path>       Get a config value",
			"config set <name> <path> <val> Set a config value",
			"config edit <name>             Open config in $EDITOR",
		}},
		{"Diagnostics", []string{
			"version                    Show version and environment",
			"update [--check]           Update this claws binary to the latest release",
			"orphans [clean]            List containers not in claws's registry (clean = remove)",
			"orphans --reverse          List registry entries that have no Docker container",
			"drift                      Umbrella: orphans + reverse + disk/registry mismatch",
			"errors [--since=2h]        Incident-triage view: container state + log errors + audit errors + orphans",
		}},
		{"Operations", []string{
			"logs <name> [-f]           View instance logs",
			"exec <name> <cmd...>       Run CLI command",
			"tunnel [name...]           Print SSH tunnel command",
			"start-all                  Start all instances",
			"stop-all                   Stop all instances",
		}},
	}

	bold := "\033[1m"
	nc := "\033[0m"
	fmt.Printf("%sclaws%s — Multi-instance OpenClaw manager\n\n", bold, nc)
	fmt.Println("Usage: claws <command> [args...]")
	for _, s := range sections {
		fmt.Printf("\n%s%s:%s\n", bold, s.title, nc)
		for _, l := range s.lines {
			fmt.Printf("  %s\n", l)
		}
	}
	fmt.Println()
}

// printFirstRun detects whether the system is initialized and prints
// an appropriate welcome message or brief status summary.
func printFirstRun() {
	bold := "\033[1m"
	nc := "\033[0m"

	paths := resolvePaths()
	if _, err := os.Stat(paths.Root); os.IsNotExist(err) {
		// Uninitialized system — welcome message
		fmt.Printf("%sWelcome to claws%s — AI agent team manager.\n\n", bold, nc)
		fmt.Println("  Looks like this is your first time. Run:")
		fmt.Println()
		fmt.Printf("    %sclaws setup%s    — guided setup (recommended)\n", bold, nc)
		fmt.Printf("    %sclaws init%s     — manual setup (advanced)\n", bold, nc)
		fmt.Printf("    %sclaws help%s     — see all commands\n", bold, nc)
		fmt.Println()
		return
	}

	// Initialized system — brief status + help hint
	count := instanceCount(paths)
	fmt.Printf("%sclaws%s — AI agent team manager\n\n", bold, nc)
	if count == 0 {
		fmt.Println("  No agents running yet.")
	} else {
		fmt.Printf("  %d agent(s) registered.\n", count)
	}
	// Replaced the hand-rolled static menu with state-driven `Next:` hints.
	// The provider chooses what to suggest based on fleet state (zero
	// agents → setup; some never-started → start-all; healthy ones →
	// dashboard; etc.). One `claws help` line stays as the always-on
	// "everything else" pointer.
	hintsRender("", hintsCtxCheap(paths))
	fmt.Println()
	fmt.Printf("  %sclaws help%s — see every command\n", bold, nc)
	fmt.Println()
}
