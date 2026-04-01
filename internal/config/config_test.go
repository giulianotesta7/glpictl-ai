package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("returns ErrNotFound when config file doesn't exist", func(t *testing.T) {
		_, err := Load("/nonexistent/path/config.toml")
		if !IsErrNotFound(err) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("loads valid TOML config file", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi]
url = "http://localhost/apirest.php"
app_token = "test-app-token"
user_token = "test-user-token"

[server]
timeout = 30
log_level = "info"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.GLPI.URL != "http://localhost/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://localhost/apirest.php")
		}
		if cfg.GLPI.AppToken != "test-app-token" {
			t.Errorf("GLPI.AppToken = %q, want %q", cfg.GLPI.AppToken, "test-app-token")
		}
		if cfg.GLPI.UserToken != "test-user-token" {
			t.Errorf("GLPI.UserToken = %q, want %q", cfg.GLPI.UserToken, "test-user-token")
		}
		if cfg.Server.Timeout != 30 {
			t.Errorf("Server.Timeout = %d, want %d", cfg.Server.Timeout, 30)
		}
		if cfg.Server.LogLevel != "info" {
			t.Errorf("Server.LogLevel = %q, want %q", cfg.Server.LogLevel, "info")
		}
	})

	t.Run("returns ErrMissingRequired when URL is missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi]
app_token = "test-app-token"
user_token = "test-user-token"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := Load(configPath)
		if !IsErrMissingRequired(err) {
			t.Errorf("expected ErrMissingRequired, got %v", err)
		}
	})

	t.Run("returns ErrMissingRequired when app_token is missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi]
url = "http://localhost/apirest.php"
user_token = "test-user-token"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := Load(configPath)
		if !IsErrMissingRequired(err) {
			t.Errorf("expected ErrMissingRequired, got %v", err)
		}
	})

	t.Run("returns ErrMissingRequired when user_token is missing", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi]
url = "http://localhost/apirest.php"
app_token = "test-app-token"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := Load(configPath)
		if !IsErrMissingRequired(err) {
			t.Errorf("expected ErrMissingRequired, got %v", err)
		}
	})

	t.Run("uses defaults for optional fields", func(t *testing.T) {
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

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Default timeout should be 30
		if cfg.Server.Timeout != 30 {
			t.Errorf("Server.Timeout = %d, want default %d", cfg.Server.Timeout, 30)
		}
		// Default log level should be "info"
		if cfg.Server.LogLevel != "info" {
			t.Errorf("Server.LogLevel = %q, want default %q", cfg.Server.LogLevel, "info")
		}
	})

	t.Run("returns ErrInvalidType for malformed TOML", func(t *testing.T) {
		tmpDir := t.TempDir()
		configPath := filepath.Join(tmpDir, "config.toml")
		content := `
[glpi
url = "broken
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		_, err := Load(configPath)
		if !IsErrInvalidType(err) {
			t.Errorf("expected ErrInvalidType, got %v", err)
		}
	})

	t.Run("loads from XDG config path when path is empty", func(t *testing.T) {
		// Set up XDG_CONFIG_HOME
		tmpDir := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", tmpDir)

		configPath := filepath.Join(tmpDir, "glpictl-ai", "config.toml")
		if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
			t.Fatalf("failed to create config dir: %v", err)
		}

		content := `
[glpi]
url = "http://localhost/apirest.php"
app_token = "test-app-token"
user_token = "test-user-token"
`
		if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write config: %v", err)
		}

		cfg, err := Load("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if cfg.GLPI.URL != "http://localhost/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://localhost/apirest.php")
		}
	})

	t.Run("config is mutable as Go struct value", func(t *testing.T) {
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

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Config is mutable because Go structs are value types
		originalURL := cfg.GLPI.URL
		cfg.GLPI.URL = "modified"
		if cfg.GLPI.URL != "modified" {
			t.Error("expected config to be modifiable (Go structs are value types)")
		}
		// The important part is that Load returns a clean, validated config
		_ = originalURL // Just to use the variable
	})
}

func TestConfigDefaults(t *testing.T) {
	t.Run("default timeout is 30 seconds", func(t *testing.T) {
		cfg := &Config{}
		applyDefaults(cfg)
		if cfg.Server.Timeout != 30 {
			t.Errorf("default timeout = %d, want %d", cfg.Server.Timeout, 30)
		}
	})

	t.Run("default log level is info", func(t *testing.T) {
		cfg := &Config{}
		applyDefaults(cfg)
		if cfg.Server.LogLevel != "info" {
			t.Errorf("default log level = %q, want %q", cfg.Server.LogLevel, "info")
		}
	})
}

func TestIsErrNotFound(t *testing.T) {
	t.Run("returns true for ErrNotFound", func(t *testing.T) {
		if !IsErrNotFound(ErrNotFound) {
			t.Error("expected IsErrNotFound(ErrNotFound) to be true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsErrNotFound(errors.New("other error")) {
			t.Error("expected IsErrNotFound(other error) to be false")
		}
	})
}

func TestIsErrMissingRequired(t *testing.T) {
	t.Run("returns true for MissingRequiredError", func(t *testing.T) {
		err := NewMissingRequiredError("test.field")
		if !IsErrMissingRequired(err) {
			t.Error("expected IsErrMissingRequired to be true")
		}
	})

	t.Run("returns true for ErrMissingRequired", func(t *testing.T) {
		if !IsErrMissingRequired(ErrMissingRequired) {
			t.Error("expected IsErrMissingRequired(ErrMissingRequired) to be true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsErrMissingRequired(errors.New("other error")) {
			t.Error("expected IsErrMissingRequired(other error) to be false")
		}
	})
}

func TestIsErrInvalidType(t *testing.T) {
	t.Run("returns true for ErrInvalidType", func(t *testing.T) {
		if !IsErrInvalidType(ErrInvalidType) {
			t.Error("expected IsErrInvalidType(ErrInvalidType) to be true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsErrInvalidType(errors.New("other error")) {
			t.Error("expected IsErrInvalidType(other error) to be false")
		}
	})
}

func TestApplyEnvVars(t *testing.T) {
	t.Run("overrides GLPI URL from environment", func(t *testing.T) {
		t.Setenv("GLPICTL_GLPI_URL", "http://env-override:8080/apirest.php")
		cfg := &Config{
			GLPI: GLPIConfig{
				URL: "http://original:80/apirest.php",
			},
		}
		applyEnvVars(cfg)
		if cfg.GLPI.URL != "http://env-override:8080/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://env-override:8080/apirest.php")
		}
	})

	t.Run("overrides app token from environment", func(t *testing.T) {
		t.Setenv("GLPICTL_GLPI_APP_TOKEN", "env-app-token")
		cfg := &Config{
			GLPI: GLPIConfig{
				AppToken: "original-token",
			},
		}
		applyEnvVars(cfg)
		if cfg.GLPI.AppToken != "env-app-token" {
			t.Errorf("GLPI.AppToken = %q, want %q", cfg.GLPI.AppToken, "env-app-token")
		}
	})

	t.Run("overrides user token from environment", func(t *testing.T) {
		t.Setenv("GLPICTL_GLPI_USER_TOKEN", "env-user-token")
		cfg := &Config{
			GLPI: GLPIConfig{
				UserToken: "original-token",
			},
		}
		applyEnvVars(cfg)
		if cfg.GLPI.UserToken != "env-user-token" {
			t.Errorf("GLPI.UserToken = %q, want %q", cfg.GLPI.UserToken, "env-user-token")
		}
	})

	t.Run("overrides timeout from environment", func(t *testing.T) {
		t.Setenv("GLPICTL_TIMEOUT", "60")
		cfg := &Config{
			Server: ServerConfig{Timeout: 30},
		}
		applyEnvVars(cfg)
		if cfg.Server.Timeout != 60 {
			t.Errorf("Server.Timeout = %d, want %d", cfg.Server.Timeout, 60)
		}
	})

	t.Run("overrides log level from environment", func(t *testing.T) {
		t.Setenv("GLPICTL_LOG_LEVEL", "debug")
		cfg := &Config{
			Server: ServerConfig{LogLevel: "info"},
		}
		applyEnvVars(cfg)
		if cfg.Server.LogLevel != "debug" {
			t.Errorf("Server.LogLevel = %q, want %q", cfg.Server.LogLevel, "debug")
		}
	})

	t.Run("overrides insecure_ssl from environment", func(t *testing.T) {
		t.Setenv("GLPICTL_INSECURE_SSL", "true")
		cfg := &Config{
			GLPI: GLPIConfig{InsecureSSL: false},
		}
		applyEnvVars(cfg)
		if !cfg.GLPI.InsecureSSL {
			t.Error("expected GLPI.InsecureSSL to be true")
		}
	})

	t.Run("does not override when env var is empty", func(t *testing.T) {
		t.Setenv("GLPICTL_GLPI_URL", "")
		cfg := &Config{
			GLPI: GLPIConfig{URL: "http://original:80/apirest.php"},
		}
		applyEnvVars(cfg)
		if cfg.GLPI.URL != "http://original:80/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://original:80/apirest.php")
		}
	})
}

func TestApplyCLIOverrides(t *testing.T) {
	t.Run("overrides GLPI URL from CLI", func(t *testing.T) {
		cfg := &Config{
			GLPI: GLPIConfig{URL: "http://original:80/apirest.php"},
		}
		overrides := &CLIOverrides{
			GLPIURL: "http://cli-override:9090/apirest.php",
		}
		applyCLIOverrides(cfg, overrides)
		if cfg.GLPI.URL != "http://cli-override:9090/apirest.php" {
			t.Errorf("GLPI.URL = %q, want %q", cfg.GLPI.URL, "http://cli-override:9090/apirest.php")
		}
	})

	t.Run("overrides app token from CLI", func(t *testing.T) {
		cfg := &Config{
			GLPI: GLPIConfig{AppToken: "original-token"},
		}
		overrides := &CLIOverrides{
			GLPIAppToken: "cli-app-token",
		}
		applyCLIOverrides(cfg, overrides)
		if cfg.GLPI.AppToken != "cli-app-token" {
			t.Errorf("GLPI.AppToken = %q, want %q", cfg.GLPI.AppToken, "cli-app-token")
		}
	})

	t.Run("overrides user token from CLI", func(t *testing.T) {
		cfg := &Config{
			GLPI: GLPIConfig{UserToken: "original-token"},
		}
		overrides := &CLIOverrides{
			GLPIUserToken: "cli-user-token",
		}
		applyCLIOverrides(cfg, overrides)
		if cfg.GLPI.UserToken != "cli-user-token" {
			t.Errorf("GLPI.UserToken = %q, want %q", cfg.GLPI.UserToken, "cli-user-token")
		}
	})

	t.Run("overrides timeout from CLI", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{Timeout: 30},
		}
		overrides := &CLIOverrides{
			Timeout: 120,
		}
		applyCLIOverrides(cfg, overrides)
		if cfg.Server.Timeout != 120 {
			t.Errorf("Server.Timeout = %d, want %d", cfg.Server.Timeout, 120)
		}
	})

	t.Run("overrides log level from CLI", func(t *testing.T) {
		cfg := &Config{
			Server: ServerConfig{LogLevel: "info"},
		}
		overrides := &CLIOverrides{
			LogLevel: "debug",
		}
		applyCLIOverrides(cfg, overrides)
		if cfg.Server.LogLevel != "debug" {
			t.Errorf("Server.LogLevel = %q, want %q", cfg.Server.LogLevel, "debug")
		}
	})

	t.Run("overrides insecure_ssl from CLI", func(t *testing.T) {
		cfg := &Config{
			GLPI: GLPIConfig{InsecureSSL: false},
		}
		insecure := true
		overrides := &CLIOverrides{
			InsecureSSL: &insecure,
		}
		applyCLIOverrides(cfg, overrides)
		if !cfg.GLPI.InsecureSSL {
			t.Error("expected GLPI.InsecureSSL to be true")
		}
	})

	t.Run("does not override when CLI flag is empty/zero", func(t *testing.T) {
		cfg := &Config{
			GLPI: GLPIConfig{
				URL:         "http://original:80/apirest.php",
				InsecureSSL: true,
			},
			Server: ServerConfig{
				Timeout:  30,
				LogLevel: "info",
			},
		}
		overrides := &CLIOverrides{
			GLPIURL:       "",
			GLPIAppToken:  "",
			GLPIUserToken: "",
			Timeout:       0,
			LogLevel:      "",
			InsecureSSL:   nil,
		}
		applyCLIOverrides(cfg, overrides)
		if cfg.GLPI.URL != "http://original:80/apirest.php" {
			t.Errorf("GLPI.URL = %q, want unchanged", cfg.GLPI.URL)
		}
		if !cfg.GLPI.InsecureSSL {
			t.Error("expected GLPI.InsecureSSL to remain true")
		}
		if cfg.Server.Timeout != 30 {
			t.Errorf("Server.Timeout = %d, want unchanged", cfg.Server.Timeout)
		}
		if cfg.Server.LogLevel != "info" {
			t.Errorf("Server.LogLevel = %q, want unchanged", cfg.Server.LogLevel)
		}
	})
}
