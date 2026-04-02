package tools

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// SearchCriterion represents a single search criterion.
type SearchCriterion struct {
	Field      int    `json:"field,omitempty"`      // Field ID to search on
	FieldName  string `json:"field_name,omitempty"` // Human-friendly field name or uid
	SearchType string `json:"searchtype"`           // Search type: contains, equals, notcontains, etc.
	Value      string `json:"value"`                // Search value
	Link       string `json:"link,omitempty"`       // Link operator: AND, OR (empty for first criterion)
}

// SearchRange represents the range of results to return.
type SearchRange struct {
	Start int `json:"start"` // Start index (0-based)
	End   int `json:"end"`   // End index (inclusive)
}

// SearchInput represents the input for the glpi_search tool.
type SearchInput struct {
	ItemType string            `json:"itemtype"` // GLPI item type (e.g., Computer, Printer)
	Criteria []SearchCriterion `json:"criteria"` // Search criteria
	Fields   []string          `json:"fields"`   // Fields to return (empty = default)
	Range    *SearchRange      `json:"range"`    // Result range (optional)
}

// SearchData represents a single search result item.
type SearchData struct {
	ID   int                    `json:"id"`
	Data map[string]interface{} `json:"data,omitempty"`
}

// SearchResult represents the result of a search operation.
type SearchResult struct {
	TotalCount int          `json:"totalcount"`
	Data       []SearchData `json:"data"`
	Error      string       `json:"error,omitempty"`
}

// SearchTool provides the search functionality for querying GLPI items.
type SearchTool struct {
	client ToolClient
}

// NewSearchTool creates a new search tool with the given client.
// Returns an error if the client is nil.
func NewSearchTool(client ToolClient) (*SearchTool, error) {
	if client == nil {
		return nil, fmt.Errorf("search tool: client cannot be nil")
	}
	return &SearchTool{client: client}, nil
}

// Name returns the tool name for registration.
func (s *SearchTool) Name() string {
	return "glpi_search"
}

// Description returns the tool description.
func (s *SearchTool) Description() string {
	return "Search GLPI items with criteria and optional field selection"
}

// GetInput returns a new input struct for the tool.
func (s *SearchTool) GetInput() *SearchInput {
	return &SearchInput{}
}

// Execute searches GLPI items using the provided criteria.
// It builds the query parameters and returns matching items.
func (s *SearchTool) Execute(ctx context.Context, itemtype string, criteria []SearchCriterion, fields []string, searchRange *SearchRange) (*SearchResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}
	if !ValidateItemType(itemtype) {
		return nil, fmt.Errorf("invalid itemtype: %q", itemtype)
	}
	if len(criteria) == 0 {
		return nil, fmt.Errorf("at least one search criterion is required")
	}

	resolvedCriteria, err := s.resolveCriteria(ctx, itemtype, criteria)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}

	// Build query parameters
	params := url.Values{}

	// Add criteria
	for i, c := range resolvedCriteria {
		prefix := fmt.Sprintf("criteria[%d]", i)
		params.Add(prefix+"[field]", fmt.Sprintf("%d", c.Field))
		params.Add(prefix+"[searchtype]", c.SearchType)
		params.Add(prefix+"[value]", c.Value)
		if c.Link != "" {
			params.Add(prefix+"[link]", c.Link)
		}
	}

	// Add fields
	for i, f := range fields {
		params.Add(fmt.Sprintf("fields[%d]", i), f)
	}

	// Add range
	if searchRange != nil {
		params.Add("range", fmt.Sprintf("%d-%d", searchRange.Start, searchRange.End))
	}

	// Build endpoint
	endpoint := fmt.Sprintf("/search/%s?%s", itemtype, params.Encode())

	var result map[string]interface{}
	err = s.client.Get(ctx, endpoint, &result)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}

	// Parse result
	searchResult := &SearchResult{}

	// Extract totalcount
	if totalcount, ok := result["totalcount"]; ok {
		switch v := totalcount.(type) {
		case float64:
			searchResult.TotalCount = int(v)
		case int:
			searchResult.TotalCount = v
		}
	}

	// Extract data
	if data, ok := result["data"]; ok {
		if dataArray, ok := data.([]interface{}); ok {
			for _, item := range dataArray {
				if itemMap, ok := item.(map[string]interface{}); ok {
					searchData := SearchData{Data: itemMap}
					// Extract ID
					if idVal, ok := itemMap["id"]; ok {
						switch v := idVal.(type) {
						case float64:
							searchData.ID = int(v)
						case int:
							searchData.ID = v
						}
					} else if idVal, ok := itemMap["2"]; ok {
						// GLPI returns ID as field "2"
						switch v := idVal.(type) {
						case float64:
							searchData.ID = int(v)
						case int:
							searchData.ID = v
						}
					}
					searchResult.Data = append(searchResult.Data, searchData)
				}
			}
		}
	}

	return searchResult, nil
}

func (s *SearchTool) resolveCriteria(ctx context.Context, itemtype string, criteria []SearchCriterion) ([]SearchCriterion, error) {
	resolved := make([]SearchCriterion, len(criteria))
	copy(resolved, criteria)

	needsResolution := false
	for _, criterion := range resolved {
		if criterion.Field <= 0 && criterion.FieldName != "" {
			needsResolution = true
			break
		}
	}

	if !needsResolution {
		return resolved, nil
	}

	searchOptions, err := s.client.GetSearchOptions(ctx, itemtype)
	if err != nil {
		return nil, fmt.Errorf("resolve field_name for %s: %w", itemtype, err)
	}

	for i, criterion := range resolved {
		if criterion.Field > 0 {
			continue
		}

		if criterion.FieldName == "" {
			return nil, fmt.Errorf("criterion %d: field or field_name is required", i)
		}

		fieldID, resolveErr := resolveFieldName(searchOptions.Fields, criterion.FieldName)
		if resolveErr != nil {
			return nil, fmt.Errorf("criterion %d: %w", i, resolveErr)
		}

		resolved[i].Field = fieldID
	}

	return resolved, nil
}

func resolveFieldName(options []glpi.SearchOption, fieldName string) (int, error) {
	uidMatches := collectMatches(options, func(option glpi.SearchOption) bool {
		return option.UID == fieldName
	})
	if len(uidMatches) == 1 {
		return uidMatches[0], nil
	}
	if len(uidMatches) > 1 {
		return 0, newAmbiguousFieldNameError(fieldName, uidMatches)
	}

	technicalMatches := collectMatches(options, func(option glpi.SearchOption) bool {
		return option.Field == fieldName || option.Name == fieldName
	})
	if len(technicalMatches) == 1 {
		return technicalMatches[0], nil
	}
	if len(technicalMatches) > 1 {
		return 0, newAmbiguousFieldNameError(fieldName, technicalMatches)
	}

	displayMatches := collectMatches(options, func(option glpi.SearchOption) bool {
		return option.DisplayName == fieldName
	})
	if len(displayMatches) == 1 {
		return displayMatches[0], nil
	}
	if len(displayMatches) > 1 {
		return 0, newAmbiguousFieldNameError(fieldName, displayMatches)
	}

	return 0, fmt.Errorf("field_name %q could not be mapped to a numeric field id", fieldName)
}

func collectMatches(options []glpi.SearchOption, matchFn func(glpi.SearchOption) bool) []int {
	ids := make([]int, 0)
	for _, option := range options {
		if matchFn(option) {
			ids = append(ids, option.ID)
		}
	}
	sort.Ints(ids)
	return ids
}

func newAmbiguousFieldNameError(fieldName string, ids []int) error {
	values := make([]string, len(ids))
	for i, id := range ids {
		values[i] = fmt.Sprintf("%d", id)
	}

	return fmt.Errorf("field_name %q is ambiguous (matched ids: %s)", fieldName, strings.Join(values, ","))
}

// Ensure SearchTool implements the Tool interface
var _ Tool = (*SearchTool)(nil)
