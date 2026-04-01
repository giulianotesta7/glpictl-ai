package tools

import (
	"context"
	"fmt"
	"strings"
)

// GetInput represents the input for the glpi_get tool.
type GetInput struct {
	ItemType string   `json:"itemtype"` // GLPI item type (e.g., Computer, Printer)
	ID       int      `json:"id"`       // Item ID
	Fields   []string `json:"fields"`   // Fields to return (empty = all)
}

// GetResult represents the result of a get operation.
type GetResult struct {
	ID    int                    `json:"id"`
	Name  string                 `json:"name,omitempty"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Error string                 `json:"error,omitempty"`
}

// GetTool provides the get functionality for retrieving a single GLPI item.
type GetTool struct {
	client ToolClient
}

// NewGetTool creates a new get tool with the given client.
// Returns an error if the client is nil.
func NewGetTool(client ToolClient) (*GetTool, error) {
	if client == nil {
		return nil, fmt.Errorf("get tool: client cannot be nil")
	}
	return &GetTool{client: client}, nil
}

// Name returns the tool name for registration.
func (g *GetTool) Name() string {
	return "glpi_get"
}

// Description returns the tool description.
func (g *GetTool) Description() string {
	return "Get a single GLPI item by type and ID"
}

// GetInput returns a new input struct for the tool.
func (g *GetTool) GetInput() *GetInput {
	return &GetInput{}
}

// Execute retrieves a GLPI item by type and ID.
// It handles field filtering and returns the item data.
func (g *GetTool) Execute(ctx context.Context, itemtype string, id int, fields []string) (*GetResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be a positive integer")
	}

	// Build endpoint with optional field filtering
	endpoint := fmt.Sprintf("/%s/%d", itemtype, id)
	if len(fields) > 0 {
		var fieldParams []string
		for i, field := range fields {
			fieldParams = append(fieldParams, fmt.Sprintf("fields[%d]=%s", i, field))
		}
		endpoint = endpoint + "?" + strings.Join(fieldParams, "&")
	}

	var result map[string]interface{}
	err := g.client.Get(ctx, endpoint, &result)
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}

	// Extract common fields
	getResult := &GetResult{
		Data: result,
	}

	// Extract ID if present
	if idVal, ok := result["id"]; ok {
		switch v := idVal.(type) {
		case float64:
			getResult.ID = int(v)
		case int:
			getResult.ID = v
		}
	}

	// Extract name if present
	if nameVal, ok := result["name"]; ok {
		if name, ok := nameVal.(string); ok {
			getResult.Name = name
		}
	}

	return getResult, nil
}

// Ensure GetTool implements the Tool interface
var _ Tool = (*GetTool)(nil)
