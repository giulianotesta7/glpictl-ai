package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestSearchTool(t *testing.T) {
	t.Run("creates search tool with valid client", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, err := NewSearchTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating search tool: %v", err)
		}
		if tool == nil {
			t.Fatal("expected non-nil search tool")
		}
	})

	t.Run("returns error on nil client", func(t *testing.T) {
		tool, err := NewSearchTool(nil)
		if err == nil {
			t.Error("expected error on nil client")
		}
		if tool != nil {
			t.Error("expected nil tool on error")
		}
	})

	t.Run("Name returns correct tool name", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewSearchTool(mockClient)
		if tool.Name() != "glpi_search" {
			t.Errorf("Name() = %q, want %q", tool.Name(), "glpi_search")
		}
	})

	t.Run("Description returns correct description", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewSearchTool(mockClient)
		if tool.Description() == "" {
			t.Error("Description() should not be empty")
		}
	})
}

func TestSearchTool_Execute(t *testing.T) {
	t.Run("searches items with simple criteria", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				// Verify endpoint contains search criteria (URL encoded)
				if !containsAll(endpoint, "/search/Computer", "criteria%5B0%5D%5Bfield%5D=1", "criteria%5B0%5D%5Bsearchtype%5D=contains", "criteria%5B0%5D%5Bvalue%5D=test") {
					t.Errorf("unexpected endpoint: %s", endpoint)
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"1": "test-computer",
							"2": "Computer-001",
						},
					},
				}
				return nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{
			{Field: 1, SearchType: "contains", Value: "test"},
		}
		result, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.TotalCount != 1 {
			t.Errorf("TotalCount = %d, want %d", result.TotalCount, 1)
		}
		if len(result.Data) != 1 {
			t.Errorf("len(Data) = %d, want %d", len(result.Data), 1)
		}
	})

	t.Run("searches items with multiple criteria", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				// Should have both criteria
				if !containsAll(endpoint, "/search/Computer", "criteria%5B0%5D", "criteria%5B1%5D") {
					t.Errorf("unexpected endpoint: %s", endpoint)
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"totalcount": float64(10),
					"data":       []interface{}{},
				}
				return nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{
			{Field: 1, SearchType: "contains", Value: "test"},
			{Field: 2, SearchType: "equals", Value: "active", Link: "AND"},
		}
		result, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.TotalCount != 10 {
			t.Errorf("TotalCount = %d, want %d", result.TotalCount, 10)
		}
	})

	t.Run("searches with field selection", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				// Should include field parameters
				if !containsAll(endpoint, "/search/Computer", "fields%5B0%5D=name", "fields%5B1%5D=serial") {
					t.Errorf("unexpected endpoint: %s", endpoint)
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"totalcount": float64(1),
					"data": []interface{}{
						map[string]interface{}{
							"name":   "Computer-001",
							"serial": "SN12345",
						},
					},
				}
				return nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{
			{Field: 1, SearchType: "contains", Value: "test"},
		}
		fields := []string{"name", "serial"}
		result, err := tool.Execute(context.Background(), "Computer", criteria, fields, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.TotalCount != 1 {
			t.Errorf("TotalCount = %d, want %d", result.TotalCount, 1)
		}
	})

	t.Run("searches with range", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				// Should include range parameter
				if !containsAll(endpoint, "/search/Computer", "range=0-10") {
					t.Errorf("unexpected endpoint: %s", endpoint)
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"totalcount": float64(100),
					"data":       []interface{}{},
				}
				return nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{
			{Field: 1, SearchType: "contains", Value: "test"},
		}
		searchRange := &SearchRange{Start: 0, End: 10}
		result, err := tool.Execute(context.Background(), "Computer", criteria, nil, searchRange)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.TotalCount != 100 {
			t.Errorf("TotalCount = %d, want %d", result.TotalCount, 100)
		}
	})

	t.Run("translates field_name using uid", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields: []glpi.SearchOption{{
						ID:          1,
						UID:         "Computer.name",
						Field:       "name",
						Name:        "Name",
						DisplayName: "Name",
					}},
				}, nil
			},
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				if !containsAll(endpoint, "criteria%5B0%5D%5Bfield%5D=1") {
					t.Errorf("unexpected endpoint: %s", endpoint)
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{"totalcount": float64(0), "data": []interface{}{}}
				return nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{{FieldName: "Computer.name", SearchType: "contains", Value: "pc"}}
		if _, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("translates field_name using technical and display name fallback", func(t *testing.T) {
		tests := []struct {
			name      string
			fieldName string
			wantField string
		}{
			{name: "technical field", fieldName: "serial", wantField: "criteria%5B0%5D%5Bfield%5D=10"},
			{name: "display name", fieldName: "Hostname", wantField: "criteria%5B0%5D%5Bfield%5D=11"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				mockClient := &MockClient{
					searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
						return &glpi.SearchOptionsResult{
							ItemType: itemtype,
							Fields: []glpi.SearchOption{
								{ID: 10, Field: "serial", Name: "Serial", DisplayName: "Serial Number"},
								{ID: 11, Field: "name", Name: "Name", DisplayName: "Hostname"},
							},
						}, nil
					},
					GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
						if !containsAll(endpoint, tt.wantField) {
							t.Errorf("unexpected endpoint: %s", endpoint)
						}
						resMap := result.(*map[string]interface{})
						*resMap = map[string]interface{}{"totalcount": float64(0), "data": []interface{}{}}
						return nil
					},
				}

				tool, _ := NewSearchTool(mockClient)
				criteria := []SearchCriterion{{FieldName: tt.fieldName, SearchType: "contains", Value: "pc"}}
				if _, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		}
	})

	t.Run("keeps numeric field fast path", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				t.Fatal("GetSearchOptions should not be called for numeric field")
				return nil, nil
			},
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				if !containsAll(endpoint, "criteria%5B0%5D%5Bfield%5D=42") {
					t.Errorf("unexpected endpoint: %s", endpoint)
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{"totalcount": float64(0), "data": []interface{}{}}
				return nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{{Field: 42, SearchType: "equals", Value: "ok"}}
		if _, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("returns error when field_name is unknown", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields:   []glpi.SearchOption{{ID: 1, UID: "Computer.name", Field: "name", Name: "Name", DisplayName: "Name"}},
				}, nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{{FieldName: "serial", SearchType: "contains", Value: "A"}}
		_, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "field_name") {
			t.Fatalf("expected field_name error, got %v", err)
		}
	})

	t.Run("returns error when field_name is ambiguous", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields: []glpi.SearchOption{
						{ID: 1, DisplayName: "Serial"},
						{ID: 2, DisplayName: "Serial"},
					},
				}, nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{{FieldName: "Serial", SearchType: "contains", Value: "A"}}
		_, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("expected ambiguous error, got %v", err)
		}
	})

	t.Run("supports mixed criteria with field and field_name", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields:   []glpi.SearchOption{{ID: 10, UID: "Computer.serial", Field: "serial", Name: "Serial", DisplayName: "Serial"}},
				}, nil
			},
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				if !containsAll(endpoint, "criteria%5B0%5D%5Bfield%5D=1", "criteria%5B1%5D%5Bfield%5D=10") {
					t.Errorf("unexpected endpoint: %s", endpoint)
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{"totalcount": float64(0), "data": []interface{}{}}
				return nil
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{
			{Field: 1, SearchType: "contains", Value: "pc"},
			{FieldName: "Computer.serial", SearchType: "contains", Value: "SN", Link: "AND"},
		}
		if _, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("returns error when item type is empty", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{{Field: 1, SearchType: "contains", Value: "test"}}
		_, err := tool.Execute(context.Background(), "", criteria, nil, nil)
		if err == nil {
			t.Fatal("expected error for empty item type")
		}
	})

	t.Run("returns error when criteria is empty", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewSearchTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", nil, nil, nil)
		if err == nil {
			t.Fatal("expected error for empty criteria")
		}
	})

	t.Run("handles authenticated error", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewAuthFailedError("invalid token")
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{{Field: 1, SearchType: "contains", Value: "test"}}
		_, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil)
		if err == nil {
			t.Fatal("expected error for auth failed")
		}
		if !errors.Is(err, glpi.ErrAuthFailed) {
			t.Errorf("expected ErrAuthFailed, got %v", err)
		}
	})

	t.Run("handles server error", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewServerError(500, "internal error")
			},
		}

		tool, _ := NewSearchTool(mockClient)
		criteria := []SearchCriterion{{Field: 1, SearchType: "contains", Value: "test"}}
		_, err := tool.Execute(context.Background(), "Computer", criteria, nil, nil)
		if err == nil {
			t.Fatal("expected error for server error")
		}
	})
}

// containsAll checks if s contains all substrings
func containsAll(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if !strings.Contains(s, substr) {
			return false
		}
	}
	return true
}

func TestSearchInput_Validation(t *testing.T) {
	t.Run("validates required fields", func(t *testing.T) {
		input := &SearchInput{
			ItemType: "Computer",
			Criteria: []SearchCriterion{
				{Field: 1, SearchType: "contains", Value: "test"},
			},
		}

		if input.ItemType == "" {
			t.Error("ItemType is required")
		}
		if len(input.Criteria) == 0 {
			t.Error("Criteria is required")
		}
	})

	t.Run("allows optional fields and range", func(t *testing.T) {
		input := &SearchInput{
			ItemType: "Computer",
			Criteria: []SearchCriterion{
				{Field: 1, SearchType: "contains", Value: "test"},
			},
			Fields: []string{"name", "serial"},
			Range:  &SearchRange{Start: 0, End: 50},
		}

		if len(input.Fields) != 2 {
			t.Errorf("Fields should have 2 elements, got %d", len(input.Fields))
		}
		if input.Range.Start != 0 || input.Range.End != 50 {
			t.Errorf("Range should be 0-50, got %d-%d", input.Range.Start, input.Range.End)
		}
	})
}

func TestSearchCriterion(t *testing.T) {
	t.Run("creates criterion with link", func(t *testing.T) {
		criterion := SearchCriterion{
			Field:      1,
			SearchType: "contains",
			Value:      "test",
			Link:       "AND",
		}

		if criterion.Field != 1 {
			t.Errorf("Field = %d, want %d", criterion.Field, 1)
		}
		if criterion.SearchType != "contains" {
			t.Errorf("SearchType = %q, want %q", criterion.SearchType, "contains")
		}
		if criterion.Value != "test" {
			t.Errorf("Value = %q, want %q", criterion.Value, "test")
		}
		if criterion.Link != "AND" {
			t.Errorf("Link = %q, want %q", criterion.Link, "AND")
		}
	})
}
