package tools

import (
	"context"
	"fmt"
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
		searchResult, err := (&SearchTool{client: u.client}).Execute(ctx, itemtype, []SearchCriterion{{
			FieldName:  fmt.Sprintf("%s.users_id", itemtype),
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
