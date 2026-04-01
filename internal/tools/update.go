package tools

import (
	"context"
	"fmt"
)

// UpdateInput represents the input for the glpi_update tool.
type UpdateInput struct {
	ItemType string                 `json:"itemtype"` // GLPI item type (e.g., Computer, Printer)
	ID       int                    `json:"id"`       // Item ID
	Data     map[string]interface{} `json:"data"`     // Fields to update
}

// UpdateResult represents the result of an update operation.
type UpdateResult struct {
	ID      int                    `json:"id"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// UpdateTool provides the update functionality for updating GLPI items.
type UpdateTool struct {
	client ToolClient
}

// NewUpdateTool creates a new update tool with the given client.
// Returns an error if the client is nil.
func NewUpdateTool(client ToolClient) (*UpdateTool, error) {
	if client == nil {
		return nil, fmt.Errorf("update tool: client cannot be nil")
	}
	return &UpdateTool{client: client}, nil
}

// Name returns the tool name for registration.
func (u *UpdateTool) Name() string {
	return "glpi_update"
}

// Description returns the tool description.
func (u *UpdateTool) Description() string {
	return "Update an existing GLPI item"
}

// GetInput returns a new input struct for the tool.
func (u *UpdateTool) GetInput() *UpdateInput {
	return &UpdateInput{}
}

// Execute updates a GLPI item with the provided data.
func (u *UpdateTool) Execute(ctx context.Context, itemtype string, id int, data map[string]interface{}) (*UpdateResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be a positive integer")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("data is required")
	}

	// Build endpoint
	endpoint := fmt.Sprintf("/%s/%d", itemtype, id)

	// Wrap data in "input" as required by GLPI API
	requestBody := map[string]interface{}{
		"input": data,
	}

	var result map[string]interface{}
	err := u.client.Put(ctx, endpoint, requestBody, &result)
	if err != nil {
		return nil, fmt.Errorf("update item: %w", err)
	}

	// Parse result
	updateResult := &UpdateResult{
		Data: result,
	}

	// Extract ID if present
	if idVal, ok := result["id"]; ok {
		switch v := idVal.(type) {
		case float64:
			updateResult.ID = int(v)
		case int:
			updateResult.ID = v
		}
	}

	// Extract message if present
	if msgVal, ok := result["message"]; ok {
		if msg, ok := msgVal.(string); ok {
			updateResult.Message = msg
		}
	}

	return updateResult, nil
}

// Ensure UpdateTool implements the Tool interface
var _ Tool = (*UpdateTool)(nil)
