---
name: claws-release
description: Cut a clean patch release of claws (the Go CLI in this repo) via scripts/publish-release.sh — bumps CHANGELOG, builds linux/darwin x amd64/arm64 artifacts, tags, pushes, and optionally creates the GitHub release. Use whenever the user asks to "ship a new claws release", "cut v1.6.X", "release claws", "publish a patch", "bump and ship", "let's release this", or any other request to produce the next claws release. Encodes the v1.6.17+ flow and the empty-CHANGELOG and artifact-ordering landmines from prior releases. Distinct from claws-bootstrap-fresh-box (installing on a new host), claws-debug-agent (diagnosing a broken agent), and claws-add-agent (adding an agent).
---

# claws-release

Cut a patch release of claws cleanly, in one `publish-release.sh` invocation, with no hand-follow-up commits.

## Hard rule: patch bumps only

**claws bumps the patch component only.** `v1.6.X` becomes `v1.6.X+1`. Never bump minor (`v1.7.0`) or major (`v2.0.0`) — the project owner has stated this repeatedly and treats it as a release-policy invariant. If asked to ship a release, always pick the next patch number, even if the changeset feels "big enough" for a minor.

## Quick reference

```bash
# 1. find current version
git tag -l 'v*' | sort -V | tail -1
# 2. populate CHANGELOG [Unreleased] with real notes, commit
# 3. publish
./scripts/publish-release.sh vX.Y.Z+1 --skip-tests --no-gh
# 4. verify
git log --oneline -3
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/release/VERSION
```

## Playbook

### 1. Determine the next version

```bash
git tag -l 'v*' | sort -V | tail -1
```

Bump the patch component by 1. `v1.6.17` becomes `v1.6.18`. That is the version you will pass to `publish-release.sh`.

### 2. Populate `[Unreleased]` in `CHANGELOG.md`

`publish-release.sh` (since v1.6.17) refuses to ship if `[Unreleased]` is empty or only contains the placeholders `_(nothing yet)_` or `_(no changes documented)_`. Populate it BEFORE running the release script.

Use the same shape as recent entries:

```markdown
## [Unreleased]

### Fixed — short imperative summary of the headline change

Body: what changed, why, what bug it fixes. Mention the user-hit
symptom if any. Be specific — not just a file list.

### Added — another grouping if applicable

...

### Honest flag

Anything intentionally out of scope, partial, or punt-to-next-patch.
```

Headings to use under each version: `### Added`, `### Fixed`, `### Changed`, `### Improved`, `### Removed`, or a descriptive heading after the kind word (the `v1.6.17` entry uses `### Fixed — publish-release.sh now ships artifacts...`).

Then commit:

```bash
git add CHANGELOG.md
git commit -m "docs: populate [Unreleased] for vX.Y.Z"
```

(The publish script will make a second `chore(release): prepare vX.Y.Z` commit that promotes `[Unreleased]` → `[vX.Y.Z] — date`. That's expected and separate from this commit.)

### 3. Verify clean working tree

```bash
git status --short
```

Must be empty. The script refuses to run on a dirty tree. Commit or stash anything else first.

### 4. Run `publish-release.sh`

```bash
./scripts/publish-release.sh vX.Y.Z --skip-tests --no-gh
```

Flag choice:

- `--skip-tests` — usually fine if you've verified locally. The integration tests want a free Docker daemon, which often isn't available in the release environment. Skip unless you specifically want to re-run `./scripts/rebuild.sh`.
- `--no-gh` — skip `gh release create`. Required if `gh` is not installed or not authenticated. The tag and artifacts still ship on `main`; only the GitHub Releases page upload is skipped.
- `--no-push` — local-only; tag and commits stay on this machine. Use rarely (debugging the release flow).
- `--dry-run` — preview every step without executing. Use as a sanity check before real runs.

Combine as needed: a typical real run is `./scripts/publish-release.sh v1.6.18 --skip-tests --no-gh`.

### 5. What `publish-release.sh` does (v1.6.17+)

For mental model — do not run these steps by hand, just understand them:

1. Sanity checks (clean tree, version format, no existing tag, branch check, `gh` check).
2. `rebuild.sh` (build + vet + short tests) — skipped if `--skip-tests`.
3. CHANGELOG bump: rewrites `## [Unreleased]` heading into `## [vX.Y.Z] — YYYY-MM-DD`, preserves the body you wrote, resets `[Unreleased]` to `_(nothing yet)_`.
4. Commit: `chore(release): prepare vX.Y.Z`.
5. Build cross-platform artifacts via `release.sh` (linux/darwin × amd64/arm64) into `release/`.
6. Commit: `release: ship vX.Y.Z artifacts (linux/darwin x amd64/arm64)`. **New in v1.6.17** — before this, the tag pointed at a commit without the artifacts and a manual follow-up commit was needed.
7. Annotated tag `vX.Y.Z` pointing at the artifact commit.
8. `git push origin <branch>` then `git push origin vX.Y.Z`.
9. Optional `gh release create` — skipped if `--no-gh`.

### 6. Verify the live release

After the push completes, three checks:

```bash
# tag exists locally
git tag -l 'vX.Y.Z'

# log looks right — top of main should be:
#   release: ship vX.Y.Z artifacts ...
#   chore(release): prepare vX.Y.Z
#   <the feature/fix work that justified this release>
# and there should be NO untracked release/ files
git log --oneline -3
git status --short

# CDN serves the new version
# (allow ~60s for GitHub's raw CDN to propagate; first attempt may
# still return the previous version — retry once or twice)
curl -fsSL https://raw.githubusercontent.com/ab0t-com/claws/main/release/VERSION
```

If all three pass, the release is live.

## Failure modes

### "CHANGELOG.md [Unreleased] section is empty"

The script aborts with a clear error and points you at the fix.

**Fix:** Populate `[Unreleased]` with real notes (see Step 2), commit, retry. This is the right outcome — empty changelogs cause the v1.6.15 / v1.6.16 follow-up-commit cycle.

**Override (rare):** `ALLOW_EMPTY_CHANGELOG=1 ./scripts/publish-release.sh vX.Y.Z ...`. Use only if shipping intentionally empty notes (e.g. an artifact-only re-cut).

### "working tree is not clean"

The script aborts.

**Fix:** `git status --short` to see what's dirty. Commit or stash, then retry. Do NOT pass any "force" flag — there isn't one and there shouldn't be.

### "tag vX.Y.Z already exists"

The script aborts.

**Fix:** That version was already released. Re-run `git tag -l 'v*' | sort -V | tail -1` to confirm the current latest, pick the next patch number, retry.

### Already shipped vX.Y.Z but `[Unreleased]` was empty (pre-v1.6.17 footgun)

Older versions of the script silently shipped empty changelog entries. If you see a published `## [vX.Y.Z] — date` with no body, backfill via a follow-up commit:

```bash
# Edit CHANGELOG.md — add real notes under the existing ## [vX.Y.Z] heading
git add CHANGELOG.md
git commit -m "docs: backfill vX.Y.Z changelog"
git push
```

This is what `9100a02 docs: backfill v1.6.15 changelog` did. v1.6.17+ prevents the situation.

### Hand-committed `release/` artifacts after `publish-release.sh` (pre-v1.6.17 footgun)

Pre-v1.6.17, the script tagged before building artifacts. The tag pointed at a commit WITHOUT the cross-platform binaries; the artifacts arrived on `main` in a separate hand-made commit AFTER the tag. The manual workaround was:

```bash
git add release/
git commit -m "release: ship vX.Y.Z artifacts (linux/darwin x amd64/arm64)"
git push
```

v1.6.17+ does this inside the script, before the tag. If you're on v1.6.17+ and somehow still ended up with untracked `release/` files after running the script, something went wrong — investigate before papering over with a hand commit.

### `gh` not installed or not authenticated

The script detects this in the sanity-checks phase and warns. If you've passed `--no-gh` it proceeds silently. If you haven't and want to keep going, just re-run with `--no-gh`. The release is still real — tag and tarballs land on `main` and are served from `https://raw.githubusercontent.com/ab0t-com/claws/main/release/...`. Only the GitHub Releases page (uploaded tarballs at `releases/download/vX.Y.Z/...`) is skipped.

### CDN serves the old version after push

`raw.githubusercontent.com` caches for ~60s. Wait and retry once or twice. If after a few minutes the CDN still returns the previous version, check `git log --oneline -3 origin/main` to confirm the push actually landed.

## What this skill does not cover

- Installing claws on a new host — see `claws-bootstrap-fresh-box`.
- Diagnosing a broken agent — see `claws-debug-agent`.
- Adding a new agent — see `claws-add-agent`.
- Minor or major version bumps — out of policy. Patch only.
