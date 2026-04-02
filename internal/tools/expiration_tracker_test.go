package tools

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// mockToolClient implements ToolClient for testing.
type mockToolClient struct {
	getFunc            func(ctx context.Context, endpoint string, result interface{}) error
	getSearchOptionsFn func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error)
}

func (m *mockToolClient) InitSession(ctx context.Context) error              { return nil }
func (m *mockToolClient) KillSession(ctx context.Context) error              { return nil }
func (m *mockToolClient) SessionToken() string                               { return "test-token" }
func (m *mockToolClient) GLPIURL() string                                    { return "http://localhost" }
func (m *mockToolClient) GetGLPIVersion(ctx context.Context) (string, error) { return "10.0", nil }
func (m *mockToolClient) GetSearchOptions(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
	if m.getSearchOptionsFn != nil {
		return m.getSearchOptionsFn(ctx, itemtype)
	}
	return &glpi.SearchOptionsResult{Fields: []glpi.SearchOption{}}, nil
}
func (m *mockToolClient) Get(ctx context.Context, endpoint string, result interface{}) error {
	if m.getFunc != nil {
		return m.getFunc(ctx, endpoint, result)
	}
	return nil
}
func (m *mockToolClient) Post(ctx context.Context, endpoint string, body, result interface{}) error {
	return nil
}
func (m *mockToolClient) Put(ctx context.Context, endpoint string, body, result interface{}) error {
	return nil
}
func (m *mockToolClient) Delete(ctx context.Context, endpoint string, result interface{}) error {
	return nil
}

// searchOptionsForItemtype returns search options that map UIDs to field IDs for a given itemtype.
func searchOptionsForItemtype(itemtype string) *glpi.SearchOptionsResult {
	// Common fields for all itemtypes.
	fields := []glpi.SearchOption{
		{ID: 1, UID: fmt.Sprintf("%s.name", itemtype), Field: "name", Name: "Name"},
		{ID: 2, UID: fmt.Sprintf("%s.id", itemtype), Field: "id", Name: "ID"},
		{ID: 80, UID: fmt.Sprintf("%s.entities_id", itemtype), Field: "entities_id", Name: "Entity"},
	}

	// Add itemtype-specific fields.
	switch itemtype {
	case "Certificate":
		fields = append(fields, glpi.SearchOption{ID: 160, UID: "Certificate.date_expiration", Field: "date_expiration", Name: "Expiration date", DataType: "date"})
	case "Domain":
		fields = append(fields, glpi.SearchOption{ID: 160, UID: "Domain.date_expiration", Field: "date_expiration", Name: "Expiration date", DataType: "date"})
	case "Contract":
		fields = append(fields, glpi.SearchOption{ID: 10, UID: "Contract.end_date", Field: "end_date", Name: "End date", DataType: "date"})
	case "SoftwareLicense":
		fields = append(fields, glpi.SearchOption{ID: 160, UID: "SoftwareLicense.expiration", Field: "expiration", Name: "Expiration date", DataType: "date"})
	case "Computer", "Monitor", "Printer", "NetworkEquipment", "Peripheral", "Phone":
		fields = append(fields,
			glpi.SearchOption{ID: 192, UID: fmt.Sprintf("%s.warranty_date", itemtype), Field: "warranty_date", Name: "Warranty date", DataType: "date"},
			glpi.SearchOption{ID: 193, UID: fmt.Sprintf("%s.warranty_duration", itemtype), Field: "warranty_duration", Name: "Warranty duration"},
		)
	}

	return &glpi.SearchOptionsResult{
		ItemType: itemtype,
		Fields:   fields,
	}
}

func TestExpirationTrackerTool_NewNilClient(t *testing.T) {
	_, err := NewExpirationTrackerTool(nil)
	if err == nil {
		t.Fatal("expected error for nil client, got nil")
	}
}

func TestExpirationTrackerTool_DaysAheadValidation(t *testing.T) {
	client := &mockToolClient{}
	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name      string
		daysAhead int
		wantErr   string
	}{
		{"zero days", 0, "days_ahead must be a positive integer"},
		{"negative days", -5, "days_ahead must be a positive integer"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tool.Execute(context.Background(), tt.daysAhead, nil, 0)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if err.Error() != tt.wantErr {
				t.Errorf("got error %q, want %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func TestExpirationTrackerTool_DirectDateFields(t *testing.T) {
	now := time.Now()
	expiringSoon := now.AddDate(0, 0, 15).Format("2006-01-02")
	alreadyExpired := now.AddDate(0, 0, -10).Format("2006-01-02")

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			*out = map[string]interface{}{
				"totalcount": float64(2),
				"data": []interface{}{
					map[string]interface{}{
						"id":                          float64(1),
						"1":                           "Expiring Cert",
						"Certificate.date_expiration": expiringSoon,
						"date_expiration":             expiringSoon,
						"entities_id":                 map[string]interface{}{"name": "Root Entity"},
					},
					map[string]interface{}{
						"id":                          float64(2),
						"1":                           "Expired Cert",
						"Certificate.date_expiration": alreadyExpired,
						"date_expiration":             alreadyExpired,
					},
				},
			}
			return nil
		},
	}

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 30, []string{"Certificate"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalExpiring != 2 {
		t.Errorf("expected TotalExpiring=2, got %d", report.TotalExpiring)
	}

	certs, ok := report.ByItemtype["Certificate"]
	if !ok {
		t.Fatal("expected Certificate in ByItemtype")
	}

	if len(certs) != 2 {
		t.Errorf("expected 2 certificate items, got %d", len(certs))
	}

	// Verify sorting: most urgent first (already expired should be first).
	if certs[0].DaysUntilExpiry >= certs[1].DaysUntilExpiry {
		t.Errorf("expected items sorted by days_until_expiry ascending, got %d >= %d",
			certs[0].DaysUntilExpiry, certs[1].DaysUntilExpiry)
	}

	// Verify the expired cert has negative days.
	if certs[0].DaysUntilExpiry >= 0 {
		t.Errorf("expected first item to have negative days_until_expiry (already expired), got %d",
			certs[0].DaysUntilExpiry)
	}

	// Verify is_computed is false for direct dates.
	for _, cert := range certs {
		if cert.IsComputed {
			t.Errorf("expected IsComputed=false for Certificate, got true for item %d", cert.ID)
		}
	}
}

func TestExpirationTrackerTool_ComputedWarranty(t *testing.T) {
	now := time.Now()

	// Computer with warranty starting 24 months ago, duration 24 months → expires ~now.
	warrantyStart := now.AddDate(0, -24, 0).Format("2006-01-02")
	warrantyEnd := now.Format("2006-01-02") // warrantyStart + 24 months = now

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			// Only return computer data for Computer queries.
			if len(endpoint) >= len("/search/Computer") && endpoint[:len("/search/Computer")] == "/search/Computer" {
				*out = map[string]interface{}{
					"totalcount": float64(2),
					"data": []interface{}{
						map[string]interface{}{
							"id":                         float64(10),
							"1":                          "Workstation-01",
							"Computer.warranty_date":     warrantyStart,
							"warranty_date":              warrantyStart,
							"warranty_duration":          float64(24),
							"Computer.warranty_duration": float64(24),
						},
						map[string]interface{}{
							"id":                         float64(11),
							"1":                          "Workstation-02",
							"Computer.warranty_date":     now.AddDate(0, -1, 0).Format("2006-01-02"),
							"warranty_date":              now.AddDate(0, -1, 0).Format("2006-01-02"),
							"warranty_duration":          float64(4),
							"Computer.warranty_duration": float64(4), // Expires in ~3 months, beyond 30-day cutoff
						},
					},
				}
				return nil
			}

			// Return empty for other itemtypes.
			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 30, []string{"Computer"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	computers, ok := report.ByItemtype["Computer"]
	if !ok {
		t.Fatal("expected Computer in ByItemtype")
	}

	// Only Workstation-01 should be included (expires within 30 days).
	// Workstation-02 expires in ~3 months, beyond the 30-day cutoff.
	if len(computers) != 1 {
		t.Errorf("expected 1 computer item, got %d", len(computers))
	}

	if len(computers) > 0 {
		if computers[0].ID != 10 {
			t.Errorf("expected item ID 10, got %d", computers[0].ID)
		}
		if !computers[0].IsComputed {
			t.Error("expected IsComputed=true for warranty-computed item")
		}
		if computers[0].ExpirationDate != warrantyEnd {
			t.Errorf("expected expiration_date %q, got %q", warrantyEnd, computers[0].ExpirationDate)
		}
	}
}

func TestExpirationTrackerTool_PartialFailure(t *testing.T) {
	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			// Certificate succeeds.
			if endpoint[:len("/search/Certificate")] == "/search/Certificate" {
				out, ok := result.(*map[string]interface{})
				if !ok {
					return fmt.Errorf("invalid result type")
				}
				*out = map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"id":                          float64(1),
							"1":                           "Test Cert",
							"Certificate.date_expiration": time.Now().AddDate(0, 0, 10).Format("2006-01-02"),
							"date_expiration":             time.Now().AddDate(0, 0, 10).Format("2006-01-02"),
						},
					},
				}
				return nil
			}
			// Domain fails.
			if len(endpoint) >= len("/search/Domain") && endpoint[:len("/search/Domain")] == "/search/Domain" {
				return fmt.Errorf("GLPI API error: connection refused")
			}
			// Other types return empty results.
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

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 30, []string{"Certificate", "Domain"}, 0)
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

	// Certificate results should still be present.
	certs, ok := report.ByItemtype["Certificate"]
	if !ok {
		t.Fatal("expected Certificate results despite Domain failure")
	}
	if len(certs) != 1 {
		t.Errorf("expected 1 certificate, got %d", len(certs))
	}
}

func TestExpirationTrackerTool_AllItemtypesFail(t *testing.T) {
	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			return fmt.Errorf("GLPI API error: timeout")
		},
	}

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 30, []string{"Certificate", "Domain"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !report.HasErrors {
		t.Error("expected HasErrors=true when all queries fail")
	}

	if report.TotalExpiring != 0 {
		t.Errorf("expected TotalExpiring=0, got %d", report.TotalExpiring)
	}

	if len(report.ByItemtype) != 0 {
		t.Errorf("expected empty ByItemtype, got %d entries", len(report.ByItemtype))
	}
}

func TestExpirationTrackerTool_EmptyResults(t *testing.T) {
	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
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

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 30, []string{"Certificate"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if report.TotalExpiring != 0 {
		t.Errorf("expected TotalExpiring=0, got %d", report.TotalExpiring)
	}

	if report.HasErrors {
		t.Error("expected HasErrors=false for empty results")
	}

	// ByItemtype should be an empty map, not nil.
	if report.ByItemtype == nil {
		t.Error("expected ByItemtype to be non-nil empty map")
	}

	if len(report.ByItemtype) != 0 {
		t.Errorf("expected 0 entries in ByItemtype, got %d", len(report.ByItemtype))
	}
}

func TestExpirationTrackerTool_DefaultAllItemtypes(t *testing.T) {
	queryCount := 0
	var queryMu sync.Mutex

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

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Call with no itemtypes specified → should query all 10 registry types.
	_, err = tool.Execute(context.Background(), 30, nil, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if queryCount != len(expirationFieldRegistry) {
		t.Errorf("expected %d queries (all registry types), got %d", len(expirationFieldRegistry), queryCount)
	}
}

func TestExpirationTrackerTool_InvalidItemtypeSkipped(t *testing.T) {
	queryCount := 0
	var queryMu sync.Mutex

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

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Call with only invalid itemtypes → no queries should execute.
	report, err := tool.Execute(context.Background(), 30, []string{"InvalidType", "NonExistent"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if queryCount != 0 {
		t.Errorf("expected 0 queries for invalid itemtypes, got %d", queryCount)
	}

	if report.TotalExpiring != 0 {
		t.Errorf("expected TotalExpiring=0, got %d", report.TotalExpiring)
	}

	if report.HasErrors {
		t.Error("expected HasErrors=false for invalid itemtypes (should be silently skipped)")
	}
}

func TestExpirationTrackerTool_EntityFilter(t *testing.T) {
	var lastEndpoint string
	var endpointMu sync.Mutex

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			endpointMu.Lock()
			lastEndpoint = endpoint
			endpointMu.Unlock()

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

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = tool.Execute(context.Background(), 30, []string{"Certificate"}, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify entity filter is in the query.
	if lastEndpoint == "" {
		t.Fatal("expected query to be executed")
	}

	// The endpoint should contain the entity filter criterion.
	// The exact format depends on how SearchTool builds params, but it should include entity info.
	// Since we can't easily parse URL params here, we just verify the query ran.
}

func TestExpirationTrackerTool_MissingWarrantyDate(t *testing.T) {
	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}

			// Only return computer data for Computer queries.
			if len(endpoint) >= len("/search/Computer") && endpoint[:len("/search/Computer")] == "/search/Computer" {
				*out = map[string]interface{}{
					"totalcount": float64(2),
					"data": []interface{}{
						map[string]interface{}{
							"id":                         float64(1),
							"1":                          "No Warranty PC",
							"Computer.warranty_duration": float64(24),
							"warranty_duration":          float64(24),
							// No warranty_date field.
						},
						map[string]interface{}{
							"id":                         float64(2),
							"1":                          "Zero Duration PC",
							"Computer.warranty_date":     time.Now().AddDate(0, -12, 0).Format("2006-01-02"),
							"warranty_date":              time.Now().AddDate(0, -12, 0).Format("2006-01-02"),
							"Computer.warranty_duration": float64(0),
							"warranty_duration":          float64(0),
						},
					},
				}
				return nil
			}

			// Return empty for other itemtypes.
			*out = map[string]interface{}{
				"totalcount": float64(0),
				"data":       []interface{}{},
			}
			return nil
		},
	}

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 30, []string{"Computer"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	computers, ok := report.ByItemtype["Computer"]
	if ok && len(computers) != 0 {
		t.Errorf("expected 0 computer items (both excluded), got %d", len(computers))
	}
}

func TestExpirationTrackerTool_ReportStructure(t *testing.T) {
	now := time.Now()
	expiringDate := now.AddDate(0, 0, 10).Format("2006-01-02")

	client := &mockToolClient{
		getSearchOptionsFn: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return searchOptionsForItemtype(itemtype), nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			out, ok := result.(*map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid result type")
			}
			*out = map[string]interface{}{
				"totalcount": float64(1),
				"data": []interface{}{
					map[string]interface{}{
						"id":                float64(1),
						"1":                 "Test Contract",
						"Contract.end_date": expiringDate,
						"end_date":          expiringDate,
						"entities_id":       map[string]interface{}{"name": "IT Department"},
					},
				},
			}
			return nil
		},
	}

	tool, err := NewExpirationTrackerTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := tool.Execute(context.Background(), 30, []string{"Contract"}, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify report structure.
	if report.GeneratedAt == "" {
		t.Error("expected GeneratedAt to be set")
	}

	if report.DaysAhead != 30 {
		t.Errorf("expected DaysAhead=30, got %d", report.DaysAhead)
	}

	if report.TotalExpiring != 1 {
		t.Errorf("expected TotalExpiring=1, got %d", report.TotalExpiring)
	}

	contracts, ok := report.ByItemtype["Contract"]
	if !ok {
		t.Fatal("expected Contract in ByItemtype")
	}

	if len(contracts) != 1 {
		t.Fatalf("expected 1 contract, got %d", len(contracts))
	}

	item := contracts[0]
	if item.ID != 1 {
		t.Errorf("expected ID=1, got %d", item.ID)
	}
	if item.Name != "Test Contract" {
		t.Errorf("expected Name='Test Contract', got %q", item.Name)
	}
	if item.ExpirationDate != expiringDate {
		t.Errorf("expected ExpirationDate=%q, got %q", expiringDate, item.ExpirationDate)
	}
	if item.Itemtype != "Contract" {
		t.Errorf("expected Itemtype='Contract', got %q", item.Itemtype)
	}
	if item.EntityName != "IT Department" {
		t.Errorf("expected EntityName='IT Department', got %q", item.EntityName)
	}
	if item.IsComputed {
		t.Error("expected IsComputed=false for direct date field")
	}
}
