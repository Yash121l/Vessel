package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Yash121l/Vessel/internal/logger"
)

// Manager handles Caddy reverse proxy configuration.
type Manager struct {
	caddyDir string
}

// NewManager creates a new proxy manager.
func NewManager(caddyDir string) *Manager {
	return &Manager{caddyDir: caddyDir}
}

// EnsureMainConfig writes the Vessel Caddyfile to /etc/caddy/Caddyfile.
// It writes a placeholder .caddy file so the glob import never fails on an
// empty directory.
func (m *Manager) EnsureMainConfig() error {
	logger.Infof("Ensuring Caddy reverse proxy main configuration is written...")
	sitesDir := m.sitesDir()
	if err := os.MkdirAll(sitesDir, 0755); err != nil {
		logger.Errorf("failed to create sites directory %s: %v", sitesDir, err)
		return err
	}

	// Write a no-op placeholder so the glob always matches at least one file
	placeholder := filepath.Join(sitesDir, "_placeholder.caddy")
	if _, err := os.Stat(placeholder); os.IsNotExist(err) {
		logger.Infof("Creating Caddy placeholder config at %s", placeholder)
		if err := os.WriteFile(placeholder, []byte("# Vessel placeholder\n"), 0644); err != nil {
			logger.Errorf("failed to write placeholder Caddy config: %v", err)
			return err
		}
	}

	// Write to /etc/caddy/Caddyfile (where Caddy's systemd unit looks by default)
	etcCaddyfile := "/etc/caddy/Caddyfile"
	if err := os.MkdirAll(filepath.Dir(etcCaddyfile), 0755); err != nil {
		logger.Errorf("failed to create etc Caddy directory: %v", err)
		return err
	}

	// Only overwrite if it's ours or doesn't exist
	existing, _ := os.ReadFile(etcCaddyfile)
	if len(existing) > 0 && !strings.Contains(string(existing), "Vessel-managed") {
		// User has a custom Caddyfile — don't touch it
		logger.Infof("Existing custom Caddyfile detected at %s. Skipping modification.", etcCaddyfile)
		return nil
	}

	content := fmt.Sprintf("# Vessel-managed Caddyfile\n# Do not edit manually\n\nimport %s/*.caddy\n", sitesDir)
	logger.Infof("Writing Vessel-managed Caddyfile config to %s", etcCaddyfile)
	if err := os.WriteFile(etcCaddyfile, []byte(content), 0644); err != nil {
		logger.Errorf("failed to write Caddyfile: %v", err)
		return err
	}

	// Reload Caddy gracefully — ignore errors (Caddy may not be running yet)
	_ = m.reload()
	return nil
}

// AddRoute creates a Caddy site config for a deployment.
func (m *Manager) AddRoute(domain string, internalPort int, deploymentName string) error {
	logger.Infof("Adding proxy route for domain '%s' targeting port '%d' (deployment: %s)...", domain, internalPort, deploymentName)
	if err := os.MkdirAll(m.sitesDir(), 0755); err != nil {
		logger.Errorf("failed to create sites directory: %v", err)
		return err
	}

	cfg := siteConfig{
		Domain:         domain,
		InternalPort:   internalPort,
		DeploymentName: deploymentName,
	}

	content, err := renderSiteConfig(cfg)
	if err != nil {
		logger.Errorf("failed to render site config template: %v", err)
		return err
	}

	dest := m.sitePath(domain)
	logger.Debugf("Writing site config file at %s", dest)
	if err := os.WriteFile(dest, []byte(content), 0644); err != nil {
		logger.Errorf("failed to write site config file to %s: %v", dest, err)
		return err
	}

	return m.reload()
}

// RemoveRoute deletes a Caddy site config.
func (m *Manager) RemoveRoute(domain string) error {
	logger.Infof("Removing proxy route for domain '%s'...", domain)
	path := m.sitePath(domain)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		logger.Errorf("failed to delete site config at %s: %v", path, err)
		return err
	}
	return m.reload()
}

// reload signals Caddy to reload its configuration gracefully.
func (m *Manager) reload() error {
	logger.Infof("Reloading Caddy configuration...")
	// Try caddy reload first
	if err := exec.Command("caddy", "reload", "--config", "/etc/caddy/Caddyfile").Run(); err == nil {
		logger.Infof("Caddy reloaded successfully using 'caddy reload'")
		return nil
	}
	// Fall back to systemctl reload
	if err := exec.Command("systemctl", "reload", "caddy").Run(); err == nil {
		logger.Infof("Caddy reloaded successfully using 'systemctl reload caddy'")
		return nil
	}
	logger.Errorf("Caddy configuration reload failed")
	return fmt.Errorf("caddy reload failed")
}

func (m *Manager) sitesDir() string {
	return filepath.Join(m.caddyDir, "sites")
}

func (m *Manager) sitePath(domain string) string {
	safe := strings.ReplaceAll(domain, ".", "_")
	safe = strings.ReplaceAll(safe, "*", "wildcard")
	return filepath.Join(m.sitesDir(), safe+".caddy")
}

// --- template ---

type siteConfig struct {
	Domain         string
	InternalPort   int
	DeploymentName string
}

const siteTemplate = `# Vessel deployment: {{ .DeploymentName }}
{{ .Domain }} {
    reverse_proxy localhost:{{ .InternalPort }} {
        health_uri /
        health_interval 30s
    }

    encode gzip

    log {
        output file /var/log/caddy/{{ .DeploymentName }}.log {
            roll_size 10mb
            roll_keep 5
        }
    }
}
`

func renderSiteConfig(cfg siteConfig) (string, error) {
	tmpl, err := template.New("site").Parse(siteTemplate)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, cfg); err != nil {
		return "", err
	}
	return sb.String(), nil
}
