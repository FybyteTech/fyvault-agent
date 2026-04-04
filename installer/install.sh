#!/bin/sh
# FyVault Agent Installer
# POSIX-compatible installer for Linux systems with systemd
set -e

FYVAULT_VERSION="${FYVAULT_VERSION:-latest}"
FYVAULT_BASE_URL="${FYVAULT_BASE_URL:-https://releases.fyvault.com}"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/fyvault"
TLS_DIR="${CONFIG_DIR}/tls"
DATA_DIR="/var/lib/fyvault"
LOG_DIR="/var/log/fyvault"
SYSTEMD_DIR="/etc/systemd/system"
FYVAULT_USER="fyvault"

info() { printf "\033[0;34m[INFO]\033[0m %s\n" "$1"; }
warn() { printf "\033[0;33m[WARN]\033[0m %s\n" "$1"; }
error() { printf "\033[0;31m[ERROR]\033[0m %s\n" "$1"; exit 1; }

# --- Pre-flight checks ---

if [ "$(id -u)" -ne 0 ]; then
    error "This installer must be run as root"
fi

if [ "$(uname -s)" != "Linux" ]; then
    error "FyVault agent only supports Linux"
fi

# Check kernel version >= 5.4
KERNEL_MAJOR=$(uname -r | cut -d. -f1)
KERNEL_MINOR=$(uname -r | cut -d. -f2)
if [ "$KERNEL_MAJOR" -lt 5 ] || { [ "$KERNEL_MAJOR" -eq 5 ] && [ "$KERNEL_MINOR" -lt 4 ]; }; then
    error "Linux kernel >= 5.4 is required (found $(uname -r))"
fi

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *)       error "Unsupported architecture: $ARCH" ;;
esac

# Check for systemd
if ! command -v systemctl >/dev/null 2>&1; then
    error "systemd is required but not found"
fi

info "Installing FyVault agent (${FYVAULT_VERSION}, ${ARCH})..."

# --- Create user ---

if ! id "$FYVAULT_USER" >/dev/null 2>&1; then
    info "Creating system user: ${FYVAULT_USER}"
    useradd --system --no-create-home --shell /usr/sbin/nologin "$FYVAULT_USER"
fi

# --- Create directories ---

info "Creating directories..."
mkdir -p "$CONFIG_DIR" "$TLS_DIR" "$DATA_DIR" "$LOG_DIR"
chown "$FYVAULT_USER":"$FYVAULT_USER" "$DATA_DIR" "$LOG_DIR"
chmod 700 "$TLS_DIR"

# --- Download binary ---

DOWNLOAD_URL="${FYVAULT_BASE_URL}/fyvaultd-${FYVAULT_VERSION}-linux-${ARCH}"
info "Downloading binary from ${DOWNLOAD_URL}..."

if command -v curl >/dev/null 2>&1; then
    curl -fsSL -o "${INSTALL_DIR}/fyvaultd" "$DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -qO "${INSTALL_DIR}/fyvaultd" "$DOWNLOAD_URL"
else
    error "curl or wget is required"
fi

chmod 755 "${INSTALL_DIR}/fyvaultd"

# --- Generate self-signed certificate (for initial setup) ---

if [ ! -f "${TLS_DIR}/device.crt" ]; then
    info "Generating self-signed TLS certificate..."
    if command -v openssl >/dev/null 2>&1; then
        openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
            -keyout "${TLS_DIR}/device.key" \
            -out "${TLS_DIR}/device.crt" \
            -days 365 -nodes \
            -subj "/CN=fyvault-device/O=FyVault" 2>/dev/null
        chown "$FYVAULT_USER":"$FYVAULT_USER" "${TLS_DIR}/device.key" "${TLS_DIR}/device.crt"
        chmod 600 "${TLS_DIR}/device.key"
    else
        warn "openssl not found — skipping certificate generation"
    fi
fi

# --- Write config ---

if [ ! -f "${CONFIG_DIR}/fyvault.conf" ]; then
    info "Writing default configuration..."
    cat > "${CONFIG_DIR}/fyvault.conf" << 'CONF'
# FyVault Agent — url must include /api/v1 (override for staging: https://test.fyvault.com/api/v1)

[cloud]
url = "https://api.fyvault.com/api/v1"
token = ""

[agent]
heartbeat_interval = 300
log_level = "info"

[keyring]
namespace = "fyvault"

[network]
interface = ""
CONF
    chown root:"$FYVAULT_USER" "${CONFIG_DIR}/fyvault.conf"
    chmod 640 "${CONFIG_DIR}/fyvault.conf"
fi

# --- Install systemd unit ---

info "Installing systemd service..."
cat > "${SYSTEMD_DIR}/fyvaultd.service" << 'UNIT'
[Unit]
Description=FyVault Agent Daemon
Documentation=https://fyvault.com/docs
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=fyvault
Group=fyvault
ExecStart=/usr/local/bin/fyvaultd --config /etc/fyvault/fyvault.conf
Restart=on-failure
RestartSec=5
LimitNOFILE=65536
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/fyvault /var/log/fyvault
PrivateTmp=true

[Install]
WantedBy=multi-user.target
UNIT

systemctl daemon-reload

# --- Enable and start ---

info "Enabling and starting fyvaultd..."
systemctl enable fyvaultd
systemctl start fyvaultd

info "FyVault agent installed successfully!"
info "  Binary:  ${INSTALL_DIR}/fyvaultd"
info "  Config:  ${CONFIG_DIR}/fyvault.conf"
info "  Service: systemctl status fyvaultd"
info ""
info "Next steps:"
info "  1. Set your device token in ${CONFIG_DIR}/fyvault.conf"
info "  2. Restart the service: systemctl restart fyvaultd"
