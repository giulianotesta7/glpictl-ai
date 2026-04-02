package tools

import (
	"context"
	"fmt"
)

// BulkUpdateItem represents a single item to update.
// Specify either ID or Name to identify the target item.
type BulkUpdateItem struct {
	ItemType string                 `json:"itemtype"`
	ID       int                    `json:"id,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Data     map[string]interface{} `json:"data"`
}

// BulkUpdateResult represents the outcome of a bulk update operation.
type BulkUpdateResult struct {
	Total   int                    `json:"total"`
	Updated int                    `json:"updated"`
	Failed  int                    `json:"failed"`
	Items   []BulkUpdateItemResult `json:"items"`
}

// BulkUpdateItemResult represents the outcome for a single item.
type BulkUpdateItemResult struct {
	ItemType string                 `json:"itemtype"`
	ID       int                    `json:"id,omitempty"`
	Name     string                 `json:"name,omitempty"`
	Status   string                 `json:"status"` // updated, not_found, ambiguous, failed
	Message  string                 `json:"message,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// BulkUpdateTool updates multiple GLPI items in one call.
type BulkUpdateTool struct {
	updateByNameTool *UpdateByNameTool
	updateTool       *UpdateTool
}

// NewBulkUpdateTool creates a new bulk update tool.
func NewBulkUpdateTool(client ToolClient) (*BulkUpdateTool, error) {
	if client == nil {
		return nil, fmt.Errorf("bulk update tool: client cannot be nil")
	}
	updateByNameTool, err := NewUpdateByNameTool(client)
	if err != nil {
		return nil, fmt.Errorf("create update-by-name tool: %w", err)
	}
	updateTool, err := NewUpdateTool(client)
	if err != nil {
		return nil, fmt.Errorf("create update tool: %w", err)
	}
	return &BulkUpdateTool{updateByNameTool: updateByNameTool, updateTool: updateTool}, nil
}

// Name returns the tool name for registration.
func (b *BulkUpdateTool) Name() string {
	return "glpi_bulk_update"
}

// Description returns the tool description.
func (b *BulkUpdateTool) Description() string {
	return "Update multiple GLPI items at once; specify each item by name or ID and receive per-item results"
}

// Execute updates each item in the list and returns per-item results.
func (b *BulkUpdateTool) Execute(ctx context.Context, items []BulkUpdateItem) (*BulkUpdateResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("items list is required and must not be empty")
	}

	result := &BulkUpdateResult{
		Total: len(items),
		Items: make([]BulkUpdateItemResult, 0, len(items)),
	}

	for _, item := range items {
		res := b.updateSingle(ctx, item)
		result.Items = append(result.Items, res)
		if res.Status == "updated" {
			result.Updated++
		} else {
			result.Failed++
		}
	}

	return result, nil
}

func (b *BulkUpdateTool) updateSingle(ctx context.Context, item BulkUpdateItem) BulkUpdateItemResult {
	base := BulkUpdateItemResult{
		ItemType: item.ItemType,
		Name:     item.Name,
	}

	if item.ItemType == "" {
		base.Status = "failed"
		base.Message = "itemtype is required"
		return base
	}
	if !ValidateItemType(item.ItemType) {
		base.Status = "failed"
		base.Message = fmt.Sprintf("invalid itemtype: %q", item.ItemType)
		return base
	}
	if len(item.Data) == 0 {
		base.Status = "failed"
		base.Message = "data is required"
		return base
	}

	// Update by ID if provided
	if item.ID > 0 {
		updateResult, err := b.updateTool.Execute(ctx, item.ItemType, item.ID, item.Data)
		if err != nil {
			base.Status = "failed"
			base.Message = err.Error()
			return base
		}
		base.ID = updateResult.ID
		base.Status = "updated"
		base.Message = updateResult.Message
		base.Data = updateResult.Data
		return base
	}

	// Update by name if provided
	if item.Name != "" {
		updateResult, err := b.updateByNameTool.Execute(ctx, item.ItemType, item.Name, item.Data)
		if err != nil {
			switch err.(type) {
			case *UpdateByNameNotFoundError:
				base.Status = "not_found"
				base.Message = err.Error()
			case *UpdateByNameAmbiguousError:
				base.Status = "ambiguous"
				base.Message = err.Error()
			default:
				base.Status = "failed"
				base.Message = err.Error()
			}
			return base
		}
		base.ID = updateResult.ID
		base.Name = updateResult.Name
		base.Status = "updated"
		base.Message = updateResult.Message
		base.Data = updateResult.Data
		return base
	}

	base.Status = "failed"
	base.Message = "either id or name is required"
	return base
}

// Ensure BulkUpdateTool implements the Tool interface.
var _ Tool = (*BulkUpdateTool)(nil)
