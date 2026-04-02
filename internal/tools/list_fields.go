package tools

import (
	"context"
	"fmt"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// ListFieldsResult represents the result of glpi_list_fields.
type ListFieldsResult struct {
	ItemType string              `json:"itemtype"`
	Fields   []glpi.SearchOption `json:"fields"`
	Cached   bool                `json:"cached"`
}

// ListFieldsTool provides searchable field discovery for GLPI item types.
type ListFieldsTool struct {
	client ToolClient
}

// NewListFieldsTool creates a new list fields tool.
func NewListFieldsTool(client ToolClient) (*ListFieldsTool, error) {
	if client == nil {
		return nil, fmt.Errorf("list fields tool: client cannot be nil")
	}

	return &ListFieldsTool{client: client}, nil
}

// Name returns the tool name.
func (l *ListFieldsTool) Name() string {
	return "glpi_list_fields"
}

// Description returns the tool description.
func (l *ListFieldsTool) Description() string {
	return "List searchable fields for a GLPI item type"
}

// Execute fetches searchable fields for a GLPI item type.
func (l *ListFieldsTool) Execute(ctx context.Context, itemtype string) (*ListFieldsResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}

	searchOptions, err := l.client.GetSearchOptions(ctx, itemtype)
	if err != nil {
		return nil, fmt.Errorf("list fields: %w", err)
	}

	return &ListFieldsResult{
		ItemType: searchOptions.ItemType,
		Fields:   searchOptions.Fields,
		Cached:   searchOptions.Cached,
	}, nil
}

var _ Tool = (*ListFieldsTool)(nil)
