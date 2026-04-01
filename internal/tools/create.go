package tools

import (
	"context"
	"fmt"
)

// CreateInput represents the input for the glpi_create tool.
type CreateInput struct {
	ItemType string                 `json:"itemtype"` // GLPI item type (e.g., Computer, Printer)
	Data     map[string]interface{} `json:"data"`     // Item data to create
}

// CreateResult represents the result of a create operation.
type CreateResult struct {
	ID      int                    `json:"id"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// CreateTool provides the create functionality for creating GLPI items.
type CreateTool struct {
	client ToolClient
}

// NewCreateTool creates a new create tool with the given client.
// Returns an error if the client is nil.
func NewCreateTool(client ToolClient) (*CreateTool, error) {
	if client == nil {
		return nil, fmt.Errorf("create tool: client cannot be nil")
	}
	return &CreateTool{client: client}, nil
}

// Name returns the tool name for registration.
func (c *CreateTool) Name() string {
	return "glpi_create"
}

// Description returns the tool description.
func (c *CreateTool) Description() string {
	return "Create a new GLPI item"
}

// GetInput returns a new input struct for the tool.
func (c *CreateTool) GetInput() *CreateInput {
	return &CreateInput{}
}

// Execute creates a GLPI item with the provided data.
func (c *CreateTool) Execute(ctx context.Context, itemtype string, data map[string]interface{}) (*CreateResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("data is required")
	}

	// Build endpoint
	endpoint := fmt.Sprintf("/%s", itemtype)

	// Wrap data in "input" as required by GLPI API
	requestBody := map[string]interface{}{
		"input": data,
	}

	var result map[string]interface{}
	err := c.client.Post(ctx, endpoint, requestBody, &result)
	if err != nil {
		return nil, fmt.Errorf("create item: %w", err)
	}

	// Parse result
	createResult := &CreateResult{
		Data: result,
	}

	// Extract ID if present
	if idVal, ok := result["id"]; ok {
		switch v := idVal.(type) {
		case float64:
			createResult.ID = int(v)
		case int:
			createResult.ID = v
		}
	}

	// Extract message if present
	if msgVal, ok := result["message"]; ok {
		if msg, ok := msgVal.(string); ok {
			createResult.Message = msg
		}
	}

	return createResult, nil
}

// Ensure CreateTool implements the Tool interface
var _ Tool = (*CreateTool)(nil)
