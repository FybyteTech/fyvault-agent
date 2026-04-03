#!/bin/sh
# FyVault Agent Uninstaller
set -e

FYVAULT_USER="fyvault"

info() { printf "\033[0;34m[INFO]\033[0m %s\n" "$1"; }
warn() { printf "\033[0;33m[WARN]\033[0m %s\n" "$1"; }
error() { printf "\033[0;31m[ERROR]\033[0m %s\n" "$1"; exit 1; }

if [ "$(id -u)" -ne 0 ]; then
    error "This uninstaller must be run as root"
fi

# --- Stop and disable service ---

if systemctl is-active --quiet fyvaultd 2>/dev/null; then
    info "Stopping fyvaultd service..."
    systemctl stop fyvaultd
fi

if systemctl is-enabled --quiet fyvaultd 2>/dev/null; then
    info "Disabling fyvaultd service..."
    systemctl disable fyvaultd
fi

# --- Remove files ---

info "Removing binary..."
rm -f /usr/local/bin/fyvaultd

info "Removing systemd unit..."
rm -f /etc/systemd/system/fyvaultd.service
systemctl daemon-reload

info "Removing configuration and data..."
rm -rf /etc/fyvault
rm -rf /var/lib/fyvault
rm -rf /var/log/fyvault

# --- Optionally remove user ---

REMOVE_USER="${1:-}"
if [ "$REMOVE_USER" = "--remove-user" ]; then
    if id "$FYVAULT_USER" >/dev/null 2>&1; then
        info "Removing system user: ${FYVAULT_USER}"
        userdel "$FYVAULT_USER" 2>/dev/null || true
    fi
else
    if id "$FYVAULT_USER" >/dev/null 2>&1; then
        info "System user '${FYVAULT_USER}' was kept. Pass --remove-user to remove it."
    fi
fi

info "FyVault agent uninstalled successfully."
