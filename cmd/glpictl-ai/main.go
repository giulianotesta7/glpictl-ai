package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Exit codes
const (
	ExitOK     = 0
	ExitError  = 1
	ExitConfig = 2
)

// Version is set at build time
var version = "dev"

func main() {
	// Parse flags
	configPath := flag.String("config", "", "Path to config file (default: ~/.config/glpictl-ai/config.toml)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("glpictl-ai %s\n", version)
		os.Exit(ExitOK)
	}

	// Load configuration
	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(ExitConfig)
	}

	// Set up slog based on log level
	setupLogger(cfg.Server.LogLevel)

	// Create GLPI client
	client, err := glpi.NewClient(cfg)
	if err != nil {
		slog.Error("Error creating GLPI client", "error", err)
		os.Exit(ExitError)
	}

	// Create MCP server
	s := server.NewMCPServer(
		"glpictl-ai",
		version,
		server.WithToolCapabilities(false),
	)

	// Register ping tool with MCP server
	pingMCPTool := mcp.NewTool("ping",
		mcp.WithDescription("Test GLPI connection and return session status"),
	)
	s.AddTool(pingMCPTool, createPingHandler(client))

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		<-sigChan
		slog.Info("Shutting down...")
		// Clean up GLPI session with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := client.KillSession(ctx); err != nil {
			slog.Error("Error killing GLPI session", "error", err)
		}
		os.Exit(ExitOK)
	}()

	// Serve MCP over stdio
	slog.Info("Starting glpictl-ai MCP server...")
	if err := server.ServeStdio(s); err != nil {
		slog.Error("Server error", "error", err)
		os.Exit(ExitError)
	}
}

// setupLogger configures slog based on the log level.
func setupLogger(level string) {
	var lvl slog.Level

	switch level {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	// MCP uses stdio, so log to stderr
	logger := slog.New(slog.NewTextHandler(os.Stderr, opts))
	slog.SetDefault(logger)
}

// loadConfig loads the configuration from the specified path or default location.
func loadConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		if config.IsErrNotFound(err) {
			fmt.Fprintln(os.Stderr, "Config file not found. Create one at ~/.config/glpictl-ai/config.toml:")
			fmt.Fprintln(os.Stderr, "[glpi]")
			fmt.Fprintln(os.Stderr, "url = \"http://localhost/apirest.php\"")
			fmt.Fprintln(os.Stderr, "app_token = \"your-app-token\"")
			fmt.Fprintln(os.Stderr, "user_token = \"your-user-token\"")
			return nil, fmt.Errorf("config file not found: %w", err)
		}
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return cfg, nil
}

// createPingHandler creates the MCP tool handler for the ping command.
func createPingHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		logger := slog.Default()

		if logger.Enabled(ctx, slog.LevelDebug) {
			slog.Debug("Executing ping tool", "url", client.GLPIURL())
		}

		err := client.InitSession(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("GLPI connection failed: %v", err)), nil
		}

		sessionToken := client.SessionToken()
		if sessionToken == "" {
			return mcp.NewToolResultError("GLPI returned empty session token"), nil
		}

		// Get GLPI version
		version, err := client.GetGLPIVersion(ctx)
		if err != nil {
			slog.Warn("Failed to get GLPI version", "error", err)
		}

		// Redact session token for security (show first4 chars only)
		redactedToken := ""
		if len(sessionToken) >= 4 {
			redactedToken = sessionToken[:4] + "..."
		} else {
			redactedToken = "***"
		}

		result := fmt.Sprintf("GLPI connection successful.\nSession token: %s\nGLPI URL: %s\nGLPI Version: %s",
			redactedToken, client.GLPIURL(), version)

		return mcp.NewToolResultText(result), nil
	}
}
