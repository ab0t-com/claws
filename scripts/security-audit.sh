#!/bin/bash
# clawctl security audit — checks your deployment against best practices
# Usage: ./scripts/security-audit.sh [OPENCLAW_ROOT]
#
# Each check explains WHAT it found and WHY it matters in plain language.
set -uo pipefail

ROOT="${1:-${OPENCLAW_ROOT:-$HOME/.openclaw}}"
BOLD="\033[1m"
NC="\033[0m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
RED="\033[0;31m"
DIM="\033[0;90m"

PASS=0; WARN=0; FAIL=0

pass() { echo -e "  [${GREEN}PASS${NC}] $1"; ((PASS++)); }
warn() { echo -e "  [${YELLOW}WARN${NC}] $1"; ((WARN++)); }
fail() { echo -e "  [${RED}FAIL${NC}] $1"; ((FAIL++)); }
hint() { echo -e "         ${DIM}→ $1${NC}"; }

echo -e "${BOLD}clawctl security audit${NC}"
echo -e "Root: $ROOT"
echo -e "Date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# =========================================================================
echo -e "${BOLD}1. Secret File Protection${NC}"
echo -e "   ${DIM}Your instance.env files contain gateway tokens (passwords).${NC}"
echo -e "   ${DIM}If other users on this server can read them, they can control your agents.${NC}"
echo ""
# =========================================================================

for f in "$ROOT"/*/instance.env "$ROOT"/*/*/instance.env; do
    [ -f "$f" ] || continue
    inst=$(echo "$f" | sed "s|$ROOT/||" | sed 's|/instance.env||')
    perm=$(stat -c '%a' "$f" 2>/dev/null)
    if [ "$perm" = "600" ]; then
        pass "$inst — only you can read the config"
    else
        fail "$inst — other users on this server can read your gateway token"
        hint "Fix: chmod 600 $f"
        hint "Or run: clawctl doctor --fix"
    fi
done

if [ -f "$ROOT/.port-registry" ]; then
    perm=$(stat -c '%a' "$ROOT/.port-registry" 2>/dev/null)
    if [ "$perm" = "600" ]; then
        pass "Port registry protected"
    else
        warn "Port registry readable by other users (shows which ports your agents use)"
        hint "Fix: chmod 600 $ROOT/.port-registry"
    fi
fi

cred_bad=0
for f in $(find "$ROOT" -path "*/credentials/*" -type f -not -perm 600 2>/dev/null); do
    ((cred_bad++))
done
if [ "$cred_bad" -eq 0 ]; then
    pass "All channel credentials protected (WhatsApp keys, Telegram tokens, etc.)"
else
    fail "$cred_bad credential files can be read by other users on this server"
    hint "These include WhatsApp session keys, Telegram auth, etc."
    hint "Fix: find $ROOT -path '*/credentials/*' -type f -exec chmod 600 {} +"
    hint "Or run: clawctl doctor --fix"
fi

echo ""

# =========================================================================
echo -e "${BOLD}2. Network Exposure${NC}"
echo -e "   ${DIM}Your agents have web interfaces. If they're accessible from the internet,${NC}"
echo -e "   ${DIM}anyone who finds the port can see the control panel.${NC}"
echo ""
# =========================================================================

public_ports=0
while IFS=: read -r idx name; do
    name=$(echo "$name" | tr -d ' ')
    ref_dir=""
    if echo "$name" | grep -q '/'; then
        group=$(echo "$name" | cut -d/ -f1)
        inst=$(echo "$name" | cut -d/ -f2)
        ref_dir="$ROOT/$group/$inst"
    else
        ref_dir="$ROOT/$name"
    fi
    [ -f "$ref_dir/instance.env" ] || continue
    port=$(grep '^OPENCLAW_GATEWAY_PORT=' "$ref_dir/instance.env" | cut -d= -f2)
    bind=$(grep '^OPENCLAW_GATEWAY_BIND=' "$ref_dir/instance.env" | cut -d= -f2)

    if ss -tlnp 2>/dev/null | grep -q "0.0.0.0:${port} "; then
        if [ "$bind" = "loopback" ] || [ "$bind" = "127.0.0.1" ]; then
            fail "$name (:$port) is accessible from the network even though it's configured for local-only"
            hint "The container is still running with the old config. Restart it:"
            hint "clawctl restart $name"
        else
            warn "$name (:$port) is accessible from your network (and possibly the internet)"
            hint "Anyone on your network (or the internet if no firewall) can reach this agent."
            hint "For local-only access: clawctl policy enforce --restart"
            hint "For SSH tunnel access: clawctl tunnel $name"
        fi
        ((public_ports++))
    else
        pass "$name (:$port) only accessible locally (via SSH tunnel)"
    fi
done < "$ROOT/.port-registry"

if command -v ufw &>/dev/null && ufw status 2>/dev/null | grep -q "active"; then
    pass "Firewall is active (UFW)"
elif iptables -L -n 2>/dev/null | grep -q "DROP\|REJECT"; then
    pass "Firewall has blocking rules"
else
    if [ "$public_ports" -gt 0 ]; then
        warn "No firewall detected — your agent ports may be open to the internet"
        hint "If this server has a public IP, anyone can access your agents."
        hint "Recommended: ufw allow 22/tcp && ufw enable"
    fi
fi

echo ""

# =========================================================================
echo -e "${BOLD}3. Container Isolation${NC}"
echo -e "   ${DIM}Each agent runs in a Docker container. These checks ensure a compromised${NC}"
echo -e "   ${DIM}agent can't escape to your server or affect other agents.${NC}"
echo ""
# =========================================================================

for container in $(docker ps --format '{{.Names}}' 2>/dev/null | grep openclaw); do
    # Extract friendly name
    friendly=$(echo "$container" | sed 's/openclaw-//' | sed 's/-openclaw-gateway-1//')
    echo -e "  ${BOLD}$friendly${NC}"

    user=$(docker inspect "$container" --format '{{.Config.User}}' 2>/dev/null)
    if [ "$user" = "node" ] || [ "$user" = "1000" ]; then
        pass "  Runs as regular user (not root)"
    elif [ -z "$user" ] || [ "$user" = "root" ] || [ "$user" = "0" ]; then
        fail "  Runs as root — a compromised agent could take over your server"
        hint "The container should run as a non-root user (e.g., 'node')."
    else
        pass "  Runs as user '$user'"
    fi

    priv=$(docker inspect "$container" --format '{{.HostConfig.Privileged}}' 2>/dev/null)
    if [ "$priv" = "false" ]; then
        pass "  No special system access"
    else
        fail "  Has FULL system access (privileged mode) — extremely dangerous"
        hint "A compromised agent has root-equivalent access to your entire server."
    fi

    caps=$(docker inspect "$container" --format '{{.HostConfig.CapDrop}}' 2>/dev/null)
    if echo "$caps" | grep -qi "all"; then
        pass "  System capabilities restricted"
    elif [ "$caps" = "[]" ]; then
        warn "  Has default system capabilities (can do more than it needs to)"
        hint "An agent only needs to run Node.js. Extra capabilities increase risk if compromised."
        hint "Fix: restart with updated docker-compose.yml (adds cap_drop: ALL)"
    fi

    secopt=$(docker inspect "$container" --format '{{.HostConfig.SecurityOpt}}' 2>/dev/null)
    if echo "$secopt" | grep -q "no-new-privileges"; then
        pass "  Cannot escalate privileges"
    else
        warn "  Could potentially escalate privileges if exploited"
        hint "Fix: restart with updated docker-compose.yml (adds no-new-privileges)"
    fi

    mem=$(docker inspect "$container" --format '{{.HostConfig.Memory}}' 2>/dev/null)
    if [ "$mem" != "0" ] && [ -n "$mem" ]; then
        pass "  Memory limited to $(( mem / 1048576 ))MB"
    else
        warn "  No memory limit — a runaway agent could use all server RAM"
        hint "Fix: restart with updated docker-compose.yml (adds memory limit)"
    fi

    mounts=$(docker inspect "$container" --format '{{range .HostConfig.Binds}}{{.}}|{{end}}' 2>/dev/null)
    if echo "$mounts" | grep -q "docker.sock"; then
        fail "  Has access to Docker — can control ALL containers on this server"
        hint "Docker socket access = root access. Only enable for sandbox mode with extreme caution."
    else
        pass "  No Docker access (isolated)"
    fi
done

echo ""

# =========================================================================
echo -e "${BOLD}4. Agent Authentication${NC}"
echo -e "   ${DIM}Each agent has a web interface. These checks ensure only you can control it.${NC}"
echo ""
# =========================================================================

while IFS=: read -r idx name; do
    name=$(echo "$name" | tr -d ' ')
    ref_dir=""
    if echo "$name" | grep -q '/'; then
        group=$(echo "$name" | cut -d/ -f1)
        inst=$(echo "$name" | cut -d/ -f2)
        ref_dir="$ROOT/$group/$inst"
    else
        ref_dir="$ROOT/$name"
    fi
    [ -f "$ref_dir/instance.env" ] || continue
    port=$(grep '^OPENCLAW_GATEWAY_PORT=' "$ref_dir/instance.env" | cut -d= -f2)
    token=$(grep '^OPENCLAW_GATEWAY_TOKEN=' "$ref_dir/instance.env" | cut -d= -f2)

    if [ -z "$token" ]; then
        fail "$name — no password set for the web interface"
        hint "Anyone who can reach the port can fully control this agent."
    elif [ ${#token} -lt 32 ]; then
        warn "$name — weak password (${#token} chars, should be 64)"
    else
        pass "$name — strong authentication token set"
    fi

    if command -v python3 &>/dev/null; then
        code=$(python3 -c "
import urllib.request
try:
    r = urllib.request.urlopen('http://127.0.0.1:$port/', timeout=3)
    print(r.getcode())
except:
    print('unreachable')
" 2>&1)
        if [ "$code" = "200" ]; then
            warn "$name (:$port) web interface loads without entering a password"
            hint "The HTML page is served to anyone. WebSocket commands require the token,"
            hint "but the UI is visible. This is an OpenClaw default, not a clawctl issue."
        elif [ "$code" = "401" ] || [ "$code" = "403" ]; then
            pass "$name (:$port) requires authentication"
        fi
    fi
done < "$ROOT/.port-registry"

echo ""

# =========================================================================
echo -e "${BOLD}5. Agent Permissions${NC}"
echo -e "   ${DIM}Agents can run tools (file access, commands, web browsing).${NC}"
echo -e "   ${DIM}These checks ensure agents aren't given more power than they need.${NC}"
echo ""
# =========================================================================

for cfg in "$ROOT"/*/openclaw.json "$ROOT"/*/*/openclaw.json; do
    [ -f "$cfg" ] || continue
    inst=$(echo "$cfg" | sed "s|$ROOT/||" | sed 's|/openclaw.json||')

    sandbox=$(python3 -c "
import json
c = json.load(open('$cfg'))
s = c.get('agents',{}).get('defaults',{}).get('sandbox',None)
print(s if s else 'not set')
" 2>/dev/null)

    profile=$(python3 -c "
import json
c = json.load(open('$cfg'))
print(c.get('tools',{}).get('profile','not set'))
" 2>/dev/null)

    if [ "$sandbox" != "not set" ] && [ "$sandbox" != "None" ]; then
        pass "$inst — agent runs in a sandbox (isolated environment)"
    else
        warn "$inst — agent can run commands directly on the server"
        hint "Without sandbox mode, the agent can read/write files and run commands"
        hint "as the container user. If someone tricks the agent with a bad prompt,"
        hint "it could do unintended things."
        hint "Enable sandbox: clawctl config set $inst agents.defaults.sandbox true"
    fi

    if [ "$profile" = "not set" ]; then
        warn "$inst — no tool restrictions (agent can use all available tools)"
        hint "Consider setting a tool profile to limit what the agent can do."
        hint "Example: clawctl config set $inst tools.profile '\"coding\"'"
    else
        pass "$inst — tool access restricted to '$profile' profile"
    fi
done

echo ""

# =========================================================================
echo -e "${BOLD}6. Messaging Security${NC}"
echo -e "   ${DIM}Your agents are connected to messaging apps. These checks ensure${NC}"
echo -e "   ${DIM}strangers can't message your agents and get responses.${NC}"
echo ""
# =========================================================================

for cfg in "$ROOT"/*/openclaw.json "$ROOT"/*/*/openclaw.json; do
    [ -f "$cfg" ] || continue
    inst=$(echo "$cfg" | sed "s|$ROOT/||" | sed 's|/openclaw.json||')

    python3 -c "
import json, sys
c = json.load(open('$cfg'))
channels = c.get('channels', {})
for ch_name, ch_cfg in channels.items():
    if not isinstance(ch_cfg, dict): continue
    if not ch_cfg.get('enabled', False): continue
    dm = ch_cfg.get('dmPolicy', 'not set')
    if dm == 'open':
        print(f'FAIL|{ch_name} on $inst — ANYONE can message this agent and get responses')
        print(f'HINT|This means random strangers can use your AI agent (and your API credits).')
        print(f'HINT|Fix: clawctl config set $inst channels.{ch_name}.dmPolicy \"pairing\"')
    elif dm == 'pairing':
        print(f'PASS|{ch_name} on $inst — new senders must be approved with a code')
    elif dm == 'allowlist':
        allow = ch_cfg.get('allowFrom', [])
        if len(allow) == 0:
            print(f'WARN|{ch_name} on $inst — allowlist is empty (nobody can message)')
            print(f'HINT|Add allowed senders or switch to pairing mode.')
        else:
            print(f'PASS|{ch_name} on $inst — only {len(allow)} approved sender(s)')
    else:
        print(f'WARN|{ch_name} on $inst — DM policy is \"{dm}\" (unknown)')
" 2>/dev/null | while IFS='|' read -r level msg; do
        case "$level" in
            PASS) pass "$msg" ;;
            WARN) warn "$msg" ;;
            FAIL) fail "$msg" ;;
            HINT) hint "$msg" ;;
        esac
    done
done

echo ""

# =========================================================================
echo -e "${BOLD}7. Docker Image${NC}"
echo -e "   ${DIM}Your agents run from a Docker image. This checks it exists and is safe.${NC}"
echo ""
# =========================================================================

image=$(grep 'OPENCLAW_IMAGE=' "$ROOT"/*/instance.env "$ROOT"/*/*/instance.env 2>/dev/null | head -1 | cut -d= -f2)
image=${image:-openclaw:local}

if docker image inspect "$image" &>/dev/null; then
    pass "Image '$image' found"
    img_user=$(docker image inspect "$image" --format '{{.Config.User}}' 2>/dev/null)
    if [ -z "$img_user" ] || [ "$img_user" = "root" ] || [ "$img_user" = "0" ]; then
        warn "Image runs as root by default (Docker compose overrides this to 'node')"
    else
        pass "Image runs as non-root user '$img_user'"
    fi
else
    fail "Image '$image' not found — agents can't start without it"
    hint "Build it: cd <openclaw-repo> && docker build -t openclaw:local ."
fi

echo ""

# =========================================================================
echo -e "${BOLD}SUMMARY${NC}"
# =========================================================================
echo ""
echo -e "  ${GREEN}$PASS passed${NC}, ${YELLOW}$WARN warnings${NC}, ${RED}$FAIL failures${NC}"
echo ""

if [ "$FAIL" -gt 0 ]; then
    echo -e "  ${RED}$FAIL issue(s) need immediate attention.${NC}"
    echo -e "  ${DIM}Run: clawctl doctor --fix     (fixes file permissions)${NC}"
    echo -e "  ${DIM}Run: clawctl policy enforce    (fixes config violations)${NC}"
    echo -e "  ${DIM}Run: clawctl policy enforce --restart  (fixes + restarts containers)${NC}"
    exit 1
elif [ "$WARN" -gt 0 ]; then
    echo -e "  ${YELLOW}$WARN item(s) to review for production use.${NC}"
    echo -e "  ${DIM}Most warnings clear after restarting with the hardened compose template.${NC}"
    echo -e "  ${DIM}Run: clawctl policy enforce --restart${NC}"
    exit 0
else
    echo -e "  ${GREEN}All checks passed. Your deployment follows best practices.${NC}"
    exit 0
fi
