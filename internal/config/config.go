package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	goconfig "github.com/tpodg/go-config"
)

type Config struct {
	Servers []ServerConfig `yaml:"servers"`
}

type ServerConfig struct {
	Name             string         `yaml:"name"`
	Address          string         `yaml:"address"`
	User             string         `yaml:"user"`
	SSHKey           string         `yaml:"ssh_key"`
	KnownHostsPath   string         `yaml:"known_hosts"`
	UseAgent         *bool          `yaml:"use_agent"`
	HandshakeTimeout time.Duration  `yaml:"handshake_timeout"`
	Tasks            map[string]any `yaml:"tasks"`
}

// Load the configuration from the given file or default locations.
func Load(cfgFile string) (*Config, error) {
	path, err := findConfigFile(cfgFile)
	if err != nil {
		return nil, err
	}

	c := goconfig.New()
	if path != "" {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("failed to get absolute path for %s: %w", path, err)
		}
		c.WithProviders(&goconfig.Yaml{Path: absPath})
	}

	c.WithProviders(&goconfig.Env{Prefix: "SETTLED"})

	cfg := &Config{}
	if err := c.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

func findConfigFile(cfgFile string) (string, error) {
	if cfgFile != "" {
		if _, err := os.Stat(cfgFile); err != nil {
			return "", fmt.Errorf("failed to read config file %s: %w", cfgFile, err)
		}
		return cfgFile, nil
	}

	if home, err := os.UserHomeDir(); err == nil {
		path := filepath.Join(home, ".settled.yaml")
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	if _, err := os.Stat(".settled.yaml"); err == nil {
		return ".settled.yaml", nil
	}

	return "", nil
}
