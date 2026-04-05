package main

import (
	"bufio"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

//go:embed skills/*
var skillsFS embed.FS

// mcpClient represents an MCP client that can be configured.
type mcpClient struct {
	Name        string
	Description string
	ConfigPath  func() (string, error)
	Write       func(path string, glpiURL, appToken, userToken string) error
	SkillsDir   func() (string, error)
}

// runInstall handles the "install" subcommand.
// Returns exit code: 0 on success, 1 on error.
func runInstall(args []string) int {
	_ = args // reserved for future flags

	// Load existing GLPI config
	cfg, err := config.Load("")
	if err != nil {
		if config.IsErrNotFound(err) {
			fmt.Fprintln(os.Stderr, "Error: GLPI configuration not found.")
			fmt.Fprintln(os.Stderr, "Run 'glpictl-ai config' first to set up your GLPI connection.")
			return 1
		}
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		return 1
	}

	// Test connection
	fmt.Println("Testing connection to GLPI...")
	client, err := glpi.NewClient(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create GLPI client: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.Server.Timeout)*time.Second)
	defer cancel()
	if err := client.InitSession(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: connection test failed: %v\n", err)
		fmt.Fprintln(os.Stderr, "Please check your URL and tokens, then try again.")
		return 1
	}
	client.KillSession(ctx)
	fmt.Println("Connection successful!")

	scanner := bufio.NewScanner(os.Stdin)

	clients := []mcpClient{
		{
			Name:        "opencode",
			Description: "OpenCode (global config)",
			ConfigPath:  opencodeConfigPath,
			Write:       writeOpenCodeConfig,
			SkillsDir:   opencodeSkillsDir,
		},
		{
			Name:        "claude-code",
			Description: "Claude Code (global settings)",
			ConfigPath:  claudeCodeConfigPath,
			Write:       writeClaudeCodeConfig,
			SkillsDir:   claudeCodeSkillsDir,
		},
		{
			Name:        "claude-desktop",
			Description: "Claude Desktop",
			ConfigPath:  claudeDesktopConfigPath,
			Write:       writeClaudeDesktopConfig,
			SkillsDir:   claudeDesktopSkillsDir,
		},
	}

	fmt.Println("=========================================")
	fmt.Println("  glpictl-ai — MCP Client Setup")
	fmt.Println("=========================================")
	fmt.Println()
	fmt.Println("Select which MCP clients to configure with your GLPI connection.")
	fmt.Println()

	// Display menu
	for i, c := range clients {
		fmt.Printf("  [%d] %s — %s\n", i+1, c.Name, c.Description)
	}
	fmt.Println()
	fmt.Println("Enter numbers separated by commas (e.g. 1,2,3) or 'a' for all:")

	input, err := readLine(scanner)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading input: %v\n", err)
		return 1
	}

	input = strings.TrimSpace(input)
	if input == "" {
		fmt.Println("No clients selected. Exiting.")
		return 0
	}

	// Parse selection
	var selected []mcpClient
	if strings.EqualFold(input, "a") || strings.EqualFold(input, "all") {
		selected = clients
	} else {
		parts := strings.Split(input, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			// Support ranges like "1-3"
			if strings.Contains(p, "-") {
				rangeParts := strings.SplitN(p, "-", 2)
				if len(rangeParts) == 2 {
					start := parseIntOrZero(strings.TrimSpace(rangeParts[0]))
					end := parseIntOrZero(strings.TrimSpace(rangeParts[1]))
					for j := start; j <= end && j <= len(clients); j++ {
						if j >= 1 {
							selected = append(selected, clients[j-1])
						}
					}
				}
			} else {
				n := parseIntOrZero(p)
				if n >= 1 && n <= len(clients) {
					selected = append(selected, clients[n-1])
				}
			}
		}
	}

	if len(selected) == 0 {
		fmt.Println("No valid clients selected. Exiting.")
		return 0
	}

	fmt.Println()
	successCount := 0
	for _, c := range selected {
		fmt.Printf("Configuring %s... ", c.Name)
		path, err := c.ConfigPath()
		if err != nil {
			fmt.Printf("SKIP (%v)\n", err)
			continue
		}

		if err := c.Write(path, cfg.GLPI.URL, cfg.GLPI.AppToken, cfg.GLPI.UserToken); err != nil {
			fmt.Printf("ERROR (%v)\n", err)
			continue
		}

		fmt.Printf("OK (%s)\n", path)
		successCount++

		// Install skills
		if c.SkillsDir != nil {
			skillsDir, err := c.SkillsDir()
			if err != nil {
				fmt.Printf("  Skills: SKIP (%v)\n", err)
				continue
			}
			if err := installSkills(skillsDir); err != nil {
				fmt.Printf("  Skills: ERROR (%v)\n", err)
			} else {
				fmt.Printf("  Skills: installed to %s\n", skillsDir)
			}
		}
	}

	fmt.Println()
	if successCount == len(selected) {
		fmt.Printf("All %d client(s) configured successfully!\n", successCount)
	} else {
		fmt.Printf("%d of %d client(s) configured. Check errors above.\n", successCount, len(selected))
	}
	return 0
}

// readLine reads a line from the scanner.
func readLine(scanner *bufio.Scanner) (string, error) {
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return "", err
		}
		return "", nil
	}
	return scanner.Text(), nil
}

// parseIntOrZero parses an integer, returns 0 on failure.
func parseIntOrZero(s string) int {
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// ---------------------------------------------------------------------------
// OpenCode
// ---------------------------------------------------------------------------

func opencodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "opencode")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create config directory: %w", err)
	}
	return filepath.Join(dir, "opencode.json"), nil
}

func writeOpenCodeConfig(path, glpiURL, appToken, userToken string) error {
	data := map[string]interface{}{
		"mcp": map[string]interface{}{
			"glpictl-ai": map[string]interface{}{
				"type":    "local",
				"command": []string{"glpictl-ai"},
				"environment": map[string]string{
					"GLPICTL_GLPI_URL":        glpiURL,
					"GLPICTL_GLPI_APP_TOKEN":  appToken,
					"GLPICTL_GLPI_USER_TOKEN": userToken,
				},
				"enabled": true,
			},
		},
	}

	if err := mergeJSONConfig(path, data); err != nil {
		return err
	}

	// Verify glpictl-ai is in PATH
	if _, err := exec.LookPath("glpictl-ai"); err != nil {
		// Not in PATH — use absolute path
		exe, exeErr := os.Executable()
		if exeErr == nil {
			cfg := data["mcp"].(map[string]interface{})
			server := cfg["glpictl-ai"].(map[string]interface{})
			server["command"] = []string{exe}
			// Re-write with absolute path
			return writeJSON(path, data)
		}
	}
	return writeJSON(path, data)
}

// ---------------------------------------------------------------------------
// Claude Code
// ---------------------------------------------------------------------------

func claudeCodeConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	// Claude Code uses ~/.claude/settings.json
	dir := filepath.Join(home, ".claude")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create config directory: %w", err)
	}
	return filepath.Join(dir, "settings.json"), nil
}

func writeClaudeCodeConfig(path, glpiURL, appToken, userToken string) error {
	// Claude Code settings.json uses "mcpServers" key
	data := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"glpictl-ai": map[string]interface{}{
				"command": "glpictl-ai",
				"env": map[string]string{
					"GLPICTL_GLPI_URL":        glpiURL,
					"GLPICTL_GLPI_APP_TOKEN":  appToken,
					"GLPICTL_GLPI_USER_TOKEN": userToken,
				},
			},
		},
	}

	// Try to use absolute path if glpictl-ai is not in PATH
	if _, err := exec.LookPath("glpictl-ai"); err != nil {
		exe, exeErr := os.Executable()
		if exeErr == nil {
			servers := data["mcpServers"].(map[string]interface{})
			server := servers["glpictl-ai"].(map[string]interface{})
			server["command"] = exe
		}
	}

	return mergeJSONConfig(path, data)
}

// ---------------------------------------------------------------------------
// Claude Desktop
// ---------------------------------------------------------------------------

func claudeDesktopConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}

	var dir string
	if runtime.GOOS == "windows" {
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		dir = filepath.Join(appData, "Claude")
	} else {
		dir = filepath.Join(home, ".config", "claude")
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create config directory: %w", err)
	}
	return filepath.Join(dir, "claude_desktop_config.json"), nil
}

func writeClaudeDesktopConfig(path, glpiURL, appToken, userToken string) error {
	data := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"glpictl-ai": map[string]interface{}{
				"command": "glpictl-ai",
				"env": map[string]string{
					"GLPICTL_GLPI_URL":        glpiURL,
					"GLPICTL_GLPI_APP_TOKEN":  appToken,
					"GLPICTL_GLPI_USER_TOKEN": userToken,
				},
			},
		},
	}

	// Try to use absolute path if glpictl-ai is not in PATH
	if _, err := exec.LookPath("glpictl-ai"); err != nil {
		exe, exeErr := os.Executable()
		if exeErr == nil {
			servers := data["mcpServers"].(map[string]interface{})
			server := servers["glpictl-ai"].(map[string]interface{})
			server["command"] = exe
		}
	}

	return mergeJSONConfig(path, data)
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

// mergeJSONConfig reads existing config, merges new data, and writes back.
// It performs a shallow merge at the top level — existing keys are preserved,
// and the "mcp" or "mcpServers" key is merged with the glpictl-ai server.
func mergeJSONConfig(path string, newData map[string]interface{}) error {
	existing := make(map[string]interface{})

	if data, err := os.ReadFile(path); err == nil && len(data) > 0 {
		if err := json.Unmarshal(data, &existing); err != nil {
			// Corrupted file — back it up and start fresh
			backup := path + ".bak"
			os.Rename(path, backup) // ignore error
		}
	}

	// Deep merge: for each key in newData, merge into existing
	for key, newVal := range newData {
		if existingVal, ok := existing[key]; ok {
			// Both are maps — merge them
			if existingMap, ok1 := existingVal.(map[string]interface{}); ok1 {
				if newMap, ok2 := newVal.(map[string]interface{}); ok2 {
					for k, v := range newMap {
						existingMap[k] = v
					}
					existing[key] = existingMap
					continue
				}
			}
		}
		existing[key] = newVal
	}

	return writeJSON(path, existing)
}

// writeJSON marshals data to a JSON file with pretty printing.
func writeJSON(path string, data map[string]interface{}) error {
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	out = append(out, '\n')

	if err := os.WriteFile(path, out, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Skills installation
// ---------------------------------------------------------------------------

func opencodeSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".config", "opencode", "skills", "glpictl-ai")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create skills directory: %w", err)
	}
	return dir, nil
}

func claudeCodeSkillsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	dir := filepath.Join(home, ".claude", "skills", "glpictl-ai")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("cannot create skills directory: %w", err)
	}
	return dir, nil
}

func claudeDesktopSkillsDir() (string, error) {
	// Claude Desktop doesn't have a standard skills directory.
	// Skills are not directly supported by Claude Desktop.
	return "", nil
}

// installSkills copies embedded SKILL.md files to the target directory.
func installSkills(targetDir string) error {
	entries, err := skillsFS.ReadDir("skills")
	if err != nil {
		return fmt.Errorf("failed to read embedded skills: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillName := entry.Name()
		destDir := filepath.Join(targetDir, skillName)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("cannot create skill directory %s: %w", destDir, err)
		}

		skillPath := filepath.Join("skills", skillName, "SKILL.md")
		content, err := skillsFS.ReadFile(skillPath)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", skillPath, err)
		}

		destFile := filepath.Join(destDir, "SKILL.md")
		if err := os.WriteFile(destFile, content, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destFile, err)
		}
	}

	return nil
}
