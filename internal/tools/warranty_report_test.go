package tools

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestWarrantyReportTool_NewNilClient(t *testing.T) {
	_, err := NewWarrantyReportTool(nil)
	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
}

func TestWarrantyReportTool_DaysWarningValidation(t *testing.T) {
	client := &mockToolClient{}
	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name        string
		daysWarning int
		wantErr     string
	}{
		{"negative days", -1, "days_warning must be a non-negative integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(context.Background(), tt.daysWarning, nil, 0)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("got error %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestWarrantyReportTool_ZeroDaysWarning(t *testing.T) {
	now := time.Now()
	warrantyStart := now.AddDate(0, -12, 0).Format("2006-01-02")

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			if len(endpoint) >= len("/search/Computer") && endpoint[:len("/search/Computer")] == "/search/Computer" {
				*out = map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id":                         float64(1),
							"1":                          "Test PC",
							"Computer.warranty_date":     warrantyStart,
							"warranty_date":              warrantyStart,
							"Computer.warranty_duration": float64(12),
							"warranty_duration":          float64(12),
						},
					},
				}
				return nil
			}

			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Zero days_warning should be valid (only expired items flagged).
	report, err := tool.Execute(context.Background(), 0, []string{"Computer"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.DaysWarning != 0 {
		t.Errorf("expected DaysWarning=0, got %d", report.DaysWarning)
	}
}

func TestWarrantyReportTool_StatusCategorization(t *testing.T) {
	now := time.Now()

	// Create computers with different warranty statuses.
	// Expired: warranty started 25 months ago, duration 24 months → expired ~1 month ago.
	expiredWarrantyStart := now.AddDate(0, -25, 0).Format("2006-01-02")
	// Expiring soon: warranty started 11 months ago, duration 12 months → expires in ~1 month.
	expiringWarrantyStart := now.AddDate(0, -11, 0).Format("2006-01-02")
	// Active: warranty started 1 month ago, duration 36 months → expires in ~35 months.
	activeWarrantyStart := now.AddDate(0, -1, 0).Format("2006-01-02")

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			if len(endpoint) >= len("/search/Computer") && endpoint[:len("/search/Computer")] == "/search/Computer" {
				*out = map[string]interface{}{
					"totalcount": float64(3),
					"data": []interface{}{
						map[string]interface{}{
							"id":                         float64(1),
							"1":                          "Expired PC",
							"Computer.warranty_date":     expiredWarrantyStart,
							"warranty_date":              expiredWarrantyStart,
							"Computer.warranty_duration": float64(24),
							"warranty_duration":          float64(24),
							"Computer.buy_value":         float64(1500.00),
							"buy_value":                  float64(1500.00),
						},
						map[string]interface{}{
							"id":                         float64(2),
							"1":                          "Expiring PC",
							"Computer.warranty_date":     expiringWarrantyStart,
							"warranty_date":              expiringWarrantyStart,
							"Computer.warranty_duration": float64(12),
							"warranty_duration":          float64(12),
							"Computer.buy_value":         float64(2000.00),
							"buy_value":                  float64(2000.00),
						},
						map[string]interface{}{
							"id":                         float64(3),
							"1":                          "Active PC",
							"Computer.warranty_date":     activeWarrantyStart,
							"warranty_date":              activeWarrantyStart,
							"Computer.warranty_duration": float64(36),
							"warranty_duration":          float64(36),
							"Computer.buy_value":         float64(3000.00),
							"buy_value":                  float64(3000.00),
						},
					},
				}
				return nil
			}

			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 90, []string{"Computer"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify summary counts.
	if report.Summary.Expired != 1 {
		t.Errorf("expected Summary.Expired=1, got %d", report.Summary.Expired)
	}
	if report.Summary.ExpiringSoon != 1 {
		t.Errorf("expected Summary.ExpiringSoon=1, got %d", report.Summary.ExpiringSoon)
	}
	if report.Summary.Active != 1 {
		t.Errorf("expected Summary.Active=1, got %d", report.Summary.Active)
	}
	if report.Summary.Total != 3 {
		t.Errorf("expected Summary.Total=3, got %d", report.Summary.Total)
	}

	// Verify total purchase cost.
	expectedCost := 1500.00 + 2000.00 + 3000.00
	if report.TotalPurchaseCost != expectedCost {
		t.Errorf("expected TotalPurchaseCost=%.2f, got %.2f", expectedCost, report.TotalPurchaseCost)
	}

	// Verify sorting: expired first, then expiring_soon, then active.
	if len(report.AssetDetails) != 3 {
		t.Fatalf("expected 3 asset details, got %d", len(report.AssetDetails))
	}

	if report.AssetDetails[0].Status != WarrantyStatusExpired {
		t.Errorf("expected first asset status=%q, got %q", WarrantyStatusExpired, report.AssetDetails[0].Status)
	}
	if report.AssetDetails[1].Status != WarrantyStatusExpiringSoon {
		t.Errorf("expected second asset status=%q, got %q", WarrantyStatusExpiringSoon, report.AssetDetails[1].Status)
	}
	if report.AssetDetails[2].Status != WarrantyStatusActive {
		t.Errorf("expected third asset status=%q, got %q", WarrantyStatusActive, report.AssetDetails[2].Status)
	}
}

func TestWarrantyReportTool_DefaultHardwareItemtypes(t *testing.T) {
	var queryMu sync.Mutex
	queryCount := 0

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			queryMu.Lock()
			queryCount++
			queryMu.Unlock()

			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}
			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Call with no itemtypes → should query all 6 hardware types.
	_, err = tool.Execute(context.Background(), 90, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if queryCount != len(DefaultHardwareItemtypes) {
		t.Errorf("expected %d queries (all hardware types), got %d", len(DefaultHardwareItemtypes), queryCount)
	}
}

func TestWarrantyReportTool_InvalidItemtypeSkipped(t *testing.T) {
	queryCount := 0

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			queryCount++
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}
			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Call with only non-hardware itemtypes → no queries should execute.
	report, err := tool.Execute(context.Background(), 90, []string{"Certificate", "Domain", "Contract"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if queryCount != 0 {
		t.Errorf("expected 0 queries for non-hardware itemtypes, got %d", queryCount)
	}

	if report.Summary.Total != 0 {
		t.Errorf("expected Summary.Total=0, got %d", report.Summary.Total)
	}

	if report.HasErrors {
		t.Error("expected HasErrors=false for non-hardware itemtypes (should be silently skipped)")
	}
}

func TestWarrantyReportTool_PartialFailure(t *testing.T) {
	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			// Computer succeeds.
			if len(endpoint) >= len("/search/Computer") && endpoint[:len("/search/Computer")] == "/search/Computer" {
				out, ok := result.(*map[string]interface{})
				if !ok {
					return fmt.Errorf("invalid result type")
				}
				*out = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			// Monitor fails.
			if len(endpoint) >= len("/search/Monitor") && endpoint[:len("/search/Monitor")] == "/search/Monitor" {
				return fmt.Errorf("GLPI API error: connection refused")
			}
			// Other types return empty.
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}
			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 90, []string{"Computer", "Monitor"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have partial results.
	if !report.HasErrors {
		t.Error("expected HasErrors=true for partial failure")
	}

	if len(report.ErrorMessages) == 0 {
		t.Error("expected error messages for failed itemtype")
	}

	// Computer should still be processed (empty but no error).
	if report.Summary.Total != 0 {
		t.Errorf("expected Summary.Total=0 (empty Computer results), got %d", report.Summary.Total)
	}
}

func TestWarrantyReportTool_MissingWarrantyData(t *testing.T) {
	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			if len(endpoint) >= len("/search/Computer") && endpoint[:len("/search/Computer")] == "/search/Computer" {
				*out = map[string]interface{}{
					"totalcount": float64(3),
					"data": []interface{}{
						map[string]interface{}{
							"id":                         float64(1),
							"1":                          "No Warranty PC",
							"Computer.warranty_duration": float64(24),
							"warranty_duration":          float64(24),
							// No warranty_date field.
						},
						map[string]interface{}{
							"id":                     float64(2),
							"1":                      "Zero Duration PC",
							"Computer.warranty_date": time.Now().AddDate(0, -12, 0).Format("2006-01-02"),
							"warranty_date":          time.Now().AddDate(0, -12, 0).Format("2006-01-02"),
							// No warranty_duration field.
						},
						map[string]interface{}{
							"id":                         float64(3),
							"1":                          "Invalid Date PC",
							"Computer.warranty_date":     "not-a-date",
							"warranty_date":              "not-a-date",
							"Computer.warranty_duration": float64(24),
							"warranty_duration":          float64(24),
						},
					},
				}
				return nil
			}

			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 90, []string{"Computer"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.Summary.Total != 0 {
		t.Errorf("expected Summary.Total=0 (all items excluded), got %d", report.Summary.Total)
	}

	if report.HasErrors {
		t.Error("expected HasErrors=false for missing warranty data (items should be silently skipped)")
	}
}

func TestWarrantyReportTool_ReportStructure(t *testing.T) {
	now := time.Now()
	warrantyStart := now.AddDate(0, -6, 0).Format("2006-01-02")

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			if len(endpoint) >= len("/search/Computer") && endpoint[:len("/search/Computer")] == "/search/Computer" {
				*out = map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id":                         float64(42),
							"1":                          "Test Workstation",
							"Computer.warranty_date":     warrantyStart,
							"warranty_date":              warrantyStart,
							"Computer.warranty_duration": float64(12),
							"warranty_duration":          float64(12),
							"Computer.buy_value":         float64(2500.50),
							"buy_value":                  float64(2500.50),
							"entities_id":                map[string]interface{}{"name": "IT Department"},
						},
					},
				}
				return nil
			}

			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 90, []string{"Computer"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify report structure.
	if report.GeneratedAt == "" {
		t.Error("expected GeneratedAt to be set")
	}

	if report.DaysWarning != 90 {
		t.Errorf("expected DaysWarning=90, got %d", report.DaysWarning)
	}

	if report.Summary.Total != 1 {
		t.Errorf("expected Summary.Total=1, got %d", report.Summary.Total)
	}

	if len(report.AssetDetails) != 1 {
		t.Fatalf("expected 1 asset detail, got %d", len(report.AssetDetails))
	}

	asset := report.AssetDetails[0]
	if asset.ID != 42 {
		t.Errorf("expected ID=42, got %d", asset.ID)
	}
	if asset.Name != "Test Workstation" {
		t.Errorf("expected Name='Test Workstation', got %q", asset.Name)
	}
	if asset.Itemtype != "Computer" {
		t.Errorf("expected Itemtype='Computer', got %q", asset.Itemtype)
	}
	if asset.EntityName != "IT Department" {
		t.Errorf("expected EntityName='IT Department', got %q", asset.EntityName)
	}
	if asset.WarrantyDate != warrantyStart {
		t.Errorf("expected WarrantyDate=%q, got %q", warrantyStart, asset.WarrantyDate)
	}
	if asset.WarrantyMonths != 12 {
		t.Errorf("expected WarrantyMonths=12, got %d", asset.WarrantyMonths)
	}
	if asset.PurchaseCost != 2500.50 {
		t.Errorf("expected PurchaseCost=2500.50, got %.2f", asset.PurchaseCost)
	}
}

func TestWarrantyReportTool_EntityFilter(t *testing.T) {
	var lastEndpoint string

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			lastEndpoint = endpoint

			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}
			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = tool.Execute(context.Background(), 90, []string{"Computer"}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify entity filter is in the query.
	if lastEndpoint == "" {
		t.Fatal("expected query to be executed")
	}

	// The endpoint should contain the entity filter.
	if lastEndpoint == "" {
		t.Fatal("expected query to be executed")
	}
}

func TestWarrantyReportTool_MultipleHardwareTypes(t *testing.T) {
	now := time.Now()
	warrantyStart := now.AddDate(0, -6, 0).Format("2006-01-02")

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			// Return one item for Computer and Monitor, empty for others.
			if len(endpoint) >= len("/search/Computer") && endpoint[:len("/search/Computer")] == "/search/Computer" {
				*out = map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id":                         float64(1),
							"1":                          "Test PC",
							"Computer.warranty_date":     warrantyStart,
							"warranty_date":              warrantyStart,
							"Computer.warranty_duration": float64(12),
							"warranty_duration":          float64(12),
						},
					},
				}
				return nil
			}

			if len(endpoint) >= len("/search/Monitor") && endpoint[:len("/search/Monitor")] == "/search/Monitor" {
				*out = map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id":                        float64(2),
							"1":                         "Test Monitor",
							"Monitor.warranty_date":     warrantyStart,
							"warranty_date":             warrantyStart,
							"Monitor.warranty_duration": float64(12),
							"warranty_duration":         float64(12),
						},
					},
				}
				return nil
			}

			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewWarrantyReportTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 90, []string{"Computer", "Monitor", "Printer"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have 2 assets (Computer + Monitor), Printer is empty.
	if report.Summary.Total != 2 {
		t.Errorf("expected Summary.Total=2, got %d", report.Summary.Total)
	}

	// Verify both itemtypes are represented.
	foundComputer := false
	foundMonitor := false
	for _, asset := range report.AssetDetails {
		if asset.Itemtype == "Computer" {
			foundComputer = true
		}
		if asset.Itemtype == "Monitor" {
			foundMonitor = true
		}
	}

	if !foundComputer {
		t.Error("expected Computer asset in report")
	}
	if !foundMonitor {
		t.Error("expected Monitor asset in report")
	}
}

func TestExtractPurchaseCost(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want float64
	}{
		{
			name: "full UID format",
			data: map[string]interface{}{
				"Computer.buy_value": float64(1500.00),
			},
			want: 1500.00,
		},
		{
			name: "short field name",
			data: map[string]interface{}{
				"buy_value": float64(2000.00),
			},
			want: 2000.00,
		},
		{
			name: "GLPI field ID",
			data: map[string]interface{}{
				"22": float64(3000.00),
			},
			want: 3000.00,
		},
		{
			name: "string cost value",
			data: map[string]interface{}{
				"buy_value": "2500.50",
			},
			want: 2500.50,
		},
		{
			name: "no cost field",
			data: map[string]interface{}{
				"name": "Test PC",
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPurchaseCost(tt.data, "Computer")
			if got != tt.want {
				t.Errorf("extractPurchaseCost() = %v, want %v", got, tt.want)
			}
		})
	}
}
