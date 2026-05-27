package system

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/Yash121l/Vessel/internal/config"
	"github.com/Yash121l/Vessel/internal/logger"
	"github.com/Yash121l/Vessel/internal/store"
)

// Bootstrapper handles system-level setup.
type Bootstrapper struct {
	cfg    *config.Config
	distro string
}

func NewBootstrapper(cfg *config.Config) *Bootstrapper {
	logger.Infof("Creating new system bootstrapper...")
	return &Bootstrapper{cfg: cfg}
}

// DetectDistro identifies the Linux distribution.
func (b *Bootstrapper) DetectDistro() error {
	logger.Infof("Detecting Linux distribution...")
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		logger.Errorf("failed to read /etc/os-release: %v", err)
		return fmt.Errorf("cannot read /etc/os-release: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "ID=") {
			b.distro = strings.Trim(strings.TrimPrefix(line, "ID="), `"`)
			logger.Infof("Detected Linux distribution: %s", b.distro)
			return nil
		}
	}
	logger.Errorf("could not determine Linux distribution from /etc/os-release")
	return fmt.Errorf("could not determine Linux distribution")
}

// CheckDependencies verifies required tools are available or installable.
func (b *Bootstrapper) CheckDependencies() error {
	logger.Infof("Checking system-level preflight dependencies...")
	required := []string{"curl", "systemctl"}
	for _, tool := range required {
		if _, err := exec.LookPath(tool); err != nil {
			logger.Errorf("required dependency '%s' not found on system PATH", tool)
			return fmt.Errorf("required tool '%s' not found", tool)
		}
		logger.Debugf("Dependency '%s' is present in PATH", tool)
	}
	logger.Infof("All system-level preflight dependencies check passed successfully")
	return nil
}

// InstallDocker installs Docker if not already present.
func (b *Bootstrapper) InstallDocker() error {
	logger.Infof("Checking if Docker is installed...")
	if _, err := exec.LookPath("docker"); err == nil {
		logger.Infof("Docker is already installed. Ensuring service is enabled and running...")
		_ = runCmd("systemctl", "enable", "--now", "docker")
		return nil
	}

	logger.Infof("Docker not found. Proceeding with Docker installation for distro: %s...", b.distro)
	switch b.distro {
	case "amzn": // Amazon Linux 2 / 2023
		logger.Infof("Installing Docker using Amazon Linux package manager...")
		if err := runCmd("dnf", "install", "-y", "docker"); err != nil {
			// fallback for Amazon Linux 2
			logger.Infof("dnf install failed, trying yum install fallback...")
			if err2 := runCmd("yum", "install", "-y", "docker"); err2 != nil {
				logger.Errorf("failed to install docker on Amazon Linux: %v", err2)
				return fmt.Errorf("install docker: %w", err)
			}
		}
		return runCmd("systemctl", "enable", "--now", "docker")
	default:
		// Universal installer works on Ubuntu, Debian, CentOS, RHEL, Fedora
		logger.Infof("Running universal Docker installer script from get.docker.com...")
		return b.runScript("https://get.docker.com")
	}
}

// InstallDockerCompose installs Docker Compose plugin if missing.
func (b *Bootstrapper) InstallDockerCompose() error {
	// Check for compose v2 plugin
	if err := runCmd("docker", "compose", "version"); err == nil {
		logger.Infof("Docker Compose v2 plugin is already installed")
		return nil
	}

	logger.Infof("Docker Compose plugin not found. Installing for distro: %s...", b.distro)
	switch b.distro {
	case "amzn": // Amazon Linux — compose plugin not in default repos, install binary
		logger.Infof("Installing Docker Compose manually on Amazon Linux...")
		return b.installComposeManually()
	case "ubuntu", "debian":
		logger.Infof("Installing Docker Compose via apt-get...")
		return runCmd("apt-get", "install", "-y", "docker-compose-plugin")
	case "centos", "rhel", "fedora", "rocky", "almalinux":
		logger.Infof("Installing Docker Compose via yum...")
		return runCmd("yum", "install", "-y", "docker-compose-plugin")
	default:
		logger.Infof("Distro unrecognized, installing Docker Compose manually...")
		return b.installComposeManually()
	}
}

// InstallCaddy installs Caddy web server if missing.
func (b *Bootstrapper) InstallCaddy() error {
	logger.Infof("Checking if Caddy is installed...")
	if _, err := exec.LookPath("caddy"); err == nil {
		logger.Infof("Caddy is already installed. Ensuring service is enabled and running...")
		_ = runCmd("systemctl", "enable", "--now", "caddy")
		return nil
	}

	logger.Infof("Caddy not found. Installing for distro: %s...", b.distro)
	switch b.distro {
	case "amzn": // Amazon Linux
		logger.Infof("No official Caddy repository for Amazon Linux. Installing binary directly...")
		if err := b.installCaddyBinary(); err != nil {
			logger.Errorf("failed to install Caddy binary: %v", err)
			return err
		}
		logger.Infof("Creating Caddy systemd unit file...")
		return b.writeCaddyService()
	case "ubuntu", "debian":
		logger.Infof("Adding Caddy apt repository and GPG key...")
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
		logger.Infof("Installing Caddy via apt...")
		if err := runCmd("apt-get", "update"); err != nil {
			return err
		}
		if err := runCmd("apt-get", "install", "-y", "caddy"); err != nil {
			return err
		}
	case "centos", "rhel", "fedora", "rocky", "almalinux":
		logger.Infof("Adding Caddy yum repository...")
		if err := runPipe(
			"curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/cfg/setup/config.rpm.txt'",
			"tee /etc/yum.repos.d/caddy-stable.repo",
		); err != nil {
			return err
		}
		logger.Infof("Installing Caddy via yum...")
		if err := runCmd("yum", "install", "-y", "caddy"); err != nil {
			return err
		}
	default:
		logger.Infof("Distro unrecognized, installing Caddy binary manually...")
		if err := b.installCaddyBinary(); err != nil {
			return err
		}
		logger.Infof("Creating Caddy systemd unit file...")
		return b.writeCaddyService()
	}

	logger.Infof("Enabling and starting Caddy service...")
	return runCmd("systemctl", "enable", "--now", "caddy")
}

// ConfigureFirewall opens ports 80, 443, and the Vessel UI port.
func (b *Bootstrapper) ConfigureFirewall() error {
	ports := []string{"80/tcp", "443/tcp", fmt.Sprintf("%d/tcp", b.cfg.Port)}
	logger.Infof("Configuring firewall to allow ports: %v...", ports)

	// Amazon Linux 2023 uses firewalld by default
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		logger.Infof("Detected firewall-cmd. Adding permanent port permissions...")
		for _, port := range ports {
			_ = runCmd("firewall-cmd", "--permanent", "--add-port="+port)
		}
		logger.Infof("Reloading firewall-cmd rules...")
		_ = runCmd("firewall-cmd", "--reload")
		return nil
	}

	// Try ufw (Ubuntu/Debian)
	if _, err := exec.LookPath("ufw"); err == nil {
		logger.Infof("Detected ufw. Allowing ports...")
		for _, port := range ports {
			_ = runCmd("ufw", "allow", port)
		}
		logger.Infof("Enabling ufw firewall...")
		_ = runCmd("ufw", "--force", "enable")
		return nil
	}

	logger.Infof("No local firewall manager (firewall-cmd/ufw) detected. Skipping local configuration.")
	return nil
}

// SetupDirectories creates the Vessel data directory structure.
func (b *Bootstrapper) SetupDirectories() error {
	logger.Infof("Setting up Vessel data directories...")
	dirs := []string{
		b.cfg.DataDir,
		b.cfg.DeploymentsDir,
		b.cfg.TemplatesDir,
		b.cfg.CaddyDir,
	}
	for _, d := range dirs {
		logger.Debugf("Ensuring directory exists: %s", d)
		if err := os.MkdirAll(d, 0755); err != nil {
			logger.Errorf("failed to create directory %s: %v", d, err)
			return fmt.Errorf("failed to create directory %s: %w", d, err)
		}
	}
	logger.Infof("Vessel data directory structure is fully configured")
	return nil
}

// InitDatabase initializes the SQLite database.
func (b *Bootstrapper) InitDatabase() error {
	logger.Infof("Initializing SQLite database during bootstrap...")
	db, err := store.Open(b.cfg.DBPath)
	if err != nil {
		logger.Errorf("failed to open database during bootstrap: %v", err)
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
