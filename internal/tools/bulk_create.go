package tools

import (
	"context"
	"fmt"
)

// BulkCreateItem represents a single item to create.
type BulkCreateItem struct {
	ItemType string                 `json:"itemtype"` // GLPI item type (e.g., Computer, Printer)
	Data     map[string]interface{} `json:"data"`     // Item data to create
}

// BulkCreateResult represents the outcome of a bulk create operation.
type BulkCreateResult struct {
	Total   int                    `json:"total"`
	Created int                    `json:"created"`
	Failed  int                    `json:"failed"`
	Items   []BulkCreateItemResult `json:"items"`
}

// BulkCreateItemResult represents the outcome for a single item.
type BulkCreateItemResult struct {
	ItemType string                 `json:"itemtype"`
	ID       int                    `json:"id,omitempty"`
	Status   string                 `json:"status"` // created, failed
	Message  string                 `json:"message,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// BulkCreateTool creates multiple GLPI items in one call.
type BulkCreateTool struct {
	createTool *CreateTool
}

// NewBulkCreateTool creates a new bulk create tool.
func NewBulkCreateTool(client ToolClient) (*BulkCreateTool, error) {
	if client == nil {
		return nil, fmt.Errorf("bulk create tool: client cannot be nil")
	}
	createTool, err := NewCreateTool(client)
	if err != nil {
		return nil, fmt.Errorf("create create tool: %w", err)
	}
	return &BulkCreateTool{createTool: createTool}, nil
}

// Name returns the tool name for registration.
func (b *BulkCreateTool) Name() string {
	return "glpi_bulk_create"
}

// Description returns the tool description.
func (b *BulkCreateTool) Description() string {
	return "Create multiple GLPI items at once; each item can have a different itemtype"
}

// Execute creates each item in the list and returns per-item results.
func (b *BulkCreateTool) Execute(ctx context.Context, items []BulkCreateItem) (*BulkCreateResult, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("items list is required and must not be empty")
	}

	result := &BulkCreateResult{
		Total: len(items),
		Items: make([]BulkCreateItemResult, 0, len(items)),
	}

	for _, item := range items {
		res := b.createSingle(ctx, item)
		result.Items = append(result.Items, res)
		if res.Status == "created" {
			result.Created++
		} else {
			result.Failed++
		}
	}

	return result, nil
}

func (b *BulkCreateTool) createSingle(ctx context.Context, item BulkCreateItem) BulkCreateItemResult {
	base := BulkCreateItemResult{
		ItemType: item.ItemType,
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

	createResult, err := b.createTool.Execute(ctx, item.ItemType, item.Data)
	if err != nil {
		base.Status = "failed"
		base.Message = err.Error()
		return base
	}

	base.ID = createResult.ID
	base.Status = "created"
	base.Message = createResult.Message
	base.Data = createResult.Data
	return base
}

// Ensure BulkCreateTool implements the Tool interface.
var _ Tool = (*BulkCreateTool)(nil)
