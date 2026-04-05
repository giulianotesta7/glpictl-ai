package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// UserAssetsResult represents the assets assigned to a user.
type UserAssetsResult struct {
	UserID int             `json:"user_id"`
	Assets []UserAssetItem `json:"assets"`
	Count  int             `json:"count"`
}

// UserAssetItem represents a single asset assigned to a user.
type UserAssetItem struct {
	Itemtype string                 `json:"itemtype"`
	ID       int                    `json:"id"`
	Name     string                 `json:"name,omitempty"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// UserAssetsTool retrieves assets assigned to a specific user.
type UserAssetsTool struct {
	client ToolClient
}

// NewUserAssetsTool creates a new user assets tool.
func NewUserAssetsTool(client ToolClient) (*UserAssetsTool, error) {
	if client == nil {
		return nil, fmt.Errorf("user assets tool: client cannot be nil")
	}
	return &UserAssetsTool{client: client}, nil
}

// Name returns the tool name for registration.
func (u *UserAssetsTool) Name() string {
	return "glpi_user_assets"
}

// Description returns the tool description.
func (u *UserAssetsTool) Description() string {
	return "Get all assets assigned to a specific user"
}

// resolveUserFieldID finds the numeric field ID for the user assignment field
// in the given itemtype by inspecting search options.
func (u *UserAssetsTool) resolveUserFieldID(ctx context.Context, itemtype string) (int, error) {
	searchOptions, err := u.client.GetSearchOptions(ctx, itemtype)
	if err != nil {
		return 0, fmt.Errorf("get search options for %s: %w", itemtype, err)
	}

	// Look for a field whose table matches the itemtype's main table and field name is "users_id"
	// GLPI stores the main table as "glpi_<lowercase_itemtype>" (e.g., "glpi_computers" for "Computer")
	mainTable := "glpi_" + strings.ToLower(itemtype)

	for _, opt := range searchOptions.Fields {
		if strings.EqualFold(opt.Table, mainTable) && strings.EqualFold(opt.Field, "users_id") {
			return opt.ID, nil
		}
	}

	// Fallback: look for any field with UID ending in ".User.name"
	// This handles itemtypes where the user field is a dropdown reference.
	// We need to find the PRIMARY user assignment field, not secondary ones like
	// users_id_tech, users_id_lastupdater, etc.
	// The primary user field UID follows the pattern: "{itemtype}.User.name"
	// (e.g., "Computer.User.name"), NOT "Computer.users_id_tech.User.name"
	prefix := itemtype + ".User.name"
	for _, opt := range searchOptions.Fields {
		if opt.UID == prefix {
			return opt.ID, nil
		}
	}

	// If exact match not found, try partial match but prefer shorter UIDs
	// (shorter = more direct, less nested = more likely the primary assignment field)
	var bestMatch *glpi.SearchOption
	for _, opt := range searchOptions.Fields {
		if strings.HasSuffix(opt.UID, ".User.name") {
			if bestMatch == nil || len(opt.UID) < len(bestMatch.UID) {
				bestMatch = &opt
			}
		}
	}
	if bestMatch != nil {
		return bestMatch.ID, nil
	}

	return 0, fmt.Errorf("no users_id field found for itemtype %q", itemtype)
}

// Execute searches across inventory types for items assigned to the given user ID.
func (u *UserAssetsTool) Execute(ctx context.Context, userID int, itemtypes []string) (*UserAssetsResult, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("user ID must be a positive integer")
	}
	if len(itemtypes) == 0 {
		itemtypes = DefaultItemtypes
	}

	result := &UserAssetsResult{
		UserID: userID,
	}

	for _, itemtype := range itemtypes {
		// Dynamically resolve the correct field ID for user assignment in this itemtype
		userFieldID, err := u.resolveUserFieldID(ctx, itemtype)
		if err != nil {
			continue // Skip itemtypes where we can't find the user field
		}

		searchResult, err := (&SearchTool{client: u.client}).Execute(ctx, itemtype, []SearchCriterion{{
			Field:      userFieldID,
			SearchType: "equals",
			Value:      fmt.Sprintf("%d", userID),
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
			result.Assets = append(result.Assets, UserAssetItem{
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

// Ensure UserAssetsTool implements the Tool interface.
var _ Tool = (*UserAssetsTool)(nil)
