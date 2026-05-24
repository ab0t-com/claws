# claws templates — v1 schema reference

Templates are declarative JSON profiles that `claws apply` reconciles your host to match. This README is the authoritative reference for every field in the v1 schema (`apiVersion: "claws.ab0t.com/v1"`).

## Quick start

```bash
# Apply a bundled template
claws apply --template=solo

# Apply a local file
claws apply --file=./my-profile.json

# Preview (no mutations)
claws apply --template=solo --dry-run

# Skip the auto-audit at the end
claws apply --template=solo --skip-audit

# List templates discoverable on this host
claws template list

# Print a template's source for inspection
claws template show solo-telegram-coder
```

## Template resolution

`--template=<name>` is resolved in this order, first match wins:

1. `./templates/<name>.json` (CWD)
2. `$XDG_DATA_HOME/claws/templates/<name>.json` (default: `~/.local/share/claws/templates/`)
3. `<dir-of-binary>/templates/<name>.json` (extracted-tarball local installs)

## Top-level schema

```json
{
  "apiVersion": "claws.ab0t.com/v1",          // required, validated
  "kind": "Profile",                          // required, validated
  "metadata": { ... },                        // see Metadata
  "runtime": { ... },                         // optional
  "policy": { ... },                          // optional — applied to policy.json
  "team": { "name": "default" },              // required
  "agents": [ { ... } ],                      // required — at least one
  "postSetup": ["audit", "verify --all"],     // optional
  "warnings": ["needs sudo"]                  // optional — `apply` prompts on these
}
```

## Metadata

```json
"metadata": {
  "name": "my-profile",          // identifier
  "version": "1.0.0",            // semver
  "description": "What this does",
  "author": "your-handle",
  "license": "MIT",
  "tags": ["solo", "telegram"]   // for marketplace filtering
}
```

## Runtime

Selects which adapter (`openclaw`, `ab0t-sandbox`, custom) the agents use, and optionally pins to a specific image.

```json
"runtime": {
  "name": "openclaw",                    // adapter name (must be registered)
  "image": "openclaw:v2026.3.25"         // image tag — passed to cmdCreate via --image
}
```

## Policy

Maps to `~/.claws-workspace/policy.json` via `writePolicy`. Each field becomes a host-wide policy constraint that `claws audit` and `claws policy enforce` check against.

```json
"policy": {
  "loopbackOnly": true,           // → allowedBindModes = ["loopback"]
  "dmDefault": "pairing",         // pairing|allowlist|open → requireDmPairing
  "outboundDefault": "off"        // off|allowlist|open → requireOutboundAllowlist
}
```

## Team

```json
"team": {
  "name": "default",      // group name (required)
  "shared": true          // shared workspace dir (default true; false skips)
}
```

## Agents

Each agent gets created, configured, and optionally authenticated + channel-connected.

```json
"agents": [
  {
    "name": "agent-1",                   // required, validated
    "role": "manager",                   // optional: manager|worker
    "manager": "lead",                   // optional: name of manager (worker only)
    "image": "openclaw:v2026.3.25",      // optional: per-agent runtime image override

    "sandbox": true,                     // → agents.defaults.sandbox (DEFAULT: enabled)
    "tools": {
      "profile": "coding",               // → tools.profile
      "allow": ["bash", "edit"],         // → tools.allow
      "deny": ["network"]                // → tools.deny
    },
    "skills": ["calendar", "memory"],    // → workspace/skills/MANIFEST.txt
    "hooks": {                           // → workspace/hooks/<event>.sh
      "onStart": "echo booted",
      "onIdle":  "echo heartbeat"
    },
    "config": {                          // → cmdConfig set (catch-all)
      "agents.defaults.maxTokens": 4096,
      "memory.enabled": true
    },

    "auth": {
      "preferred": "codex",              // codex|apikey
      "fallbackApiKey": {                // applied non-interactively if env/file resolves
        "provider": "openai",
        "fromEnv": "OPENAI_API_KEY",
        "fromFile": "/etc/claws/secrets/openai.key"
      }
    },

    "channels": [
      {
        "type": "telegram",
        "tokenFrom": { "env": "TELEGRAM_BOT_TOKEN" },
        "dmPolicy": "pairing"
      },
      {
        "type": "slack",
        "botTokenFrom": { "env": "SLACK_BOT_TOKEN" },
        "appTokenFrom": { "env": "SLACK_APP_TOKEN" },
        "dmPolicy": "allowlist"
      }
    ]
  }
]
```

## Secret references

Every credential field uses a `SecretRef` — resolved at apply-time, never serialized.

```json
{ "env": "TELEGRAM_BOT_TOKEN" }
{ "file": "/etc/claws/secrets/telegram.token" }
{ "command": ["vault", "kv", "get", "-field=token", "secret/claws/telegram"] }
```

If the reference doesn't resolve, the dependent step is skipped with a clear warning — not a failure.

## PostSetup

After agents + channels + auth, apply runs each verb. Currently supported:

```json
"postSetup": [
  "audit",            // claws audit
  "verify --all",     // claws auth verify (all instances)
  "start --all"       // claws start <each>
]
```

Unknown verbs are skipped with a warning.

## Idempotence guarantees

`claws apply` is **idempotent** — re-running with the same profile converges to the same state:

| Step | Behavior on re-run |
|---|---|
| init / policy init / access init | Skipped if already done |
| policy block | Reapplied (writePolicy is overwrite-safe; honoring latest profile) |
| group create | Skipped if exists |
| agent create | Skipped if exists |
| sandbox / tools / config | Reapplied via `cmdConfig set` (overwrite-safe) |
| skills | Manifest re-written only if changed |
| hooks | Hook scripts re-written only if changed |
| channel add | Skipped if `openclaw.json` `channels.<type>.enabled == true` |
| auth apikey | Short-circuited by `cmdAuth`'s own idempotence (`already authed`) |
| postSetup verbs | Re-run each time (audit / verify are read-only; start is no-op when running) |

## Schema validation

v1.3 validates: `apiVersion`, `kind`, presence of `team.name`, at least one agent, valid agent names. Unknown fields are silently ignored (so adding new fields in v1.4+ is backward-compatible). For stricter validation, use `claws apply --dry-run` and review the planned actions.

## Examples

See bundled templates:

- `solo.json` — bare minimum, one agent
- `solo-telegram-coder.json` — coding bot, Telegram, Codex+OpenAI fallback
- `personal-assistant.json` — full personal AI with skills/hooks/memory

## Reporting issues

Open at https://github.com/ab0t-com/claws/issues with `template:` prefix.
