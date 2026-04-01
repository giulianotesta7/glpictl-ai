package tools

import (
	"context"
	"fmt"
)

// DeleteInput represents the input for the glpi_delete tool.
type DeleteInput struct {
	ItemType string `json:"itemtype"` // GLPI item type (e.g., Computer, Printer)
	ID       int    `json:"id"`       // Item ID
}

// DeleteResult represents the result of a delete operation.
type DeleteResult struct {
	ID      int                    `json:"id"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// DeleteTool provides the delete functionality for deleting GLPI items.
type DeleteTool struct {
	client ToolClient
}

// NewDeleteTool creates a new delete tool with the given client.
// Returns an error if the client is nil.
func NewDeleteTool(client ToolClient) (*DeleteTool, error) {
	if client == nil {
		return nil, fmt.Errorf("delete tool: client cannot be nil")
	}
	return &DeleteTool{client: client}, nil
}

// Name returns the tool name for registration.
func (d *DeleteTool) Name() string {
	return "glpi_delete"
}

// Description returns the tool description.
func (d *DeleteTool) Description() string {
	return "Delete a GLPI item by type and ID"
}

// GetInput returns a new input struct for the tool.
func (d *DeleteTool) GetInput() *DeleteInput {
	return &DeleteInput{}
}

// Execute deletes a GLPI item by type and ID.
func (d *DeleteTool) Execute(ctx context.Context, itemtype string, id int) (*DeleteResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be a positive integer")
	}

	// Build endpoint
	endpoint := fmt.Sprintf("/%s/%d", itemtype, id)

	var result map[string]interface{}
	err := d.client.Delete(ctx, endpoint, &result)
	if err != nil {
		return nil, fmt.Errorf("delete item: %w", err)
	}

	// Parse result
	deleteResult := &DeleteResult{
		Data: result,
	}

	// Extract ID if present
	if idVal, ok := result["id"]; ok {
		switch v := idVal.(type) {
		case float64:
			deleteResult.ID = int(v)
		case int:
			deleteResult.ID = v
		}
	}

	// Extract message if present
	if msgVal, ok := result["message"]; ok {
		if msg, ok := msgVal.(string); ok {
			deleteResult.Message = msg
		}
	}

	return deleteResult, nil
}

// Ensure DeleteTool implements the Tool interface
var _ Tool = (*DeleteTool)(nil)
