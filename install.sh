#!/usr/bin/env bash
# Vessel Installer
# Usage: curl -sSL https://raw.githubusercontent.com/vessel-app/vessel/main/install.sh | bash
set -euo pipefail

VESSEL_VERSION="${VESSEL_VERSION:-latest}"
VESSEL_PORT="${VESSEL_PORT:-4800}"
VESSEL_DATA_DIR="${VESSEL_DATA_DIR:-/var/lib/vessel}"
INSTALL_DIR="/usr/local/bin"
SERVICE_FILE="/etc/systemd/system/vessel.service"
CONFIG_DIR="/etc/vessel"
REPO="vessel-app/vessel"

# ── Colors ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'
info()    { echo -e "${BLUE}[vessel]${NC} $*"; }
success() { echo -e "${GREEN}[vessel]${NC} $*"; }
warn()    { echo -e "${YELLOW}[vessel]${NC} $*"; }
error()   { echo -e "${RED}[vessel]${NC} $*" >&2; exit 1; }

# ── Checks ───────────────────────────────────────────────────────────────────
[[ $EUID -ne 0 ]] && error "This installer must be run as root. Try: sudo bash"
command -v curl >/dev/null 2>&1 || error "curl is required but not installed"

# ── Detect arch ──────────────────────────────────────────────────────────────
ARCH=$(uname -m)
case $ARCH in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  armv7l)  ARCH="armv7" ;;
  *) error "Unsupported architecture: $ARCH" ;;
esac

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
[[ "$OS" != "linux" ]] && error "Vessel only supports Linux"

echo ""
echo "  ⚓  Vessel Installer"
echo "  ════════════════════"
echo ""

# ── Download binary ──────────────────────────────────────────────────────────
info "Downloading Vessel ${VESSEL_VERSION} (${OS}/${ARCH})..."

if [[ "$VESSEL_VERSION" == "latest" ]]; then
  DOWNLOAD_URL="https://github.com/${REPO}/releases/latest/download/vessel_${OS}_${ARCH}"
else
  DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VESSEL_VERSION}/vessel_${OS}_${ARCH}"
fi

TMP_BIN=$(mktemp)
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_BIN"; then
  # Fallback: build from source if binary not available
  warn "Binary not found, attempting to build from source..."
  install_from_source
fi

chmod +x "$TMP_BIN"
mv "$TMP_BIN" "${INSTALL_DIR}/vessel"
success "Vessel binary installed to ${INSTALL_DIR}/vessel"

# ── Create config ─────────────────────────────────────────────────────────────
info "Creating configuration..."
mkdir -p "$CONFIG_DIR"
cat > "${CONFIG_DIR}/config.yaml" <<EOF
port: ${VESSEL_PORT}
data_dir: ${VESSEL_DATA_DIR}
EOF
success "Config written to ${CONFIG_DIR}/config.yaml"

# ── Create data directories ───────────────────────────────────────────────────
mkdir -p "${VESSEL_DATA_DIR}"/{deployments,templates,caddy/sites}
success "Data directories created at ${VESSEL_DATA_DIR}"

# ── Bootstrap system ──────────────────────────────────────────────────────────
info "Bootstrapping system (Docker, Caddy, firewall)..."
VESSEL_CONFIG="${CONFIG_DIR}/config.yaml" vessel bootstrap

# ── Install systemd service ───────────────────────────────────────────────────
info "Installing systemd service..."
cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=Vessel - Self-hosted App Deployment Manager
Documentation=https://github.com/vessel-app/vessel
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=root
ExecStart=${INSTALL_DIR}/vessel serve
Restart=on-failure
RestartSec=5s
Environment=VESSEL_CONFIG=${CONFIG_DIR}/config.yaml
StandardOutput=journal
StandardError=journal
SyslogIdentifier=vessel

# Security hardening
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=${VESSEL_DATA_DIR} ${CONFIG_DIR} /etc/caddy /var/log/caddy
PrivateTmp=yes

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable vessel
systemctl start vessel
success "Vessel service started"

# ── Done ──────────────────────────────────────────────────────────────────────
echo ""
echo -e "  ${GREEN}✅ Vessel installed successfully!${NC}"
echo ""
echo "  Management UI:  http://$(hostname -I | awk '{print $1}'):${VESSEL_PORT}"
echo "  Logs:           journalctl -u vessel -f"
echo "  Config:         ${CONFIG_DIR}/config.yaml"
echo "  Data:           ${VESSEL_DATA_DIR}"
echo ""
echo "  Commands:"
echo "    vessel serve      — start the server"
echo "    vessel bootstrap  — re-run system setup"
echo "    vessel version    — show version"
echo ""

install_from_source() {
  command -v go >/dev/null 2>&1 || {
    info "Installing Go..."
    GO_VERSION="1.22.4"
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" | tar -C /usr/local -xz
    export PATH=$PATH:/usr/local/go/bin
  }
  info "Building Vessel from source..."
  TMP_DIR=$(mktemp -d)
  git clone --depth=1 "https://github.com/${REPO}.git" "$TMP_DIR"
  cd "$TMP_DIR"
  go build -ldflags="-s -w -X github.com/vessel-app/vessel/internal/cli.Version=${VESSEL_VERSION}" -o "${INSTALL_DIR}/vessel" .
  cd /
  rm -rf "$TMP_DIR"
  success "Built from source"
}
