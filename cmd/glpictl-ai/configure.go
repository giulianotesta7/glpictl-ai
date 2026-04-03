package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// runConfigure handles the "configure" subcommand.
// Returns exit code: 0 on success, 1 on error.
func runConfigure(args []string) int {
	fs := flag.NewFlagSet("configure", flag.ExitOnError)
	url := fs.String("url", "", "GLPI API URL (e.g., http://localhost/apirest.php)")
	appToken := fs.String("app-token", "", "GLPI application token")
	userToken := fs.String("user-token", "", "GLPI user token")
	insecureSSL := fs.Bool("insecure-ssl", false, "Skip SSL certificate verification")
	timeout := fs.Int("timeout", 30, "HTTP timeout in seconds")
	fs.Parse(args)

	// Gather values with priority: flags > env vars > interactive prompts
	nonInteractive := *url != "" || *appToken != "" || *userToken != ""

	glpiURL := firstNonEmpty(*url, os.Getenv("GLPICTL_GLPI_URL"))
	glpiAppToken := firstNonEmpty(*appToken, os.Getenv("GLPICTL_GLPI_APP_TOKEN"))
	glpiUserToken := firstNonEmpty(*userToken, os.Getenv("GLPICTL_GLPI_USER_TOKEN"))

	if !nonInteractive {
		// Interactive mode: prompt for missing values
		scanner := bufio.NewScanner(os.Stdin)

		if glpiURL == "" {
			glpiURL = prompt(scanner, "GLPI URL", "http://localhost/apirest.php")
		}
		if glpiAppToken == "" {
			glpiAppToken = prompt(scanner, "App Token", "")
		}
		if glpiUserToken == "" {
			glpiUserToken = prompt(scanner, "User Token", "")
		}
		if !*insecureSSL {
			envInsecure := os.Getenv("GLPICTL_INSECURE_SSL")
			if envInsecure == "" {
				resp := prompt(scanner, "Insecure SSL (y/N)", "N")
				*insecureSSL = strings.EqualFold(resp, "y") || strings.EqualFold(resp, "yes")
			}
		}
		if *timeout == 30 {
			envTimeout := os.Getenv("GLPICTL_TIMEOUT")
			if envTimeout == "" {
				resp := prompt(scanner, "Timeout (seconds)", "30")
				if resp != "" {
					if t, err := parseInt(resp, 30); err == nil {
						*timeout = t
					}
				}
			}
		}
	}

	// Validate required fields
	if glpiURL == "" {
		fmt.Fprintln(os.Stderr, "Error: GLPI URL is required")
		return 1
	}
	if glpiAppToken == "" {
		fmt.Fprintln(os.Stderr, "Error: App Token is required")
		return 1
	}
	if glpiUserToken == "" {
		fmt.Fprintln(os.Stderr, "Error: User Token is required")
		return 1
	}

	// Build config
	cfg := &config.Config{
		GLPI: config.GLPIConfig{
			URL:         glpiURL,
			AppToken:    glpiAppToken,
			UserToken:   glpiUserToken,
			InsecureSSL: *insecureSSL,
		},
		Server: config.ServerConfig{
			Timeout:  *timeout,
			LogLevel: "info",
		},
	}

	// Test connection
	fmt.Println("Testing connection to GLPI...")
	if err := testConnection(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error: connection test failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "Please check your URL and tokens, then try again.")
		return 1
	}
	fmt.Println("Connection successful!")

	// Check if config already exists
	configPath, err := getConfigPathForConfigure("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if fileExists(configPath) {
		scanner := bufio.NewScanner(os.Stdin)
		resp := prompt(scanner, fmt.Sprintf("Config file already exists at %s. Overwrite? (y/N)", configPath), "N")
		if !strings.EqualFold(resp, "y") && !strings.EqualFold(resp, "yes") {
			fmt.Println("Configuration cancelled.")
			return 0
		}
	}

	// Save config
	if err := config.Save(cfg, ""); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to save config: %v\n", err)
		return 1
	}

	fmt.Printf("Configuration saved to %s\n", configPath)
	return 0
}

// testConnection creates a GLPI client, initializes a session, and kills it.
func testConnection(cfg *config.Config) error {
	client, err := glpi.NewClient(cfg)
	if err != nil {
		return fmt.Errorf("create client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.Timeout)*time.Second)
	defer cancel()

	if err := client.InitSession(ctx); err != nil {
		return fmt.Errorf("init session: %w", err)
	}

	// Kill session to clean up
	if err := client.KillSession(ctx); err != nil {
		// Log but don't fail — session was established, kill is best-effort
		fmt.Fprintf(os.Stderr, "Warning: failed to clean up session: %v\n", err)
	}

	return nil
}

// prompt displays a prompt and reads a line from stdin.
// If defaultVal is non-empty, it is shown in the prompt and returned on empty input.
func prompt(scanner *bufio.Scanner, label, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}

	if !scanner.Scan() {
		return defaultVal
	}
	val := strings.TrimSpace(scanner.Text())
	if val == "" {
		return defaultVal
	}
	return val
}

// firstNonEmpty returns the first non-empty string from the arguments.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseInt parses a string as an integer, returning defaultVal on failure.
func parseInt(s string, defaultVal int) (int, error) {
	var result int
	if _, err := fmt.Sscanf(s, "%d", &result); err != nil {
		return defaultVal, err
	}
	return result, nil
}

// fileExists returns true if the file exists.
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// getConfigPathForConfigure returns the config file path for the configure command.
// Duplicated from config.getConfigPath since that function is unexported.
func getConfigPathForConfigure(path string) (string, error) {
	if path != "" {
		return path, nil
	}

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
