package tools

import (
	"context"
	"fmt"
	"sync"
)

// SummaryResult represents the inventory summary dashboard output.
type SummaryResult struct {
	Counts    map[string]int `json:"counts"`
	Total     int            `json:"total"`
	Itemtypes []string       `json:"itemtypes"`
}

// SummaryTool provides a summary dashboard of GLPI inventory counts by itemtype.
type SummaryTool struct {
	client ToolClient
}

// NewSummaryTool creates a new summary tool.
func NewSummaryTool(client ToolClient) (*SummaryTool, error) {
	if client == nil {
		return nil, fmt.Errorf("summary tool: client cannot be nil")
	}
	return &SummaryTool{client: client}, nil
}

// Name returns the tool name for registration.
func (s *SummaryTool) Name() string {
	return "glpi_summary"
}

// Description returns the tool description.
func (s *SummaryTool) Description() string {
	return "Return a summary dashboard with item counts by inventory type"
}

// Execute queries each inventory itemtype and returns aggregate counts.
func (s *SummaryTool) Execute(ctx context.Context, itemtypes []string) (*SummaryResult, error) {
	if len(itemtypes) == 0 {
		itemtypes = DefaultItemtypes
	}

	counts := make(map[string]int, len(itemtypes))
	mu := sync.Mutex{}
	var firstErr error
	var errMu sync.Mutex

	var wg sync.WaitGroup
	for _, it := range itemtypes {
		wg.Add(1)
		go func(itemtype string) {
			defer wg.Done()

			searchResult, err := (&SearchTool{client: s.client}).Execute(ctx, itemtype, []SearchCriterion{{
				FieldName:  fmt.Sprintf("%s.id", itemtype),
				SearchType: "contains",
				Value:      "",
			}}, []string{}, &SearchRange{Start: 0, End: 0})
			if err != nil {
				errMu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("summary [%s]: %w", itemtype, err)
				}
				errMu.Unlock()
				return
			}

			mu.Lock()
			counts[itemtype] = searchResult.TotalCount
			mu.Unlock()
		}(it)
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	total := 0
	for _, c := range counts {
		total += c
	}

	return &SummaryResult{
		Counts:    counts,
		Total:     total,
		Itemtypes: itemtypes,
	}, nil
}

// Ensure SummaryTool implements the Tool interface.
var _ Tool = (*SummaryTool)(nil)
