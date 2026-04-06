package tools

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DashboardResult represents the consolidated dashboard output.
type DashboardResult struct {
	// Inventory counts
	InventoryCounts map[string]int `json:"inventory_counts"`
	InventoryTotal  int            `json:"inventory_total"`
	// Expiring items (next 30 days)
	ExpiringItems int `json:"expiring_items"`
	// Active sessions/tickets count (if available)
	TicketsCount int `json:"tickets_count,omitempty"`
	// License compliance status
	LicenseCompliance struct {
		Compliant    int `json:"compliant"`
		NonCompliant int `json:"non_compliant"`
	} `json:"license_compliance,omitempty"`
	// Last updated timestamp
	UpdatedAt time.Time `json:"updated_at"`
	// Errors encountered during collection (non-fatal)
	Errors []string `json:"errors,omitempty"`
}

// DashboardTool provides a consolidated metrics dashboard.
type DashboardTool struct {
	client ToolClient
}

// NewDashboardTool creates a new dashboard tool.
func NewDashboardTool(client ToolClient) (*DashboardTool, error) {
	if client == nil {
		return nil, fmt.Errorf("dashboard tool: client cannot be nil")
	}
	return &DashboardTool{client: client}, nil
}

// Name returns the tool name for registration.
func (d *DashboardTool) Name() string {
	return "glpi_dashboard"
}

// Description returns the tool description.
func (d *DashboardTool) Description() string {
	return "Return a consolidated dashboard with inventory counts, expiring items, and license compliance"
}

// Execute returns a consolidated dashboard with multiple metrics.
func (d *DashboardTool) Execute(ctx context.Context) (*DashboardResult, error) {
	result := &DashboardResult{
		InventoryCounts: make(map[string]int),
		UpdatedAt:       time.Now().UTC(),
		Errors:          []string{},
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// 1. Get inventory counts (same as summary)
	wg.Add(1)
	go func() {
		defer wg.Done()
		summary, err := (&SummaryTool{client: d.client}).Execute(ctx, DefaultItemtypes)
		if err != nil {
			mu.Lock()
			result.Errors = append(result.Errors, fmt.Sprintf("inventory: %v", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		result.InventoryCounts = summary.Counts
		result.InventoryTotal = summary.Total
		mu.Unlock()
	}()

	// 2. Get expiring items (next 30 days)
	wg.Add(1)
	go func() {
		defer wg.Done()
		tracker, err := (&ExpirationTrackerTool{client: d.client}).Execute(ctx, 30, nil, 0)
		if err != nil {
			mu.Lock()
			result.Errors = append(result.Errors, fmt.Sprintf("expirations: %v", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		result.ExpiringItems = tracker.TotalExpiring
		mu.Unlock()
	}()

	// 3. Get tickets count (if Ticket itemtype exists)
	wg.Add(1)
	go func() {
		defer wg.Done()
		searchResult, err := (&SearchTool{client: d.client}).Execute(ctx, "Ticket", []SearchCriterion{{
			FieldName:  "Ticket.id",
			SearchType: "contains",
			Value:      "",
		}}, []string{}, &SearchRange{Start: 0, End: 0})
		if err != nil {
			// Non-fatal - Ticket might not exist or be accessible
			mu.Lock()
			result.Errors = append(result.Errors, fmt.Sprintf("tickets: %v", err))
			mu.Unlock()
			return
		}
		mu.Lock()
		result.TicketsCount = searchResult.TotalCount
		mu.Unlock()
	}()

	wg.Wait()

	return result, nil
}

// Ensure DashboardTool implements the Tool interface.
var _ Tool = (*DashboardTool)(nil)
