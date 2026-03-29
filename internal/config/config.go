package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// AgentConfig holds per-agent sync configuration.
type AgentConfig struct {
	Source string            `mapstructure:"source" yaml:"source"`
	Sync   []string          `mapstructure:"sync" yaml:"sync"`
	Links  map[string]string `mapstructure:"links,omitempty" yaml:"links,omitempty"` // remote project dir -> local path
}

// Config is the top-level negent configuration.
type Config struct {
	Backend string                 `mapstructure:"backend" yaml:"backend"`
	Repo    string                 `mapstructure:"repo" yaml:"repo"`
	Machine string                 `mapstructure:"machine" yaml:"machine"`
	Agents  map[string]AgentConfig `mapstructure:"agents" yaml:"agents"`
}

// DefaultPath returns the default config file path.
// On Linux: $XDG_CONFIG_HOME/negent/config.yaml or ~/.config/negent/config.yaml
// On macOS: ~/Library/Application Support/negent/config.yaml
func DefaultPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "negent", "config.yaml")
}

// Load reads the config from the given path.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	// Defaults
	v.SetDefault("backend", "git")
	v.SetDefault("agents", map[string]AgentConfig{})

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	return &cfg, nil
}

// Save writes the config to the given path, creating parent directories
// as needed.
func Save(cfg *Config, path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating config dir: %w", err)
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	v.Set("backend", cfg.Backend)
	v.Set("repo", cfg.Repo)
	v.Set("machine", cfg.Machine)
	v.Set("agents", cfg.Agents)

	if Exists(path) {
		return v.WriteConfig()
	}
	return v.WriteConfigAs(path)
}

// Exists returns true if the config file exists at the given path.
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
