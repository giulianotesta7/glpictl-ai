package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
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

// resolveGroupFieldID finds the numeric field ID for the group assignment field
// in the given itemtype by inspecting search options.
func (g *GroupAssetsTool) resolveGroupFieldID(ctx context.Context, itemtype string) (int, error) {
	searchOptions, err := g.client.GetSearchOptions(ctx, itemtype)
	if err != nil {
		return 0, fmt.Errorf("get search options for %s: %w", itemtype, err)
	}

	// GLPI stores group assignment via the Group_Item linking table.
	// There may be multiple group fields (e.g., "Group" vs "Group in Charge").
	// We want the primary group assignment field, not the tech/support one.
	// Look for fields whose UID contains "Group_Item.Group" and prefer the one
	// whose name is simply "Group" (not "Group in Charge" or similar).
	var bestMatch *glpi.SearchOption
	for _, opt := range searchOptions.Fields {
		if strings.Contains(opt.UID, "Group_Item.Group") {
			// Prefer the field whose display name is exactly "Group"
			if opt.DisplayName == "Group" || opt.Name == "Group" {
				return opt.ID, nil
			}
			// Otherwise keep the first match as fallback
			if bestMatch == nil {
				bestMatch = &opt
			}
		}
	}
	if bestMatch != nil {
		return bestMatch.ID, nil
	}

	// Fallback: look for field with table containing "group" and field "completename" or "name"
	for _, opt := range searchOptions.Fields {
		if strings.Contains(strings.ToLower(opt.Table), "group") &&
			(strings.EqualFold(opt.Field, "completename") || strings.EqualFold(opt.Field, "name")) {
			return opt.ID, nil
		}
	}

	return 0, fmt.Errorf("no groups_id field found for itemtype %q", itemtype)
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
		// Dynamically resolve the correct field ID for group assignment in this itemtype
		groupFieldID, err := g.resolveGroupFieldID(ctx, itemtype)
		if err != nil {
			continue // Skip itemtypes where we can't find the group field
		}

		searchResult, err := (&SearchTool{client: g.client}).Execute(ctx, itemtype, []SearchCriterion{{
			Field:      groupFieldID,
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
