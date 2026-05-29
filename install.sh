#!/usr/bin/env bash
# Vessel Installer
# Usage: curl -sSL https://raw.githubusercontent.com/Yash121l/Vessel/main/install.sh | sudo bash
set -euo pipefail

# Dynamically fetch the latest release tag from GitHub if not specified
if [[ -z "${VESSEL_VERSION:-}" ]]; then
  # Try to fetch latest tag name, e.g. "v1.1.8"
  LATEST_TAG=$(curl -fsSL https://api.github.com/repos/Yash121l/Vessel/releases/latest | grep '"tag_name":' | cut -d'"' -f4 || true)
  if [[ -n "$LATEST_TAG" ]]; then
    # Strip leading 'v' if present
    VESSEL_VERSION="${LATEST_TAG#v}"
  else
    VESSEL_VERSION="1.1.8"
  fi
else
  # If specified via env, strip leading 'v' as well
  VESSEL_VERSION="${VESSEL_VERSION#v}"
fi

VESSEL_PORT="${VESSEL_PORT:-4800}"
VESSEL_DATA_DIR="${VESSEL_DATA_DIR:-/var/lib/vessel}"
VESSEL_CONFIG_DIR="/etc/vessel"
VESSEL_BIN="/usr/local/bin/vessel"
VESSEL_REPO="Yash121l/Vessel"
VESSEL_BRANCH="${VESSEL_BRANCH:-main}"

# ── Colors ──────────────────────────────────────────────────────────────────
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info()    { echo -e "  ${CYAN}→${NC}  $*"; }
success() { echo -e "  ${GREEN}✓${NC}  $*"; }
warn()    { echo -e "  ${YELLOW}!${NC}  $*"; }
error()   { echo -e "  ${RED}✗${NC}  $*" >&2; exit 1; }

rpm_install() {
  if command -v dnf >/dev/null 2>&1; then
    dnf install -y "$@" -q 2>/dev/null || yum install -y "$@" -q
  else
    yum install -y "$@" -q
  fi
}

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
      amzn)                    rpm_install git ;;
      ubuntu|debian)           apt-get install -y git -q ;;
      centos|rhel|fedora|rocky|almalinux) rpm_install git ;;
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
    -ldflags="-s -w -X github.com/Yash121l/Vessel/internal/cli.Version=${VESSEL_VERSION}" \
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
      rpm_install docker-compose-plugin
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

# ── Step 4: Install Nginx ────────────────────────────────────────────────────
install_nginx() {
  if command -v nginx >/dev/null 2>&1; then
    info "Nginx already installed — skipping"
    systemctl enable --now nginx 2>/dev/null || true
    return
  fi

  info "Installing Nginx..."
  case "$DISTRO" in
    amzn)
      dnf install -y nginx -q 2>/dev/null || yum install -y nginx -q
      systemctl enable --now nginx
      ;;
    ubuntu|debian)
      apt-get update -q
      apt-get install -y nginx -q
      systemctl enable --now nginx
      ;;
    centos|rhel|fedora|rocky|almalinux)
      rpm_install nginx
      systemctl enable --now nginx
      ;;
    *) error "Automatic nginx installation is not supported on distro: $DISTRO" ;;
  esac
  success "Nginx installed"
}

# ── Step 5: Install Certbot nginx plugin ─────────────────────────────────────
install_certbot() {
  if command -v certbot >/dev/null 2>&1 && certbot plugins 2>/dev/null | grep -qi nginx; then
    info "Certbot nginx plugin already installed — skipping"
    return
  fi

  info "Installing Certbot nginx plugin..."
  case "$DISTRO" in
    amzn)
      dnf install -y certbot python3-certbot-nginx -q 2>/dev/null || yum install -y certbot python3-certbot-nginx -q
      ;;
    ubuntu|debian)
      apt-get update -q
      apt-get install -y certbot python3-certbot-nginx -q
      ;;
    centos|rhel|fedora|rocky|almalinux)
      rpm_install certbot python3-certbot-nginx
      ;;
    *) error "Automatic certbot installation is not supported on distro: $DISTRO" ;;
  esac
  success "Certbot nginx plugin installed"
}

# ── Step 6: Configure firewall ───────────────────────────────────────────────
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

# ── Step 7: Write Vessel config + directories ────────────────────────────────
setup_vessel() {
  info "Setting up Vessel directories and config..."

  mkdir -p \
    "$VESSEL_CONFIG_DIR" \
    "$VESSEL_DATA_DIR/deployments" \
    "$VESSEL_DATA_DIR/templates" \
    "$VESSEL_DATA_DIR/backups"

  # Write config only if it doesn't already exist
  if [[ ! -f "$VESSEL_CONFIG_DIR/config.yaml" ]]; then
    cat > "$VESSEL_CONFIG_DIR/config.yaml" <<EOF
port: ${VESSEL_PORT}
data_dir: ${VESSEL_DATA_DIR}
EOF
  fi

  success "Vessel configured"
}

# ── Step 8: Install systemd service ─────────────────────────────────────────
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
  install_nginx
  install_certbot
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
