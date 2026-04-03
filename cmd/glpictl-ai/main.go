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
	// Check for subcommands first (before flag parsing)
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "configure":
			os.Exit(runConfigure(os.Args[2:]))
		case "version":
			printVersion()
			os.Exit(ExitOK)
		}
	}

	// Parse flags
	configPath := flag.String("config", "", "Path to config file (default: ~/.config/glpictl-ai/config.toml)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		printVersion()
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
		mcp.WithArray("include", mcp.Description("Read-only related details to include: software, network_ports, connected_devices, contracts, history, licenses, software_versions"), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithBoolean("expand_dropdowns", mcp.Description("Expand dropdown IDs to their display names")),
	)
	s.AddTool(getTool, createGetHandler(client))

	// Register glpi_search tool
	searchTool := newSearchMCPTool()
	s.AddTool(searchTool, createSearchHandler(client))

	// Register glpi_global_search tool
	globalSearchTool := newGlobalSearchMCPTool()
	s.AddTool(globalSearchTool, createGlobalSearchHandler(client))

	// Register glpi_summary tool
	summaryTool := mcp.NewTool("glpi_summary",
		mcp.WithDescription("Return a summary dashboard with item counts by inventory type"),
		mcp.WithArray("itemtypes", mcp.Description("Itemtypes to count (default: all inventory types)"), mcp.Items(map[string]any{"type": "string"})),
	)
	s.AddTool(summaryTool, createSummaryHandler(client))

	// Register glpi_license_compliance tool
	licenseComplianceTool := mcp.NewTool("glpi_license_compliance",
		mcp.WithDescription("Return a software license compliance report comparing purchased licenses vs actual installations"),
		mcp.WithNumber("software_id", mcp.Required(), mcp.Description("GLPI ID of the Software item to check compliance for")),
		mcp.WithNumber("entity_id", mcp.Description("Restrict compliance check to this entity scope")),
	)
	s.AddTool(licenseComplianceTool, createLicenseComplianceHandler(client))

	// Register glpi_list_fields tool
	listFieldsTool := newListFieldsMCPTool()
	s.AddTool(listFieldsTool, createListFieldsHandler(client))

	// Register glpi_user_assets tool
	userAssetsTool := mcp.NewTool("glpi_user_assets",
		mcp.WithDescription("Get all assets assigned to a specific user"),
		mcp.WithNumber("user_id", mcp.Required(), mcp.Description("User ID")),
		mcp.WithArray("itemtypes", mcp.Description("Itemtypes to search (default: all inventory types)"), mcp.Items(map[string]any{"type": "string"})),
	)
	s.AddTool(userAssetsTool, createUserAssetsHandler(client))

	// Register glpi_group_assets tool
	groupAssetsTool := mcp.NewTool("glpi_group_assets",
		mcp.WithDescription("Get all assets assigned to a specific group"),
		mcp.WithNumber("group_id", mcp.Required(), mcp.Description("Group ID")),
		mcp.WithArray("itemtypes", mcp.Description("Itemtypes to search (default: all inventory types)"), mcp.Items(map[string]any{"type": "string"})),
	)
	s.AddTool(groupAssetsTool, createGroupAssetsHandler(client))

	// Register glpi_rack_capacity tool
	rackCapacityTool := mcp.NewTool("glpi_rack_capacity",
		mcp.WithDescription("Return rack capacity and utilization report for DCIM management, with equipment positions and optional unplaced equipment listing"),
		mcp.WithNumber("rack_id", mcp.Description("Check a specific rack by ID (default: all racks)")),
		mcp.WithBoolean("include_unplaced", mcp.Description("Include equipment not assigned to any rack (default: false)")),
	)
	s.AddTool(rackCapacityTool, createRackCapacityHandler(client))

	// Register glpi_expiration_tracker tool
	expirationTrackerTool := mcp.NewTool("glpi_expiration_tracker",
		mcp.WithDescription("Check expiration dates across multiple GLPI itemtypes (certificates, domains, contracts, software licenses, hardware warranties) and return a consolidated report"),
		mcp.WithNumber("days_ahead", mcp.Required(), mcp.Description("Number of days ahead to look for expiring items (must be > 0)")),
		mcp.WithArray("itemtypes", mcp.Description("Itemtypes to check (default: all expiration-bearing types: Certificate, Domain, Contract, SoftwareLicense, Computer, Monitor, Printer, NetworkEquipment, Peripheral, Phone)"), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithNumber("entity_id", mcp.Description("Restrict to this entity scope")),
	)
	s.AddTool(expirationTrackerTool, createExpirationTrackerHandler(client))

	// Register glpi_cost_summary tool
	costSummaryTool := mcp.NewTool("glpi_cost_summary",
		mcp.WithDescription("Return a cost summary with total purchase value by asset type, contract costs, and budget allocations"),
		mcp.WithNumber("entity_id", mcp.Description("Restrict to this entity scope")),
		mcp.WithBoolean("include_contracts", mcp.Description("Include contract costs (default: true)")),
		mcp.WithBoolean("include_budgets", mcp.Description("Include budget allocations (default: true)")),
	)
	s.AddTool(costSummaryTool, createCostSummaryHandler(client))

	// Register glpi_warranty_report tool
	warrantyReportTool := mcp.NewTool("glpi_warranty_report",
		mcp.WithDescription("Generate a warranty status report for hardware assets with active, expired, and expiring-soon categorization, including purchase cost aggregation"),
		mcp.WithNumber("days_warning", mcp.Description("Number of days ahead to flag items as expiring soon (default: 90)")),
		mcp.WithArray("itemtypes", mcp.Description("Hardware itemtypes to check (default: Computer, Monitor, Printer, NetworkEquipment, Peripheral, Phone)"), mcp.Items(map[string]any{"type": "string"})),
		mcp.WithNumber("entity_id", mcp.Description("Restrict to this entity scope")),
	)
	s.AddTool(warrantyReportTool, createWarrantyReportHandler(client))

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

	// Register glpi_bulk_update tool
	bulkUpdateTool := mcp.NewTool("glpi_bulk_update",
		mcp.WithDescription("Update multiple GLPI items at once; specify each item by name or ID and receive per-item results"),
		mcp.WithArray("items", mcp.Required(), mcp.Description("Items to update; each must have itemtype, data, and either id or name"), mcp.Items(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"itemtype": map[string]any{"type": "string", "description": "GLPI item type"},
				"id":       map[string]any{"type": "number", "description": "Item ID (use id or name)"},
				"name":     map[string]any{"type": "string", "description": "Exact item name (use id or name)"},
				"data":     map[string]any{"type": "object", "description": "Fields to update"},
			},
		})),
	)
	s.AddTool(bulkUpdateTool, createBulkUpdateHandler(client))

	// Register glpi_delete tool
	deleteTool := mcp.NewTool("glpi_delete",
		mcp.WithDescription("Delete a GLPI item by type and ID"),
		mcp.WithString("itemtype", mcp.Required(), mcp.Description("GLPI item type (e.g., Computer, Printer)")),
		mcp.WithNumber("id", mcp.Required(), mcp.Description("Item ID")),
	)
	s.AddTool(deleteTool, createDeleteHandler(client))

	// Register glpi_network_topology tool
	networkTopologyTool := mcp.NewTool("glpi_network_topology",
		mcp.WithDescription("Trace network port connections and cable topology in GLPI inventory, with optional VLAN info"),
		mcp.WithNumber("port_id", mcp.Description("Trace a specific port by ID (use port_id OR device_id+device_type)")),
		mcp.WithNumber("device_id", mcp.Description("Show all ports for a device (use with device_type)")),
		mcp.WithString("device_type", mcp.Description("GLPI itemtype of the device (e.g., Computer, NetworkEquipment)")),
		mcp.WithBoolean("show_vlans", mcp.Description("Include VLAN assignments on ports (default: false)")),
	)
	s.AddTool(networkTopologyTool, createNetworkTopologyHandler(client))

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

// newSearchMCPTool returns the MCP tool definition for glpi_search.
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

// newListFieldsMCPTool returns the MCP tool definition for glpi_list_fields.
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

// createBulkUpdateHandler creates the MCP tool handler for the glpi_bulk_update command.
func createBulkUpdateHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rawArgs := request.GetRawArguments()
		argsMap, ok := rawArgs.(map[string]interface{})
		if !ok {
			return mcp.NewToolResultError("invalid arguments format"), nil
		}

		itemsVal, ok := argsMap["items"].([]interface{})
		if !ok || len(itemsVal) == 0 {
			return mcp.NewToolResultError("items is required and must be a non-empty array"), nil
		}

		var items []tools.BulkUpdateItem
		for _, rawItem := range itemsVal {
			itemMap, ok := rawItem.(map[string]interface{})
			if !ok {
				return mcp.NewToolResultError("each item must be an object"), nil
			}

			item := tools.BulkUpdateItem{}
			if it, ok := itemMap["itemtype"].(string); ok {
				item.ItemType = it
			}
			if id, ok := itemMap["id"].(float64); ok {
				item.ID = int(id)
			}
			if name, ok := itemMap["name"].(string); ok {
				item.Name = name
			}
			if data, ok := itemMap["data"].(map[string]interface{}); ok {
				item.Data = data
			}
			items = append(items, item)
		}

		tool, err := tools.NewBulkUpdateTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create bulk update tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, items)
		if err != nil {
			wrappedErr := fmt.Errorf("bulk update: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("Bulk update completed.\nUpdated: %d\nFailed: %d\nTotal: %d",
			result.Updated, result.Failed, result.Total)), nil
	}
}

// createSummaryHandler creates the MCP tool handler for the glpi_summary command.
func createSummaryHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var itemtypes []string
		if itVal := request.GetStringSlice("itemtypes", nil); itVal != nil {
			itemtypes = itVal
		}

		tool, err := tools.NewSummaryTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create summary tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, itemtypes)
		if err != nil {
			wrappedErr := fmt.Errorf("summary: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("Inventory summary.\nTotal items: %d\nItemtypes queried: %d",
			result.Total, len(result.Itemtypes))), nil
	}
}

// createLicenseComplianceHandler creates the MCP tool handler for the glpi_license_compliance command.
func createLicenseComplianceHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		softwareID, err := request.RequireInt("software_id")
		if err != nil {
			return mcp.NewToolResultError("software_id is required and must be a number"), nil
		}

		entityID := 0
		if eid, err := request.RequireInt("entity_id"); err == nil {
			entityID = int(eid)
		}

		tool, err := tools.NewLicenseComplianceTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create license compliance tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, softwareID, entityID)
		if err != nil {
			wrappedErr := fmt.Errorf("license compliance: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("License compliance report.\nSoftware: %s (ID: %d)\nPurchased: %d\nInstalled: %d\nGap: %d\nStatus: %s",
			result.SoftwareName, result.SoftwareID, result.PurchasedCount, result.InstalledCount, result.ComplianceGap, result.Status)), nil
	}
}

// createUserAssetsHandler creates the MCP tool handler for the glpi_user_assets command.
func createUserAssetsHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID, err := request.RequireInt("user_id")
		if err != nil {
			return mcp.NewToolResultError("user_id is required and must be a number"), nil
		}

		var itemtypes []string
		if itVal := request.GetStringSlice("itemtypes", nil); itVal != nil {
			itemtypes = itVal
		}

		tool, err := tools.NewUserAssetsTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create user assets tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, userID, itemtypes)
		if err != nil {
			wrappedErr := fmt.Errorf("user assets: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("User %d has %d assigned assets.", userID, result.Count)), nil
	}
}

// createGroupAssetsHandler creates the MCP tool handler for the glpi_group_assets command.
func createGroupAssetsHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		groupID, err := request.RequireInt("group_id")
		if err != nil {
			return mcp.NewToolResultError("group_id is required and must be a number"), nil
		}

		var itemtypes []string
		if itVal := request.GetStringSlice("itemtypes", nil); itVal != nil {
			itemtypes = itVal
		}

		tool, err := tools.NewGroupAssetsTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create group assets tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, groupID, itemtypes)
		if err != nil {
			wrappedErr := fmt.Errorf("group assets: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("Group %d has %d assigned assets.", groupID, result.Count)), nil
	}
}

// createExpirationTrackerHandler creates the MCP tool handler for the glpi_expiration_tracker command.
func createExpirationTrackerHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		daysAhead, err := request.RequireInt("days_ahead")
		if err != nil {
			return mcp.NewToolResultError("days_ahead is required and must be a number"), nil
		}

		var itemtypes []string
		if itVal := request.GetStringSlice("itemtypes", nil); itVal != nil {
			itemtypes = itVal
		}

		entityID := 0
		if eid, err := request.RequireInt("entity_id"); err == nil {
			entityID = int(eid)
		}

		tool, err := tools.NewExpirationTrackerTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create expiration tracker tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, daysAhead, itemtypes, entityID)
		if err != nil {
			wrappedErr := fmt.Errorf("expiration tracker: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("Expiration check completed.\nDays ahead: %d\nTotal expiring items: %d\nItemtypes queried: %d",
			result.DaysAhead, result.TotalExpiring, len(result.ByItemtype))), nil
	}
}

// createWarrantyReportHandler creates the MCP tool handler for the glpi_warranty_report command.
func createWarrantyReportHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		daysWarning := 90
		if dw, err := request.RequireInt("days_warning"); err == nil {
			daysWarning = int(dw)
		}

		var itemtypes []string
		if itVal := request.GetStringSlice("itemtypes", nil); itVal != nil {
			itemtypes = itVal
		}

		entityID := 0
		if eid, err := request.RequireInt("entity_id"); err == nil {
			entityID = int(eid)
		}

		tool, err := tools.NewWarrantyReportTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create warranty report tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, daysWarning, itemtypes, entityID)
		if err != nil {
			wrappedErr := fmt.Errorf("warranty report: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("Warranty report completed.\nActive: %d\nExpired: %d\nExpiring soon: %d\nTotal assets: %d\nTotal purchase cost: %.2f",
			result.Summary.Active, result.Summary.Expired, result.Summary.ExpiringSoon, result.Summary.Total, result.TotalPurchaseCost)), nil
	}
}

// createCostSummaryHandler creates the MCP tool handler for the glpi_cost_summary command.
func createCostSummaryHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		entityID := 0
		if eid, err := request.RequireInt("entity_id"); err == nil {
			entityID = int(eid)
		}

		includeContracts := request.GetBool("include_contracts", true)
		includeBudgets := request.GetBool("include_budgets", true)

		tool, err := tools.NewCostSummaryTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create cost summary tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, entityID, includeContracts, includeBudgets)
		if err != nil {
			wrappedErr := fmt.Errorf("cost summary: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("Cost summary completed.\nAsset types: %d\nContracts: %d\nBudgets: %d\nTotal asset cost: %.2f\nTotal contract cost: %.2f\nTotal budget allocated: %.2f\nGrand total: %.2f",
			len(result.AssetTypeCosts), len(result.ContractCosts), len(result.BudgetAllocations),
			result.TotalAssetCost, result.TotalContractCost, result.TotalBudgetAllocated, result.GrandTotal)), nil
	}
}

// createRackCapacityHandler creates the MCP tool handler for the glpi_rack_capacity command.
func createRackCapacityHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rackID := 0
		if rid, err := request.RequireInt("rack_id"); err == nil {
			rackID = int(rid)
		}

		includeUnplaced := request.GetBool("include_unplaced", false)

		tool, err := tools.NewRackCapacityTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create rack capacity tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, rackID, includeUnplaced)
		if err != nil {
			wrappedErr := fmt.Errorf("rack capacity: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("Rack capacity report.\nRacks: %d\nTotal U: %d\nUsed U: %d\nAvailable U: %d\nOverall utilization: %.1f%%",
			result.RackCount, result.TotalRackU, result.TotalUsedU, result.TotalAvailableU, result.OverallUtilizationPct)), nil
	}
}

// createNetworkTopologyHandler creates the MCP tool handler for the glpi_network_topology command.
func createNetworkTopologyHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		portID := 0
		if pid, err := request.RequireInt("port_id"); err == nil {
			portID = int(pid)
		}

		deviceID := 0
		if did, err := request.RequireInt("device_id"); err == nil {
			deviceID = int(did)
		}

		deviceType := request.GetString("device_type", "")
		showVLANs := request.GetBool("show_vlans", false)

		tool, err := tools.NewNetworkTopologyTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create network topology tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, portID, deviceID, deviceType, showVLANs)
		if err != nil {
			wrappedErr := fmt.Errorf("network topology: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, tools.BuildTopologyText(result)), nil
	}
}
