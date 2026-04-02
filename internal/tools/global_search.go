package tools

import (
	"context"
	"fmt"
	"sync"
)

// DefaultItemtypes is the set of itemtypes searched by global_search when no filter is provided.
var DefaultItemtypes = []string{
	"Computer",
	"Printer",
	"Monitor",
	"NetworkEquipment",
	"Phone",
	"Rack",
	"Peripheral",
	"Enclosure",
}

// GlobalSearchResult represents the aggregated result of a global search.
type GlobalSearchResult struct {
	Itemtypes        []string           `json:"itemtypes"`
	Range            *SearchRange       `json:"range,omitempty"`
	TotalCount       int                `json:"totalcount"`
	ReturnedCount    int                `json:"returned_count"`
	PerItemtypeCount map[string]int     `json:"per_itemtype_count"`
	Items            []GlobalSearchItem `json:"items"`
}

// GlobalSearchItem represents a single item annotated with its itemtype.
type GlobalSearchItem struct {
	Itemtype string                 `json:"itemtype"`
	ID       int                    `json:"id"`
	Data     map[string]interface{} `json:"data,omitempty"`
}

// GlobalSearchTool searches across multiple GLPI itemtypes.
type GlobalSearchTool struct {
	client ToolClient
}

// NewGlobalSearchTool creates a new global search tool.
func NewGlobalSearchTool(client ToolClient) (*GlobalSearchTool, error) {
	if client == nil {
		return nil, fmt.Errorf("global search tool: client cannot be nil")
	}
	return &GlobalSearchTool{client: client}, nil
}

// Name returns the tool name for registration.
func (g *GlobalSearchTool) Name() string {
	return "glpi_global_search"
}

// Description returns the tool description.
func (g *GlobalSearchTool) Description() string {
	return "Search multiple GLPI inventory itemtypes in one request"
}

// Execute runs a parallel search across the given itemtypes.
func (g *GlobalSearchTool) Execute(ctx context.Context, criteria []SearchCriterion, fields []string, searchRange *SearchRange, itemtypes []string) (*GlobalSearchResult, error) {
	if len(criteria) == 0 {
		return nil, fmt.Errorf("at least one search criterion is required")
	}
	if len(itemtypes) == 0 {
		itemtypes = DefaultItemtypes
	}

	perItemtype := make(map[string]int, len(itemtypes))
	perItemtypeMu := sync.Mutex{}
	allItemsMu := sync.Mutex{}
	var allItems []GlobalSearchItem

	var wg sync.WaitGroup
	var firstErr error
	var firstErrMu sync.Mutex

	for _, itemtype := range itemtypes {
		wg.Add(1)
		go func(it string) {
			defer wg.Done()

			searchResult, err := (&SearchTool{client: g.client}).Execute(ctx, it, criteria, fields, searchRange)
			if err != nil {
				firstErrMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("global search [%s]: %w", it, err)
				}
				firstErrMu.Unlock()
				return
			}

			perItemtypeMu.Lock()
			perItemtype[it] = searchResult.TotalCount
			perItemtypeMu.Unlock()

			for _, item := range searchResult.Data {
				allItemsMu.Lock()
				allItems = append(allItems, GlobalSearchItem{
					Itemtype: it,
					ID:       item.ID,
					Data:     item.Data,
				})
				allItemsMu.Unlock()
			}
		}(itemtype)
	}

	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	totalCount := 0
	for _, c := range perItemtype {
		totalCount += c
	}

	return &GlobalSearchResult{
		Itemtypes:        itemtypes,
		Range:            searchRange,
		TotalCount:       totalCount,
		ReturnedCount:    len(allItems),
		PerItemtypeCount: perItemtype,
		Items:            allItems,
	}, nil
}

// Ensure GlobalSearchTool implements the Tool interface.
var _ Tool = (*GlobalSearchTool)(nil)
