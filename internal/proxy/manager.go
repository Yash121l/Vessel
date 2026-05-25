package proxy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// Manager handles Caddy reverse proxy configuration.
type Manager struct {
	caddyDir string
}

// NewManager creates a new proxy manager.
func NewManager(caddyDir string) *Manager {
	return &Manager{caddyDir: caddyDir}
}

// AddRoute creates a Caddy site config for a deployment.
func (m *Manager) AddRoute(domain string, internalPort int, deploymentName string) error {
	if err := os.MkdirAll(m.sitesDir(), 0755); err != nil {
		return err
	}

	cfg := siteConfig{
		Domain:         domain,
		InternalPort:   internalPort,
		DeploymentName: deploymentName,
	}

	content, err := renderSiteConfig(cfg)
	if err != nil {
		return err
	}

	path := m.sitePath(domain)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return err
	}

	return m.reload()
}

// RemoveRoute deletes a Caddy site config.
func (m *Manager) RemoveRoute(domain string) error {
	path := m.sitePath(domain)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return m.reload()
}

// EnsureMainConfig writes the main Caddyfile that imports site configs.
func (m *Manager) EnsureMainConfig() error {
	if err := os.MkdirAll(m.sitesDir(), 0755); err != nil {
		return err
	}

	mainCaddyfile := filepath.Join(m.caddyDir, "Caddyfile")
	content := fmt.Sprintf(`# Vessel-managed Caddyfile
# Do not edit manually — managed by Vessel

import %s/*.caddy
`, m.sitesDir())

	// Only write if it doesn't exist or is managed by us
	existing, err := os.ReadFile(mainCaddyfile)
	if err == nil && !strings.Contains(string(existing), "Vessel-managed") {
		// Don't overwrite a user-managed Caddyfile
		return nil
	}

	if err := os.WriteFile(mainCaddyfile, []byte(content), 0644); err != nil {
		return err
	}

	// Symlink to /etc/caddy/Caddyfile if it exists
	etcCaddy := "/etc/caddy/Caddyfile"
	if _, err := os.Stat(filepath.Dir(etcCaddy)); err == nil {
		_ = os.Remove(etcCaddy)
		_ = os.Symlink(mainCaddyfile, etcCaddy)
	}

	return m.reload()
}

// reload signals Caddy to reload its configuration.
func (m *Manager) reload() error {
	// Try caddy reload first (graceful)
	if err := exec.Command("caddy", "reload", "--config", filepath.Join(m.caddyDir, "Caddyfile")).Run(); err == nil {
		return nil
	}
	// Fall back to systemctl reload
	return exec.Command("systemctl", "reload", "caddy").Run()
}

func (m *Manager) sitesDir() string {
	return filepath.Join(m.caddyDir, "sites")
}

func (m *Manager) sitePath(domain string) string {
	// Sanitize domain for use as filename
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
