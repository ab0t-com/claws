package main

import "fmt"

// subcommandHelp maps commands to their detailed help text.
var subcommandHelp = map[string]string{
	"create": `Usage: clawctl create <name|group/name> [options]

Create a new OpenClaw instance.

Arguments:
  <name>              Instance name (lowercase, letters/numbers/hyphens)
  <group/name>        Create instance inside an existing group

Options:
  --from=<instance>   Copy config from an existing instance (template)
  --role=manager      Assign manager role (group only)
  --role=worker       Assign worker role (group only)
  --manager=<name>    Specify manager instance for this worker
  --shared-skills     Mount shared skills directory
  --shared-workspace  Mount shared workspace directory
  --shared-hooks      Mount shared hooks directory
  --shared            Enable shared skills + workspace
  --no-shared-workspace  Disable default group workspace sharing
  --bind=<mode>       Network binding: loopback (default, local only), lan, wan
  --image=<image>     Docker image (default: openclaw:local)

Examples:
  clawctl create alice
  clawctl create alice --bind=lan          # accessible from network
  clawctl create bravo --from=alice
  clawctl create bravo --image=openclaw:v2026.3.25
  clawctl create backend/sarah
  clawctl create team/lead --role=manager
  clawctl create team/dev1 --role=worker --manager=lead`,

	"start": `Usage: clawctl start <name>

Start an instance. Waits up to 30s for health check to pass.

Examples:
  clawctl start alice
  clawctl start backend/sarah`,

	"stop": `Usage: clawctl stop <name>

Stop a running instance.

Examples:
  clawctl stop alice`,

	"restart": `Usage: clawctl restart <name> [--hard]

Restart an instance.

Without --hard: restarts the process inside the existing container (fast, ~2s).
With --hard: tears down and recreates the container (slower, ~15s). Use this
when the docker-compose template has changed (e.g., after security updates,
memory limits, or capability changes).

Examples:
  clawctl restart alice           # quick restart (config changes only)
  clawctl restart alice --hard    # recreate container (template changes)`,

	"list": `Usage: clawctl list [--json]

List all instances with status, port, RAM, and uptime.

Options:
  --json    Output as JSON array

Aliases: ls`,

	"status": `Usage: clawctl status <name> [--json]

Show detailed information about an instance.

Options:
  --json    Output as JSON object

Examples:
  clawctl status alice
  clawctl status alice --json`,

	"remove": `Usage: clawctl remove <name> [--purge] [--yes]

Remove an instance. Without --purge, data is kept on disk.

Options:
  --purge   Delete all instance data (config, credentials, workspace)
  --yes     Skip confirmation prompt for --purge

Aliases: rm

Examples:
  clawctl remove alice             # stops, keeps data
  clawctl remove alice --purge     # prompts, then deletes everything
  clawctl remove alice --purge --yes  # no prompt`,

	"health": `Usage: clawctl health [name...] [--json]

Deep health probe of instances. Checks container status, /healthz (liveness),
and /readyz (readiness).

Options:
  --json    Output as JSON array

Examples:
  clawctl health              # all instances
  clawctl health alice bravo  # specific instances
  clawctl health --json`,

	"logs": `Usage: clawctl logs <name> [-f]

View instance container logs.

Options:
  -f    Follow log output (stream)

Examples:
  clawctl logs alice
  clawctl logs alice -f`,

	"exec": `Usage: clawctl exec <name> <command...>

Run an OpenClaw CLI command inside the instance container.

Examples:
  clawctl exec alice channels status
  clawctl exec alice config get gateway.port`,

	"channel": `Usage: clawctl channel add <instance> <channel> [--token=...]
       clawctl channel status <instance>
       clawctl channel remove <instance> <channel>
       clawctl channel security <instance> [<channel>]
       clawctl channel send <instance> <channel> --enable|--disable
       clawctl channel allow <instance> <channel> <contact...>
       clawctl channel deny <instance> <channel> <contact>
       clawctl channel <instance> <channel>  (interactive wizard)

Quick-add a messaging channel with one command. Sets config, restarts
the gateway, and tells you what to do next.

Supported quick-add channels:
  telegram    --token=<botToken>                  (@BotFather token)
  discord     --token=<botToken>                  (developer portal)
  slack       --bot-token=<xoxb> --app-token=<xapp>
  signal      --number=<+E.164>
  whatsapp    (no token needed — starts QR login)

Options:
  --dm-policy=<policy>   DM access policy: pairing (default), allowlist, open
  --allow-send           Enable outbound messaging (OFF by default)

Security:
  security <instance> [<channel>]   Show security posture (policies, actions, contacts)
  send <instance> <ch> --enable     Enable outbound messaging
  send <instance> <ch> --disable    Disable outbound messaging
  allow <instance> <ch> <contact>   Add contact to allowFrom list
  deny <instance> <ch> <contact>    Remove contact from allowFrom list

Safe defaults applied on 'channel add':
  - Outbound messaging (sendMessage/messages): OFF
  - Reactions and read-only lookups: ON
  - Group policy: allowlist
  Use --allow-send to enable outbound messaging during setup.

Examples:
  clawctl channel add alice telegram --token=123:ABC...
  clawctl channel add alice whatsapp
  clawctl channel add alice whatsapp --allow-send
  clawctl channel security alice
  clawctl channel security alice whatsapp
  clawctl channel send alice whatsapp --enable
  clawctl channel allow alice whatsapp +1234567890
  clawctl channel deny alice whatsapp +1234567890
  clawctl channel status alice
  clawctl channel remove alice telegram

After adding, approve the pairing code:
  clawctl approve alice telegram <CODE>

See docs/channels.md for the full setup guide.`,

	"approve": `Usage: clawctl approve <instance> <channel> <code>

Approve a DM pairing code. When someone messages your bot for the first
time, the bot replies with a one-time code. Run this command to approve
that sender.

Examples:
  clawctl approve alice telegram YBCAN5RA
  clawctl approve alice whatsapp ABC123
  clawctl approve backend/sarah discord XYZ789`,

	"auth": `Usage: clawctl auth <name> codex
       clawctl auth <name> apikey <provider> <key>

Configure authentication for an instance.

Methods:
  codex                    Start OAuth flow for OpenAI Codex
  apikey <provider> <key>  Register an API key (openai, anthropic, etc.)

Examples:
  clawctl auth alice codex
  clawctl auth alice apikey openai sk-...
  clawctl auth alice apikey anthropic sk-ant-...`,

	"backup": `Usage: clawctl backup <name> [<output-path>] [--exclude-credentials]

Create a tarball backup of an instance.

Options:
  --exclude-credentials   Omit credentials/ directory from backup

Examples:
  clawctl backup alice
  clawctl backup alice /backups/alice-2026.tar.gz
  clawctl backup alice --exclude-credentials`,

	"restore": `Usage: clawctl restore <name> <backup-file>

Restore an instance from a backup tarball. The instance must not already exist.

Examples:
  clawctl restore alice alice-backup-20260317.tar.gz`,

	"group": `Usage: clawctl group <subcommand> [args...]

Manage instance groups.

Subcommands:
  create <name>                Create a new group
  list                         List all groups
  add <group> <instance...>    Move instances into a group
  remove <name> [--purge]      Remove a group
  shared <name> --all          Enable group-level shared resources

Examples:
  clawctl group create backend
  clawctl group list
  clawctl group add backend alice bob
  clawctl group shared backend --all`,

	"task": `Usage: clawctl task <subcommand> [args...]

Manage tasks in a group's task queue (manager/worker coordination).
Tasks only work on local storage — not supported on S3 FUSE mounts.

Subcommands:
  create <group> <title> [--from=<instance>]   Create a new task
  list <group> [--status=pending|claimed|done]  List tasks
  claim <group> <id> --by=<instance>            Claim a pending task
  complete <group> <id> [--result=<text>]        Mark task as done
  status <group> <id>                            Show task details

Examples:
  clawctl task create team "review PR #42" --from=lead
  clawctl task list team
  clawctl task list team --status=pending
  clawctl task claim team a1b2c3d4 --by=dev1
  clawctl task complete team a1b2c3d4 --result="approved"`,

	"storage": `Usage: clawctl storage <subcommand> [args...]

Manage S3 storage for instance backups and shared workspaces.

Subcommands:
  setup --bucket=<name> [--region=<r>]   Configure S3 bucket + rclone
  sync [--dry-run] [--mirror]            Copy to S3 (additive by default)
  mount [--path=<p>]                     FUSE mount shared workspace from S3
  unmount                                Unmount FUSE
  status                                 Show storage config and state
  cron --enable|--disable                Auto-sync on a schedule

Examples:
  clawctl storage setup --bucket=my-openclaw-backup
  clawctl storage sync
  clawctl storage sync --mirror --yes
  clawctl storage mount
  clawctl storage status`,

	"proxy": `Usage: clawctl proxy <subcommand> [args...]

Manage Caddy reverse proxy for HTTPS access to instances.

Subcommands:
  setup --domain=<d> [--subdomain|--path] [--dry-run] [--no-auth]
  reload                                  Reload Caddy config
  status                                  Show proxy state

Options for setup:
  --domain=<domain>   Required. Domain name for the proxy.
  --subdomain         Route by subdomain (alice.example.com)
  --path              Route by path (example.com/alice) — default
  --dry-run           Print config without writing
  --no-auth           Skip injecting Authorization headers

Examples:
  clawctl proxy setup --domain=ai.example.com
  clawctl proxy setup --domain=ai.example.com --subdomain
  clawctl proxy setup --domain=ai.example.com --dry-run
  clawctl proxy status`,

	"runtime": `Usage: clawctl runtime <subcommand>

Manage agent runtimes. clawctl can manage different containerized agent
gateways, not just OpenClaw.

Most users just need --image= (same runtime, different Docker image):
  clawctl create alice --image=openclaw:slim
  clawctl create alice --image=nemoclaw:latest

Custom runtimes are for agents with different ports, health endpoints,
or CLI commands:

Subcommands:
  list                     List available runtimes
  show <name> [--json]     Show full runtime configuration
  add <name> [options]     Register a custom runtime
  init <name>              Scaffold runtime JSON + compose template
  test <name>              Validate runtime (image, compose, health)
  detect <image>           Auto-detect settings from Docker image
  export <name>            Export runtime definition (for sharing)
  import <file>            Import a shared runtime definition
  remove <name>            Remove a custom runtime

Options for add:
  --from=<runtime>         Inherit from an existing runtime (override only what's different)
  --image=<image>          Default Docker image
  --health=<endpoint>      Health check endpoint (default: /healthz)
  --port=<port>            Internal gateway port (default: 18789)
  --no-channels            Disable channel support
  --no-cli                 No CLI service

Examples:
  # Compatible fork — inherit from openclaw, change image
  clawctl runtime add nemoclaw --from=openclaw --image=nemoclaw:latest

  # Divergent fork — different health endpoint
  clawctl runtime add nanoclaw --from=openclaw --image=nanoclaw:latest --health=/status

  # Completely different agent — scaffold and customize
  clawctl runtime init my-python-agent
  clawctl runtime test my-python-agent
  clawctl create alice --runtime=my-python-agent

  # Auto-detect from image
  clawctl runtime detect my-agent:latest

  # Share with team
  clawctl runtime export nemoclaw > nemoclaw.json
  clawctl runtime import nemoclaw.json`,

	"access": `Usage: clawctl access <init|show|grant|revoke|audit>

Manage who can use clawctl and what they can do.

Subcommands:
  init                         Create access control with you as admin
  show                         Show current roles and users
  grant <user> <role>          Assign a role (admin, operator, user)
  revoke <user>                Remove all access for a user
  audit [--since=24h]          Show command audit log

Roles:
  admin     Full access to all commands and instances
  operator  Start/stop/restart/logs/health/backup (no create/remove/policy)
  user      Read-only: status, health, logs (scoped to specific instances)

Examples:
  clawctl access init
  clawctl access grant deploy-bot operator
  clawctl access grant alice-dev user
  clawctl access audit --since=1h`,

	"token": `Usage: clawctl token <rotate|show> <instance>

Manage gateway authentication tokens.

Subcommands:
  rotate <instance>            Generate a new token (restart to apply)
  show <instance> [--full]     Show current token (truncated by default)

Examples:
  clawctl token rotate team/sarah
  clawctl token show team/sarah
  clawctl token show team/sarah --full`,

	"image": `Usage: clawctl image <list|pull|pin>

Manage Docker images for agent instances.

Subcommands:
  list                     List local openclaw images
  pull <image:tag>         Pull an image from a registry
  pin <instance> <image>   Pin an instance to a specific image version

Examples:
  clawctl image list
  clawctl image pull openclaw:v2026.3.25
  clawctl image pin team/sarah openclaw:v2026.3.25`,

	"upgrade": `Usage: clawctl upgrade <instance> [--image=<image:tag>]
       clawctl upgrade --all [--image=<image:tag>]

Upgrade an instance to a new image version. Stops the old container,
starts with the new image, waits for health check. If health fails,
automatically rolls back to the previous image.

Options:
  --image=<image:tag>   Specific image to upgrade to (default: same image, recreate)
  --all                 Upgrade all instances

Examples:
  clawctl upgrade team/sarah --image=openclaw:v2026.4.1
  clawctl upgrade --all --image=openclaw:v2026.4.1
  clawctl upgrade team/sarah    # recreate with same image (picks up compose changes)`,

	"policy": `Usage: clawctl policy <init|show|validate|enforce>

Manage admin security policy. Policy constrains what instances can do.

Subcommands:
  init                Create policy.json with secure defaults (--force to overwrite)
  show                Print current policy
  validate            Check all instances against policy
  enforce [--restart] Fix all violations automatically

Policy controls:
  allowedBindModes         Restrict network binding (e.g., ["loopback"] only)
  maxInstances             Hard cap on instance count
  memoryLimitMB            Per-instance memory limit
  cpuLimit                 Per-instance CPU limit
  requireSandbox           Force sandbox mode on all agents
  allowedToolProfiles      Restrict tool profiles
  requireDmPairing         Force pairing on all channels
  requireOutboundAllowlist Require allowFrom when outbound messaging is enabled
  blockedChannels          Block specific channels
  allowedImages            Restrict which Docker images can be used
  auditLog                 Enable command audit logging

Examples:
  clawctl policy init
  clawctl policy show
  clawctl policy validate
  clawctl policy enforce --restart`,

	"config": `Usage: clawctl config <show|get|set|edit> <instance> [args...]

View and modify instance configuration.

Subcommands:
  show <instance> [--no-secrets]     Print full openclaw.json
  get <instance> <path>              Get a value (dotted path)
  set <instance> <path> <value>      Set a value (restarts needed)
  edit <instance>                    Open in $EDITOR

Examples:
  clawctl config show team/sarah
  clawctl config show team/sarah --no-secrets
  clawctl config get team/sarah channels.telegram.enabled
  clawctl config get team/sarah gateway.port
  clawctl config set team/sarah channels.telegram.dmPolicy '"allowlist"'
  clawctl config edit team/sarah`,

	"init": `Usage: clawctl init

First-run setup. Creates the OPENCLAW_ROOT directory structure, checks Docker
and the OpenClaw image, and writes default config files.

Safe to run multiple times (idempotent).`,

	"version": `Usage: clawctl version

Show clawctl version, Go runtime, Docker version, Compose version, and
the configured OpenClaw image.`,

	"doctor": `Usage: clawctl doctor

Diagnose the environment. Checks Docker, Compose, OPENCLAW_ROOT, compose
template, image, disk space, instance count, and optional tools (rclone,
aws, mount-s3, caddy).`,

	"dashboard": `Usage: clawctl dashboard [--interval=5s]

Live-refreshing status view of all instances. Shows health, RAM, and uptime.

Options:
  --interval=<duration>   Refresh interval (default: 5s)

Press Ctrl+C to exit.`,

	"tunnel": `Usage: clawctl tunnel [name...]

Print an SSH tunnel command for all (or specified) instances.

Examples:
  clawctl tunnel              # all instances
  clawctl tunnel alice bravo  # specific instances`,

	"activity": `Usage: clawctl activity [--since=2h] [--group=<name>] [--limit=50]

Show recent activity across instances (file changes, log errors).

Options:
  --since=<duration>   Time window (default: 2h)
  --group=<name>       Filter to a specific group
  --limit=<n>          Maximum entries (default: 50)`,

	"share": `Usage: clawctl share <name> --skills|--workspace|--hooks|--all

Enable shared resource mounts for an instance. Requires restart.

Examples:
  clawctl share alice --skills
  clawctl share alice --all`,

	"unshare": `Usage: clawctl unshare <name> --skills|--workspace|--hooks|--all

Disable shared resource mounts for an instance. Requires restart.

Examples:
  clawctl unshare alice --skills`,

	"migrate": `Usage: clawctl migrate <name> --to s3 [--cleanup]

Move an instance's workspace to an S3 FUSE mount. Stops the instance,
copies data via rsync, then restarts.

Options:
  --cleanup   Remove local workspace copy after migration

Examples:
  clawctl migrate alice --to s3
  clawctl migrate alice --to s3 --cleanup`,

	"stats": `Usage: clawctl stats

Show docker stats (CPU, RAM, network) for all instance containers.`,
}

// printSubcommandHelp prints help for a specific command if --help/-h is the first arg.
// Returns true if help was printed (caller should return nil).
func printSubcommandHelp(cmd string, args []string) bool {
	if len(args) == 0 {
		return false
	}
	if args[0] != "--help" && args[0] != "-h" {
		return false
	}
	if help, ok := subcommandHelp[cmd]; ok {
		fmt.Println(help)
		return true
	}
	return false
}
