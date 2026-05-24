# hand-hold-non-technical

**Filed:** 2026-05-24
**Target:** v1.6.4 (patch bump)
**Status:** Open
**Severity:** P0 — design constraint from project owner: "our users are
non-technical, our goal is to hold their hand and make it as easy as
possible."

## Background

v1.6.3 ships the GO path: install + image bootstrap + setup-secrets +
two token pastes + apply + start. Each step works, but the path
**assumes the user can copy-paste a 46-char Telegram bot token from
their phone into a server file via SSH**. That's a non-technical
deal-breaker.

User flagged: "that's a real design problem how are we meant to get a
really long token from a phone to the setup?" — followed by the
priority statement above.

## Goal

A non-technical user — someone who's never used SSH, never opened a
terminal at home, never edited a config file — can get a personal
Telegram bot online by following on-screen prompts only. They never
need to:

- Switch between phone and laptop to copy a token
- Edit a file by hand
- Know what `chmod`, `--secrets-dir`, or `apply` means
- Read documentation longer than the prompt they're answering

`claws setup` is the one command they type. Everything else is
prompts + URL clicks + the wizard polling for their input.

## Scope — In

### Task A — `claws paste-secret <name>` (new command)

Ephemeral local HTTP server that bridges phone → server for secret
values. Used by the wizard for token entry; also standalone.

- New file `cmd/claws/paste_secret.go`.
- Args: `<name>` (required, e.g. `telegram.token`), `--secrets-dir=<path>`
  (default `/tmp/claws-secrets`), `--port=<n>` (default `8765`),
  `--bind=<addr>` (default `0.0.0.0`; `--bind=127.0.0.1` for SSH-tunnel
  only), `--timeout=<dur>` (default `5m`).
- Generates: random 7-char URL token + 6-digit verification code.
- Prints to terminal:
  ```
  Open on your phone:
      http://<your-ip>:8765/<random-token>
  Enter this code on the page:
      417-302
  Listening... (5 min timeout, Ctrl-C to cancel)
  ```
- Serves a single HTML page: textarea + code field + submit. Mobile-friendly
  (viewport meta, large touch targets, monospace font for value).
- On successful POST with matching code: writes to
  `<secrets-dir>/<name>`, server exits 0.
- Single-use, 5-min auto-expire, mismatched code rejected.
- HTTPS not required for ephemeral local-network paste — URL is the secret.
- No new dependencies (Go `net/http` only).

### Task B — Enhance `claws setup` for hand-hold

Existing 6-step structure stays. Enhancements per step:

- **Step 1 (prereqs)**: if `openclaw:local` not present, offer to run
  `image bootstrap --yes` inline. With clear "this takes ~5 minutes,
  it's a one-time download/build."
- **Step 5 (auth)**: when user picks apikey, ask if they have the key
  handy. If no, offer to wait while they paste it via
  `paste-secret openai.key`.
- **Step 6 (channel)**: same — when user picks Telegram, offer the
  paste-secret bridge. Print the BotFather URL + step-by-step
  instructions (the same prose from `quickstart_guide.md`).
- **Friendlier prompts throughout**: replace technical terms.
  "Team" → "what should your group of agents be called?".
  "Agent" → "what should you call your first bot?".
- **Resumable**: if interrupted (Ctrl-C), re-running picks up where
  it left off (it already does this via idempotent steps; just make
  the messaging clear: "you've already done X, picking up at Y").
- **Final success message**: after start, show the bot URL the
  user can DM (`t.me/<bot-username>`).

### Task C — Docs

- Update `docs/goal-instant-claw.md` to mention `claws setup` as the
  primary path (paste-tokens approach is now the secondary path for
  experienced users).
- Update `quickstart_guide.md` to lead with `claws setup`, with the
  manual two-paste approach as a "if you'd rather" alternative.
- Add `claws paste-secret` to the help index + per-command help.

## Scope — Out

- Auto-clicking the BotFather link / scripting Telegram. We can't —
  Telegram doesn't provide a programmatic bot-creation API for new
  users.
- HTTPS for the paste server. Out of scope for ephemeral local-network
  use. Operator can use `--bind=127.0.0.1` + SSH port-forward if they
  want network-level protection.
- Persistent storage promotion (`/tmp` → `~/.config/claws/secrets/`).
  Separate doc-only follow-up.
- QR code generation. Mobile users tap URLs; QR adds dependency for
  marginal benefit.

## Acceptance criteria

1. A user who has never opened a terminal before can complete
   `claws setup` by following on-screen prompts only, ending up with
   a responding bot on Telegram. (Test by handing it to someone.)
2. `claws paste-secret telegram.token` prints a URL + code, listens,
   accepts the form submission, writes the file, exits 0.
3. The same paste-secret URL/code combo can't be replayed (single-use).
4. After timeout, server exits non-zero.
5. `claws setup` integrates the paste-secret flow seamlessly — user
   doesn't need to know it's two separate commands.

## Estimate

~350 LOC + tests + docs. ~2 hours focused.
