#!/bin/bash
# setup-secrets.sh — initialise a claws secrets directory.
#
# Creates an EMPTY-but-correctly-shaped secrets dir (default: /tmp/claws-secrets)
# with placeholder files for each known provider/channel. The operator pastes
# their actual values into each file. claws apply --secrets-dir=<dir> then
# auto-resolves fromEnv references that match.
#
# Idempotent: re-runs preserve existing values, only create what's missing.
#
# Usage:
#   ./scripts/setup-secrets.sh                              # default /tmp/claws-secrets
#   ./scripts/setup-secrets.sh --dir=/etc/claws/secrets      # custom location
#   ./scripts/setup-secrets.sh --dir=~/.config/claws/secrets # XDG-style
set -euo pipefail

DIR="/tmp/claws-secrets"
for arg in "$@"; do
    case "$arg" in
        --dir=*) DIR="${arg#--dir=}" ;;
        -h|--help)
            sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
    esac
done

BOLD="\033[1m"
GREEN="\033[0;32m"
YELLOW="\033[0;33m"
DIM="\033[0;90m"
NC="\033[0m"

echo -e "${BOLD}claws — secrets directory setup${NC}"
echo ""
echo -e "  ${DIM}target: $DIR${NC}"
echo ""

mkdir -p "$DIR"
chmod 700 "$DIR"

# Each entry: filename:provider URL:short description
SECRETS=(
    "openai.key|https://platform.openai.com/api-keys|OpenAI API key (sk-...)"
    "anthropic.key|https://console.anthropic.com/settings/keys|Anthropic API key (sk-ant-...)"
    "google.key|https://aistudio.google.com/app/apikey|Google AI Studio key"
    "groq.key|https://console.groq.com/keys|Groq API key"
    "openrouter.key|https://openrouter.ai/keys|OpenRouter API key"
    "telegram.token|https://t.me/BotFather|Telegram bot token (from /newbot)"
    "discord.token|https://discord.com/developers/applications|Discord bot token"
    "slack.bot-token|https://api.slack.com/apps|Slack Bot User OAuth Token (xoxb-...)"
    "slack.app-token|https://api.slack.com/apps|Slack App-Level Token (xapp-...)"
)

created=0; existed=0
for entry in "${SECRETS[@]}"; do
    name="${entry%%|*}"
    rest="${entry#*|}"
    url="${rest%%|*}"
    desc="${rest#*|}"
    f="$DIR/$name"
    # "Has a real value" = at least one non-blank, non-comment line.
    if [ -f "$f" ] && grep -qvE '^\s*(#|$)' "$f" 2>/dev/null; then
        echo -e "  ${GREEN}✓${NC} ${name} ${DIM}(already has a value, keeping)${NC}"
        existed=$((existed + 1))
        continue
    fi
    if [ ! -f "$f" ]; then
        cat > "$f" <<EOF
# $desc
# Get one at: $url
# Paste the value below this line (delete the comments first or leave them — claws strips lines starting with #).
EOF
        chmod 600 "$f"
        echo -e "  ${YELLOW}…${NC} ${name} ${DIM}(placeholder created — paste value)${NC}"
        created=$((created + 1))
    fi
done

# README explaining the dir.
README="$DIR/README.md"
if [ ! -f "$README" ]; then
    cat > "$README" <<EOF
# claws secrets directory

This directory holds credentials referenced by claws templates via
\`--secrets-dir=$DIR\`.

## Naming convention

Each file is named \`<lowercased-env-var>.{key,token,secret}\`.
When \`claws apply --secrets-dir=$DIR\` encounters a
\`tokenFrom: { env: "OPENAI_API_KEY" }\` reference and that env var is
unset, it falls back to \`$DIR/openai.key\`.

## Putting values in

\`\`\`bash
\$EDITOR $DIR/openai.key
\$EDITOR $DIR/telegram.token
# etc.
\`\`\`

Comments (lines starting with \`#\`) and blank lines are stripped.
The remaining content is the secret value.

## Permissions

The directory is \`chmod 700\` (you only). Each file is \`chmod 600\`.
Don't \`git\` this dir.

## Reset / rotate

Delete the file you want to refresh, re-run \`setup-secrets.sh\`, paste new value.
EOF
fi

echo ""
echo -e "${BOLD}Summary${NC}"
echo -e "  ${GREEN}$existed${NC} already had values, ${YELLOW}$created${NC} new placeholders created."
echo ""
echo -e "${BOLD}Next${NC}"
echo -e "  1. ${DIM}Paste your tokens into the placeholders:${NC}"
echo -e "     \$EDITOR $DIR/openai.key"
echo -e "     \$EDITOR $DIR/telegram.token"
echo -e ""
echo -e "  2. ${DIM}Apply a demo template:${NC}"
echo -e "     claws apply --template=demo/instant-bot --secrets-dir=$DIR"
echo -e ""
echo -e "  3. ${DIM}Start your bot:${NC}"
echo -e "     claws start default/instant"
echo -e ""
echo -e "  ${DIM}(See $README for naming + permissions notes.)${NC}"
