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
		return nil // already installed
	}

	switch b.distro {
	case "ubuntu", "debian":
		return b.runScript("https://get.docker.com")
	case "centos", "rhel", "fedora", "rocky", "almalinux":
		return b.runScript("https://get.docker.com")
	default:
		return b.runScript("https://get.docker.com")
	}
}

// InstallDockerCompose installs Docker Compose plugin if missing.
func (b *Bootstrapper) InstallDockerCompose() error {
	// Check for compose v2 plugin
	if err := runCmd("docker", "compose", "version"); err == nil {
		return nil
	}

	// Install compose plugin
	switch b.distro {
	case "ubuntu", "debian":
		return runCmd("apt-get", "install", "-y", "docker-compose-plugin")
	case "centos", "rhel", "fedora", "rocky", "almalinux":
		return runCmd("yum", "install", "-y", "docker-compose-plugin")
	default:
		// Fallback: install binary directly
		return b.installComposeManually()
	}
}

// InstallCaddy installs Caddy web server if missing.
func (b *Bootstrapper) InstallCaddy() error {
	if _, err := exec.LookPath("caddy"); err == nil {
		// Ensure it's running as a service
		_ = runCmd("systemctl", "enable", "--now", "caddy")
		return nil
	}

	switch b.distro {
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
		return fmt.Errorf("unsupported distribution for Caddy installation: %s", b.distro)
	}

	return runCmd("systemctl", "enable", "--now", "caddy")
}

// ConfigureFirewall opens ports 80, 443, and the Vessel UI port.
func (b *Bootstrapper) ConfigureFirewall() error {
	ports := []string{"80/tcp", "443/tcp", fmt.Sprintf("%d/tcp", b.cfg.Port)}

	// Try ufw first (Ubuntu/Debian)
	if _, err := exec.LookPath("ufw"); err == nil {
		for _, port := range ports {
			_ = runCmd("ufw", "allow", port)
		}
		_ = runCmd("ufw", "--force", "enable")
		return nil
	}

	// Try firewall-cmd (RHEL/CentOS)
	if _, err := exec.LookPath("firewall-cmd"); err == nil {
		for _, port := range ports {
			_ = runCmd("firewall-cmd", "--permanent", "--add-port="+port)
		}
		_ = runCmd("firewall-cmd", "--reload")
		return nil
	}

	// No firewall manager found — not an error, just skip
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
COMPOSE_VERSION=$(curl -s https://api.github.com/repos/docker/compose/releases/latest | grep '"tag_name"' | cut -d'"' -f4)
curl -SL "https://github.com/docker/compose/releases/download/${COMPOSE_VERSION}/docker-compose-linux-x86_64" -o /usr/local/bin/docker-compose
chmod +x /usr/local/bin/docker-compose
`
	cmd := exec.Command("sh", "-c", script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
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
