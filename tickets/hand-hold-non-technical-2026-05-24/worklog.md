# hand-hold-non-technical — worklog

Append-only. Why + what.

---

## 2026-05-24 — Kickoff

### Why

User priority statement: "our users are non-technical, our goal is to
hold their hand and make it as easy as possible."

The v1.6.3 GO path works but assumes phone↔laptop copy-paste fluency.
That's a fundamental design mismatch with the target audience.

### Approach

Two complementary pieces:
1. **`claws paste-secret`** — bridges phone → server for any secret
   value via a single-page local HTTP form. No more multi-device
   copy-paste juggling.
2. **`claws setup` enhancements** — the existing wizard becomes the
   ONE COMMAND a non-technical user runs. Auto-bootstraps image,
   integrates paste-secret for tokens, friendlier prompts.

Existing `claws setup` already has the 6-step skeleton — extending,
not replacing.

### Patch bump

v1.6.3 → v1.6.4 per `feedback_patch_bumps_only` memory.

### Order

1. `paste-secret` standalone first (smaller, no new UX surface in setup).
2. Enhance setup to call paste-secret + friendlier prompts.
3. Doc updates.
4. Tests.
5. Release.

## 2026-05-24 — Shipped (all tasks)

**Task A — paste-secret (`cmd/claws/paste_secret.go`)**
- ~280 LOC, net/http only.
- 7-char random URL token + 6-digit verification code.
- Mobile-friendly HTML form (viewport meta, large touch targets,
  monospace textarea).
- Single-use (server exits on first successful paste).
- 5-minute auto-expire.
- `--bind=127.0.0.1` mode for SSH-tunnel-only.
- LAN IP discovery via `net.Interfaces()` for the URL hint.
- Name validation (no path traversal, no slashes).

**Bugs caught + fixed in dev:**
- Initial validation rule `ContainsAny(name, "/\\..")` rejected single
  dots (so `openai.key` failed). Fixed to use `Contains("..")` + slash
  check.
- Test was hand-rolling HTTP; status-line vs body race. Switched to
  `http.Client.PostForm` — clean.
- Test grep'd for `didn't match` but HTML escapes the apostrophe
  (`&#39;`). Loosened to `Code` + `match`.

**Task B — setup integration**
- Step 1 (prereqs): if image missing AND interactive → offer
  `cmdImageBootstrap([--yes])` inline. Skipped cleanly with warning
  in non-interactive mode.
- Step 6 (channel): when picking telegram/discord/slack, asks "paste
  here vs phone-paste" — phone-paste invokes `cmdPasteSecret` and
  reads back the file.
- Reused existing `prompt` closure for the menu — no new helpers.

**Task C — tests + docs**
- 3 tests in `paste_secret_test.go`: shape/randomness, invalid-name
  rejection, end-to-end HTTP round-trip.
- `docs/goal-instant-claw.md` gained a v1.6.4 banner pointing at
  `claws setup` as the lead path.
- CHANGELOG entry documents the full security model + the why.

**Status: CLOSED. Ready for v1.6.4 tag.**
