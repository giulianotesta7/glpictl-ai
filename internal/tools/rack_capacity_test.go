package tools

import (
	"context"
	"fmt"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// mockRackCapacityClient implements ToolClient for rack capacity tests.
type mockRackCapacityClient struct {
	getFunc       func(ctx context.Context, endpoint string, result interface{}) error
	searchOptions func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error)
}

func (m *mockRackCapacityClient) InitSession(ctx context.Context) error { return nil }
func (m *mockRackCapacityClient) KillSession(ctx context.Context) error { return nil }
func (m *mockRackCapacityClient) SessionToken() string                  { return "test-token" }
func (m *mockRackCapacityClient) GLPIURL() string                       { return "http://test" }
func (m *mockRackCapacityClient) GetGLPIVersion(ctx context.Context) (string, error) {
	return "10.0.0", nil
}
func (m *mockRackCapacityClient) GetSearchOptions(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
	if m.searchOptions != nil {
		return m.searchOptions(ctx, itemtype)
	}
	return &glpi.SearchOptionsResult{ItemType: itemtype, Fields: []glpi.SearchOption{}}, nil
}
func (m *mockRackCapacityClient) Get(ctx context.Context, endpoint string, result interface{}) error {
	if m.getFunc != nil {
		return m.getFunc(ctx, endpoint, result)
	}
	return nil
}
func (m *mockRackCapacityClient) Post(ctx context.Context, endpoint string, body, result interface{}) error {
	return nil
}
func (m *mockRackCapacityClient) Put(ctx context.Context, endpoint string, body, result interface{}) error {
	return nil
}
func (m *mockRackCapacityClient) Delete(ctx context.Context, endpoint string, result interface{}) error {
	return nil
}

func TestNewRackCapacityTool_NilClient(t *testing.T) {
	_, err := NewRackCapacityTool(nil)
	if err == nil {
		t.Error("expected error for nil client, got nil")
	}
}

func TestNewRackCapacityTool_ValidClient(t *testing.T) {
	client := &mockRackCapacityClient{}
	tool, err := NewRackCapacityTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tool == nil {
		t.Fatal("expected tool, got nil")
	}
}

func TestRackCapacityTool_Name(t *testing.T) {
	client := &mockRackCapacityClient{}
	tool, _ := NewRackCapacityTool(client)
	if got := tool.Name(); got != "glpi_rack_capacity" {
		t.Errorf("Name() = %q, want %q", got, "glpi_rack_capacity")
	}
}

func TestRackCapacityTool_Description(t *testing.T) {
	client := &mockRackCapacityClient{}
	tool, _ := NewRackCapacityTool(client)
	desc := tool.Description()
	if desc == "" {
		t.Error("Description() returned empty string")
	}
}

func TestRackCapacityTool_GetInput(t *testing.T) {
	client := &mockRackCapacityClient{}
	tool, _ := NewRackCapacityTool(client)
	input := tool.GetInput()
	if input == nil {
		t.Error("GetInput() returned nil")
	}
}

func TestRackCapacityTool_Execute_SingleRackNotFound(t *testing.T) {
	client := &mockRackCapacityClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			// Simulate rack not found — return empty map
			if m, ok := result.(*map[string]interface{}); ok {
				*m = map[string]interface{}{}
			}
			return nil
		},
	}
	tool, _ := NewRackCapacityTool(client)

	_, err := tool.Execute(context.Background(), 999, false)
	if err == nil {
		t.Error("expected error for nonexistent rack, got nil")
	}
}

func TestRackCapacityTool_Execute_SingleRackEmpty(t *testing.T) {
	client := &mockRackCapacityClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if m, ok := result.(*map[string]interface{}); ok {
				*m = map[string]interface{}{
					"id":     float64(1),
					"name":   "Test-Rack",
					"height": float64(42),
				}
			}
			return nil
		},
		searchOptions: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			// Return empty search options so search returns no results
			return &glpi.SearchOptionsResult{ItemType: itemtype, Fields: []glpi.SearchOption{}}, nil
		},
	}
	tool, _ := NewRackCapacityTool(client)

	result, err := tool.Execute(context.Background(), 1, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RackCount != 1 {
		t.Errorf("RackCount = %d, want 1", result.RackCount)
	}
	if result.Racks[0].RackID != 1 {
		t.Errorf("Racks[0].RackID = %d, want 1", result.Racks[0].RackID)
	}
	if result.Racks[0].RackName != "Test-Rack" {
		t.Errorf("Racks[0].RackName = %q, want %q", result.Racks[0].RackName, "Test-Rack")
	}
	if result.Racks[0].TotalU != 42 {
		t.Errorf("Racks[0].TotalU = %d, want 42", result.Racks[0].TotalU)
	}
	if result.Racks[0].UsedU != 0 {
		t.Errorf("Racks[0].UsedU = %d, want 0", result.Racks[0].UsedU)
	}
	if result.Racks[0].AvailableU != 42 {
		t.Errorf("Racks[0].AvailableU = %d, want 42", result.Racks[0].AvailableU)
	}
	if result.Racks[0].UtilizationPct != 0 {
		t.Errorf("Racks[0].UtilizationPct = %f, want 0", result.Racks[0].UtilizationPct)
	}
}

func TestRackCapacityTool_Execute_AllRacksEmpty(t *testing.T) {
	client := &mockRackCapacityClient{
		searchOptions: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			// Return search options that include the "Rack.id" field so field name resolution works
			fields := []glpi.SearchOption{
				{ID: 1, UID: "Rack.id", Name: "id", Field: "id", DisplayName: "ID"},
				{ID: 3, Name: "name", Field: "name", DisplayName: "Name"},
				{ID: 5, Name: "height", Field: "height", DisplayName: "Height"},
			}
			return &glpi.SearchOptionsResult{ItemType: itemtype, Fields: fields}, nil
		},
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			// Return empty search results
			if m, ok := result.(*map[string]interface{}); ok {
				*m = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
			}
			return nil
		},
	}
	tool, _ := NewRackCapacityTool(client)

	result, err := tool.Execute(context.Background(), 0, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RackCount != 0 {
		t.Errorf("RackCount = %d, want 0", result.RackCount)
	}
	if result.TotalRackU != 0 {
		t.Errorf("TotalRackU = %d, want 0", result.TotalRackU)
	}
	if result.OverallUtilizationPct != 0 {
		t.Errorf("OverallUtilizationPct = %f, want 0", result.OverallUtilizationPct)
	}
}

func TestPercentage(t *testing.T) {
	tests := []struct {
		name  string
		part  int
		total int
		want  float64
	}{
		{"zero total", 10, 0, 0},
		{"zero part", 0, 100, 0},
		{"half", 50, 100, 50.0},
		{"full", 100, 100, 100.0},
		{"third", 33, 100, 33.0},
		{"over", 120, 100, 120.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := percentage(tt.part, tt.total)
			if got != tt.want {
				t.Errorf("percentage(%d, %d) = %f, want %f", tt.part, tt.total, got, tt.want)
			}
		})
	}
}

func TestRoundToOneDecimal(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  float64
	}{
		{"zero", 0, 0},
		{"exact one decimal", 71.4, 71.4},
		{"round up", 71.45, 71.5},
		{"round down", 71.44, 71.4},
		{"hundred", 100.0, 100.0},
		{"third", 33.333333, 33.3},
		{"two-thirds", 66.666666, 66.7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := roundToOneDecimal(tt.input)
			if got != tt.want {
				t.Errorf("roundToOneDecimal(%f) = %f, want %f", tt.input, got, tt.want)
			}
		})
	}
}

func TestMapOrientation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"code 0", "0", "front"},
		{"code 1", "1", "rear"},
		{"code 2", "2", "middle"},
		{"already front", "front", "front"},
		{"already rear", "rear", "rear"},
		{"already middle", "middle", "middle"},
		{"empty", "", ""},
		{"unknown", "unknown", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapOrientation(tt.input)
			if got != tt.want {
				t.Errorf("mapOrientation(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExtractInt(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		key  string
		want int
	}{
		{"float64", map[string]interface{}{"key": float64(42)}, "key", 42},
		{"int", map[string]interface{}{"key": 42}, "key", 42},
		{"string", map[string]interface{}{"key": "42"}, "key", 42},
		{"missing", map[string]interface{}{}, "key", 0},
		{"nil value", map[string]interface{}{"key": nil}, "key", 0},
		{"non-numeric string", map[string]interface{}{"key": "abc"}, "key", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractInt(tt.data, tt.key)
			if got != tt.want {
				t.Errorf("extractInt(%v, %q) = %d, want %d", tt.data, tt.key, got, tt.want)
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		key  string
		want string
	}{
		{"string", map[string]interface{}{"key": "value"}, "key", "value"},
		{"missing", map[string]interface{}{}, "key", ""},
		{"nil value", map[string]interface{}{"key": nil}, "key", "<nil>"},
		{"number to string", map[string]interface{}{"key": float64(42)}, "key", "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractString(tt.data, tt.key)
			if got != tt.want {
				t.Errorf("extractString(%v, %q) = %q, want %q", tt.data, tt.key, got, tt.want)
			}
		})
	}
}

func TestRackCapacityTool_ImplementsTool(t *testing.T) {
	client := &mockRackCapacityClient{}
	tool, err := NewRackCapacityTool(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var _ Tool = tool
}

// Test that the tool interface compile-time check is satisfied.
func TestRackCapacityTool_InterfaceCheck(t *testing.T) {
	// This is a compile-time check; if it compiles, the test passes.
	var _ Tool = (*RackCapacityTool)(nil)
}

// TestRackCapacityTool_Execute_SingleRackZeroHeight tests the division-by-zero guard.
func TestRackCapacityTool_Execute_SingleRackZeroHeight(t *testing.T) {
	client := &mockRackCapacityClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if m, ok := result.(*map[string]interface{}); ok {
				*m = map[string]interface{}{
					"id":     float64(5),
					"name":   "Zero-Rack",
					"height": float64(0),
				}
			}
			return nil
		},
		searchOptions: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{ItemType: itemtype, Fields: []glpi.SearchOption{}}, nil
		},
	}
	tool, _ := NewRackCapacityTool(client)

	result, err := tool.Execute(context.Background(), 5, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RackCount != 1 {
		t.Errorf("RackCount = %d, want 1", result.RackCount)
	}
	if result.Racks[0].TotalU != 0 {
		t.Errorf("TotalU = %d, want 0", result.Racks[0].TotalU)
	}
	if result.Racks[0].UtilizationPct != 0 {
		t.Errorf("UtilizationPct = %f, want 0", result.Racks[0].UtilizationPct)
	}
}

// TestRackCapacityInput_StructFields verifies the input struct has expected fields.
func TestRackCapacityInput_StructFields(t *testing.T) {
	input := RackCapacityInput{
		RackID:          42,
		IncludeUnplaced: true,
	}
	if input.RackID != 42 {
		t.Errorf("RackID = %d, want 42", input.RackID)
	}
	if !input.IncludeUnplaced {
		t.Error("IncludeUnplaced = false, want true")
	}
}

// TestRackCapacityReport_StructFields verifies the report struct has expected fields.
func TestRackCapacityReport_StructFields(t *testing.T) {
	report := RackCapacityReport{
		RackCount:             3,
		TotalRackU:            126,
		TotalUsedU:            80,
		TotalAvailableU:       46,
		OverallUtilizationPct: 63.5,
		Racks: []RackCapacity{
			{
				RackID:         1,
				RackName:       "Rack-A1",
				TotalU:         42,
				UsedU:          30,
				AvailableU:     12,
				UtilizationPct: 71.4,
				Equipment: []RackEquipment{
					{ID: 10, Name: "Switch-01", ItemType: "NetworkEquipment", Position: 1, Orientation: "front"},
				},
			},
		},
		UnplacedEquipment: []UnplacedItem{
			{ID: 55, Name: "AP-Floor2", ItemType: "NetworkEquipment"},
		},
	}

	if report.RackCount != 3 {
		t.Errorf("RackCount = %d, want 3", report.RackCount)
	}
	if report.TotalRackU != 126 {
		t.Errorf("TotalRackU = %d, want 126", report.TotalRackU)
	}
	if report.TotalUsedU != 80 {
		t.Errorf("TotalUsedU = %d, want 80", report.TotalUsedU)
	}
	if report.TotalAvailableU != 46 {
		t.Errorf("TotalAvailableU = %d, want 46", report.TotalAvailableU)
	}
	if report.OverallUtilizationPct != 63.5 {
		t.Errorf("OverallUtilizationPct = %f, want 63.5", report.OverallUtilizationPct)
	}
	if len(report.Racks) != 1 {
		t.Errorf("len(Racks) = %d, want 1", len(report.Racks))
	}
	if report.Racks[0].RackName != "Rack-A1" {
		t.Errorf("Racks[0].RackName = %q, want %q", report.Racks[0].RackName, "Rack-A1")
	}
	if len(report.Racks[0].Equipment) != 1 {
		t.Errorf("len(Equipment) = %d, want 1", len(report.Racks[0].Equipment))
	}
	if len(report.UnplacedEquipment) != 1 {
		t.Errorf("len(UnplacedEquipment) = %d, want 1", len(report.UnplacedEquipment))
	}
}

// TestRackEquipment_StructFields verifies the equipment struct.
func TestRackEquipment_StructFields(t *testing.T) {
	eq := RackEquipment{
		ID:          10,
		Name:        "Core-Switch-01",
		ItemType:    "NetworkEquipment",
		Position:    5,
		Orientation: "front",
	}
	if eq.ID != 10 {
		t.Errorf("ID = %d, want 10", eq.ID)
	}
	if eq.Name != "Core-Switch-01" {
		t.Errorf("Name = %q, want %q", eq.Name, "Core-Switch-01")
	}
	if eq.ItemType != "NetworkEquipment" {
		t.Errorf("ItemType = %q, want %q", eq.ItemType, "NetworkEquipment")
	}
	if eq.Position != 5 {
		t.Errorf("Position = %d, want 5", eq.Position)
	}
	if eq.Orientation != "front" {
		t.Errorf("Orientation = %q, want %q", eq.Orientation, "front")
	}
}

// TestUnplacedItem_StructFields verifies the unplaced item struct.
func TestUnplacedItem_StructFields(t *testing.T) {
	item := UnplacedItem{
		ID:       55,
		Name:     "AP-Floor2",
		ItemType: "NetworkEquipment",
	}
	if item.ID != 55 {
		t.Errorf("ID = %d, want 55", item.ID)
	}
	if item.Name != "AP-Floor2" {
		t.Errorf("Name = %q, want %q", item.Name, "AP-Floor2")
	}
	if item.ItemType != "NetworkEquipment" {
		t.Errorf("ItemType = %q, want %q", item.ItemType, "NetworkEquipment")
	}
}

// TestRackCapacity_StructFields verifies the rack capacity struct.
func TestRackCapacity_StructFields(t *testing.T) {
	rc := RackCapacity{
		RackID:         1,
		RackName:       "Rack-A1",
		TotalU:         42,
		UsedU:          30,
		AvailableU:     12,
		UtilizationPct: 71.4,
		Equipment:      []RackEquipment{},
	}
	if rc.RackID != 1 {
		t.Errorf("RackID = %d, want 1", rc.RackID)
	}
	if rc.RackName != "Rack-A1" {
		t.Errorf("RackName = %q, want %q", rc.RackName, "Rack-A1")
	}
	if rc.TotalU != 42 {
		t.Errorf("TotalU = %d, want 42", rc.TotalU)
	}
	if rc.UsedU != 30 {
		t.Errorf("UsedU = %d, want 30", rc.UsedU)
	}
	if rc.AvailableU != 12 {
		t.Errorf("AvailableU = %d, want 12", rc.AvailableU)
	}
	if rc.UtilizationPct != 71.4 {
		t.Errorf("UtilizationPct = %f, want 71.4", rc.UtilizationPct)
	}
	if rc.Equipment == nil {
		t.Error("Equipment should not be nil")
	}
}

// TestRackCapacityTool_Execute_SingleRackError tests error propagation from Get.
func TestRackCapacityTool_Execute_SingleRackError(t *testing.T) {
	expectedErr := fmt.Errorf("connection refused")
	client := &mockRackCapacityClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			return expectedErr
		},
	}
	tool, _ := NewRackCapacityTool(client)

	_, err := tool.Execute(context.Background(), 1, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Check that the error message contains context
	if !containsStr(err.Error(), "rack capacity") {
		t.Errorf("error should contain 'rack capacity', got: %v", err)
	}
}

// TestRackCapacityTool_Execute_AllRacksErrorPropagation tests that search errors are handled.
func TestRackCapacityTool_Execute_AllRacksErrorPropagation(t *testing.T) {
	expectedErr := fmt.Errorf("search failed")
	client := &mockRackCapacityClient{
		getFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			return expectedErr
		},
		searchOptions: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return nil, expectedErr
		},
	}
	tool, _ := NewRackCapacityTool(client)

	_, err := tool.Execute(context.Background(), 0, false)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Helper: check if a string contains a substring.
func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
