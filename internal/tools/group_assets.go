package tools

import (
	"context"
	"fmt"
)

// GroupAssetsResult represents the assets assigned to a group.
type GroupAssetsResult struct {
	GroupID int              `json:"group_id"`
	Assets  []GroupAssetItem `json:"assets"`
	Count   int              `json:"count"`
}

// GroupAssetItem represents a single asset assigned to a group.
type GroupAssetItem struct {
	Itemtype string                 `json:"itemtype"`
	ID       int                    `json:"id"`
	Name     string                 `json:"name,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// GroupAssetsTool retrieves assets assigned to a specific group.
type GroupAssetsTool struct {
	client ToolClient
}

// NewGroupAssetsTool creates a new group assets tool.
func NewGroupAssetsTool(client ToolClient) (*GroupAssetsTool, error) {
	if client == nil {
		return nil, fmt.Errorf("group assets tool: client cannot be nil")
	}
	return &GroupAssetsTool{client: client}, nil
}

// Name returns the tool name for registration.
func (g *GroupAssetsTool) Name() string {
	return "glpi_group_assets"
}

// Description returns the tool description.
func (g *GroupAssetsTool) Description() string {
	return "Get all assets assigned to a specific group"
}

// Execute searches across inventory types for items assigned to the given group ID.
func (g *GroupAssetsTool) Execute(ctx context.Context, groupID int, itemtypes []string) (*GroupAssetsResult, error) {
	if groupID <= 0 {
		return nil, fmt.Errorf("group ID must be a positive integer")
	}
	if len(itemtypes) == 0 {
		itemtypes = DefaultItemtypes
	}

	result := &GroupAssetsResult{
		GroupID: groupID,
	}

	for _, itemtype := range itemtypes {
		searchResult, err := (&SearchTool{client: g.client}).Execute(ctx, itemtype, []SearchCriterion{{
			FieldName:  fmt.Sprintf("%s.groups_id", itemtype),
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", groupID),
		}}, []string{"name"}, nil)
		if err != nil {
			continue
		}

		for _, item := range searchResult.Data {
			name := ""
			if n, ok := item.Data["name"].(string); ok {
				name = n
			} else if n, ok := item.Data["1"].(string); ok {
				name = n
			}
			result.Assets = append(result.Assets, GroupAssetItem{
				Itemtype: itemtype,
				ID:       item.ID,
				Name:     name,
				Data:     item.Data,
			})
		}
	}

	result.Count = len(result.Assets)
	return result, nil
}

// Ensure GroupAssetsTool implements the Tool interface.
var _ Tool = (*GroupAssetsTool)(nil)
