package tools

import (
	"context"
	"encoding/json"
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

// parseDeleteResponse handles GLPI's inconsistent delete response formats.
// GLPI may return:
// - {"id": 5, "message": "Item deleted"} (object format)
// - [{"5": true, "message": ""}] (array format with ID as key)
// - [{"5": true}] (minimal array format)
func parseDeleteResponse(raw interface{}, id int) *DeleteResult {
	result := &DeleteResult{ID: id}

	// Handle object format: {"id": 5, "message": "Item deleted"}
	if obj, ok := raw.(map[string]interface{}); ok {
		if msgVal, ok := obj["message"]; ok {
			if msg, ok := msgVal.(string); ok {
				result.Message = msg
			}
		}
		if idVal, ok := obj["id"]; ok {
			switch v := idVal.(type) {
			case float64:
				result.ID = int(v)
			case int:
				result.ID = v
			}
		}
		result.Data = obj
		return result
	}

	// Handle array format: [{"5": true, "message": ""}]
	if arr, ok := raw.([]interface{}); ok && len(arr) > 0 {
		if entry, ok := arr[0].(map[string]interface{}); ok {
			// Extract message if present
			if msgVal, ok := entry["message"]; ok {
				if msg, ok := msgVal.(string); ok && msg != "" {
					result.Message = msg
				}
			}
			// ID might be a key in the map (e.g., "5": true)
			for key, val := range entry {
				if key == "message" {
					continue
				}
				if parsedID, err := parseInt(key); err == nil {
					result.ID = parsedID
					// If the value is a map, merge it into Data
					if valMap, ok := val.(map[string]interface{}); ok {
						result.Data = valMap
					} else if valBool, ok := val.(bool); ok && valBool {
						result.Data = map[string]interface{}{key: true}
					}
					break
				}
			}
			// If no ID found in keys, use the provided id
			if result.ID == 0 {
				result.ID = id
			}
		}
	}

	return result
}

// parseInt converts a string key to int (e.g., "5" -> 5)
func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
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

	// GLPI returns delete responses in multiple formats:
	// - Object: {"id": 5, "message": "Item deleted"}
	// - Array: [{"5": true, "message": ""}]
	// We use json.RawMessage to capture the raw response and parse it flexibly.
	var raw json.RawMessage
	err := d.client.Delete(ctx, endpoint, &raw)
	if err != nil {
		return nil, fmt.Errorf("delete item: %w", err)
	}

	// Try to parse as object first
	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return parseDeleteResponse(obj, id), nil
	}

	// Try to parse as array
	var arr []interface{}
	if err := json.Unmarshal(raw, &arr); err == nil {
		return parseDeleteResponse(arr, id), nil
	}

	// If both fail, return a result with the raw data
	return &DeleteResult{
		ID:      id,
		Message: "Item deleted (raw response could not be parsed)",
		Data:    map[string]interface{}{"raw": string(raw)},
	}, nil
}

// Ensure DeleteTool implements the Tool interface
var _ Tool = (*DeleteTool)(nil)
