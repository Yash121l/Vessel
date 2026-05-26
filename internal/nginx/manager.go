package nginx

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Common nginx config paths across distros.
var configRoots = []string{
	"/etc/nginx",
	"/usr/local/nginx/conf",
	"/opt/nginx/conf",
}

// Manager provides nginx management operations.
type Manager struct {
	configRoot string
}

// NewManager detects the nginx config root and returns a Manager.
func NewManager() *Manager {
	for _, root := range configRoots {
		if _, err := os.Stat(filepath.Join(root, "nginx.conf")); err == nil {
			return &Manager{configRoot: root}
		}
	}
	return &Manager{configRoot: "/etc/nginx"}
}

// Status holds the current nginx process status.
type Status struct {
	Running    bool   `json:"running"`
	Version    string `json:"version"`
	ConfigRoot string `json:"config_root"`
	PID        string `json:"pid"`
}

// GetStatus returns nginx running state and version.
func (m *Manager) GetStatus() Status {
	s := Status{ConfigRoot: m.configRoot}

	// Check if running via systemctl or pidfile
	if out, err := exec.Command("systemctl", "is-active", "nginx").Output(); err == nil {
		s.Running = strings.TrimSpace(string(out)) == "active"
	} else {
		// Fallback: check pidfile
		pidFile := filepath.Join(m.configRoot, "../run/nginx.pid")
		if _, err := os.Stat("/run/nginx.pid"); err == nil {
			pidFile = "/run/nginx.pid"
		}
		if data, err := os.ReadFile(pidFile); err == nil {
			s.PID = strings.TrimSpace(string(data))
			s.Running = s.PID != ""
		}
	}

	// Get version
	if out, err := exec.Command("nginx", "-v").CombinedOutput(); err == nil {
		s.Version = strings.TrimSpace(string(out))
	}

	return s
}

// SiteFile represents an nginx site config file.
type SiteFile struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Enabled bool      `json:"enabled"`
	Content string    `json:"content,omitempty"`
	ModTime time.Time `json:"mod_time"`
}

// ListSites returns all site configs from sites-available and conf.d.
func (m *Manager) ListSites() ([]SiteFile, error) {
	available := filepath.Join(m.configRoot, "sites-available")
	enabled := filepath.Join(m.configRoot, "sites-enabled")

	// Build set of enabled sites (symlinks in sites-enabled)
	enabledSet := map[string]bool{}
	if ee, err := os.ReadDir(enabled); err == nil {
		for _, e := range ee {
			target, err := os.Readlink(filepath.Join(enabled, e.Name()))
			if err == nil {
				enabledSet[filepath.Base(target)] = true
			} else {
				enabledSet[e.Name()] = true
			}
		}
	}

	var sites []SiteFile

	// Scan sites-available
	if entries, err := os.ReadDir(available); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			info, _ := e.Info()
			sites = append(sites, SiteFile{
				Name:    e.Name(),
				Path:    filepath.Join(available, e.Name()),
				Enabled: enabledSet[e.Name()],
				ModTime: info.ModTime(),
			})
		}
	}

	// Also scan conf.d (files ending in .conf are active, .disabled are not)
	confD := filepath.Join(m.configRoot, "conf.d")
	if entries, err := os.ReadDir(confD); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			if !strings.HasSuffix(name, ".conf") && !strings.HasSuffix(name, ".disabled") {
				continue
			}
			info, _ := e.Info()
			sites = append(sites, SiteFile{
				Name:    name,
				Path:    filepath.Join(confD, name),
				Enabled: strings.HasSuffix(name, ".conf"),
				ModTime: info.ModTime(),
			})
		}
	}

	if len(sites) == 0 {
		return []SiteFile{}, nil
	}
	return sites, nil
}

// GetSite returns the content of a site config file.
func (m *Manager) GetSite(name string) (*SiteFile, error) {
	sites, err := m.ListSites()
	if err != nil {
		return nil, err
	}
	for _, s := range sites {
		if s.Name == name {
			data, err := os.ReadFile(s.Path)
			if err != nil {
				return nil, err
			}
			s.Content = string(data)
			return &s, nil
		}
	}
	return nil, fmt.Errorf("site not found: %s", name)
}

// SaveSite writes content to a site config file (preserves original path).
func (m *Manager) SaveSite(name, content string) error {
	// Find existing path first
	sites, _ := m.ListSites()
	for _, s := range sites {
		if s.Name == name {
			return os.WriteFile(s.Path, []byte(content), 0644)
		}
	}
	// Default: write to sites-available
	available := filepath.Join(m.configRoot, "sites-available")
	if err := os.MkdirAll(available, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(available, name), []byte(content), 0644)
}

// CreateSite creates a new site config from a template.
func (m *Manager) CreateSite(name, serverName string, port int, upstream string) error {
	content := buildSiteConfig(serverName, port, upstream)
	return m.SaveSite(name, content)
}

// EnableSite creates a symlink in sites-enabled.
func (m *Manager) EnableSite(name string) error {
	available := filepath.Join(m.configRoot, "sites-available", name)
	enabled := filepath.Join(m.configRoot, "sites-enabled", name)
	if err := os.MkdirAll(filepath.Dir(enabled), 0755); err != nil {
		return err
	}
	_ = os.Remove(enabled)
	return os.Symlink(available, enabled)
}

// DisableSite removes the symlink from sites-enabled.
func (m *Manager) DisableSite(name string) error {
	enabled := filepath.Join(m.configRoot, "sites-enabled", name)
	return os.Remove(enabled)
}

// DeleteSite removes a site config file (and its symlink).
func (m *Manager) DeleteSite(name string) error {
	_ = m.DisableSite(name)
	path := filepath.Join(m.configRoot, "sites-available", name)
	return os.Remove(path)
}

// TestConfig runs nginx -t and returns the output.
func (m *Manager) TestConfig() (string, bool) {
	out, err := exec.Command("nginx", "-t").CombinedOutput()
	return string(out), err == nil
}

// Reload sends SIGHUP to nginx (graceful reload).
func (m *Manager) Reload() error {
	if err := exec.Command("systemctl", "reload", "nginx").Run(); err != nil {
		// Fallback: nginx -s reload
		return exec.Command("nginx", "-s", "reload").Run()
	}
	return nil
}

// Restart restarts nginx.
func (m *Manager) Restart() error {
	if err := exec.Command("systemctl", "restart", "nginx").Run(); err != nil {
		return exec.Command("nginx", "-s", "stop").Run()
	}
	return nil
}

// Stop stops nginx.
func (m *Manager) Stop() error {
	return exec.Command("systemctl", "stop", "nginx").Run()
}

// Start starts nginx.
func (m *Manager) Start() error {
	return exec.Command("systemctl", "start", "nginx").Run()
}

// GetMainConfig returns the content of nginx.conf.
func (m *Manager) GetMainConfig() (string, error) {
	data, err := os.ReadFile(filepath.Join(m.configRoot, "nginx.conf"))
	return string(data), err
}

// SaveMainConfig writes nginx.conf.
func (m *Manager) SaveMainConfig(content string) error {
	return os.WriteFile(filepath.Join(m.configRoot, "nginx.conf"), []byte(content), 0644)
}

// LogEntry is a parsed nginx access log line.
type LogEntry struct {
	Raw string `json:"raw"`
}

// TailLog streams the last N lines of an nginx log file.
func (m *Manager) TailLog(logType string, n int) ([]string, error) {
	paths := map[string][]string{
		"access": {"/var/log/nginx/access.log", filepath.Join(m.configRoot, "../log/nginx/access.log")},
		"error":  {"/var/log/nginx/error.log", filepath.Join(m.configRoot, "../log/nginx/error.log")},
	}
	candidates, ok := paths[logType]
	if !ok {
		return nil, fmt.Errorf("unknown log type: %s", logType)
	}

	var logPath string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			logPath = p
			break
		}
	}
	if logPath == "" {
		return nil, fmt.Errorf("log file not found")
	}

	out, err := exec.Command("tail", "-n", fmt.Sprintf("%d", n), logPath).Output()
	if err != nil {
		return nil, err
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, nil
}

// StreamLog streams a log file live to a channel.
func (m *Manager) StreamLog(ctx context.Context, logType string, lines chan<- string) error {
	paths := map[string]string{
		"access": "/var/log/nginx/access.log",
		"error":  "/var/log/nginx/error.log",
	}
	logPath, ok := paths[logType]
	if !ok {
		return fmt.Errorf("unknown log type: %s", logType)
	}

	cmd := exec.CommandContext(ctx, "tail", "-f", "-n", "100", logPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	scanner := bufio.NewScanner(stdout)
	done := make(chan error, 1)
	go func() {
		for scanner.Scan() {
			select {
			case lines <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}
		done <- cmd.Wait()
	}()

	select {
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// buildSiteConfig generates a basic reverse proxy nginx site config.
func buildSiteConfig(serverName string, port int, upstream string) string {
	if upstream == "" {
		upstream = fmt.Sprintf("localhost:%d", port)
	}
	return fmt.Sprintf(`server {
    listen 80;
    server_name %s;

    location / {
        proxy_pass http://%s;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_cache_bypass $http_upgrade;
    }
}
`, serverName, upstream)
}
