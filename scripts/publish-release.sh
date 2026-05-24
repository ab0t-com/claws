#!/bin/bash
# claws publish-release — tag, build, and ship a release
#
# Usage:
#   ./scripts/publish-release.sh <version> [flags]
#
# Example:
#   ./scripts/publish-release.sh v1.0.0
#   ./scripts/publish-release.sh v1.0.0 --dry-run
#   ./scripts/publish-release.sh v1.0.0 --skip-tests
#   ./scripts/publish-release.sh v1.0.0 --no-push   # local only, no remote
#
# Flow:
#   1. Sanity checks (clean tree, on a branch, version format, no existing tag)
#   2. Run rebuild.sh (build + vet + short tests) — skippable
#   3. Update CHANGELOG.md heading from [Unreleased] to the version + today's date
#   4. Commit the changelog bump
#   5. Annotated git tag with the changelog excerpt
#   6. Run release.sh — produces release/ artifacts
#   7. Push branch + tag to origin (unless --no-push)
#   8. Create GitHub release with gh, upload tarballs + SHA256SUMS
#
# Safety:
#   - Refuses to run on a dirty working tree
#   - Refuses to run if the tag already exists
#   - --dry-run prints every action without executing
#   - Stops at the first failure (set -euo pipefail)
#   - Never force-pushes
set -euo pipefail

# =====================================================================
# Config
# =====================================================================
REPO="ab0t-com/claws"
REMOTE="origin"
DEFAULT_BRANCH="main"

# =====================================================================
# Colors
# =====================================================================
BOLD="\033[1m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
RED="\033[0;31m"
DIM="\033[0;90m"
NC="\033[0m"

step()  { echo -e "\n${BOLD}==>${NC} $1"; }
info()  { echo -e "  ${DIM}$1${NC}"; }
ok()    { echo -e "  ${GREEN}✓${NC} $1"; }
warn()  { echo -e "  ${YELLOW}!${NC} $1" >&2; }
die()   { echo -e "  ${RED}✗${NC} $1" >&2; exit 1; }

# =====================================================================
# Args
# =====================================================================
VERSION=""
DRY_RUN=0
SKIP_TESTS=0
NO_PUSH=0
NO_GH=0

for arg in "$@"; do
    case "$arg" in
        --dry-run)    DRY_RUN=1 ;;
        --skip-tests) SKIP_TESTS=1 ;;
        --no-push)    NO_PUSH=1 ;;
        --no-gh)      NO_GH=1 ;;
        -h|--help)
            sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        v*.*.*)       VERSION="$arg" ;;
        *.*.*)        VERSION="v$arg" ;;
        *)            die "unknown argument: $arg (use --help)" ;;
    esac
done

[ -n "$VERSION" ] || die "version required (e.g. v1.0.0)"

# Validate semver-ish format
if ! [[ "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[A-Za-z0-9.-]+)?$ ]]; then
    die "version must be vMAJOR.MINOR.PATCH (got: $VERSION)"
fi

run() {
    if [ "$DRY_RUN" -eq 1 ]; then
        echo "  [dry-run] $*"
    else
        eval "$*"
    fi
}

# =====================================================================
# Locate repo root
# =====================================================================
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

echo -e "${BOLD}claws publish-release${NC}"
info "version:   $VERSION"
info "repo:      $REPO"
info "remote:    $REMOTE"
info "root:      $ROOT"
[ "$DRY_RUN" -eq 1 ] && warn "DRY RUN — no changes will be made"

# =====================================================================
# 1. Sanity checks
# =====================================================================
step "Sanity checks"

# Clean tree?
if [ "$DRY_RUN" -eq 0 ] && [ -n "$(git status --porcelain)" ]; then
    git status --short >&2
    die "working tree is not clean — commit or stash first"
fi
ok "working tree clean"

# Tag doesn't already exist?
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    die "tag $VERSION already exists"
fi
ok "tag $VERSION is available"

# Origin remote configured?
if ! git remote get-url "$REMOTE" >/dev/null 2>&1; then
    warn "remote '$REMOTE' is not configured"
    warn "  add it: git remote add $REMOTE git@github.com:$REPO.git"
    [ "$NO_PUSH" -eq 1 ] || die "use --no-push if you want to skip remote ops"
fi

# Current branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
info "current branch: $CURRENT_BRANCH"
if [ "$CURRENT_BRANCH" != "$DEFAULT_BRANCH" ]; then
    warn "you are not on $DEFAULT_BRANCH — releases normally cut from $DEFAULT_BRANCH"
    if [ "$DRY_RUN" -eq 0 ] && [ "$NO_PUSH" -eq 0 ]; then
        read -p "  Continue anyway? [y/N] " yn
        [[ "$yn" =~ ^[Yy]$ ]] || die "aborted"
    fi
fi
ok "branch check done"

# gh CLI available?
if [ "$NO_GH" -eq 0 ] && [ "$NO_PUSH" -eq 0 ]; then
    if ! command -v gh >/dev/null 2>&1; then
        warn "gh (GitHub CLI) not installed — release will be tagged but not published"
        warn "  install: https://cli.github.com/"
        NO_GH=1
    elif ! gh auth status >/dev/null 2>&1; then
        warn "gh not authenticated — run 'gh auth login'"
        NO_GH=1
    else
        ok "gh CLI ready"
    fi
fi

# =====================================================================
# 2. Tests
# =====================================================================
if [ "$SKIP_TESTS" -eq 1 ]; then
    warn "skipping tests (--skip-tests)"
else
    step "Running build + tests"
    run "\"$ROOT/scripts/rebuild.sh\""
    ok "tests pass"
fi

# =====================================================================
# 3. CHANGELOG bump
# =====================================================================
step "Updating CHANGELOG.md"

CHANGELOG="$ROOT/CHANGELOG.md"
if [ ! -f "$CHANGELOG" ]; then
    warn "CHANGELOG.md not found — skipping bump"
else
    TODAY=$(date -u +%Y-%m-%d)
    if grep -q "^## \[Unreleased\]" "$CHANGELOG"; then
        info "promoting [Unreleased] → [$VERSION] — $TODAY"
        if [ "$DRY_RUN" -eq 0 ]; then
            # Insert new Unreleased section above the old one, then rename old.
            python3 - "$CHANGELOG" "$VERSION" "$TODAY" <<'PYEOF'
import sys, re
path, version, today = sys.argv[1], sys.argv[2], sys.argv[3]
with open(path) as f:
    body = f.read()
new = re.sub(
    r"^## \[Unreleased\].*?(?=^## \[)",
    f"## [Unreleased]\n\n_(nothing yet)_\n\n## [{version}] — {today}\n\n",
    body,
    count=1,
    flags=re.MULTILINE | re.DOTALL,
)
# If the file ends with the Unreleased section (no later release yet), handle that case:
if new == body and "## [Unreleased]" in body:
    new = body.replace(
        "## [Unreleased]\n\n_(nothing yet)_",
        f"## [Unreleased]\n\n_(nothing yet)_\n\n## [{version}] — {today}",
        1,
    )
with open(path, "w") as f:
    f.write(new)
PYEOF
        fi
        ok "CHANGELOG.md updated"
    else
        warn "CHANGELOG.md has no [Unreleased] section — leaving as-is"
    fi
fi

# Extract the release notes for this version (used in tag + gh release)
NOTES_FILE=$(mktemp)
trap 'rm -f "$NOTES_FILE"' EXIT
if [ -f "$CHANGELOG" ] && [ "$DRY_RUN" -eq 0 ]; then
    awk -v v="[$VERSION]" '
        $0 ~ "^## " v {flag=1; next}
        flag && /^## \[/ {exit}
        flag {print}
    ' "$CHANGELOG" > "$NOTES_FILE"
fi
[ -s "$NOTES_FILE" ] || echo "Release $VERSION" > "$NOTES_FILE"

# =====================================================================
# 4. Commit changelog bump
# =====================================================================
if [ "$DRY_RUN" -eq 0 ] && [ -n "$(git status --porcelain CHANGELOG.md 2>/dev/null)" ]; then
    step "Committing CHANGELOG bump"
    run "git add CHANGELOG.md"
    run "git commit -m \"chore(release): prepare $VERSION\""
    ok "changelog commit created"
fi

# =====================================================================
# 5. Annotated tag
# =====================================================================
step "Creating annotated tag $VERSION"
if [ "$DRY_RUN" -eq 1 ]; then
    echo "  [dry-run] git tag -a $VERSION -F <notes>"
else
    git tag -a "$VERSION" -F "$NOTES_FILE"
fi
ok "tag $VERSION created"

# =====================================================================
# 6. Build release artifacts
# =====================================================================
step "Building release artifacts"
run "\"$ROOT/scripts/release.sh\" \"$VERSION\""
ok "artifacts in $ROOT/release/"

# =====================================================================
# 7. Push branch + tag
# =====================================================================
if [ "$NO_PUSH" -eq 1 ]; then
    warn "skipping push (--no-push)"
else
    step "Pushing to $REMOTE"
    run "git push $REMOTE $CURRENT_BRANCH"
    run "git push $REMOTE $VERSION"
    ok "pushed branch + tag"
fi

# =====================================================================
# 8. GitHub release
# =====================================================================
if [ "$NO_GH" -eq 1 ] || [ "$NO_PUSH" -eq 1 ]; then
    warn "skipping GitHub release creation"
    info "manual command:"
    info "  gh release create $VERSION \\"
    info "    --title \"$VERSION\" \\"
    info "    --notes-file <(awk '/## \\[$VERSION\\]/{flag=1;next}flag&&/## \\[/{exit}flag' CHANGELOG.md) \\"
    info "    release/*.tar.gz release/SHA256SUMS"
else
    step "Creating GitHub release"
    run "gh release create \"$VERSION\" \
        --title \"$VERSION\" \
        --notes-file \"$NOTES_FILE\" \
        \"$ROOT/release\"/*.tar.gz \
        \"$ROOT/release/SHA256SUMS\""
    ok "GitHub release created"
fi

# =====================================================================
# Done
# =====================================================================
echo ""
echo -e "${BOLD}Release $VERSION complete.${NC}"
[ "$DRY_RUN" -eq 1 ] && echo -e "  ${YELLOW}(dry-run — nothing was actually done)${NC}"
echo ""
if [ "$NO_PUSH" -eq 0 ]; then
    echo "  Browse: https://github.com/$REPO/releases/tag/$VERSION"
fi
echo ""
