# day-one-friction

**Filed:** 2026-05-24
**Target:** v1.6.1 (patch bump)
**Status:** Open
**Severity:** P0 — three concrete failures every brand-new user hits

## Background

Day-one audit of v1.6.0 turned up three friction points that gate a
new user from going from `curl install.sh | sh` to a responding bot:

1. **OpenClaw image not bootstrapped.** `claws doctor` says
   "Image `openclaw:local` not found" and offers no path to fix it.
   User has to know to clone openclaw + `docker build` separately.
   Single biggest "I followed the install and nothing happened"
   failure mode.

2. **`claws apply` silently no-ops on missing secrets.** Apply a
   profile with `OPENAI_API_KEY` unset and the auth step prints
   "✓ skipped — secret env:OPENAI_API_KEY did not resolve" then
   exits 0. User has no idea why their agent doesn't respond.

3. **No way to verify "is my agent actually responding".** After
   `claws start`, the only signal is whether messaging the bot on
   Telegram gets a reply. If silent → was it auth? channel? image?
   network? Operator has to grep logs.

Three small, well-scoped fixes close all three.

## Goal

A brand-new user's path from `curl install.sh | sh` to a working
agent has zero silent failures. Every step that can't be auto-done
fails loud with the exact next command.

## Scope — In

### Task 3 — `claws agent ping <name>` (smallest, no external deps)

- Reads gateway port from instance.env, hits `/healthz` and `/readyz`.
- Calls `cmdAuthVerify` for the auth-liveness check.
- Tails last 30s of logs.
- Renders one screen: gateway up Y/N, ready Y/N, auth verified Y/N,
  channel configured Y/N, last log lines.
- Single-line summary at the bottom: "✓ healthy" / "✗ <thing> wrong".
- Exit non-zero on any check failure.
- Pure read-only.

### Task 2 — Missing-env detection in `claws apply`

- Walk the profile at apply-time. Collect every `SecretRef.Env` and
  `SecretRef.File` reference (skill, hook, channel token, apikey fallback).
- Pre-check: `os.Getenv` returns non-empty for every Env; `os.Stat`
  succeeds for every File. (Skip `command:` refs — can't pre-check.)
- If any missing: **fail at parse-time, before mutating state**. List
  every missing secret with provider URL hint where known
  (`OPENAI_API_KEY` → platform.openai.com/api-keys, etc.).
- Flag `--allow-missing` keeps v1.5/v1.6.0 behavior (skip step, exit 0).

### Task 1 — `claws image bootstrap [--source=<url>] [--no-build]`

- New command. Goal: zero-thought way to get `openclaw:local` onto the
  host.
- Order of operations:
  1. If `docker image inspect openclaw:local` works → "already
     present, nothing to do", exit 0.
  2. Try `docker pull` from `OPENCLAW_IMAGE_SOURCE` env (or `--source=`
     flag). Default empty for now since OpenClaw doesn't publish to a
     public registry yet.
  3. If pull skipped/fails and `--no-build` not set → offer to
     `git clone github.com/openclaw/openclaw /tmp/openclaw-build &&
     docker build -t openclaw:local /tmp/openclaw-build`. Show
     command first; require `--yes` to actually run.
- Idempotent: re-running when image exists is a no-op.

## Scope — Out

- Skills catalog browser — deferred.
- Telegram BotFather wizard — deferred (quickstart already shows the URL).
- Friendlier audit modes — deferred.
- Runtime image auto-upgrade — separate v1.7+ ticket.

## Acceptance

1. `claws agent ping <name>` on a healthy agent shows all green and
   exits 0. On an agent without auth, prints "auth not configured" red
   and exits non-zero.
2. `claws apply --template=solo-telegram-coder` with `OPENAI_API_KEY`
   and `TELEGRAM_BOT_TOKEN` unset fails at parse-time with a table
   listing both missing vars + their provider URLs.
3. `claws image bootstrap` on a host without `openclaw:local`
   walks the user through pull/build with a clear next step.

## Estimate

~190 LOC + tests + CHANGELOG. ~1 hour focused.
