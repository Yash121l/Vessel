package registry

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/Yash121l/Vessel/internal/logger"
	"gopkg.in/yaml.v3"
)

//go:embed templates/*.yaml
var embeddedTemplateFS embed.FS

const DefaultRemoteCatalogURL = "https://yash121l.github.io/Vessel/templates/index.json"

// EnvVar defines a required or optional environment variable for an app.
type EnvVar struct {
	Key         string `yaml:"key"         json:"key"`
	Description string `yaml:"description" json:"description"`
	Default     string `yaml:"default"     json:"default"`
	Required    bool   `yaml:"required"    json:"required"`
	Secret      bool   `yaml:"secret"      json:"secret"`
}

// Port defines a port mapping for an app.
type Port struct {
	Internal int    `yaml:"internal" json:"internal"`
	External int    `yaml:"external" json:"external"`
	Protocol string `yaml:"protocol" json:"protocol"`
}

// Volume defines a persistent volume for an app.
type Volume struct {
	Name        string `yaml:"name"        json:"name"`
	MountPath   string `yaml:"mount_path"  json:"mount_path"`
	Description string `yaml:"description" json:"description"`
}

// HealthCheck defines a Docker health check.
type HealthCheck struct {
	Test     []string `yaml:"test"     json:"test"`
	Interval string   `yaml:"interval" json:"interval"`
	Timeout  string   `yaml:"timeout"  json:"timeout"`
	Retries  int      `yaml:"retries"  json:"retries"`
}

// AppTemplate is the full definition of a deployable application.
type AppTemplate struct {
	ID            string       `yaml:"id"             json:"id"`
	Name          string       `yaml:"name"           json:"name"`
	Description   string       `yaml:"description"    json:"description"`
	Category      string       `yaml:"category"       json:"category"`
	Icon          string       `yaml:"icon"           json:"icon"`
	Version       string       `yaml:"version"        json:"version"`
	Image         string       `yaml:"image"          json:"image"`
	Ports         []Port       `yaml:"ports"          json:"ports"`
	Volumes       []Volume     `yaml:"volumes"        json:"volumes"`
	EnvVars       []EnvVar     `yaml:"env_vars"       json:"env_vars"`
	HealthCheck   HealthCheck  `yaml:"health_check"   json:"health_check"`
	ProxyPort     int          `yaml:"proxy_port"     json:"proxy_port"`
	ExtraServices []ServiceDef `yaml:"extra_services" json:"extra_services"`
}

// ServiceDef defines a sidecar service (e.g. postgres, redis).
type ServiceDef struct {
	Name        string            `yaml:"name"         json:"json"`
	Image       string            `yaml:"image"        json:"image"`
	Environment map[string]string `yaml:"environment"  json:"environment"`
	Volumes     []Volume          `yaml:"volumes"      json:"volumes"`
	HealthCheck HealthCheck       `yaml:"health_check" json:"health_check"`
	Ports       []Port            `yaml:"ports"        json:"ports"`
	// Optional marks this service as opt-in. When true, the user can choose
	// to skip it and provide their own external instance (e.g. an existing DB).
	Optional bool `yaml:"optional" json:"optional"`
	// Role is a human-readable label for the service type, e.g. "database", "cache".
	Role string `yaml:"role" json:"role"`
}

// Registry holds all available app templates.
type Registry struct {
	templates map[string]*AppTemplate
}

// New creates a Registry pre-loaded with built-in templates.
func New() *Registry {
	r := &Registry{templates: make(map[string]*AppTemplate)}
	// Keep the hand-written Go definitions as a compatibility fallback. The
	// YAML catalog is loaded next and is the source of truth for bundled apps.
	for _, t := range builtinTemplates() {
		r.templates[t.ID] = t
	}
	if err := r.LoadEmbedded(); err != nil {
		fmt.Printf("warning: failed to load embedded templates: %v\n", err)
	}
	return r
}

// LoadEmbedded loads the YAML catalog bundled with the binary.
func (r *Registry) LoadEmbedded() error {
	logger.Infof("Loading embedded YAML templates...")
	entries, err := embeddedTemplateFS.ReadDir("templates")
	if err != nil {
		logger.Errorf("failed to read embedded templates directory: %v", err)
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		data, err := embeddedTemplateFS.ReadFile(filepath.Join("templates", e.Name()))
		if err != nil {
			logger.Errorf("failed to read embedded template %s: %v", e.Name(), err)
			return err
		}
		if err := r.registerYAML(e.Name(), data); err != nil {
			logger.Errorf("failed to register embedded template %s: %v", e.Name(), err)
			return err
		}
	}
	return nil
}

// LoadFromDir loads additional YAML templates from a directory.
func (r *Registry) LoadFromDir(dir string) error {
	logger.Infof("Loading custom YAML templates from directory: %s", dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Infof("Custom templates directory %s does not exist. Skipping.", dir)
			return nil
		}
		logger.Errorf("failed to read custom templates directory %s: %v", dir, err)
		return err
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			logger.Errorf("failed to read custom template %s: %v", path, err)
			return err
		}
		if err := r.registerYAML(e.Name(), data); err != nil {
			logger.Errorf("failed to register custom template %s: %v", path, err)
			return err
		}
	}
	return nil
}

type remoteCatalog struct {
	Templates []remoteTemplate `json:"templates"`
}

type remoteTemplate struct {
	ID      string `json:"id"`
	URL     string `json:"url"`
	Content string `json:"content"`
}

// LoadFromRemote loads YAML templates from a public catalog. Relative template
// URLs are resolved against the catalog URL, which lets GitHub Pages host both
// index.json and the template files without requiring a new Vessel release.
func (r *Registry) LoadFromRemote(catalogURL string) error {
	if catalogURL == "" {
		catalogURL = DefaultRemoteCatalogURL
	}
	logger.Infof("Fetching templates from remote catalog URL: %s", catalogURL)
	client := &http.Client{Timeout: 4 * time.Second}
	resp, err := client.Get(catalogURL)
	if err != nil {
		logger.Errorf("failed to fetch remote catalog from %s: %v", catalogURL, err)
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Errorf("remote catalog fetch returned status code: %s", resp.Status)
		return fmt.Errorf("catalog returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		logger.Errorf("failed to read remote catalog body: %v", err)
		return err
	}
	var catalog remoteCatalog
	if err := json.Unmarshal(body, &catalog); err != nil {
		logger.Errorf("failed to parse remote catalog json: %v", err)
		return fmt.Errorf("parse remote catalog: %w", err)
	}
	logger.Infof("Remote catalog loaded successfully. Registering %d templates...", len(catalog.Templates))
	base := resp.Request.URL
	for _, item := range catalog.Templates {
		if item.Content != "" {
			name := item.ID
			if name == "" {
				name = "remote template"
			}
			if err := r.registerYAML(name, []byte(item.Content)); err != nil {
				logger.Errorf("failed to register inline remote template %s: %v", name, err)
				return err
			}
			continue
		}
		if item.URL == "" {
			continue
		}
		u, err := base.Parse(item.URL)
		if err != nil {
			logger.Errorf("failed to parse remote template url %s: %v", item.URL, err)
			return fmt.Errorf("resolve template %s: %w", item.ID, err)
		}
		logger.Debugf("Fetching remote template content from: %s", u.String())
		tResp, err := client.Get(u.String())
		if err != nil {
			logger.Errorf("failed to fetch remote template %s: %v", item.ID, err)
			return fmt.Errorf("fetch template %s: %w", item.ID, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(tResp.Body, 2<<20))
		closeErr := tResp.Body.Close()
		if readErr != nil {
			logger.Errorf("failed to read remote template content: %v", readErr)
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if tResp.StatusCode != http.StatusOK {
			logger.Errorf("remote template fetch returned status: %s", tResp.Status)
			return fmt.Errorf("template %s returned %s", item.ID, tResp.Status)
		}
		if err := r.registerYAML(item.URL, data); err != nil {
			logger.Errorf("failed to register remote template %s: %v", item.URL, err)
			return err
		}
	}
	return nil
}

// Register adds or replaces a template in the registry.
func (r *Registry) Register(t *AppTemplate) {
	r.templates[t.ID] = t
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
	sort.Slice(out, func(i, j int) bool {
		if out[i].Category == out[j].Category {
			return out[i].Name < out[j].Name
		}
		return out[i].Category < out[j].Category
	})
	return out
}

func (r *Registry) registerYAML(name string, data []byte) error {
	var t AppTemplate
	if err := yaml.Unmarshal(data, &t); err != nil {
		logger.Errorf("failed to unmarshal template YAML %s: %v", name, err)
		return fmt.Errorf("parse template %s: %w", name, err)
	}
	if t.ID == "" {
		logger.Errorf("template YAML %s missing required field 'id'", name)
		return fmt.Errorf("parse template %s: missing id", name)
	}
	logger.Debugf("Successfully registered YAML template: %s (ID: %s)", t.Name, t.ID)
	r.templates[t.ID] = &t
	return nil
}
