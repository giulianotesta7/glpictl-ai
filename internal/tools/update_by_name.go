package tools

import (
	"context"
	"fmt"
	"strings"
)

// UpdateByNameInput represents the input for the glpi_update_by_name tool.
type UpdateByNameInput struct {
	ItemType string                 `json:"itemtype"`
	Name     string                 `json:"name"`
	Data     map[string]interface{} `json:"data"`
}

// MatchCandidate identifies a candidate item for an ambiguous name match.
type MatchCandidate struct {
	ID   int    `json:"id"`
	Name string `json:"name,omitempty"`
}

// UpdateByNameResult represents the result of an update-by-name operation.
type UpdateByNameResult struct {
	ID      int                    `json:"id"`
	Name    string                 `json:"name,omitempty"`
	Message string                 `json:"message,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

// UpdateByNameNotFoundError reports that no exact name match was found.
type UpdateByNameNotFoundError struct {
	ItemType string
	Name     string
}

// Error implements error.
func (e *UpdateByNameNotFoundError) Error() string {
	return fmt.Sprintf("no %s found with name %q", e.ItemType, e.Name)
}

// UpdateByNameAmbiguousError reports that more than one exact name match was found.
type UpdateByNameAmbiguousError struct {
	ItemType   string
	Name       string
	TotalCount int
	Matches    []MatchCandidate
}

// Error implements error.
func (e *UpdateByNameAmbiguousError) Error() string {
	return fmt.Sprintf("multiple %s items found with name %q (%d matches)", e.ItemType, e.Name, e.TotalCount)
}

// UpdateByNameTool updates a GLPI item after resolving a unique exact name match.
type UpdateByNameTool struct {
	searchTool *SearchTool
	updateTool *UpdateTool
}

// NewUpdateByNameTool creates a new update-by-name tool with the given client.
func NewUpdateByNameTool(client ToolClient) (*UpdateByNameTool, error) {
	if client == nil {
		return nil, fmt.Errorf("update-by-name tool: client cannot be nil")
	}

	searchTool, err := NewSearchTool(client)
	if err != nil {
		return nil, fmt.Errorf("create search tool: %w", err)
	}
	updateTool, err := NewUpdateTool(client)
	if err != nil {
		return nil, fmt.Errorf("create update tool: %w", err)
	}

	return &UpdateByNameTool{searchTool: searchTool, updateTool: updateTool}, nil
}

// Name returns the tool name for registration.
func (u *UpdateByNameTool) Name() string {
	return "glpi_update_by_name"
}

// Description returns the tool description.
func (u *UpdateByNameTool) Description() string {
	return "Update a GLPI item only when exactly one exact name match exists; returns an explicit error on zero or duplicate matches"
}

// GetInput returns a new input struct for the tool.
func (u *UpdateByNameTool) GetInput() *UpdateByNameInput {
	return &UpdateByNameInput{}
}

// Execute resolves a unique exact name match and updates it.
func (u *UpdateByNameTool) Execute(ctx context.Context, itemtype string, name string, data map[string]interface{}) (*UpdateByNameResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("data is required")
	}

	// Search using canonical UID form and exact match semantics
	searchResult, err := u.searchTool.Execute(ctx, itemtype, []SearchCriterion{{
		FieldName:  fmt.Sprintf("%s.name", itemtype),
		SearchType: "equals",
		Value:      name,
	}}, []string{"name"}, &SearchRange{Start: 0, End: 20})
	if err != nil {
		return nil, fmt.Errorf("resolve item by name: %w", err)
	}

	if searchResult.TotalCount == 0 || len(searchResult.Data) == 0 {
		return nil, &UpdateByNameNotFoundError{ItemType: itemtype, Name: name}
	}
	if searchResult.TotalCount > 1 {
		return nil, &UpdateByNameAmbiguousError{
			ItemType:   itemtype,
			Name:       name,
			TotalCount: searchResult.TotalCount,
			Matches:    collectMatchCandidates(searchResult.Data),
		}
	}

	// Verify literal exact match after singleton search
	match := searchResult.Data[0]
	if candidateName(match) != name {
		return nil, &UpdateByNameNotFoundError{ItemType: itemtype, Name: name}
	}

	// Perform the update
	updateResult, err := u.updateTool.Execute(ctx, itemtype, match.ID, data)
	if err != nil {
		return nil, fmt.Errorf("update item by name: %w", err)
	}

	return &UpdateByNameResult{
		ID:      updateResult.ID,
		Name:    candidateName(match),
		Message: updateResult.Message,
		Data:    updateResult.Data,
	}, nil
}

func collectMatchCandidates(data []SearchData) []MatchCandidate {
	matches := make([]MatchCandidate, 0, len(data))
	for _, item := range data {
		matches = append(matches, MatchCandidate{ID: item.ID, Name: candidateName(item)})
	}
	return matches
}

func candidateName(item SearchData) string {
	if value, ok := item.Data["name"].(string); ok {
		return value
	}
	if value, ok := item.Data["1"].(string); ok {
		return value
	}
	return ""
}

// Ensure UpdateByNameTool implements the Tool interface.
var _ Tool = (*UpdateByNameTool)(nil)
