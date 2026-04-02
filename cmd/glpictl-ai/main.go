package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
	"github.com/giulianotesta7/glpictl-ai/internal/tools"
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

	// Register glpi_get tool
	getTool := mcp.NewTool("glpi_get",
		mcp.WithDescription("Get a single GLPI item by type and ID, with optional related details"),
		mcp.WithString("itemtype", mcp.Required(), mcp.Description("GLPI item type (e.g., Computer, Printer)")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Item ID")),
		mcp.WithArray("fields", mcp.Description("Fields to return (empty = all)"), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithArray("include", mcp.Description("Read-only related details to include: software, network_ports, connected_devices, contracts, history"), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithBoolean("expand_dropdowns", mcp.Description("Expand dropdown IDs to their display names")),
	)
	s.AddTool(getTool, createGetHandler(client))

	// Register glpi_search tool
	searchTool := newSearchMCPTool()
	s.AddTool(searchTool, createSearchHandler(client))

	// Register glpi_global_search tool
	globalSearchTool := newGlobalSearchMCPTool()
	s.AddTool(globalSearchTool, createGlobalSearchHandler(client))

	// Register glpi_list_fields tool
	listFieldsTool := newListFieldsMCPTool()
	s.AddTool(listFieldsTool, createListFieldsHandler(client))

	// Register glpi_create tool
	createTool := mcp.NewTool("glpi_create",
		mcp.WithDescription("Create a new GLPI item"),
		mcp.WithString("itemtype", mcp.Required(), mcp.Description("GLPI item type (e.g., Computer, Printer)")),
		mcp.WithObject("data", mcp.Required(), mcp.Description("Item data to create")),
	)
	s.AddTool(createTool, createCreateHandler(client))

	// Register glpi_update tool
	updateTool := mcp.NewTool("glpi_update",
		mcp.WithDescription("Update an existing GLPI item"),
		mcp.WithString("itemtype", mcp.Required(), mcp.Description("GLPI item type (e.g., Computer, Printer)")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Item ID")),
		mcp.WithObject("data", mcp.Required(), mcp.Description("Fields to update")),
	)
	s.AddTool(updateTool, createUpdateHandler(client))

	// Register glpi_update_by_name tool
	updateByNameTool := mcp.NewTool("glpi_update_by_name",
		mcp.WithDescription("Update a GLPI item only when exactly one exact name match exists; returns an explicit error on zero or duplicate matches"),
		mcp.WithString("itemtype", mcp.Required(), mcp.Description("GLPI item type (e.g., Computer, Printer)")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Exact item name to match")),
		mcp.WithObject("data", mcp.Required(), mcp.Description("Fields to update")),
	)
	s.AddTool(updateByNameTool, createUpdateByNameHandler(client))

	// Register glpi_delete tool
	deleteTool := mcp.NewTool("glpi_delete",
		mcp.WithDescription("Delete a GLPI item by type and ID"),
		mcp.WithString("itemtype", mcp.Required(), mcp.Description("GLPI item type (e.g., Computer, Printer)")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Item ID")),
	)
	s.AddTool(deleteTool, createDeleteHandler(client))

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

func newSearchMCPTool() mcp.Tool {
	return mcp.NewTool("glpi_search",
		mcp.WithDescription("Search GLPI items with criteria and optional field selection"),
		mcp.WithString("itemtype", mcp.Required(), mcp.Description("GLPI item type (e.g., Computer, Printer)")),
		mcp.WithArray("criteria", mcp.Required(), mcp.Description("Search criteria array"), mcp.Items(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"field":      map[string]any{"type": "number", "description": "Field ID to search on"},
				"field_name": map[string]any{"type": "string", "description": "Field name or uid to translate to GLPI field ID"},
				"searchtype": map[string]any{"type": "string", "description": "Search type (contains, equals, etc.)"},
				"value":      map[string]any{"type": "string", "description": "Search value"},
				"link":       map[string]any{"type": "string", "description": "Link operator (AND, OR)"},
			},
		})),
		mcp.WithArray("fields", mcp.Description("Fields to return"), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithObject("range", mcp.Description("Result range (start-end)")),
	)
}

func newListFieldsMCPTool() mcp.Tool {
	return mcp.NewTool("glpi_list_fields",
		mcp.WithDescription("List searchable fields for a GLPI item type"),
		mcp.WithString("itemtype", mcp.Required(), mcp.Description("GLPI item type (e.g., Computer, Printer)")),
	)
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
			wrappedErr := fmt.Errorf("ping tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
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

// createGetHandler creates the MCP tool handler for the glpi_get command.
func createGetHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		itemType, err := request.RequireString("itemtype")
		if err != nil {
			return mcp.NewToolResultError("itemtype is required and must be a string"), nil
		}

		id, err := request.RequireInt("id")
		if err != nil {
			return mcp.NewToolResultError("id is required and must be a number"), nil
		}

		// Extract optional fields
		var fields []string
		if fieldsVal := request.GetStringSlice("fields", nil); fieldsVal != nil {
			fields = fieldsVal
		}

		// Extract optional includes
		var includes []string
		if includeVal := request.GetStringSlice("include", nil); includeVal != nil {
			includes = includeVal
		}

		// Extract expand_dropdowns
		expandDropdowns := request.GetBool("expand_dropdowns", false)

		tool, err := tools.NewGetTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create get tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, itemType, id, fields, includes, expandDropdowns)
		if err != nil {
			wrappedErr := fmt.Errorf("get item: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Item retrieved successfully.\nID: %d\nName: %s\nData: %v",
			result.ID, result.Name, result.Data)), nil
	}
}

// createSearchHandler creates the MCP tool handler for the glpi_search command.
func createSearchHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		itemType, err := request.RequireString("itemtype")
		if err != nil {
			return mcp.NewToolResultError("itemtype is required and must be a string"), nil
		}

		// Get raw arguments to extract criteria
		rawArgs := request.GetRawArguments()
		argsMap, ok := rawArgs.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments format"), nil
		}

		var criteria []tools.SearchCriterion
		if criteriaVal, ok := argsMap["criteria"].([]interface{}); ok {
			for _, c := range criteriaVal {
				if cmap, ok := c.(map[string]interface{}); ok {
					criterion := tools.SearchCriterion{}
					if f, ok := cmap["field"].(float64); ok {
						criterion.Field = int(f)
					}
					if st, ok := cmap["searchtype"].(string); ok {
						criterion.SearchType = st
					}
					if fieldName, ok := cmap["field_name"].(string); ok {
						criterion.FieldName = fieldName
					}
					if v, ok := cmap["value"].(string); ok {
						criterion.Value = v
					}
					if l, ok := cmap["link"].(string); ok {
						criterion.Link = l
					}
					criteria = append(criteria, criterion)
				}
			}
		}

		if len(criteria) == 0 {
			return mcp.NewToolResultError("criteria is required and must be a non-empty array"), nil
		}

		// Extract optional fields
		var fields []string
		if fieldsVal := request.GetStringSlice("fields", nil); fieldsVal != nil {
			fields = fieldsVal
		}

		// Extract optional range
		var searchRange *tools.SearchRange
		if rangeVal, ok := argsMap["range"].(map[string]interface{}); ok {
			searchRange = &tools.SearchRange{}
			if start, ok := rangeVal["start"].(float64); ok {
				searchRange.Start = int(start)
			}
			if end, ok := rangeVal["end"].(float64); ok {
				searchRange.End = int(end)
			}
		}

		tool, err := tools.NewSearchTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create search tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, itemType, criteria, fields, searchRange)
		if err != nil {
			wrappedErr := fmt.Errorf("search items: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Search completed.\nTotal count: %d\nResults: %d items",
			result.TotalCount, len(result.Data))), nil
	}
}

// createListFieldsHandler creates the MCP tool handler for glpi_list_fields.
func createListFieldsHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		itemType, err := request.RequireString("itemtype")
		if err != nil {
			return mcp.NewToolResultError("itemtype is required and must be a string"), nil
		}

		tool, err := tools.NewListFieldsTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create list fields tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, itemType)
		if err != nil {
			wrappedErr := fmt.Errorf("list fields: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("List fields completed.\nItemType: %s\nFields: %d\nCached: %t",
			result.ItemType, len(result.Fields), result.Cached)), nil
	}
}

// createCreateHandler creates the MCP tool handler for the glpi_create command.
func createCreateHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		itemType, err := request.RequireString("itemtype")
		if err != nil {
			return mcp.NewToolResultError("itemtype is required and must be a string"), nil
		}

		// Get raw arguments to extract data
		rawArgs := request.GetRawArguments()
		argsMap, ok := rawArgs.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments format"), nil
		}

		data, ok := argsMap["data"].(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("data is required and must be an object"), nil
		}

		tool, err := tools.NewCreateTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create create tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, itemType, data)
		if err != nil {
			wrappedErr := fmt.Errorf("create item: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Item created successfully.\nID: %d\nMessage: %s",
			result.ID, result.Message)), nil
	}
}

// createUpdateHandler creates the MCP tool handler for the glpi_update command.
func createUpdateHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		itemType, err := request.RequireString("itemtype")
		if err != nil {
			return mcp.NewToolResultError("itemtype is required and must be a string"), nil
		}

		id, err := request.RequireInt("id")
		if err != nil {
			return mcp.NewToolResultError("id is required and must be a number"), nil
		}

		// Get raw arguments to extract data
		rawArgs := request.GetRawArguments()
		argsMap, ok := rawArgs.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments format"), nil
		}

		data, ok := argsMap["data"].(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("data is required and must be an object"), nil
		}

		tool, err := tools.NewUpdateTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create update tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, itemType, id, data)
		if err != nil {
			wrappedErr := fmt.Errorf("update item: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Item updated successfully.\nID: %d\nMessage: %s",
			result.ID, result.Message)), nil
	}
}

// createDeleteHandler creates the MCP tool handler for the glpi_delete command.
func createDeleteHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		itemType, err := request.RequireString("itemtype")
		if err != nil {
			return mcp.NewToolResultError("itemtype is required and must be a string"), nil
		}

		id, err := request.RequireInt("id")
		if err != nil {
			return mcp.NewToolResultError("id is required and must be a number"), nil
		}

		tool, err := tools.NewDeleteTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create delete tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, itemType, id)
		if err != nil {
			wrappedErr := fmt.Errorf("delete item: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Item deleted successfully.\nID: %d\nMessage: %s",
			result.ID, result.Message)), nil
	}
}

// createUpdateByNameHandler creates the MCP tool handler for the glpi_update_by_name command.
func createUpdateByNameHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		itemType, err := request.RequireString("itemtype")
		if err != nil {
			return mcp.NewToolResultError("itemtype is required and must be a string"), nil
		}

		name, err := request.RequireString("name")
		if err != nil {
			return mcp.NewToolResultError("name is required and must be a string"), nil
		}

		// Extract data from raw args
		rawArgs := request.GetRawArguments()
		argsMap, ok := rawArgs.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments format"), nil
		}

		data, ok := argsMap["data"].(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("data is required and must be an object"), nil
		}

		tool, err := tools.NewUpdateByNameTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create update-by-name tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, itemType, name, data)
		if err != nil {
			// Surface typed errors with useful disambiguation data
			if ambiguousErr, ok := err.(*tools.UpdateByNameAmbiguousError); ok {
				matchIDs := make([]string, 0, len(ambiguousErr.Matches))
				for _, m := range ambiguousErr.Matches {
					matchIDs = append(matchIDs, fmt.Sprintf("%d (%s)", m.ID, m.Name))
				}
				wrappedErr := fmt.Errorf("%s; showing first %d of %d matches: %s", err, len(ambiguousErr.Matches), ambiguousErr.TotalCount, strings.Join(matchIDs, ", "))
				return mcp.NewToolResultError(wrappedErr.Error()), nil
			}
			wrappedErr := fmt.Errorf("update by name: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("Item updated successfully by name.\nID: %d\nName: %s\nMessage: %s",
			result.ID, result.Name, result.Message)), nil
	}
}
