#!/bin/bash
# Install the claws auth-monitor systemd timer.
#
# Usage:
#   sudo bash scripts/systemd/install.sh
#
# Then stage your fallback API key (no SSH required after this):
#   claws paste-secret .recovery-api-key --secrets-dir=$HOME/.openclaw
# Visit the URL it prints from your phone, paste the OpenAI sk-... key.
# The timer's next tick auto-recovers any failing agents using that key.

set -euo pipefail

if [[ $EUID -ne 0 ]]; then
    echo "Run with sudo: sudo bash $0"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET_USER="${SUDO_USER:-ubuntu}"
SED_USER="$(systemd-escape "$TARGET_USER")"

# Rewrite User= / Group= / ReadWritePaths= for whoever's running this.
install -m 0644 "$SCRIPT_DIR/claws-auth-monitor.service" /etc/systemd/system/claws-auth-monitor.service
install -m 0644 "$SCRIPT_DIR/claws-auth-monitor.timer"   /etc/systemd/system/claws-auth-monitor.timer

# Substitute the actual user's home into the service unit.
sed -i "s|User=ubuntu|User=$TARGET_USER|"            /etc/systemd/system/claws-auth-monitor.service
sed -i "s|Group=ubuntu|Group=$TARGET_USER|"          /etc/systemd/system/claws-auth-monitor.service
sed -i "s|/home/ubuntu/.openclaw|/home/$TARGET_USER/.openclaw|" /etc/systemd/system/claws-auth-monitor.service

systemctl daemon-reload
systemctl enable --now claws-auth-monitor.timer

echo
echo "Installed. The timer will fire every 5 minutes."
echo
echo "Next steps:"
echo "  1. Stage your OpenAI API key (no SSH after this):"
echo "     claws paste-secret .recovery-api-key --secrets-dir=/home/$TARGET_USER/.openclaw"
echo
echo "  2. Watch the audit log to see recoveries land:"
echo "     tail -f /home/$TARGET_USER/.openclaw/.audit.log | grep auth.monitor"
echo
echo "  3. Confirm timer is scheduled:"
echo "     systemctl list-timers claws-auth-monitor"
