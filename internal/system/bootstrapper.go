package system

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vessel-app/vessel/internal/config"
	"github.com/vessel-app/vessel/internal/store"
)

// Bootstrapper handles system-level setup.
type Bootstrapper struct {
	cfg    *config.Config
	distro string
}

func NewBootstrapper(cfg *config.Config) *Bootstrapper {
	return &Bootstrapper{cfg: cfg}
}

// DetectDistro identifies the Linux distribution.
func (b *Bootstrapper) DetectDistro() error {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return fmt.Errorf("cannot read /etc/os-release: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			b.distro = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
			return nil
		}
	}
	return fmt.Errorf("could not determine Linux distribution")
}

// CheckDependencies verifies required tools are available or installable.
func (b *Bootstrapper) CheckDependencies() error {
	required := []string{"curl", "systemctl"}
	for _, tool := range required {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required tool '%s' not found", tool)
		}
	}
	return nil
}

// InstallDocker installs Docker if not already present.
func (b *Bootstrapper) InstallDocker() error {
	if _, err := exec.LookPath("docker"); err == nil {
		// Make sure the daemon is running
		_ = runCmd("systemctl", "enable", "--now", "docker")
		return nil
	}

	switch b.distro {
	case "amzn": // Amazon Linux 2 / 2023
		if err := runCmd("dnf", "install", "-y", "docker"); err != nil {
			// fallback for Amazon Linux 2
			if err2 := runCmd("yum", "install", "-y", "docker"); err2 != nil {
				return fmt.Errorf("install docker: %w", err)
			}
		}
		return runCmd("systemctl", "enable", "--now", "docker")
	default:
		// Universal installer works on Ubuntu, Debian, CentOS, RHEL, Fedora
		return b.runScript("https://get.docker.com")
	}
}

// InstallDockerCompose installs Docker Compose plugin if missing.
func (b *Bootstrapper) InstallDockerCompose() error {
	// Check for compose v2 plugin
	if err := runCmd("docker", "compose", "version"); err == nil {
		return nil
	}

	switch b.distro {
	case "amzn": // Amazon Linux — compose plugin not in default repos, install binary
		return b.installComposeManually()
	case "ubuntu", "debian":
		return runCmd("apt-get", "install", "-y", "docker-compose-plugin")
	case "centos", "rhel", "fedora", "rocky", "almalinux":
		return runCmd("yum", "install", "-y", "docker-compose-plugin")
	default:
		return b.installComposeManually()
	}
}

// InstallCaddy installs Caddy web server if missing.
func (b *Bootstrapper) InstallCaddy() error {
	if _, err := exec.LookPath("caddy"); err == nil {
		_ = runCmd("systemctl", "enable", "--now", "caddy")
		return nil
	}

	switch b.distro {
	case "amzn": // Amazon Linux
		// No official Caddy repo for Amazon Linux — install binary directly
		if err := b.installCaddyBinary(); err != nil {
			return err
		}
		// Write a systemd unit for it
		return b.writeCaddyService()
	case "ubuntu", "debian":
		if err := runCmd("apt-get", "install", "-y", "debian-keyring", "debian-archive-keyring", "apt-transport-https"); err != nil {
			return err
		}
		if err := runPipe(
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key'",
			"gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg",
		); err != nil {
			return err
		}
		if err := runPipe(
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt'",
			"tee /etc/apt/sources.list.d/caddy-stable.list",
		); err != nil {
			return err
		}
		if err := runCmd("apt-get", "update"); err != nil {
			return err
		}
		if err := runCmd("apt-get", "install", "-y", "caddy"); err != nil {
			return err
		}
	case "centos", "rhel", "fedora", "rocky", "almalinux":
		if err := runPipe(
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/cfg/setup/config.rpm.txt'",
			"tee /etc/yum.repos.d/caddy-stable.repo",
		); err != nil {
			return err
		}
		if err := runCmd("yum", "install", "-y", "caddy"); err != nil {
			return err
		}
	default:
		if err := b.installCaddyBinary(); err != nil {
			return err
		}
		return b.writeCaddyService()
	}

	return runCmd("systemctl", "enable", "--now", "caddy")
}

// ConfigureFirewall opens ports 80, 443, and the Vessel UI port.
func (b *Bootstrapper) ConfigureFirewall() error {
	ports := []string{"80/tcp", "443/tcp", fmt.Sprintf("%d/tcp", b.cfg.Port)}

	// Amazon Linux 2023 uses firewalld by default
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		for _, port := range ports {
			_ = runCmd("firewall-cmd", "--permanent", "--add-port="+port)
		}
		_ = runCmd("firewall-cmd", "--reload")
		return nil
	}

	// Try ufw (Ubuntu/Debian)
	if _, err := exec.LookPath("ufw"); err == nil {
		for _, port := range ports {
			_ = runCmd("ufw", "allow", port)
		}
		_ = runCmd("ufw", "--force", "enable")
		return nil
	}

	// No firewall manager found — not an error, EC2 security groups handle it
	return nil
}

// SetupDirectories creates the Vessel data directory structure.
func (b *Bootstrapper) SetupDirectories() error {
	dirs := []string{
		b.cfg.DataDir,
		b.cfg.DeploymentsDir,
		b.cfg.TemplatesDir,
		b.cfg.CaddyDir,
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	return nil
}

// InitDatabase initializes the SQLite database.
func (b *Bootstrapper) InitDatabase() error {
	db, err := store.Open(b.cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	return db.Migrate()
}

// --- helpers ---

func (b *Bootstrapper) runScript(url string) error {
	cmd := exec.Command("sh", "-c", fmt.Sprintf("curl -fsSL %s | sh", url))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *Bootstrapper) installComposeManually() error {
	script := `
ARCH=$(uname -m)
case $ARCH in x86_64) ARCH="x86_64" ;; aarch64) ARCH="aarch64" ;; *) ARCH="x86_64" ;; esac
COMPOSE_VERSION=$(curl -fsSL https://api.github.com/repos/docker/compose/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
mkdir -p /usr/local/lib/docker/cli-plugins
curl -fsSL "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-${ARCH}" \
  -o /usr/local/lib/docker/cli-plugins/docker-compose
chmod +x /usr/local/lib/docker/cli-plugins/docker-compose
`
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *Bootstrapper) installCaddyBinary() error {
	script := `
ARCH=$(uname -m)
case $ARCH in x86_64) ARCH="amd64" ;; aarch64) ARCH="arm64" ;; armv7l) ARCH="armv7" ;; *) ARCH="amd64" ;; esac
CADDY_VERSION=$(curl -fsSL https://api.github.com/repos/caddyserver/caddy/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
curl -fsSL "https://github.com/caddyserver/caddy/releases/download/${CADDY_VERSION}/caddy_${CADDY_VERSION#v}_linux_${ARCH}.tar.gz" \
  | tar -xz -C /usr/local/bin caddy
chmod +x /usr/local/bin/caddy
mkdir -p /etc/caddy /var/log/caddy
`
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (b *Bootstrapper) writeCaddyService() error {
	unit := `[Unit]
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
`
	if err := os.WriteFile("/etc/systemd/system/caddy.service", []byte(unit), 0644); err != nil {
		return err
	}
	if err := runCmd("systemctl", "daemon-reload"); err != nil {
		return err
	}
	return runCmd("systemctl", "enable", "--now", "caddy")
}

func runCmd(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runPipe(src, dst string) error {
	cmd := exec.Command("sh", "-c", src+" | "+dst)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
