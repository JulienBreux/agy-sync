package config

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
	"go.yaml.in/yaml/v3"
)

// Config represents the core configuration parameters for agy-sync.
type Config struct {
	ProjectID           string `mapstructure:"project_id" yaml:"project_id"`
	DatabaseID          string `mapstructure:"database_id" yaml:"database_id"`
	MachineID           string `mapstructure:"machine_id" yaml:"machine_id"`
	BrainDir            string `mapstructure:"brain_dir" yaml:"brain_dir"`
	ConversationsDir    string `mapstructure:"conversations_dir" yaml:"conversations_dir"`
	SummariesDB         string `mapstructure:"summaries_db" yaml:"summaries_db"`
	TransactionsDB      string `mapstructure:"transactions_db" yaml:"transactions_db"`
	NoDBSync            bool   `mapstructure:"no_db_sync" yaml:"no_db_sync"`
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

// DefaultConversationsDir returns the default directory for Antigravity SQLite conversation storage.
func DefaultConversationsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "conversations")
}

// DefaultSummariesDB returns the default path for Antigravity conversation summaries database.
func DefaultSummariesDB() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".gemini", "antigravity-cli", "conversation_summaries.db")
}

// DefaultTransactionsDB returns the default path for the sync transactions database.
func DefaultTransactionsDB() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "transactions.db"
	}
	return filepath.Join(home, ".config", "agy-sync", "transactions.db")
}

// DefaultConfigPath returns the default path to the user's agy-sync config.yaml.
func DefaultConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "config.yaml"
	}
	return filepath.Join(home, ".config", "agy-sync", "config.yaml")
}

// DefaultMachineID returns the hostname or a fallback identifier.
func DefaultMachineID() string {
	hostname, _ := os.Hostname()
	return cmp.Or(hostname, "unknown-machine")
}

// DefaultConfig returns a Config populated with standard defaults.
func DefaultConfig() *Config {
	return &Config{
		DatabaseID:          "(default)",
		MachineID:           DefaultMachineID(),
		BrainDir:            DefaultBrainDir(),
		ConversationsDir:    DefaultConversationsDir(),
		SummariesDB:         DefaultSummariesDB(),
		TransactionsDB:      DefaultTransactionsDB(),
		NoDBSync:            false,
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
	if !c.NoDBSync {
		if strings.TrimSpace(c.ConversationsDir) == "" {
			return errors.New("conversations_dir is required when db sync is enabled")
		}
		if strings.TrimSpace(c.SummariesDB) == "" {
			return errors.New("summaries_db is required when db sync is enabled")
		}
	}
	return nil
}

// LoadConfig loads configuration from a file path with environment variable overrides.
func LoadConfig(path string) (*Config, error) {
	v := viper.New()

	v.SetDefault("database_id", "(default)")
	v.SetDefault("machine_id", DefaultMachineID())
	v.SetDefault("brain_dir", DefaultBrainDir())
	v.SetDefault("conversations_dir", DefaultConversationsDir())
	v.SetDefault("summaries_db", DefaultSummariesDB())
	v.SetDefault("transactions_db", DefaultTransactionsDB())
	v.SetDefault("no_db_sync", false)
	v.SetDefault("sync_interval_seconds", 2)
	v.SetDefault("log_level", "INFO")

	// Environment variable support: AGY_SYNC_PROJECT_ID -> project_id
	v.SetEnvPrefix("AGY_SYNC")
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
