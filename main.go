package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Per-subcommand help: clawctl <cmd> --help
	if printSubcommandHelp(cmd, args) {
		os.Exit(0)
	}

	// Access control enforcement
	paths := resolvePaths()
	if err := enforceAccess(paths, cmd, args); err != nil {
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
	case "access":
		err = cmdAccess(args)
	case "token":
		err = cmdToken(args)
	case "image":
		err = cmdImage(args)
	case "upgrade":
		err = cmdUpgrade(args)
	case "policy":
		err = cmdPolicy(args)
	case "audit":
		err = cmdAudit(args)
	case "config":
		err = cmdConfig(args)
	case "init":
		err = cmdInit(args)
	case "version", "--version", "-v":
		err = cmdVersion(args)
	case "doctor":
		err = cmdDoctor(args)
	case "help", "--help", "-h":
		printHelp()
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
			"list                       List all instances",
			"status <name>              Show instance details",
			"health [name...]           Deep health probe (live + ready)",
			"stats                      Docker stats for all instances",
			"dashboard [--interval=5s]  Live refreshing status view",
		}},
		{"Auth & Channels", []string{
			"auth <name> codex              OAuth flow for OpenAI Codex",
			"auth <name> apikey <p> <k>     Register API key",
			"channel add <n> telegram --token=<t>  Quick-add Telegram",
			"channel add <n> discord --token=<t>   Quick-add Discord",
			"channel add <n> slack --bot-token=<t> --app-token=<t>",
			"channel add <n> whatsapp       Add WhatsApp (QR login)",
			"channel status <name>          Show configured channels",
			"channel remove <name> <ch>     Disable a channel",
			"channel <name> <ch>            Interactive wizard (legacy)",
			"approve <name> <ch> <code>     Approve DM pairing code",
		}},
		{"Backup", []string{
			"backup <name> [<file>]     Tarball instance to file",
			"restore <name> <file>      Restore instance from backup",
		}},
		{"Groups", []string{
			"group create <name>        Create a group",
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
			"init                       First-run setup (creates dirs, checks deps)",
			"version                    Show version and environment",
			"doctor                     Check Docker, image, disk, tools",
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
	fmt.Printf("%sclawctl%s — Multi-instance OpenClaw manager\n\n", bold, nc)
	fmt.Println("Usage: clawctl <command> [args...]")
	for _, s := range sections {
		fmt.Printf("\n%s%s:%s\n", bold, s.title, nc)
		for _, l := range s.lines {
			fmt.Printf("  %s\n", l)
		}
	}
	fmt.Println()
}
