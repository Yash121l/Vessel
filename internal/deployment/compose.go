package deployment

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/vessel-app/vessel/internal/registry"
	"github.com/vessel-app/vessel/internal/store"
)

// ComposeFile represents a Docker Compose file structure.
type ComposeFile struct {
	Version  string                    `yaml:"version"`
	Services map[string]ComposeService `yaml:"services"`
	Volumes  map[string]ComposeVolume  `yaml:"volumes,omitempty"`
	Networks map[string]ComposeNetwork `yaml:"networks,omitempty"`
}

// ComposeService represents a single service in a Compose file.
type ComposeService struct {
	Image       string            `yaml:"image"`
	Restart     string            `yaml:"restart"`
	Ports       []string          `yaml:"ports,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	DependsOn   map[string]DependsOnCondition `yaml:"depends_on,omitempty"`
	HealthCheck *ComposeHealthCheck           `yaml:"healthcheck,omitempty"`
	Labels      map[string]string             `yaml:"labels,omitempty"`
	MemLimit    string                        `yaml:"mem_limit,omitempty"`
}

// DependsOnCondition specifies the condition for depends_on.
type DependsOnCondition struct {
	Condition string `yaml:"condition"`
}

// ComposeHealthCheck mirrors Docker's healthcheck config.
type ComposeHealthCheck struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
}

// ComposeVolume is a named volume definition.
type ComposeVolume struct {
	Driver string `yaml:"driver,omitempty"`
}

// ComposeNetwork is a network definition.
type ComposeNetwork struct {
	Driver string `yaml:"driver,omitempty"`
}

// GenerateCompose builds a Docker Compose file for a deployment.
func GenerateCompose(tmpl *registry.AppTemplate, d *store.Deployment) (*ComposeFile, error) {
	networkName := fmt.Sprintf("vessel-%s", d.Name)

	cf := &ComposeFile{
		Version:  "3.8",
		Services: make(map[string]ComposeService),
		Volumes:  make(map[string]ComposeVolume),
		Networks: map[string]ComposeNetwork{
			networkName: {Driver: "bridge"},
		},
	}

	// Build main service
	mainSvc := ComposeService{
		Image:       tmpl.Image,
		Restart:     "unless-stopped",
		Networks:    []string{networkName},
		Environment: make(map[string]string),
		Labels: map[string]string{
			"vessel.app":        tmpl.ID,
			"vessel.deployment": d.Name,
			"vessel.managed":    "true",
		},
	}

	// Ports — only expose if no domain (Caddy handles routing when domain is set)
	if d.Domain == "" {
		for _, p := range tmpl.Ports {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			mainSvc.Ports = append(mainSvc.Ports, fmt.Sprintf("%d:%d/%s", p.External, p.Internal, proto))
		}
	}

	// Environment variables
	for _, ev := range tmpl.EnvVars {
		val := ev.Default
		if v, ok := d.Env[ev.Key]; ok {
			val = v
		}
		mainSvc.Environment[ev.Key] = val
	}
	// Merge any extra env vars from deployment
	for k, v := range d.Env {
		if _, exists := mainSvc.Environment[k]; !exists {
			mainSvc.Environment[k] = v
		}
	}

	// Volumes
	for _, vol := range tmpl.Volumes {
		volName := fmt.Sprintf("%s-%s", d.Name, vol.Name)
		mainSvc.Volumes = append(mainSvc.Volumes, fmt.Sprintf("%s:%s", volName, vol.MountPath))
		cf.Volumes[volName] = ComposeVolume{}
	}

	// Health check
	if len(tmpl.HealthCheck.Test) > 0 {
		mainSvc.HealthCheck = &ComposeHealthCheck{
			Test:     tmpl.HealthCheck.Test,
			Interval: tmpl.HealthCheck.Interval,
			Timeout:  tmpl.HealthCheck.Timeout,
			Retries:  tmpl.HealthCheck.Retries,
		}
	}

	// Extra services (sidecars)
	if len(tmpl.ExtraServices) > 0 {
		mainSvc.DependsOn = make(map[string]DependsOnCondition)
	}

	for _, svc := range tmpl.ExtraServices {
		svcName := fmt.Sprintf("%s-%s", d.Name, svc.Name)
		// Also register the bare name as an alias for inter-service DNS
		extraSvc := ComposeService{
			Image:       svc.Image,
			Restart:     "unless-stopped",
			Networks:    []string{networkName},
			Environment: svc.Environment,
			Labels: map[string]string{
				"vessel.app":        tmpl.ID,
				"vessel.deployment": d.Name,
				"vessel.managed":    "true",
				"vessel.sidecar":    "true",
			},
		}

		for _, vol := range svc.Volumes {
			volName := fmt.Sprintf("%s-%s", d.Name, vol.Name)
			extraSvc.Volumes = append(extraSvc.Volumes, fmt.Sprintf("%s:%s", volName, vol.MountPath))
			cf.Volumes[volName] = ComposeVolume{}
		}

		if len(svc.HealthCheck.Test) > 0 {
			extraSvc.HealthCheck = &ComposeHealthCheck{
				Test:     svc.HealthCheck.Test,
				Interval: svc.HealthCheck.Interval,
				Timeout:  svc.HealthCheck.Timeout,
				Retries:  svc.HealthCheck.Retries,
			}
			mainSvc.DependsOn[svcName] = DependsOnCondition{Condition: "service_healthy"}
		}

		cf.Services[svcName] = extraSvc

		// Rewrite env vars in main service that reference the bare sidecar name
		// to use the prefixed name (e.g. "umami-db" → "<deployment>-umami-db")
		for k, v := range mainSvc.Environment {
			mainSvc.Environment[k] = strings.ReplaceAll(v, svc.Name, svcName)
		}
	}

	cf.Services[d.Name] = mainSvc
	return cf, nil
}

// WriteCompose serializes a ComposeFile to disk.
func WriteCompose(cf *ComposeFile, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cf)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "docker-compose.yml"), data, 0644)
}

// WriteEnvFile writes a .env file for a deployment.
func WriteEnvFile(env map[string]string, dir string) error {
	var sb strings.Builder
	for k, v := range env {
		sb.WriteString(fmt.Sprintf("%s=%s\n", k, v))
	}
	return os.WriteFile(filepath.Join(dir, ".env"), []byte(sb.String()), 0600)
}
