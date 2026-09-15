package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

// Config represents the core configuration parameters for ayg-sync.
type Config struct {
	ProjectID           string `mapstructure:"project_id" yaml:"project_id"`
	DatabaseID          string `mapstructure:"database_id" yaml:"database_id"`
	MachineID           string `mapstructure:"machine_id" yaml:"machine_id"`
	BrainDir            string `mapstructure:"brain_dir" yaml:"brain_dir"`
	SyncIntervalSeconds int    `mapstructure:"sync_interval_seconds" yaml:"sync_interval_seconds"`
	LogLevel            string `mapstructure:"log_level" yaml:"log_level"`
}

// DefaultBrainDir returns the default directory for Antigravity conversation brain storage.
func DefaultBrainDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "brain")
}

// DefaultConfigPath returns the default path to the user's ayg-sync config.yaml.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "ayg-sync", "config.yaml")
}

// DefaultMachineID returns the hostname or a fallback identifier.
func DefaultMachineID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "unknown-machine"
	}
	return hostname
}

// DefaultConfig returns a Config populated with standard defaults.
func DefaultConfig() *Config {
	return &Config{
		DatabaseID:          "(default)",
		MachineID:           DefaultMachineID(),
		BrainDir:            DefaultBrainDir(),
		SyncIntervalSeconds: 2,
		LogLevel:            "INFO",
	}
}

// Validate verifies that required fields are present and valid.
func (c *Config) Validate() error {
	if strings.TrimSpace(c.ProjectID) == "" {
		return errors.New("project_id is required")
	}
	if strings.TrimSpace(c.MachineID) == "" {
		return errors.New("machine_id is required")
	}
	if strings.TrimSpace(c.BrainDir) == "" {
		return errors.New("brain_dir is required")
	}
	return nil
}

// LoadConfig loads configuration from a file path with environment variable overrides.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	v.SetDefault("database_id", "(default)")
	v.SetDefault("machine_id", DefaultMachineID())
	v.SetDefault("brain_dir", DefaultBrainDir())
	v.SetDefault("sync_interval_seconds", 2)
	v.SetDefault("log_level", "INFO")

	// Environment variable support: AYG_SYNC_PROJECT_ID -> project_id
	v.SetEnvPrefix("AYG_SYNC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if path != "" {
		v.SetConfigFile(path)
		if err := v.ReadInConfig(); err != nil {
			if !os.IsNotExist(err) && !errors.As(err, &viper.ConfigFileNotFoundError{}) {
				return nil, fmt.Errorf("failed to read config file: %w", err)
			}
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	return &cfg, nil
}

// SaveConfig serializes the configuration to a YAML file at the specified path.
func SaveConfig(path string, cfg *Config) error {
	if cfg == nil {
		return errors.New("config cannot be nil")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("failed to create config directory %s: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode config to yaml: %w", err)
	}

	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}

	return nil
}
