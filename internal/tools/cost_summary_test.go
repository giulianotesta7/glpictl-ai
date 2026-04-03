package tools

import (
	"context"
	"sync"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestNewCostSummaryTool(t *testing.T) {
	tests := []struct {
		name    string
		client  ToolClient
		wantErr bool
	}{
		{
			name:    "valid client",
			client:  &MockClient{},
			wantErr: false,
		},
		{
			name:    "nil client",
			client:  nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := NewCostSummaryTool(tt.client)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error for nil client, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if tool == nil {
				t.Error("expected tool, got nil")
			}
			if tool.Name() != "glpi_cost_summary" {
				t.Errorf("expected name 'glpi_cost_summary', got %q", tool.Name())
			}
		})
	}
}

func TestCostSummaryTool_Description(t *testing.T) {
	tool, err := NewCostSummaryTool(&MockClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	desc := tool.Description()
	if desc == "" {
		t.Error("expected non-empty description")
	}
}

func TestCostSummaryTool_GetInput(t *testing.T) {
	tool, err := NewCostSummaryTool(&MockClient{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	input := tool.GetInput()
	if input == nil {
		t.Error("expected non-nil input")
	}
}

func TestCostSummaryTool_Execute_WithMockClient(t *testing.T) {
	mockClient := &MockClient{
		GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if resultMap, ok := result.(*map[string]interface{}); ok {
				*resultMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			return nil
		},
		searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: itemtype + ".name", Name: "Name", Field: "name"},
					{ID: 80, UID: itemtype + ".entities_id", Name: "Entity", Field: "entities_id"},
					{ID: 15, UID: itemtype + ".warranty_date", Name: "Warranty date", Field: "warranty_date"},
					{ID: 16, UID: itemtype + ".warranty_duration", Name: "Warranty duration", Field: "warranty_duration"},
					{ID: 22, UID: itemtype + ".buy_value", Name: "Purchase value", Field: "buy_value"},
				},
			}, nil
		},
	}

	tool, err := NewCostSummaryTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, 0, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.GrandTotal != 0 {
		t.Errorf("expected grand total 0, got %v", result.GrandTotal)
	}
	if result.TotalAssetCost != 0 {
		t.Errorf("expected total asset cost 0, got %v", result.TotalAssetCost)
	}
	if result.TotalContractCost != 0 {
		t.Errorf("expected total contract cost 0, got %v", result.TotalContractCost)
	}
	if result.TotalBudgetAllocated != 0 {
		t.Errorf("expected total budget allocated 0, got %v", result.TotalBudgetAllocated)
	}
	if result.GeneratedAt == "" {
		t.Error("expected generated_at to be set")
	}
}

func TestCostSummaryTool_Execute_ExcludeContracts(t *testing.T) {
	contractsQueried := false
	mockClient := &MockClient{
		GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if endpoint != "" && containsAll(endpoint, "/search/Contract") {
				contractsQueried = true
			}
			if resultMap, ok := result.(*map[string]interface{}); ok {
				*resultMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			return nil
		},
		searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: itemtype + ".name", Name: "Name", Field: "name"},
					{ID: 80, UID: itemtype + ".entities_id", Name: "Entity", Field: "entities_id"},
					{ID: 15, UID: itemtype + ".warranty_date", Name: "Warranty date", Field: "warranty_date"},
					{ID: 16, UID: itemtype + ".warranty_duration", Name: "Warranty duration", Field: "warranty_duration"},
					{ID: 22, UID: itemtype + ".buy_value", Name: "Purchase value", Field: "buy_value"},
					{ID: 15, UID: "Contract.cost", Name: "Cost", Field: "cost"},
					{ID: 11, UID: "Budget.buy_value", Name: "Value", Field: "buy_value"},
				},
			}, nil
		},
	}

	tool, err := NewCostSummaryTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, 0, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if contractsQueried {
		t.Error("expected contracts NOT to be queried when includeContracts=false")
	}
}

func TestCostSummaryTool_Execute_ExcludeBudgets(t *testing.T) {
	budgetsQueried := false
	mockClient := &MockClient{
		GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if endpoint != "" && containsAll(endpoint, "/search/Budget") {
				budgetsQueried = true
			}
			if resultMap, ok := result.(*map[string]interface{}); ok {
				*resultMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			return nil
		},
		searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: itemtype + ".name", Name: "Name", Field: "name"},
					{ID: 80, UID: itemtype + ".entities_id", Name: "Entity", Field: "entities_id"},
					{ID: 15, UID: itemtype + ".warranty_date", Name: "Warranty date", Field: "warranty_date"},
					{ID: 16, UID: itemtype + ".warranty_duration", Name: "Warranty duration", Field: "warranty_duration"},
					{ID: 22, UID: itemtype + ".buy_value", Name: "Purchase value", Field: "buy_value"},
					{ID: 11, UID: "Budget.buy_value", Name: "Value", Field: "buy_value"},
				},
			}, nil
		},
	}

	tool, err := NewCostSummaryTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, 0, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if budgetsQueried {
		t.Error("expected budgets NOT to be queried when includeBudgets=false")
	}
}

func TestCostSummaryTool_Execute_WithEntityID(t *testing.T) {
	var mu sync.Mutex
	var endpoints []string
	mockClient := &MockClient{
		GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			mu.Lock()
			endpoints = append(endpoints, endpoint)
			mu.Unlock()
			if resultMap, ok := result.(*map[string]interface{}); ok {
				*resultMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			return nil
		},
		searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: itemtype + ".name", Name: "Name", Field: "name"},
					{ID: 80, UID: itemtype + ".entities_id", Name: "Entity", Field: "entities_id"},
					{ID: 15, UID: itemtype + ".warranty_date", Name: "Warranty date", Field: "warranty_date"},
					{ID: 16, UID: itemtype + ".warranty_duration", Name: "Warranty duration", Field: "warranty_duration"},
					{ID: 22, UID: itemtype + ".buy_value", Name: "Purchase value", Field: "buy_value"},
				},
			}, nil
		},
	}

	tool, err := NewCostSummaryTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	_, err = tool.Execute(ctx, 5, true, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that at least one endpoint contains the entity filter value
	mu.Lock()
	defer mu.Unlock()
	found := false
	for _, ep := range endpoints {
		if containsStr(ep, "value%5D=5") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected entity filter in endpoints, got: %v", endpoints)
	}
}

func TestExtractContractCost(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want float64
	}{
		{
			name: "cost as float64",
			data: map[string]interface{}{
				"Contract.cost": 1500.50,
			},
			want: 1500.50,
		},
		{
			name: "cost as string",
			data: map[string]interface{}{
				"cost": "2500.00",
			},
			want: 2500.00,
		},
		{
			name: "cost as field ID",
			data: map[string]interface{}{
				"15": 3000.00,
			},
			want: 3000.00,
		},
		{
			name: "no cost field",
			data: map[string]interface{}{
				"name": "Test Contract",
			},
			want: 0,
		},
		{
			name: "nil data",
			data: nil,
			want: 0,
		},
		{
			name: "invalid cost string",
			data: map[string]interface{}{
				"cost": "not-a-number",
			},
			want: 0,
		},
		{
			name: "nil cost value",
			data: map[string]interface{}{
				"Contract.cost": nil,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractContractCost(tt.data)
			if got != tt.want {
				t.Errorf("extractContractCost() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractBudgetValue(t *testing.T) {
	tests := []struct {
		name string
		data map[string]interface{}
		want float64
	}{
		{
			name: "value as float64",
			data: map[string]interface{}{
				"Budget.buy_value": 50000.00,
			},
			want: 50000.00,
		},
		{
			name: "value as string",
			data: map[string]interface{}{
				"value": "10000.00",
			},
			want: 10000.00,
		},
		{
			name: "value as field ID",
			data: map[string]interface{}{
				"11": 25000.00,
			},
			want: 25000.00,
		},
		{
			name: "no value field",
			data: map[string]interface{}{
				"name": "Test Budget",
			},
			want: 0,
		},
		{
			name: "nil data",
			data: nil,
			want: 0,
		},
		{
			name: "invalid value string",
			data: map[string]interface{}{
				"value": "invalid",
			},
			want: 0,
		},
		{
			name: "nil value",
			data: map[string]interface{}{
				"Budget.buy_value": nil,
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractBudgetValue(tt.data)
			if got != tt.want {
				t.Errorf("extractBudgetValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCostSummaryResult_Structure(t *testing.T) {
	result := &CostSummaryResult{
		GeneratedAt:          "2026-04-02T00:00:00Z",
		EntityID:             1,
		AssetTypeCosts:       []AssetTypeCost{},
		ContractCosts:        []ContractCost{},
		BudgetAllocations:    []BudgetAllocation{},
		TotalAssetCost:       0,
		TotalContractCost:    0,
		TotalBudgetAllocated: 0,
		GrandTotal:           0,
		HasErrors:            false,
	}

	if result.GeneratedAt == "" {
		t.Error("expected generated_at to be set")
	}
	if result.AssetTypeCosts == nil {
		t.Error("expected asset_type_costs to be initialized")
	}
	if result.ContractCosts == nil {
		t.Error("expected contract_costs to be initialized")
	}
	if result.BudgetAllocations == nil {
		t.Error("expected budget_allocations to be initialized")
	}
}

func TestAssetTypeCost_Calculation(t *testing.T) {
	tests := []struct {
		name        string
		totalCost   float64
		count       int
		wantAverage float64
	}{
		{
			name:        "simple division",
			totalCost:   10000.00,
			count:       5,
			wantAverage: 2000.00,
		},
		{
			name:        "single item",
			totalCost:   1500.00,
			count:       1,
			wantAverage: 1500.00,
		},
		{
			name:        "zero count edge case",
			totalCost:   0,
			count:       0,
			wantAverage: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var avg float64
			if tt.count > 0 {
				avg = tt.totalCost / float64(tt.count)
			}
			if avg != tt.wantAverage {
				t.Errorf("average = %v, want %v", avg, tt.wantAverage)
			}
		})
	}
}

func TestCostSummaryTool_Execute_WithAssetCosts(t *testing.T) {
	mockClient := &MockClient{
		GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if resultMap, ok := result.(*map[string]interface{}); ok {
				if containsAll(endpoint, "/search/Computer") {
					*resultMap = map[string]interface{}{
						"totalcount": float64(2),
						"data": []interface{}{
							map[string]interface{}{
								"id":                     float64(1),
								"1":                      "PC-001",
								"Computer.buy_value":     float64(1500.00),
								"Computer.warranty_date": "2024-01-01",
							},
							map[string]interface{}{
								"id":                     float64(2),
								"1":                      "PC-002",
								"Computer.buy_value":     float64(2000.00),
								"Computer.warranty_date": "2024-06-01",
							},
						},
					}
					return nil
				}
				*resultMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			return nil
		},
		searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: itemtype + ".name", Name: "Name", Field: "name"},
					{ID: 80, UID: itemtype + ".entities_id", Name: "Entity", Field: "entities_id"},
					{ID: 15, UID: itemtype + ".warranty_date", Name: "Warranty date", Field: "warranty_date"},
					{ID: 16, UID: itemtype + ".warranty_duration", Name: "Warranty duration", Field: "warranty_duration"},
					{ID: 22, UID: itemtype + ".buy_value", Name: "Purchase value", Field: "buy_value"},
				},
			}, nil
		},
	}

	tool, err := NewCostSummaryTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, 0, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalAssetCost != 3500.00 {
		t.Errorf("expected total asset cost 3500.00, got %v", result.TotalAssetCost)
	}
	if result.GrandTotal != 3500.00 {
		t.Errorf("expected grand total 3500.00, got %v", result.GrandTotal)
	}

	var foundComputer bool
	for _, ac := range result.AssetTypeCosts {
		if ac.Itemtype == "Computer" {
			foundComputer = true
			if ac.Count != 2 {
				t.Errorf("expected Computer count 2, got %d", ac.Count)
			}
			if ac.TotalCost != 3500.00 {
				t.Errorf("expected Computer total cost 3500.00, got %v", ac.TotalCost)
			}
			if ac.AverageCost != 1750.00 {
				t.Errorf("expected Computer average cost 1750.00, got %v", ac.AverageCost)
			}
		}
	}
	if !foundComputer {
		t.Error("expected Computer in asset type costs")
	}
}

func TestCostSummaryTool_Execute_WithContractCosts(t *testing.T) {
	mockClient := &MockClient{
		GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if resultMap, ok := result.(*map[string]interface{}); ok {
				if containsAll(endpoint, "/search/Contract") {
					*resultMap = map[string]interface{}{
						"totalcount": float64(1),
						"data": []interface{}{
							map[string]interface{}{
								"id":                float64(1),
								"1":                 "Maintenance Contract",
								"Contract.cost":     float64(5000.00),
								"Contract.end_date": "2026-12-31",
							},
						},
					}
					return nil
				}
				*resultMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			return nil
		},
		searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: itemtype + ".name", Name: "Name", Field: "name"},
					{ID: 80, UID: itemtype + ".entities_id", Name: "Entity", Field: "entities_id"},
					{ID: 15, UID: itemtype + ".warranty_date", Name: "Warranty date", Field: "warranty_date"},
					{ID: 16, UID: itemtype + ".warranty_duration", Name: "Warranty duration", Field: "warranty_duration"},
					{ID: 22, UID: itemtype + ".buy_value", Name: "Purchase value", Field: "buy_value"},
					{ID: 15, UID: "Contract.cost", Name: "Cost", Field: "cost"},
					{ID: 11, UID: "Budget.buy_value", Name: "Value", Field: "buy_value"},
				},
			}, nil
		},
	}

	tool, err := NewCostSummaryTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, 0, true, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalContractCost != 5000.00 {
		t.Errorf("expected total contract cost 5000.00, got %v", result.TotalContractCost)
	}
	if len(result.ContractCosts) != 1 {
		t.Errorf("expected 1 contract cost, got %d", len(result.ContractCosts))
	}
	if result.ContractCosts[0].Name != "Maintenance Contract" {
		t.Errorf("expected contract name 'Maintenance Contract', got %q", result.ContractCosts[0].Name)
	}
}

func TestCostSummaryTool_Execute_WithBudgetAllocations(t *testing.T) {
	mockClient := &MockClient{
		GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if resultMap, ok := result.(*map[string]interface{}); ok {
				if containsAll(endpoint, "/search/Budget") {
					*resultMap = map[string]interface{}{
						"totalcount": float64(1),
						"data": []interface{}{
							map[string]interface{}{
								"id":               float64(1),
								"1":                "IT Budget 2026",
								"Budget.buy_value": float64(100000.00),
							},
						},
					}
					return nil
				}
				*resultMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			return nil
		},
		searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: itemtype + ".name", Name: "Name", Field: "name"},
					{ID: 80, UID: itemtype + ".entities_id", Name: "Entity", Field: "entities_id"},
					{ID: 15, UID: itemtype + ".warranty_date", Name: "Warranty date", Field: "warranty_date"},
					{ID: 16, UID: itemtype + ".warranty_duration", Name: "Warranty duration", Field: "warranty_duration"},
					{ID: 22, UID: itemtype + ".buy_value", Name: "Purchase value", Field: "buy_value"},
					{ID: 11, UID: "Budget.buy_value", Name: "Value", Field: "buy_value"},
				},
			}, nil
		},
	}

	tool, err := NewCostSummaryTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, 0, false, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalBudgetAllocated != 100000.00 {
		t.Errorf("expected total budget allocated 100000.00, got %v", result.TotalBudgetAllocated)
	}
	if len(result.BudgetAllocations) != 1 {
		t.Errorf("expected 1 budget allocation, got %d", len(result.BudgetAllocations))
	}
	if result.BudgetAllocations[0].Name != "IT Budget 2026" {
		t.Errorf("expected budget name 'IT Budget 2026', got %q", result.BudgetAllocations[0].Name)
	}
}

func TestCostSummaryTool_Execute_PartialFailure(t *testing.T) {
	mockClient := &MockClient{
		GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
			if resultMap, ok := result.(*map[string]interface{}); ok {
				if containsAll(endpoint, "/search/Computer") {
					*resultMap = map[string]interface{}{
						"totalcount": float64(1),
						"data": []interface{}{
							map[string]interface{}{
								"id":                 float64(1),
								"Computer.buy_value": float64(1000.00),
							},
						},
					}
					return nil
				}
				if containsAll(endpoint, "/search/Monitor") {
					return context.Canceled
				}
				*resultMap = map[string]interface{}{
					"totalcount": float64(0),
					"data":       []interface{}{},
				}
				return nil
			}
			return nil
		},
		searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
			return &glpi.SearchOptionsResult{
				ItemType: itemtype,
				Fields: []glpi.SearchOption{
					{ID: 1, UID: itemtype + ".name", Name: "Name", Field: "name"},
					{ID: 80, UID: itemtype + ".entities_id", Name: "Entity", Field: "entities_id"},
					{ID: 15, UID: itemtype + ".warranty_date", Name: "Warranty date", Field: "warranty_date"},
					{ID: 16, UID: itemtype + ".warranty_duration", Name: "Warranty duration", Field: "warranty_duration"},
					{ID: 22, UID: itemtype + ".buy_value", Name: "Purchase value", Field: "buy_value"},
				},
			}, nil
		},
	}

	tool, err := NewCostSummaryTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	result, err := tool.Execute(ctx, 0, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.HasErrors {
		t.Error("expected HasErrors to be true")
	}
	if len(result.ErrorMessages) == 0 {
		t.Error("expected error messages")
	}
	if result.TotalAssetCost != 1000.00 {
		t.Errorf("expected partial total asset cost 1000.00, got %v", result.TotalAssetCost)
	}
}
