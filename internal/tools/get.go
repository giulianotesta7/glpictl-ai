package tools

import (
	"context"
	"fmt"
	"strings"
)

var getIncludeQueryParams = map[string]string{
	"software":          "with_softwares=true",
	"network_ports":     "with_networkports=true",
	"connected_devices": "with_connections=true",
	"contracts":         "with_contracts=true",
	"history":           "with_logs=true",
}

// GetInput represents the input for the glpi_get tool.
type GetInput struct {
	ItemType        string   `json:"itemtype"`
	ID              int      `json:"id"`
	Fields          []string `json:"fields,omitempty"`
	Include         []string `json:"include,omitempty"`
	ExpandDropdowns bool     `json:"expand_dropdowns,omitempty"`
}

// GetResult represents the result of a get operation.
type GetResult struct {
	ID    int                    `json:"id"`
	Name  string                 `json:"name,omitempty"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Error string                 `json:"error,omitempty"`
}

// GetTool provides the get functionality for retrieving a single GLPI item.
type GetTool struct {
	client ToolClient
}

// NewGetTool creates a new get tool with the given client.
func NewGetTool(client ToolClient) (*GetTool, error) {
	if client == nil {
		return nil, fmt.Errorf("get tool: client cannot be nil")
	}
	return &GetTool{client: client}, nil
}

// Name returns the tool name for registration.
func (g *GetTool) Name() string {
	return "glpi_get"
}

// Description returns the tool description.
func (g *GetTool) Description() string {
	return "Get a single GLPI item by type and ID, with optional related details"
}

// GetInput returns a new input struct for the tool.
func (g *GetTool) GetInput() *GetInput {
	return &GetInput{}
}

// Execute retrieves a GLPI item by type and ID.
func (g *GetTool) Execute(ctx context.Context, itemtype string, id int, fields []string, include []string, expandDropdowns bool) (*GetResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}
	if id <= 0 {
		return nil, fmt.Errorf("id must be a positive integer")
	}

	endpoint, err := buildGetEndpoint(itemtype, id, fields, include, expandDropdowns)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	err = g.client.Get(ctx, endpoint, &result)
	if err != nil {
		return nil, fmt.Errorf("get item: %w", err)
	}

	getResult := &GetResult{Data: result}

	if idVal, ok := result["id"]; ok {
		switch v := idVal.(type) {
		case float64:
			getResult.ID = int(v)
		case int:
			getResult.ID = v
		}
	}

	if nameVal, ok := result["name"]; ok {
		if name, ok := nameVal.(string); ok {
			getResult.Name = name
		}
	}

	return getResult, nil
}

func buildGetEndpoint(itemtype string, id int, fields []string, include []string, expandDropdowns bool) (string, error) {
	endpoint := fmt.Sprintf("/%s/%d", itemtype, id)
	params := make([]string, 0, len(fields)+len(include)+1)

	for i, field := range fields {
		params = append(params, fmt.Sprintf("fields[%d]=%s", i, field))
	}

	if expandDropdowns {
		params = append(params, "expand_dropdowns=true")
	}

	seen := make(map[string]struct{}, len(include))
	for _, rawInclude := range include {
		queryParam, ok := getIncludeQueryParams[rawInclude]
		if !ok {
			return "", fmt.Errorf("unsupported include %q", rawInclude)
		}
		if _, exists := seen[rawInclude]; exists {
			continue
		}
		seen[rawInclude] = struct{}{}
		params = append(params, queryParam)
	}

	if len(params) == 0 {
		return endpoint, nil
	}

	return endpoint + "?" + strings.Join(params, "&"), nil
}

// Ensure GetTool implements the Tool interface
var _ Tool = (*GetTool)(nil)
