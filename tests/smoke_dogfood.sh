#!/bin/bash
# Smoke / dogfood script for claws (read-only against the host).
#
# SAFETY CONTRACT (see tests/test_plan_2026-05-20.md):
#   - Always uses a throwaway $(mktemp -d) as OPENCLAW_ROOT.
#   - Refuses to run if OPENCLAW_ROOT ever resolves to $HOME/.openclaw.
#   - Sets CLAWS_SKIP_VALIDATE=1 so we never call `docker compose up`.
#   - No `rm -rf`. Tempdir is left on disk for inspection — operator can remove it
#     manually if/when they want.
#
# Run from the repo root:
#   bash tests/smoke_dogfood.sh /tmp/claws-dogfood-binary 2>&1 | tee tests/dogfood_log_$(date +%Y-%m-%d).out
#
# Arg 1: path to the claws binary to test (default: /tmp/claws-dogfood).

set -uo pipefail

BIN="${1:-/tmp/claws-dogfood}"
if [ ! -x "$BIN" ]; then
    echo "FATAL: binary not found or not executable: $BIN" >&2
    exit 1
fi

ROOT="$(mktemp -d -t claws-dogfood-XXXXXX)"
LIVE_ROOT="${HOME}/.openclaw"
if [ "$ROOT" = "$LIVE_ROOT" ] || [ "${ROOT#$LIVE_ROOT/}" != "$ROOT" ]; then
    echo "FATAL: refusing to run — temp root '$ROOT' is under live root '$LIVE_ROOT'" >&2
    exit 1
fi
export OPENCLAW_ROOT="$ROOT"
export CLAWS_SKIP_VALIDATE=1
export CLAWS_BASE_PORT=29789
export USER="${USER:-ubuntu}"

# Make sure the binary can find a compose template — copy it next to the binary
# (claws looks here as one of its search paths).
if [ -f "$(dirname "$BIN")/docker-compose.yml" ]; then
    :
elif [ -f "./docker-compose.yml" ]; then
    cp ./docker-compose.yml "$(dirname "$BIN")/docker-compose.yml" 2>/dev/null || true
fi

BOLD="\033[1m"
DIM="\033[0;90m"
NC="\033[0m"

step() {
    echo ""
    echo -e "${BOLD}# $*${NC}"
    echo -e "${DIM}  OPENCLAW_ROOT=$OPENCLAW_ROOT${NC}"
}

run() {
    echo -e "${DIM}\$ claws $*${NC}"
    "$BIN" "$@"
    echo -e "${DIM}  (exit=$?)${NC}"
}

run_expect_fail() {
    echo -e "${DIM}\$ claws $*  (expect failure)${NC}"
    "$BIN" "$@"
    local rc=$?
    if [ "$rc" -eq 0 ]; then
        echo "  WARN: expected nonzero exit, got 0"
    else
        echo -e "${DIM}  (exit=$rc — expected)${NC}"
    fi
}

echo -e "${BOLD}=== claws smoke / dogfood ===${NC}"
echo "binary: $BIN"
echo "root:   $OPENCLAW_ROOT  (will NOT touch $LIVE_ROOT)"
echo "skip:   CLAWS_SKIP_VALIDATE=$CLAWS_SKIP_VALIDATE"
echo "port:   CLAWS_BASE_PORT=$CLAWS_BASE_PORT"
echo ""

# ---------------------------------------------------------------------------
step "B1 — first-run welcome (uninitialized)"
# Special-case: ROOT exists because mktemp made it. Delete the empty dir so the
# "uninitialized" branch fires. mktemp made it empty; rmdir is safe (empty-only).
rmdir "$ROOT" 2>/dev/null
run
mkdir -p "$ROOT"

step "B2 — claws --version"
run --version

step "B3 — claws help (count sections)"
"$BIN" help | tee /tmp/claws-help.$$ | head -20
SECTIONS=$(grep -c ':$' /tmp/claws-help.$$ 2>/dev/null || echo 0)
LINES=$(wc -l < /tmp/claws-help.$$)
echo "  (help has $LINES lines / $SECTIONS sections)"

for topic in setup security channels groups commands bogus; do
    step "B4-B9 — claws help $topic"
    "$BIN" help "$topic" | head -15
done

step "B10 — claws create --help"
"$BIN" create --help | head -25

# ---------------------------------------------------------------------------
step "C1 — claws init"
run init

step "C1 — verify announced files exist"
for f in .port-registry defaults.json policy.json .access.json shared/skills shared/workspace; do
    if [ -e "$ROOT/$f" ]; then echo "  OK   $f"; else echo "  MISS $f"; fi
done

step "C2 — claws doctor"
run doctor

step "C3 — claws policy show"
run policy show

step "C4 — claws access show"
run access show

step "C5 — claws status (empty system)"
run status

step "C6 — claws init (idempotent)"
run init

# ---------------------------------------------------------------------------
step "D1 — claws create alpha"
run create alpha

step "D2 — claws create bravo"
run create bravo

step "D3 — claws list"
run list

step "D4 — claws list --json"
run list --json

step "D5 — claws status alpha"
run status alpha

step "D6 — claws status alpha --json"
run status alpha --json

step "D7 — claws status (overview)"
run status

step "D8 — claws health alpha (expect down)"
run health alpha

step "D9 — claws health --json"
run health --json

step "D10 — claws tunnel"
run tunnel

step "D11 — claws config show alpha (--no-secrets)"
run config show alpha --no-secrets

step "D12 — claws config get alpha gateway.port"
run config get alpha gateway.port

step "D13 — claws config set alpha tools.profile messaging"
run config set alpha tools.profile '"messaging"'

step "D14 — claws token show alpha"
run token show alpha

step "D15 — claws remove alpha (no purge)"
run remove alpha
echo "  (data should still exist under $ROOT/alpha)"
ls "$ROOT/alpha" 2>/dev/null | head -5

step "D16 — claws remove bravo --purge --yes"
run remove bravo --purge --yes
ls "$ROOT/bravo" 2>/dev/null && echo "  WARN: bravo dir still present" || echo "  OK: bravo dir gone"

step "D17 — port registry should be empty"
cat "$ROOT/.port-registry" 2>/dev/null

# ---------------------------------------------------------------------------
step "E1 — claws team create research"
run team create research

step "E2 — claws create research/sarah"
run create research/sarah

step "E3 — claws create research/lead --role=manager"
run create research/lead --role=manager

step "E4 — claws create research/dev1 --role=worker --manager=lead"
run create research/dev1 --role=worker --manager=lead

step "E5 — re-inspect lead's override (should now mount dev1's workspace)"
cat "$ROOT/research/lead/docker-compose.override.yml" 2>/dev/null

step "E6 — claws group list"
run group list

step "E7 — claws group role research/dev1 none"
run group role research/dev1 none

step "E8 — worker outside group should fail"
run_expect_fail create standalone-worker --role=worker

step "E9 — instance in nonexistent group should fail"
run_expect_fail create nonexistent/foo

# ---------------------------------------------------------------------------
step "F1 — claws task create research"
TASK_OUT="$("$BIN" task create research 'review the docs')"
echo "$TASK_OUT"
TASK_ID="$(echo "$TASK_OUT" | grep -oE 'Task created: [a-f0-9]+' | awk '{print $3}')"
echo "  (parsed task id: $TASK_ID)"

step "F2 — claws task list research"
run task list research

if [ -n "${TASK_ID:-}" ]; then
    step "F3 — claws task claim"
    run task claim research "$TASK_ID" --by=dev1
    step "F4 — claws task list --status=claimed"
    run task list research --status=claimed
    step "F5 — claws task complete"
    run task complete research "$TASK_ID" --result=done
    step "F6 — claws task status"
    run task status research "$TASK_ID"
fi

# ---------------------------------------------------------------------------
step "G1 — claws channel add research/sarah telegram --token=test:abc"
run channel add research/sarah telegram --token=test:abc

step "G2 — claws channel status research/sarah"
run channel status research/sarah

step "G3 — claws channel security research/sarah telegram"
run channel security research/sarah telegram

step "G4 — claws channel allow research/sarah telegram 123456"
run channel allow research/sarah telegram 123456

step "G5 — claws channel send research/sarah telegram --enable"
run channel send research/sarah telegram --enable

step "G6 — claws channel deny research/sarah telegram 123456"
run channel deny research/sarah telegram 123456

step "G7 — claws channel remove research/sarah telegram"
run channel remove research/sarah telegram

# ---------------------------------------------------------------------------
step "H1 — claws policy validate"
run policy validate

step "H2 — bind=wan should be rejected by policy"
run_expect_fail create exposed --bind=wan

step "H3 — disallowed image should be rejected"
run_expect_fail create badimg --image=evil/foo:latest

step "H4 — claws access show"
run access show

step "H5 — claws access grant deploy-bot operator"
run access grant deploy-bot operator

step "H6 — claws access audit --since=24h (this session)"
run access audit --since=24h

step "H7 — claws audit (security-audit.sh fallback)"
run audit

# ---------------------------------------------------------------------------
step "I1 — claws runtime list"
run runtime list

step "I2 — claws runtime show openclaw"
run runtime show openclaw

step "I3 — claws runtime add nemoclaw --from=openclaw --image=nemoclaw:latest"
run runtime add nemoclaw --from=openclaw --image=nemoclaw:latest

step "I4 — claws runtime show nemoclaw --json"
run runtime show nemoclaw --json

step "I5 — claws runtime export nemoclaw"
"$BIN" runtime export nemoclaw | head -20

step "I6 — claws runtime init demo"
run runtime init demo

step "I7 — claws runtime remove nemoclaw"
run runtime remove nemoclaw

step "I8 — cannot remove built-in"
run_expect_fail runtime remove openclaw

# ---------------------------------------------------------------------------
step "J1-J3 — invalid names"
run_expect_fail create MyBot
run_expect_fail create shared
run_expect_fail create 1bot

step "J4-J5 — operations on missing instance"
run_expect_fail status nope
run_expect_fail remove nope

step "J7 — group remove with members (no --purge)"
run_expect_fail group remove research

step "J8 — proxy setup without --domain"
run_expect_fail proxy setup

# ---------------------------------------------------------------------------
echo ""
echo -e "${BOLD}=== smoke complete ===${NC}"
echo "test root preserved at: $OPENCLAW_ROOT"
echo "(remove manually if you want — this script will never delete it)"
