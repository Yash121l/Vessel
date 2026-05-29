package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	DefaultPort       = 4800
	DefaultDataDir    = "/var/lib/vessel"
	DefaultConfigFile = "/etc/vessel/config.yaml"
)

// Config holds all Vessel runtime configuration.
type Config struct {
	Port       int    `yaml:"port"`
	DataDir    string `yaml:"data_dir"`
	ConfigFile string `yaml:"-"`

	// Derived paths (not in YAML)
	DeploymentsDir string `yaml:"-"`
	TemplatesDir   string `yaml:"-"`
	BackupsDir     string `yaml:"-"`
	CaddyDir       string `yaml:"-"`
	DBPath         string `yaml:"-"`
}

type fileConfig struct {
	Port    int    `yaml:"port"`
	DataDir string `yaml:"data_dir"`
}

// Load reads config from disk, falling back to defaults.
func Load() (*Config, error) {
	cfg := &Config{
		Port:       DefaultPort,
		DataDir:    DefaultDataDir,
		ConfigFile: DefaultConfigFile,
	}

	// Allow override via env
	if d := os.Getenv("VESSEL_DATA_DIR"); d != "" {
		cfg.DataDir = d
	}
	if p := os.Getenv("VESSEL_PORT"); p != "" {
		var port int
		if _, err := fmt.Sscanf(p, "%d", &port); err == nil {
			cfg.Port = port
		}
	}

	// Try loading from file
	cfgPath := os.Getenv("VESSEL_CONFIG")
	if cfgPath == "" {
		cfgPath = DefaultConfigFile
	}
	cfg.ConfigFile = cfgPath

	if data, err := os.ReadFile(cfgPath); err == nil {
		var fc fileConfig
		if err := yaml.Unmarshal(data, &fc); err == nil {
			if fc.Port != 0 {
				cfg.Port = fc.Port
			}
			if fc.DataDir != "" {
				cfg.DataDir = fc.DataDir
			}
		}
	}

	cfg.derivePaths()
	return cfg, nil
}

func (c *Config) derivePaths() {
	c.DeploymentsDir = filepath.Join(c.DataDir, "deployments")
	c.TemplatesDir = filepath.Join(c.DataDir, "templates")
	c.BackupsDir = filepath.Join(c.DataDir, "backups")
	c.CaddyDir = filepath.Join(c.DataDir, "caddy")
	c.DBPath = filepath.Join(c.DataDir, "vessel.db")
}

// Save writes the current config to disk.
func (c *Config) Save() error {
	if err := os.MkdirAll(filepath.Dir(c.ConfigFile), 0755); err != nil {
		return err
	}
	fc := fileConfig{
		Port:    c.Port,
		DataDir: c.DataDir,
	}
	data, err := yaml.Marshal(fc)
	if err != nil {
		return err
	}
	return os.WriteFile(c.ConfigFile, data, 0644)
}
