package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnv(t *testing.T) {
	t.Run("env vars override TOML values", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi]
url = "http://localhost/apirest.php"
app_token = "toml-app-token"
user_token = "toml-user-token"

[server]
timeout = 30
log_level = "info"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Set environment variables
		t.Setenv("GLPICTL_GLPI_URL", "http://override-url/apirest.php")
		t.Setenv("GLPICTL_GLPI_APP_TOKEN", "override-app-token")
		t.Setenv("GLPICTL_GLPI_USER_TOKEN", "override-user-token")
		t.Setenv("GLPICTL_TIMEOUT", "60")
		t.Setenv("GLPICTL_LOG_LEVEL", "debug")

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Env vars should override TOML values
		if cfg.GLPI.URL != "http://override-url/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://override-url/apirest.php")
		}
		if cfg.GLPI.AppToken != "override-app-token" {
			t.Errorf("GLPI.AppToken = %q, want %q", cfg.GLPI.AppToken, "override-app-token")
		}
		if cfg.GLPI.UserToken != "override-user-token" {
			t.Errorf("GLPI.UserToken = %q, want %q", cfg.GLPI.UserToken, "override-user-token")
		}
		if cfg.Server.Timeout != 60 {
			t.Errorf("Server.Timeout = %d, want %d", cfg.Server.Timeout, 60)
		}
		if cfg.Server.LogLevel != "debug" {
			t.Errorf("Server.LogLevel = %q, want %q", cfg.Server.LogLevel, "debug")
		}
	})

	t.Run("env vars work without TOML file", func(t *testing.T) {
		// Set all required env vars
		t.Setenv("GLPICTL_GLPI_URL", "http://env-url/apirest.php")
		t.Setenv("GLPICTL_GLPI_APP_TOKEN", "env-app-token")
		t.Setenv("GLPICTL_GLPI_USER_TOKEN", "env-user-token")

		cfg, err := LoadFromEnv()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.GLPI.URL != "http://env-url/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://env-url/apirest.php")
		}
		if cfg.GLPI.AppToken != "env-app-token" {
			t.Errorf("GLPI.AppToken = %q, want %q", cfg.GLPI.AppToken, "env-app-token")
		}
		if cfg.GLPI.UserToken != "env-user-token" {
			t.Errorf("GLPI.UserToken = %q, want %q", cfg.GLPI.UserToken, "env-user-token")
		}
		// Defaults should apply
		if cfg.Server.Timeout != 30 {
			t.Errorf("Server.Timeout = %d, want default %d", cfg.Server.Timeout, 30)
		}
		if cfg.Server.LogLevel != "info" {
			t.Errorf("Server.LogLevel = %q, want default %q", cfg.Server.LogLevel, "info")
		}
	})

	t.Run("returns error when required env vars missing", func(t *testing.T) {
		// Clear all env vars
		os.Unsetenv("GLPICTL_GLPI_URL")
		os.Unsetenv("GLPICTL_GLPI_APP_TOKEN")
		os.Unsetenv("GLPICTL_GLPI_USER_TOKEN")

		_, err := LoadFromEnv()
		if !IsErrMissingRequired(err) {
			t.Errorf("expected ErrMissingRequired, got %v", err)
		}
	})

	t.Run("priority: CLI > env > TOML > defaults", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi]
url = "http://toml-url/apirest.php"
app_token = "toml-app-token"
user_token = "toml-user-token"

[server]
timeout = 30
log_level = "info"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		// Set environment variables (lower priority than CLI)
		t.Setenv("GLPICTL_GLPI_URL", "http://env-url/apirest.php")
		t.Setenv("GLPICTL_TIMEOUT", "45")

		// CLI overrides (highest priority)
		cliOverrides := &CLIOverrides{
			GLPIURL:      "http://cli-url/apirest.php",
			GLPIAppToken: "cli-app-token",
			Timeout:      120,
		}

		cfg, err := LoadWithOverrides(configPath, cliOverrides)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// CLI should win over env and TOML
		if cfg.GLPI.URL != "http://cli-url/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q from CLI", cfg.GLPI.URL, "http://cli-url/apirest.php")
		}
		if cfg.GLPI.AppToken != "cli-app-token" {
			t.Errorf("GLPI.AppToken = %q, want %q from CLI", cfg.GLPI.AppToken, "cli-app-token")
		}
		// No CLI override for UserToken, so env takes priority over TOML
		if cfg.GLPI.UserToken != "toml-user-token" {
			t.Errorf("GLPI.UserToken = %q, want %q from TOML", cfg.GLPI.UserToken, "toml-user-token")
		}
		// CLI timeout wins over env
		if cfg.Server.Timeout != 120 {
			t.Errorf("Server.Timeout = %d, want %d from CLI", cfg.Server.Timeout, 120)
		}
	})
}

func TestCLIOverrides(t *testing.T) {
	t.Run("empty CLI overrides do not affect config", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi]
url = "http://localhost/apirest.php"
app_token = "test-app-token"
user_token = "test-user-token"

[server]
timeout = 30
log_level = "debug"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cliOverrides := &CLIOverrides{} // Empty overrides
		cfg, err := LoadWithOverrides(configPath, cliOverrides)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should use TOML values
		if cfg.GLPI.URL != "http://localhost/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://localhost/apirest.php")
		}
		if cfg.Server.Timeout != 30 {
			t.Errorf("Server.Timeout = %d, want %d", cfg.Server.Timeout, 30)
		}
		if cfg.Server.LogLevel != "debug" {
			t.Errorf("Server.LogLevel = %q, want %q", cfg.Server.LogLevel, "debug")
		}
	})

	t.Run("partial CLI overrides work correctly", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi]
url = "http://localhost/apirest.php"
app_token = "test-app-token"
user_token = "test-user-token"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cliOverrides := &CLIOverrides{
			LogLevel: "error", // Only override log level
		}
		cfg, err := LoadWithOverrides(configPath, cliOverrides)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// TOML values preserved
		if cfg.GLPI.URL != "http://localhost/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://localhost/apirest.php")
		}
		// CLI override applied
		if cfg.Server.LogLevel != "error" {
			t.Errorf("Server.LogLevel = %q, want %q", cfg.Server.LogLevel, "error")
		}
	})
}
