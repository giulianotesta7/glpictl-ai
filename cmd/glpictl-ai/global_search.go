package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
	"github.com/giulianotesta7/glpictl-ai/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func newGlobalSearchMCPTool() mcp.Tool {
	return mcp.NewTool("glpi_global_search",
		mcp.WithDescription("Search multiple GLPI inventory itemtypes in one request"),
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
		mcp.WithArray("itemtypes", mcp.Description("Itemtypes to search (default: all inventory types)"), mcp.Items(map[string]any{"type": "string"})),
	)
}

func createGlobalSearchHandler(client *glpi.Client) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		criteria, fields, searchRange := parseSearchArguments(request)

		if len(criteria) == 0 {
			return mcp.NewToolResultError("criteria is required and must be a non-empty array"), nil
		}

		// Extract optional itemtypes filter
		var itemtypes []string
		if itVal := request.GetStringSlice("itemtypes", nil); itVal != nil {
			itemtypes = itVal
		}

		tool, err := tools.NewGlobalSearchTool(client)
		if err != nil {
			wrappedErr := fmt.Errorf("create global search tool: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		result, err := tool.Execute(ctx, criteria, fields, searchRange, itemtypes)
		if err != nil {
			wrappedErr := fmt.Errorf("global search: %w", err)
			return mcp.NewToolResultError(wrappedErr.Error()), nil
		}

		return mcp.NewToolResultStructured(result, fmt.Sprintf("Global search completed.\nTotal count: %d\nResults: %d items",
			result.TotalCount, result.ReturnedCount)), nil
	}
}

func parseSearchArguments(request mcp.CallToolRequest) ([]tools.SearchCriterion, []string, *tools.SearchRange) {
	rawArgs := request.GetRawArguments()
	argsMap, ok := rawArgs.(map[string]interface{})
	if !ok {
		return nil, nil, nil
	}

	var criteria []tools.SearchCriterion
	if criteriaVal, ok := argsMap["criteria"].([]interface{}); ok {
		for _, c := range criteriaVal {
			cmap, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
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

	var fields []string
	if fieldsVal := request.GetStringSlice("fields", nil); fieldsVal != nil {
		fields = fieldsVal
	}

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

	_ = strings.Join // import kept for potential future use
	return criteria, fields, searchRange
}
