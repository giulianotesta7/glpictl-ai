package tools

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
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
	client ToolClient
}

// NewUpdateByNameTool creates a new update-by-name tool with the given client.
func NewUpdateByNameTool(client ToolClient) (*UpdateByNameTool, error) {
	if client == nil {
		return nil, fmt.Errorf("update-by-name tool: client cannot be nil")
	}

	return &UpdateByNameTool{client: client}, nil
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

	// Build search query with forcedisplay to ensure ID (field 2) is returned
	params := url.Values{}
	params.Add("criteria[0][field]", "1")
	params.Add("criteria[0][searchtype]", "contains")
	params.Add("criteria[0][value]", name)
	params.Add("forcedisplay[0]", "2")
	params.Add("range", "0-50")

	endpoint := fmt.Sprintf("/search/%s?%s", itemtype, params.Encode())

	var result map[string]interface{}
	err := u.client.Get(ctx, endpoint, &result)
	if err != nil {
		return nil, fmt.Errorf("resolve item by name: %w", err)
	}

	// Parse totalcount
	totalCount := 0
	if tc, ok := result["totalcount"]; ok {
		switch v := tc.(type) {
		case float64:
			totalCount = int(v)
		case int:
			totalCount = v
		}
	}

	if totalCount == 0 {
		return nil, &UpdateByNameNotFoundError{ItemType: itemtype, Name: name}
	}

	// Parse data array and find exact name match
	var exactID int
	var exactName string
	var matchCount int
	var matches []MatchCandidate

	if dataArray, ok := result["data"].([]interface{}); ok {
		for _, rawItem := range dataArray {
			itemMap, ok := rawItem.(map[string]interface{})
			if !ok {
				continue
			}

			// Extract name from field "1" (GLPI returns it as string when fields are specified)
			itemName := ""
			if v, ok := itemMap["1"]; ok {
				switch val := v.(type) {
				case string:
					itemName = val
				case float64:
					itemName = fmt.Sprintf("%.0f", val)
				}
			}

			if itemName != name {
				continue
			}

			// Extract ID from field "2" (forcedisplay ensures it's present)
			itemID := 0
			if v, ok := itemMap["2"]; ok {
				switch val := v.(type) {
				case float64:
					itemID = int(val)
				case int:
					itemID = val
				case string:
					itemID, _ = strconv.Atoi(val)
				}
			}

			if itemID > 0 {
				exactID = itemID
				exactName = itemName
				matchCount++
				matches = append(matches, MatchCandidate{ID: itemID, Name: itemName})
			}
		}
	}

	if matchCount == 0 {
		return nil, &UpdateByNameNotFoundError{ItemType: itemtype, Name: name}
	}
	if matchCount > 1 {
		return nil, &UpdateByNameAmbiguousError{
			ItemType:   itemtype,
			Name:       name,
			TotalCount: matchCount,
			Matches:    matches,
		}
	}

	// Perform the update
	updateResult, err := (&UpdateTool{client: u.client}).Execute(ctx, itemtype, exactID, data)
	if err != nil {
		return nil, fmt.Errorf("update item by name: %w", err)
	}

	return &UpdateByNameResult{
		ID:      updateResult.ID,
		Name:    exactName,
		Message: updateResult.Message,
		Data:    updateResult.Data,
	}, nil
}

func collectMatchCandidates(data []SearchData) []MatchCandidate {
	matches := make([]MatchCandidate, 0, len(data))
	for _, item := range data {
		matches = append(matches, MatchCandidate{ID: item.ID, Name: candidateNameFromData(item.Data)})
	}
	return matches
}

// candidateNameFromData extracts the name from search result data.
func candidateNameFromData(data map[string]interface{}) string {
	if value, ok := data["name"].(string); ok {
		return value
	}
	if value, ok := data["1"].(string); ok {
		return value
	}
	return ""
}

// Ensure UpdateByNameTool implements the Tool interface.
var _ Tool = (*UpdateByNameTool)(nil)
