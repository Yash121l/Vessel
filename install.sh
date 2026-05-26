#!/usr/bin/env bash
# Vessel Installer
# Usage: curl -sSL https://raw.githubusercontent.com/vessel-app/vessel/main/install.sh | sudo bash
set -euo pipefail

VESSEL_VERSION="${VESSEL_VERSION:-0.1.0}"
VESSEL_PORT="${VESSEL_PORT:-4800}"
VESSEL_DATA_DIR="${VESSEL_DATA_DIR:-/var/lib/vessel}"
VESSEL_CONFIG_DIR="/etc/vessel"
VESSEL_BIN="/usr/local/bin/vessel"
VESSEL_REPO="vessel-app/vessel"
VESSEL_BRANCH="${VESSEL_BRANCH:-main}"

# ── Colors ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()    { echo -e "  ${CYAN}→${NC}  $*"; }
success() { echo -e "  ${GREEN}✓${NC}  $*"; }
warn()    { echo -e "  ${YELLOW}!${NC}  $*"; }
error()   { echo -e "  ${RED}✗${NC}  $*" >&2; exit 1; }

# ── Preflight ────────────────────────────────────────────────────────────────
[[ $EUID -ne 0 ]] && error "Run as root: curl -sSL ... | sudo bash"
command -v curl >/dev/null 2>&1 || error "curl is required but not installed"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
[[ "$OS" != "linux" ]] && error "Vessel only supports Linux"

ARCH=$(uname -m)
case $ARCH in
  x86_64)  GOARCH="amd64";  ARCH_LABEL="amd64"  ;;
  aarch64) GOARCH="arm64";  ARCH_LABEL="arm64"  ;;
  armv7l)  GOARCH="arm";    ARCH_LABEL="armv7"  ;;
  *) error "Unsupported architecture: $ARCH" ;;
esac

# Detect distro
DISTRO=""
if [[ -f /etc/os-release ]]; then
  DISTRO=$(grep "^ID=" /etc/os-release | cut -d= -f2 | tr -d '"')
fi

echo ""
echo "  ⚓  Vessel Installer v${VESSEL_VERSION}"
echo "  ══════════════════════════════"
echo "  OS:   $OS / $ARCH_LABEL"
echo "  Dist: ${DISTRO:-unknown}"
echo ""

# ── Step 1: Install Vessel binary ────────────────────────────────────────────
install_vessel_binary() {
  info "Downloading Vessel binary..."

  RELEASE_URL="https://github.com/${VESSEL_REPO}/releases/download/v${VESSEL_VERSION}/vessel_linux_${ARCH_LABEL}"
  TMP_BIN=$(mktemp)

  if curl -fsSL "$RELEASE_URL" -o "$TMP_BIN" 2>/dev/null; then
    chmod +x "$TMP_BIN"
    mv "$TMP_BIN" "$VESSEL_BIN"
    success "Binary downloaded from GitHub release"
  else
    warn "Release binary not found — building from source (this takes ~2 min)"
    build_from_source
  fi
}

build_from_source() {
  # Install Go if missing
  if ! command -v go >/dev/null 2>&1; then
    info "Installing Go 1.22..."
    GO_VERSION="1.22.4"
    curl -fsSL "https://go.dev/dl/go${GO_VERSION}.linux-${GOARCH}.tar.gz" \
      | tar -C /usr/local -xz
    export PATH=$PATH:/usr/local/go/bin
    success "Go installed"
  fi

  # Install git if missing
  if ! command -v git >/dev/null 2>&1; then
    info "Installing git..."
    case "$DISTRO" in
      amzn)                    dnf install -y git -q ;;
      ubuntu|debian)           apt-get install -y git -q ;;
      centos|rhel|fedora|rocky|almalinux) yum install -y git -q ;;
      *) error "Cannot install git on distro: $DISTRO" ;;
    esac
  fi

  info "Cloning Vessel source..."
  TMP_SRC=$(mktemp -d)
  git clone --depth=1 --branch "$VESSEL_BRANCH" \
    "https://github.com/${VESSEL_REPO}.git" "$TMP_SRC" -q

  info "Compiling..."
  cd "$TMP_SRC"
  GOFLAGS="-mod=mod" go build \
    -ldflags="-s -w -X github.com/vessel-app/vessel/internal/cli.Version=${VESSEL_VERSION}" \
    -o "$VESSEL_BIN" . 2>&1
  cd /
  rm -rf "$TMP_SRC"
  success "Built from source"
}

# ── Step 2: Install Docker ───────────────────────────────────────────────────
install_docker() {
  if command -v docker >/dev/null 2>&1; then
    info "Docker already installed — skipping"
    systemctl enable --now docker 2>/dev/null || true
    return
  fi

  info "Installing Docker..."
  case "$DISTRO" in
    amzn)
      # Amazon Linux has Docker in its own repos
      dnf install -y docker -q 2>/dev/null || yum install -y docker -q
      systemctl enable --now docker
      ;;
    *)
      # Universal Docker installer (Ubuntu, Debian, CentOS, RHEL, Fedora)
      curl -fsSL https://get.docker.com | sh
      systemctl enable --now docker
      ;;
  esac
  success "Docker installed"
}

# ── Step 3: Install Docker Compose plugin ────────────────────────────────────
install_compose() {
  if docker compose version >/dev/null 2>&1; then
    info "Docker Compose already installed — skipping"
    return
  fi

  info "Installing Docker Compose plugin..."
  case "$DISTRO" in
    ubuntu|debian)
      apt-get install -y docker-compose-plugin -q
      ;;
    centos|rhel|fedora|rocky|almalinux)
      yum install -y docker-compose-plugin -q
      ;;
    *)
      # Install binary into Docker CLI plugins directory
      COMPOSE_VERSION=$(curl -fsSL https://api.github.com/repos/docker/compose/releases/latest \
        | grep '"tag_name"' | cut -d'"' -f4)
      mkdir -p /usr/local/lib/docker/cli-plugins
      curl -fsSL \
        "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-$(uname -m)" \
        -o /usr/local/lib/docker/cli-plugins/docker-compose
      chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
      ;;
  esac
  success "Docker Compose installed"
}

# ── Step 4: Install Caddy ────────────────────────────────────────────────────
install_caddy() {
  if command -v caddy >/dev/null 2>&1; then
    info "Caddy already installed — skipping"
    systemctl enable --now caddy 2>/dev/null || true
    return
  fi

  info "Installing Caddy..."
  case "$DISTRO" in
    ubuntu|debian)
      apt-get install -y debian-keyring debian-archive-keyring apt-transport-https -q
      curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' \
        | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
      curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' \
        | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
      apt-get update -q
      apt-get install -y caddy -q
      systemctl enable --now caddy
      ;;
    centos|rhel|fedora|rocky|almalinux)
      curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/cfg/setup/config.rpm.txt' \
        | tee /etc/yum.repos.d/caddy-stable.repo >/dev/null
      yum install -y caddy -q
      systemctl enable --now caddy
      ;;
    *)
      # Amazon Linux + fallback: install binary + write systemd unit
      CADDY_VERSION=$(curl -fsSL https://api.github.com/repos/caddyserver/caddy/releases/latest \
        | grep '"tag_name"' | cut -d'"' -f4)
      curl -fsSL \
        "https://github.com/caddyserver/caddy/releases/download/${CADDY_VERSION}/caddy_${CADDY_VERSION#v}_linux_${ARCH_LABEL}.tar.gz" \
        | tar -xz -C /usr/local/bin caddy
      chmod +x /usr/local/bin/caddy
      mkdir -p /etc/caddy /var/log/caddy

      cat > /etc/systemd/system/caddy.service <<'UNIT'
[Unit]
Description=Caddy
Documentation=https://caddyserver.com/docs/
After=network.target network-online.target
Requires=network-online.target

[Service]
Type=notify
User=root
ExecStart=/usr/local/bin/caddy run --environ --config /etc/caddy/Caddyfile
ExecReload=/usr/local/bin/caddy reload --config /etc/caddy/Caddyfile --force
TimeoutStopSec=5s
LimitNOFILE=1048576
PrivateTmp=true
ProtectSystem=full
AmbientCapabilities=CAP_NET_BIND_SERVICE

[Install]
WantedBy=multi-user.target
UNIT

      systemctl daemon-reload
      systemctl enable --now caddy
      ;;
  esac
  success "Caddy installed"
}

# ── Step 5: Configure firewall ───────────────────────────────────────────────
configure_firewall() {
  info "Configuring firewall..."
  PORTS=("80/tcp" "443/tcp" "${VESSEL_PORT}/tcp")

  if command -v firewall-cmd >/dev/null 2>&1; then
    for port in "${PORTS[@]}"; do
      firewall-cmd --permanent --add-port="$port" >/dev/null 2>&1 || true
    done
    firewall-cmd --reload >/dev/null 2>&1 || true
    success "firewalld rules added"
  elif command -v ufw >/dev/null 2>&1; then
    for port in "${PORTS[@]}"; do
      ufw allow "$port" >/dev/null 2>&1 || true
    done
    ufw --force enable >/dev/null 2>&1 || true
    success "ufw rules added"
  else
    warn "No firewall manager found — make sure ports 80, 443, ${VESSEL_PORT} are open in your cloud security group"
  fi
}

# ── Step 6: Write Vessel config + directories ────────────────────────────────
setup_vessel() {
  info "Setting up Vessel directories and config..."

  mkdir -p \
    "$VESSEL_CONFIG_DIR" \
    "$VESSEL_DATA_DIR/deployments" \
    "$VESSEL_DATA_DIR/templates" \
    "$VESSEL_DATA_DIR/caddy/sites" \
    /var/log/caddy

  # Write config only if it doesn't already exist
  if [[ ! -f "$VESSEL_CONFIG_DIR/config.yaml" ]]; then
    cat > "$VESSEL_CONFIG_DIR/config.yaml" <<EOF
port: ${VESSEL_PORT}
data_dir: ${VESSEL_DATA_DIR}
EOF
  fi

  # Write a minimal Caddyfile if one doesn't exist
  if [[ ! -f /etc/caddy/Caddyfile ]]; then
    cat > /etc/caddy/Caddyfile <<EOF
# Vessel-managed Caddyfile
# Do not edit manually

import ${VESSEL_DATA_DIR}/caddy/sites/*.caddy
EOF
    # Symlink so Caddy picks it up from its default location
    [[ -d /etc/caddy ]] && ln -sf /etc/caddy/Caddyfile /etc/caddy/Caddyfile 2>/dev/null || true
    systemctl reload caddy 2>/dev/null || true
  fi

  success "Vessel configured"
}

# ── Step 7: Install systemd service ─────────────────────────────────────────
install_service() {
  info "Installing systemd service..."

  cat > /etc/systemd/system/vessel.service <<EOF
[Unit]
Description=Vessel - Self-hosted App Deployment Manager
Documentation=https://github.com/${VESSEL_REPO}
After=network.target docker.service
Requires=docker.service

[Service]
Type=simple
User=root
ExecStart=${VESSEL_BIN} serve
Restart=on-failure
RestartSec=5s
Environment=VESSEL_CONFIG=${VESSEL_CONFIG_DIR}/config.yaml
StandardOutput=journal
StandardError=journal
SyslogIdentifier=vessel

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable vessel
  systemctl restart vessel
  success "Vessel service started"
}

# ── Run all steps ────────────────────────────────────────────────────────────
main() {
  install_vessel_binary
  install_docker
  install_compose
  install_caddy
  configure_firewall
  setup_vessel
  install_service

  # Get the public IP for the final message
  PUBLIC_IP=$(curl -fsSL https://checkip.amazonaws.com 2>/dev/null || hostname -I | awk '{print $1}')

  echo ""
  echo -e "  ${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "  ${GREEN}✅  Vessel is running!${NC}"
  echo -e "  ${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo ""
  echo "  UI:      http://${PUBLIC_IP}:${VESSEL_PORT}"
  echo "  Logs:    journalctl -u vessel -f"
  echo "  Config:  ${VESSEL_CONFIG_DIR}/config.yaml"
  echo "  Data:    ${VESSEL_DATA_DIR}"
  echo ""
  echo "  ⚠️  Make sure your security group allows:"
  echo "     port 80  (HTTP / Let's Encrypt)"
  echo "     port 443 (HTTPS)"
  echo "     port ${VESSEL_PORT} (Vessel UI)"
  echo ""
}

main
