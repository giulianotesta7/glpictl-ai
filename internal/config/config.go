package config

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

// Config holds GLPI connection settings loaded from TOML, env vars, or flags.
// After Load(), the config is immutable and validated.
type Config struct {
	GLPI   GLPIConfig
	Server ServerConfig
}

// GLPIConfig holds GLPI API connection settings.
type GLPIConfig struct {
	URL         string `toml:"url"`
	AppToken    string `toml:"app_token"`
	UserToken   string `toml:"user_token"`
	InsecureSSL bool   `toml:"insecure_ssl"`
}

// ServerConfig holds server settings.
type ServerConfig struct {
	Timeout  int    `toml:"timeout"`
	LogLevel string `toml:"log_level"`
}

// CLIOverrides holds command-line flag values that override config.
type CLIOverrides struct {
	GLPIURL       string
	GLPIAppToken  string
	GLPIUserToken string
	Timeout       int
	LogLevel      string
	InsecureSSL   *bool // Use pointer to distinguish unset from false
}

// defaultConfig returns a Config with default values.
func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Timeout:  30,
			LogLevel: "info",
		},
	}
}

// applyDefaults fills in default values for missing fields.
func applyDefaults(cfg *Config) {
	if cfg.Server.Timeout == 0 {
		cfg.Server.Timeout = 30
	}
	if cfg.Server.LogLevel == "" {
		cfg.Server.LogLevel = "info"
	}
}

// validate checks that all required fields are set.
func validate(cfg *Config) error {
	if cfg.GLPI.URL == "" {
		return NewMissingRequiredError("glpi.url")
	}
	if cfg.GLPI.AppToken == "" {
		return NewMissingRequiredError("glpi.app_token")
	}
	if cfg.GLPI.UserToken == "" {
		return NewMissingRequiredError("glpi.user_token")
	}
	if cfg.Server.Timeout <= 0 {
		return fmt.Errorf("config: server.timeout must be positive, got %d", cfg.Server.Timeout)
	}
	return nil
}

// GetConfigPath returns the path to the config file.
// If path is empty, it uses XDG_CONFIG_HOME/glpictl-ai/config.toml.
func GetConfigPath(path string) (string, error) {
	if path != "" {
		return path, nil
	}

	// Use XDG_CONFIG_HOME or default to ~/.config
	xdgConfigHome := os.Getenv("XDG_CONFIG_HOME")
	if xdgConfigHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		xdgConfigHome = filepath.Join(homeDir, ".config")
	}

	return filepath.Join(xdgConfigHome, "glpictl-ai", "config.toml"), nil
}

// getConfigPath is the internal alias used by Load and LoadWithOverrides.
func getConfigPath(path string) (string, error) {
	return GetConfigPath(path)
}

// Load reads the config file from the given path and applies env vars.
// Priority: env vars > TOML file > defaults.
// If path is empty, it uses XDG_CONFIG_HOME/glpictl-ai/config.toml.
func Load(path string) (*Config, error) {
	configPath, err := getConfigPath(path)
	if err != nil {
		return nil, err
	}

	// Check if file exists
	if _, err := os.Stat(configPath); err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("cannot access config file %s: %w", configPath, err)
	}

	cfg := defaultConfig()
	if _, err := toml.DecodeFile(configPath, cfg); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidType, err)
	}

	applyDefaults(cfg)
	applyEnvVars(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadFromEnv creates a config solely from environment variables.
// Env vars: GLPICTL_GLPI_URL, GLPICTL_GLPI_APP_TOKEN, GLPICTL_GLPI_USER_TOKEN,
// GLPICTL_TIMEOUT, GLPICTL_LOG_LEVEL, GLPICTL_INSECURE_SSL.
func LoadFromEnv() (*Config, error) {
	cfg := defaultConfig()
	applyEnvVars(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// LoadWithOverrides reads config with priority: CLI > env > TOML > defaults.
func LoadWithOverrides(path string, overrides *CLIOverrides) (*Config, error) {
	configPath, err := getConfigPath(path)
	if err != nil {
		return nil, err
	}

	cfg := defaultConfig()

	// Check if file exists
	if _, err := os.Stat(configPath); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("cannot access config file %s: %w", configPath, err)
		}
		// File doesn't exist — proceed with defaults + env + CLI
	} else {
		if _, err := toml.DecodeFile(configPath, cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidType, err)
		}
	}

	applyDefaults(cfg)
	applyEnvVars(cfg)

	if overrides != nil {
		applyCLIOverrides(cfg, overrides)
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnvVars applies environment variable overrides to the config.
func applyEnvVars(cfg *Config) {
	if v := os.Getenv("GLPICTL_GLPI_URL"); v != "" {
		cfg.GLPI.URL = v
	}
	if v := os.Getenv("GLPICTL_GLPI_APP_TOKEN"); v != "" {
		cfg.GLPI.AppToken = v
	}
	if v := os.Getenv("GLPICTL_GLPI_USER_TOKEN"); v != "" {
		cfg.GLPI.UserToken = v
	}
	if v := os.Getenv("GLPICTL_TIMEOUT"); v != "" {
		if timeout, err := strconv.Atoi(v); err == nil {
			cfg.Server.Timeout = timeout
		} else {
			slog.Warn("ignoring invalid GLPICTL_TIMEOUT", "value", v, "error", err)
		}
	}
	if v := os.Getenv("GLPICTL_LOG_LEVEL"); v != "" {
		cfg.Server.LogLevel = v
	}
	if v := os.Getenv("GLPICTL_INSECURE_SSL"); v != "" {
		if insecure, err := strconv.ParseBool(v); err == nil {
			cfg.GLPI.InsecureSSL = insecure
		} else {
			slog.Warn("ignoring invalid GLPICTL_INSECURE_SSL", "value", v, "error", err)
		}
	}
}

// applyCLIOverrides applies CLI flag overrides to the config.
func applyCLIOverrides(cfg *Config, overrides *CLIOverrides) {
	if overrides.GLPIURL != "" {
		cfg.GLPI.URL = overrides.GLPIURL
	}
	if overrides.GLPIAppToken != "" {
		cfg.GLPI.AppToken = overrides.GLPIAppToken
	}
	if overrides.GLPIUserToken != "" {
		cfg.GLPI.UserToken = overrides.GLPIUserToken
	}
	if overrides.Timeout > 0 {
		cfg.Server.Timeout = overrides.Timeout
	}
	if overrides.LogLevel != "" {
		cfg.Server.LogLevel = overrides.LogLevel
	}
	if overrides.InsecureSSL != nil {
		cfg.GLPI.InsecureSSL = *overrides.InsecureSSL
	}
}

// Save writes the config to the specified path as TOML.
// If path is empty, it uses XDG_CONFIG_HOME/glpictl-ai/config.toml.
// Parent directories are created if they don't exist.
// The file is written with 0600 permissions to protect sensitive tokens.
// Uses atomic write (temp file + rename) to prevent partial writes.
func Save(cfg *Config, path string) error {
	if err := validate(cfg); err != nil {
		return fmt.Errorf("cannot save invalid config: %w", err)
	}

	configPath, err := GetConfigPath(path)
	if err != nil {
		return err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("cannot create config directory %s: %w", dir, err)
	}

	// Encode config to TOML
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	// Atomic write: write to temp file in the same directory, then rename
	tmpFile, err := os.CreateTemp(dir, ".config-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()

	// Ensure cleanup on failure
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
		}
	}()

	if _, err := tmpFile.Write(buf.Bytes()); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := tmpFile.Chmod(0600); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to set permissions on temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := atomicRename(tmpPath, configPath); err != nil {
		return fmt.Errorf("failed to atomically replace config file: %w", err)
	}

	success = true
	return nil
}

// IsErrNotFound returns true if err is ErrNotFound.
func IsErrNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsErrMissingRequired returns true if err is ErrMissingRequired.
func IsErrMissingRequired(err error) bool {
	return errors.Is(err, ErrMissingRequired)
}

// IsErrInvalidType returns true if err is ErrInvalidType.
func IsErrInvalidType(err error) bool {
	return errors.Is(err, ErrInvalidType)
}

// atomicRename renames oldPath to newPath, handling Windows where
// os.Rename cannot overwrite an existing file.
func atomicRename(oldPath, newPath string) error {
	err := os.Rename(oldPath, newPath)
	if err == nil {
		return nil
	}
	// On Windows, rename fails if newPath already exists.
	// Remove the destination and retry.
	if removeErr := os.Remove(newPath); removeErr != nil {
		return fmt.Errorf("rename failed (%w) and cleanup of destination also failed: %w", err, removeErr)
	}
	return os.Rename(oldPath, newPath)
}
