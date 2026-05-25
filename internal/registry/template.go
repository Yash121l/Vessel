package registry

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// EnvVar defines a required or optional environment variable for an app.
type EnvVar struct {
	Key         string `yaml:"key"`
	Description string `yaml:"description"`
	Default     string `yaml:"default"`
	Required    bool   `yaml:"required"`
	Secret      bool   `yaml:"secret"`
}

// Port defines a port mapping for an app.
type Port struct {
	Internal int    `yaml:"internal"`
	External int    `yaml:"external"`
	Protocol string `yaml:"protocol"` // tcp, udp
}

// Volume defines a persistent volume for an app.
type Volume struct {
	Name        string `yaml:"name"`
	MountPath   string `yaml:"mount_path"`
	Description string `yaml:"description"`
}

// HealthCheck defines a Docker health check.
type HealthCheck struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
}

// AppTemplate is the full definition of a deployable application.
type AppTemplate struct {
	ID          string      `yaml:"id"`
	Name        string      `yaml:"name"`
	Description string      `yaml:"description"`
	Category    string      `yaml:"category"`
	Icon        string      `yaml:"icon"`
	Version     string      `yaml:"version"`
	Image       string      `yaml:"image"`
	Ports       []Port      `yaml:"ports"`
	Volumes     []Volume    `yaml:"volumes"`
	EnvVars     []EnvVar    `yaml:"env_vars"`
	HealthCheck HealthCheck `yaml:"health_check"`
	// ProxyPort is the internal port Caddy should forward to.
	ProxyPort int `yaml:"proxy_port"`
	// ExtraServices are sidecar services (e.g. databases) defined inline.
	ExtraServices []ServiceDef `yaml:"extra_services"`
}

// ServiceDef defines a sidecar service (e.g. postgres, redis).
type ServiceDef struct {
	Name        string            `yaml:"name"`
	Image       string            `yaml:"image"`
	Environment map[string]string `yaml:"environment"`
	Volumes     []Volume          `yaml:"volumes"`
	HealthCheck HealthCheck       `yaml:"health_check"`
}

// Registry holds all available app templates.
type Registry struct {
	templates map[string]*AppTemplate
}

// New creates a Registry pre-loaded with built-in templates.
func New() *Registry {
	r := &Registry{templates: make(map[string]*AppTemplate)}
	for _, t := range builtinTemplates() {
		r.templates[t.ID] = t
	}
	return r
}

// LoadFromDir loads additional YAML templates from a directory.
func (r *Registry) LoadFromDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		var t AppTemplate
		if err := yaml.Unmarshal(data, &t); err != nil {
			return fmt.Errorf("parse template %s: %w", e.Name(), err)
		}
		r.templates[t.ID] = &t
	}
	return nil
}

// Get returns a template by ID.
func (r *Registry) Get(id string) (*AppTemplate, bool) {
	t, ok := r.templates[id]
	return t, ok
}

// List returns all templates.
func (r *Registry) List() []*AppTemplate {
	out := make([]*AppTemplate, 0, len(r.templates))
	for _, t := range r.templates {
		out = append(out, t)
	}
	return out
}
