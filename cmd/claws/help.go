package main

import "fmt"

// subcommandHelp maps commands to their detailed help text.
var subcommandHelp = map[string]string{
	"setup": `Usage: claws setup [options]

Guided interactive onboarding — from zero to a working team of agents.

Combines init + policy + access + group create + agent create + auth +
channel add + start into a single interactive flow with safe defaults.

Options (non-interactive mode):
  --non-interactive         Run without prompts (requires flags below)
  --team=<name>             Team/group name
  --agent=<name>            First agent name
  --auth=<codex|apikey>     Auth method
  --anthropic-key=<key>     API key (when --auth=apikey)
  --channel=<type>          Channel type (telegram, discord, slack, whatsapp)
  --telegram-token=<token>  Telegram bot token
  --discord-token=<token>   Discord bot token
  --slack-bot-token=<token> Slack bot token

Examples:
  claws setup                           # interactive guided flow
  claws setup --non-interactive \
    --team=research --agent=sarah \
    --auth=codex --channel=telegram \
    --telegram-token=TOKEN`,

	"create": `Usage: claws create <name|group/name> [options]

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
  --auth=codex        Chain Codex OAuth after creation
  --telegram=<token>  Chain Telegram channel add after creation
  --discord=<token>   Chain Discord channel add after creation
  --slack-bot=<token> Chain Slack channel add (use with --slack-app=<token>)

Examples:
  claws create alice
  claws create alice --bind=lan          # accessible from network
  claws create bravo --from=alice
  claws create bravo --image=openclaw:v2026.3.25
  claws create backend/sarah
  claws create team/lead --role=manager
  claws create team/dev1 --role=worker --manager=lead
  claws create alice --auth=codex --telegram=TOKEN  # create + auth + channel`,

	"start": `Usage: claws start <name>
       claws start --group=<name>

Start an instance or every member of a group. Waits up to 30s for the health
check to pass per instance. Group fan-out is sequential.

Options:
  --group=<name>   Start every instance in the named group

Examples:
  claws start alice
  claws start backend/sarah
  claws start --group=backend          # start whole team`,

	"stop": `Usage: claws stop <name>
       claws stop --group=<name> [--yes]

Stop a running instance or every member of a group. Group fan-out is
sequential and prompts for confirmation; pass --yes to skip the prompt.

Options:
  --group=<name>   Stop every instance in the named group
  --yes            Skip confirmation when using --group=

Examples:
  claws stop alice
  claws stop --group=backend           # prompts before stopping all
  claws stop --group=backend --yes     # scripted, no prompt`,

	"restart": `Usage: claws restart <name> [--hard]
       claws restart --group=<name> [--hard] [--yes]

Restart an instance or every member of a group.

Without --hard: restarts the process inside the existing container (fast, ~2s).
With --hard: tears down and recreates the container (slower, ~15s). Use this
when the docker-compose template has changed (e.g., after security updates,
memory limits, or capability changes).

Group fan-out is sequential and prompts for confirmation; pass --yes to skip.

Options:
  --hard           Recreate the container (picks up compose template changes)
  --group=<name>   Restart every instance in the named group
  --yes            Skip confirmation when using --group=

Examples:
  claws restart alice                  # quick restart
  claws restart alice --hard           # recreate container
  claws restart --group=backend        # prompts, then restarts all
  claws restart --group=backend --hard --yes  # full team container refresh`,

	"list": `Usage: claws list [--rich|--wide] [--group=<name>] [--json]

List instances with status, port, RAM, and uptime.

The default view is intentionally narrow. Pass --rich (alias --wide) to also
show model, role, and enabled channels per agent — useful when you want
"what is each of my agents on?" at a glance without running 'claws info'
or 'config get' per instance.

Options:
  --rich, --wide   Show model, role, image, and channels columns
  --group=<name>   Only show instances in the named group
  --json           Output as JSON array

Aliases: ls

Examples:
  claws list
  claws list --rich
  claws list --rich --group=backend
  claws list --json
  claws list --rich --json    # full identity record per agent`,

	"drift": `Usage: claws drift [--json]

State consistency check across four dimensions:
  1. Forward orphans   — Docker containers with the openclaw- prefix that
                          aren't in the port registry. (Same data as
                          'claws orphans'.)
  2. Reverse orphans   — Registry entries whose expected Docker container
                          is missing. (Same data as 'claws orphans --reverse'.)
  3. Disk drift        — Instance directories on disk (have instance.env)
                          that aren't in the registry.
  4. Registry drift    — Registry entries whose instance directory is gone.

Read-only. Emits per-finding fix commands; never executes anything.

Options:
  --json    Machine-readable output with all four sections.

Examples:
  claws drift
  claws drift --json | jq '.forward,.reverse'`,

	"errors": `Usage: claws errors [--since=<dur>] [--group=<name>] [--json]

Incident-triage umbrella view. Composes four read paths into one screen:
  1. Container state (running/restarting/exited + restart count)
  2. Recent log errors per instance
  3. Recent claws operations that returned error
  4. Orphan Docker containers (containers not in claws's registry)

Plus a "Fix paths" trailer with directive commands to address each finding.

Read-only. Composes existing surfaces ('activity', 'access audit', 'orphans');
no new state. Useful as the first command to run when something feels off.

Options:
  --since=<dur>    Time window for log and audit errors (default: 2h)
  --group=<name>   Scope container + log + audit sections to one team
                   (orphans are global by definition)
  --json           Machine-readable output: { containers, logErrors,
                   auditErrors, orphans, fixPaths }

Examples:
  claws errors
  claws errors --since=24h --group=team
  claws errors --json   # for dashboards / alert pipelines`,

	"channels": `Usage: claws channels [--group=<name>] [--json]

Fleet-wide channel matrix: rows are agents, columns are channel types
(telegram, discord, slack, signal, whatsapp). Cells show the dmPolicy when
the channel is enabled, or — when not configured.

Use this to answer "which agents are on which channels?" without running
'channel status' per agent.

Note: pluralised form ('channels') is the fleet view. Singular 'channel
<verb>' (add/remove/status/security/send/allow/deny) operates on one
instance — see 'claws help channel'.

Options:
  --group=<name>   Only show instances in the named group
  --json           Output as { columns: [...], rows: [{name, cells}] }

Examples:
  claws channels
  claws channels --group=backend
  claws channels --json   # for dashboards`,

	"orphans": `Usage: claws orphans [list] [--json]
       claws orphans clean <container> [--yes]
       claws orphans clean --all [--yes]

Surface Docker containers managed-by-naming but not by claws's port
registry. The canonical case is a test run (or a manual 'docker compose up')
that left a container running after claws thought the instance was gone.
A restart-looping orphan can burn CPU and confuse incident triage.

Today's detection covers containers whose name starts with 'openclaw-'.
Custom runtimes with a different projectPrefix are not detected yet — open
issue for follow-up if you need that.

Subcommands:
  list (default)                List orphan containers with mount + status
  clean <container> [--yes]     docker rm -f a specific orphan
  clean --all [--yes]           docker rm -f every orphan

The list view shows:
  - the container name and its compose project
  - status (running/restarting/exited/...)
  - mount paths, with ✗ marking host paths that no longer exist
  - the exact 'clean' command to remove it

Options:
  --json    Machine-readable list output
  --yes     Skip the confirmation prompt for 'clean'

Examples:
  claws orphans
  claws orphans --json
  claws orphans clean openclaw-bob-openclaw-gateway-1
  claws orphans clean --all --yes`,

	"info": `Usage: claws info <name> [--json]

Deep single-agent identity card. Consolidates what 'status' + 'channel
status' + 'config get' + 'token show' + 'access audit' would tell you about
one instance, in one screen. Read-only — no probes, no model calls.

Shows:
  - Status (healthy/starting/stopped/created), group, role, created time
  - Identity: model, image, runtime
  - Network: gateway port, token (truncated)
  - Channels enabled (no tokens)
  - Credential files present (filenames only — no secrets)
  - Filesystem layout
  - Recent audit-log entries scoped to this instance (last 24h, max 8)

Options:
  --json    Output as JSON object (machine-readable; includes everything above)

Examples:
  claws info team/sarah
  claws info team/sarah --json
  claws info standalone-agent`,

	"status": `Usage: claws status [name] [--group=<name>] [--json]

Without arguments: unified system overview showing all instance health,
policy compliance, audit status, and access control. Use --group= to scope
the overview to a single team.

With a name: detailed information about a specific instance.

Options:
  --group=<name>   Scope overview to a single group (no-name form only)
  --json           Output as JSON object (instance mode only)

Examples:
  claws status                      # whole-system overview
  claws status --group=backend      # team-scoped overview
  claws status alice                # instance detail
  claws status alice --json`,

	"remove": `Usage: claws remove <name> [--purge] [--yes]

Remove an instance. Without --purge, data is kept on disk.

Options:
  --purge   Delete all instance data (config, credentials, workspace)
  --yes     Skip confirmation prompt for --purge

Aliases: rm

Examples:
  claws remove alice             # stops, keeps data
  claws remove alice --purge     # prompts, then deletes everything
  claws remove alice --purge --yes  # no prompt`,

	"health": `Usage: claws health [name...] [--group=<name>] [--json]

Deep health probe of instances. Checks container status, /healthz (liveness),
and /readyz (readiness). Positional names and --group= are mutually
exclusive — pass either an explicit set of instances or a group, not both.

Options:
  --group=<name>   Probe every instance in the named group
  --json           Output as JSON array

Examples:
  claws health                       # whole fleet
  claws health alice bravo           # specific instances
  claws health --group=backend       # team-scoped
  claws health --group=backend --json`,

	"logs": `Usage: claws logs <name> [-f] [--since=<dur>] [--grep=<pat>]
       claws logs --group=<name> [--since=<dur>] [--grep=<pat>]

View instance container logs.

Options:
  -f                Follow log output (stream)
  --since=<dur>     Only show logs since this duration ago (e.g. 1h, 24h)
  --grep=<pattern>  Case-insensitive substring filter (in-process; preserves order)
  --group=<name>    Tail every member of a group. Without -f: sequential dump
                    with section headers. With -f: live multiplex with a
                    per-member ANSI color prefix; Ctrl-C exits cleanly.

Examples:
  claws logs alice
  claws logs alice -f
  claws logs alice --since=1h --grep=error
  claws logs --group=backend --since=24h --grep=401
  claws logs --group=backend -f                # live multiplex tail
  claws logs --group=backend -f --grep=401     # filtered live tail`,

	"exec": `Usage: claws exec <name> <command...>

Run an OpenClaw CLI command inside the instance container.

Examples:
  claws exec alice channels status
  claws exec alice config get gateway.port`,

	"channel": `Usage: claws channel add <instance> <channel> [--token=...]
       claws channel status <instance> [--json]
       claws channel remove <instance> <channel>
       claws channel security <instance> [<channel>] [--json]
       claws channel send <instance> <channel> --enable|--disable
       claws channel allow <instance> <channel> <contact...>
       claws channel deny <instance> <channel> <contact>
       claws channel <instance> <channel>  (interactive wizard)

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
  claws channel add alice telegram --token=123:ABC...
  claws channel add alice whatsapp
  claws channel add alice whatsapp --allow-send
  claws channel security alice
  claws channel security alice whatsapp
  claws channel send alice whatsapp --enable
  claws channel allow alice whatsapp +1234567890
  claws channel deny alice whatsapp +1234567890
  claws channel status alice
  claws channel remove alice telegram

After adding, approve the pairing code:
  claws approve alice telegram <CODE>

See docs/channels.md for the full setup guide.`,

	"approve": `Usage: claws approve <instance> <channel> <code>

Approve a DM pairing code. When someone messages your bot for the first
time, the bot replies with a one-time code. Run this command to approve
that sender.

Examples:
  claws approve alice telegram YBCAN5RA
  claws approve alice whatsapp ABC123
  claws approve backend/sarah discord XYZ789`,

	"auth": `Usage: claws auth <name> codex
       claws auth <name> apikey <provider> <key>
       claws auth status [name] [--group=<g>] [--json]
       claws auth verify <name> [--json]

Configure or inspect authentication for an instance.

Verbs:
  codex                    Start OAuth flow for OpenAI Codex (interactive)
  apikey <provider> <key>  Register an API key (openai, anthropic, etc.)
  status [name]            Read-only inventory: model, token, channel
                           creds, last auth event from the audit log.
                           No name: covers every registered instance.
                           Add --probe to also run the per-instance verify
                           and add a VERIFIED column.
  verify <name>            Per-instance liveness check: tries (1) the auth-
                           check endpoint, (2) /readyz failing[] inspection,
                           (3) log scan for auth errors in the last 5m.
                           Exits 0 only on verified ok. Honest about
                           confidence — log-scan "ok" means "no errors seen",
                           not "next call will succeed."

Options for codex / apikey:
  --force                  Re-run even if auth verify already passes
                           (idempotence is on by default — retry-safe)

Options for status / verify:
  --group=<name>           Limit status to one group/team
  --json                   Machine-readable output
  --probe                  (status only) Run verify per instance, add
                           VERIFIED column, list per-failure fix commands

Examples:
  claws auth alice codex
  claws auth alice apikey openai sk-...
  claws auth alice apikey anthropic sk-ant-...
  claws auth status                          # whole fleet inventory
  claws auth status team/sarah               # one instance
  claws auth status --group=backend          # one team
  claws auth status --probe                  # fleet auth + verify in one screen
  claws auth status --probe --group=team --json   # for monitoring / CI
  claws auth verify team/sarah               # "is auth actually working?"
  claws auth verify team/sarah --json        # for CI / alerts
  claws auth team/sarah codex --force        # rotate (overrides idempotence)`,

	"backup": `Usage: claws backup <name> [<output-path>] [--exclude-credentials]

Create a tarball backup of an instance.

Options:
  --exclude-credentials   Omit credentials/ directory from backup

Examples:
  claws backup alice
  claws backup alice /backups/alice-2026.tar.gz
  claws backup alice --exclude-credentials`,

	"restore": `Usage: claws restore <name> <backup-file>

Restore an instance from a backup tarball. The instance must not already exist.

Examples:
  claws restore alice alice-backup-20260317.tar.gz`,

	"team": `Usage: claws team <subcommand> [args...]

The 'team' noun is the operator-friendly way to act on every member of a
group at once. Most subcommands are thin wrappers over the corresponding
per-instance command with --group=<team> injected.

Subcommands:
  create <team>                                Create team (group + shared + tasks)
  list                                         List all teams/groups
  start <team>                                 Start every member
  stop <team> [--yes]                          Stop every member (prompts unless --yes)
  restart <team> [--hard] [--yes]              Restart every member (prompts unless --yes)
  status <team>                                Per-member health + policy + activity overview
  health <team> [--json]                       Deep health probe across the team
  show <team> [--json]                         Members + identity + shared + task-queue summary
  rotate-tokens <team> [--yes]                 Bulk token rotation (prompts unless --yes)
  upgrade <team> [--image=...] [--yes]         Bulk image upgrade with health-check rollback
  apply-policy <team> [--yes]                  Enforce admin policy across the team and restart affected
  apply-config <team> <key> <value> [--yes]    Set the same config key on every member

All non-create/list subcommands require the team to already exist.

Examples:
  claws team create backend
  claws team show backend
  claws team start backend
  claws team restart backend --hard --yes
  claws team rotate-tokens backend --yes
  claws team upgrade backend --image=openclaw:v2026.5.20 --yes
  claws team create research          # create a fully-configured team
  claws create research/sarah         # add a member
  claws create research/john          # add another`,

	"group": `Usage: claws group <subcommand> [args...]

Manage instance groups.

Subcommands:
  create <name>                Create a new group
  list [--json]                List all groups
  add <group> <instance...>    Move instances into a group
  remove <name> [--purge]      Remove a group
  shared <name> --all          Enable group-level shared resources

Examples:
  claws group create backend
  claws group list
  claws group add backend alice bob
  claws group shared backend --all`,

	"task": `Usage: claws task <subcommand> [args...]

Manage tasks in a group's task queue (manager/worker coordination).
Tasks only work on local storage — not supported on S3 FUSE mounts.

Subcommands:
  create <group> <title> [--from=<instance>]    Create a new task
  list <group> [--status=...] [--json]          List tasks
  claim <group> <id> --by=<instance>            Claim a pending task
  complete <group> <id> [--result=<text>]       Mark task as done
  status <group> <id>                           Show task details

Examples:
  claws task create team "review PR #42" --from=lead
  claws task list team
  claws task list team --status=pending
  claws task claim team a1b2c3d4 --by=dev1
  claws task complete team a1b2c3d4 --result="approved"`,

	"storage": `Usage: claws storage <subcommand> [args...]

Manage S3 storage for instance backups and shared workspaces.

Subcommands:
  setup --bucket=<name> [--region=<r>]   Configure S3 bucket + rclone
  sync [--dry-run] [--mirror]            Copy to S3 (additive by default)
  mount [--path=<p>]                     FUSE mount shared workspace from S3
  unmount                                Unmount FUSE
  status                                 Show storage config and state
  cron --enable|--disable                Auto-sync on a schedule

Examples:
  claws storage setup --bucket=my-openclaw-backup
  claws storage sync
  claws storage sync --mirror --yes
  claws storage mount
  claws storage status`,

	"proxy": `Usage: claws proxy <subcommand> [args...]

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
  claws proxy setup --domain=ai.example.com
  claws proxy setup --domain=ai.example.com --subdomain
  claws proxy setup --domain=ai.example.com --dry-run
  claws proxy status`,

	"runtime": `Usage: claws runtime <subcommand>

Manage agent runtimes. claws can manage different containerized agent
gateways, not just OpenClaw.

Most users just need --image= (same runtime, different Docker image):
  claws create alice --image=openclaw:slim
  claws create alice --image=nemoclaw:latest

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
  claws runtime add nemoclaw --from=openclaw --image=nemoclaw:latest

  # Divergent fork — different health endpoint
  claws runtime add nanoclaw --from=openclaw --image=nanoclaw:latest --health=/status

  # Completely different agent — scaffold and customize
  claws runtime init my-python-agent
  claws runtime test my-python-agent
  claws create alice --runtime=my-python-agent

  # Auto-detect from image
  claws runtime detect my-agent:latest

  # Share with team
  claws runtime export nemoclaw > nemoclaw.json
  claws runtime import nemoclaw.json`,

	"access": `Usage: claws access <init|show|grant|revoke|audit>

Manage who can use claws and what they can do.

Subcommands:
  init                                       Create access control with you as admin
  show                                       Show current roles and users
  grant <user> <role>                        Assign a role (admin, operator, user)
  revoke <user>                              Remove all access for a user
  audit [--since=24h] [--group=<name>]       Show command audit log
  tail [-f] [--tail=20]                      Tail the audit log (live with -f)

Roles:
  admin     Full access to all commands and instances
  operator  Start/stop/restart/logs/health/backup (no create/remove/policy)
  user      Read-only: status, health, logs (scoped to specific instances)

Examples:
  claws access init
  claws access grant deploy-bot operator
  claws access grant alice-dev user
  claws access audit --since=1h
  claws access audit --since=24h --group=backend   # team-scoped audit`,

	"token": `Usage: claws token <rotate|show> <instance>
       claws token rotate --group=<name> [--yes]

Manage gateway authentication tokens.

Subcommands:
  rotate <instance>                        Generate a new token (restart to apply)
  rotate --group=<name> [--yes]            Rotate tokens for every member of a group
  show <instance> [--full]                 Show current token (truncated by default)

Group-scoped rotation prompts for confirmation; pass --yes to skip.

Examples:
  claws token rotate team/sarah
  claws token rotate --group=team --yes        # bulk team rotation
  claws token show team/sarah
  claws token show team/sarah --full`,

	"image": `Usage: claws image <list|pull|pin>

Manage Docker images for agent instances.

Subcommands:
  list                     List local openclaw images
  pull <image:tag>         Pull an image from a registry
  pin <instance> <image>   Pin an instance to a specific image version

Examples:
  claws image list
  claws image pull openclaw:v2026.3.25
  claws image pin team/sarah openclaw:v2026.3.25`,

	"upgrade": `Usage: claws upgrade <instance> [--image=<image:tag>]
       claws upgrade --all [--image=<image:tag>]
       claws upgrade --group=<name> [--image=<image:tag>] [--yes]

Upgrade one instance, every instance in a group, or every instance overall to
a new image version. Stops the old container, starts with the new image,
waits for health check. If health fails, automatically rolls back to the
previous image. Group fan-out is sequential and prompts for confirmation;
pass --yes to skip.

Options:
  --image=<image:tag>   Specific image (default: same image, recreate)
  --all                 Upgrade every instance
  --group=<name>        Upgrade every instance in the named group
  --yes                 Skip confirmation when using --group=

(Exactly one of <instance>, --all, or --group= is required.)

Examples:
  claws upgrade team/sarah --image=openclaw:v2026.4.1
  claws upgrade --group=team --image=openclaw:v2026.4.1
  claws upgrade --all --image=openclaw:v2026.4.1
  claws upgrade team/sarah    # recreate with same image (picks up compose changes)`,

	"policy": `Usage: claws policy <init|show|validate|enforce>

Manage admin security policy. Policy constrains what instances can do.

Subcommands:
  init                                       Create policy.json with secure defaults (--force to overwrite)
  show                                       Print current policy (always JSON)
  validate [--group=<name>] [--json]         Check all instances (or just a group) against policy
  enforce [--restart] [--group=<name>]       Fix violations (all or just a group)

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
  claws policy init
  claws policy show
  claws policy validate
  claws policy enforce --restart`,

	"config": `Usage: claws config <show|get|set|edit> <instance> [args...]

View and modify instance configuration.

Subcommands:
  show <instance> [--no-secrets]     Print full openclaw.json
  get <instance> <path>              Get a value (dotted path)
  set <instance> <path> <value>      Set a value (restarts needed)
  edit <instance>                    Open in $EDITOR

Examples:
  claws config show team/sarah
  claws config show team/sarah --no-secrets
  claws config get team/sarah channels.telegram.enabled
  claws config get team/sarah gateway.port
  claws config set team/sarah channels.telegram.dmPolicy '"allowlist"'
  claws config edit team/sarah`,

	"init": `Usage: claws init

First-run setup. Creates the OPENCLAW_ROOT directory structure, checks Docker
and the OpenClaw image, and writes default config files.

Safe to run multiple times (idempotent).`,

	"quickstart": quickstartHelp,
	"apply":      applyHelp,

	"version": `Usage: claws version

Show claws version, Go runtime, Docker version, Compose version, and
the configured OpenClaw image.`,

	"doctor": `Usage: claws doctor

Diagnose the environment. Checks Docker, Compose, OPENCLAW_ROOT, compose
template, image, disk space, instance count, and optional tools (rclone,
aws, mount-s3, caddy).`,

	"dashboard": `Usage: claws dashboard [--interval=5s]

Live-refreshing status view of all instances. Shows health, RAM, and uptime.

Options:
  --interval=<duration>   Refresh interval (default: 5s)

Press Ctrl+C to exit.`,

	"tunnel": `Usage: claws tunnel [name...]

Print an SSH tunnel command for all (or specified) instances.

Examples:
  claws tunnel              # all instances
  claws tunnel alice bravo  # specific instances`,

	"activity": `Usage: claws activity [--since=2h] [--group=<name>] [--limit=50]

Show recent activity across instances (file changes, log errors).

Options:
  --since=<duration>   Time window (default: 2h)
  --group=<name>       Filter to a specific group
  --limit=<n>          Maximum entries (default: 50)`,

	"share": `Usage: claws share <name> --skills|--workspace|--hooks|--all

Enable shared resource mounts for an instance. Requires restart.

Examples:
  claws share alice --skills
  claws share alice --all`,

	"unshare": `Usage: claws unshare <name> --skills|--workspace|--hooks|--all

Disable shared resource mounts for an instance. Requires restart.

Examples:
  claws unshare alice --skills`,

	"migrate": `Usage: claws migrate <name> --to s3 [--cleanup]

Move an instance's workspace to an S3 FUSE mount. Stops the instance,
copies data via rsync, then restarts.

Options:
  --cleanup   Remove local workspace copy after migration

Examples:
  claws migrate alice --to s3
  claws migrate alice --to s3 --cleanup`,

	"stats": `Usage: claws stats

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

// topicHelp maps topic names to detailed guide text.
var topicHelp = map[string]string{
	"setup": `Getting Started with claws

  The fastest way to get running:

    claws quickstart    One-click first agent (idempotent, no flags)
    claws setup         Guided interactive onboarding
    claws init          Manual setup only (creates dirs, policy, access)
    claws apply --file=<profile.json>   Apply a declarative profile
    claws doctor        Verify environment is ready

  After setup, manage your agents:

    claws list          See all agents
    claws start <name>  Start an agent
    claws dashboard     Live status view

  For non-interactive setup (CI/scripts):

    claws setup --non-interactive \
      --team=research --agent=sarah \
      --auth=codex --channel=telegram \
      --telegram-token=TOKEN`,

	"security": `Security Guide

  claws ships with secure defaults. On init/setup, it creates:

    policy.json     Admin security policy (bind modes, limits, DM pairing)
    .access.json    Role-based access control (admin/operator/user)
    .audit.log      Command audit trail (when policy.auditLog = true)

  Key commands:

    claws policy show              Show current policy
    claws policy validate          Check all instances against policy
    claws policy enforce --restart Fix violations and restart affected
    claws access show              Show access roles
    claws access grant <user> <role>  Grant a role
    claws access audit --since=24h Review audit log
    claws audit                    Full security audit
    claws token rotate <name>      Rotate gateway token

  Safe defaults applied on init:

    Network bind:     loopback only (no external access)
    DM policy:        pairing required (no open DMs)
    Outbound msgs:    OFF by default (must enable + add allowFrom)
    Memory limit:     2 GB per instance
    Audit logging:    ON
    Docker socket:    NOT mounted`,

	"channels": `Channel Guide

  Get bot tokens:

    Telegram   t.me/BotFather                         (~2 min)
    Discord    discord.com/developers/applications     (~5 min)
    Slack      api.slack.com/apps                      (~10 min)
    WhatsApp   No token needed (QR scan)               (~5 min)
    Signal     Needs signal-cli + phone number         (~5 min)

  Full step-by-step guides: see html/channels-guide.html

  Connect to an agent:

    claws channel add <name> telegram --token=TOKEN
    claws channel add <name> discord  --token=TOKEN
    claws channel add <name> slack    --bot-token=TOKEN --app-token=TOKEN
    claws channel add <name> whatsapp
    claws channel add <name> signal   --number=+1234567890

  Or inline with create:

    claws create alice --telegram=TOKEN
    claws create alice --discord=TOKEN

  After adding a channel, approve DM pairing:

    1. Message the bot on the platform
    2. Note the pairing code shown
    3. claws approve <name> <channel> <CODE>

  Manage channels:

    claws channel status <name>          Show configured channels
    claws channel security <name>        Show channel security posture
    claws channel send <name> <ch> --enable   Enable outbound messaging
    claws channel allow <name> <ch> <id>      Add approved contact
    claws channel remove <name> <ch>          Disable a channel`,

	"groups": `Groups & Teams Guide

  Groups organize agents into teams with shared resources:

    claws group create <name>        Create a group
    claws group list                 List all groups
    claws group shared <name> --all  Enable shared skills + workspace

  Create agents in a group:

    claws create <group>/<name>              Auto-shares resources
    claws create team/lead --role=manager    Assign manager role
    claws create team/dev1 --role=worker     Assign worker role

  Manager/worker task queue:

    claws task create <group> <title>    Dispatch a task
    claws task list <group>              List tasks
    claws task claim <g> <id> --by=<i>   Claim for a worker
    claws task complete <group> <id>     Mark done`,

	"commands": `Command Quick Reference

  quickstart            One-click first agent (smart defaults, idempotent)
  setup                 Guided interactive onboarding
  apply --file=<f>      Apply a declarative profile (JSON)
  create <name>         Create an agent (supports --auth=, --telegram=, etc.)
  start/stop <name>     Start or stop an agent
  list                  List all agents
  status                System overview (health + policy + access)
  team create <name>    Create a team with shared resources
  channel add <n> <ch>  Connect a messaging channel
  policy show           View security policy
  audit                 Run security audit
  doctor                Check environment health

  Run 'claws help' for the full command list (all 40+ commands).
  Run 'claws <command> --help' for detailed help on any command.

  Help topics:
    claws help setup      Getting started guide
    claws help security   Security and policy guide
    claws help channels   Messaging channel guide
    claws help groups     Groups and teams guide`,
}

// printTopicHelp prints help for a topic, or falls back to subcommand help,
// or lists available topics.
func printTopicHelp(topic string) {
	// Try topic guide first
	if text, ok := topicHelp[topic]; ok {
		bold := "\033[1m"
		nc := "\033[0m"
		fmt.Printf("%s%s%s\n\n", bold, topic, nc)
		fmt.Println(text)
		fmt.Println()
		return
	}

	// Try subcommand help
	if text, ok := subcommandHelp[topic]; ok {
		fmt.Println(text)
		return
	}

	// Unknown topic — list what's available
	fmt.Printf("Unknown help topic: %s\n\n", topic)
	fmt.Println("Available topics:")
	fmt.Println("  setup      Getting started guide")
	fmt.Println("  security   Security and policy guide")
	fmt.Println("  channels   Messaging channel guide")
	fmt.Println("  groups     Groups and teams guide")
	fmt.Println("  commands   Full command reference")
	fmt.Println()
	fmt.Println("Or get help for any command:")
	fmt.Println("  claws <command> --help")
}
