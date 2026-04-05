package tools

import (
	"context"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestGroupAssetsTool_Execute(t *testing.T) {
	t.Run("returns error for invalid group ID", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, err := NewGroupAssetsTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating tool: %v", err)
		}

		_, err = tool.Execute(context.Background(), 0, nil)
		if err == nil {
			t.Fatal("expected error for group ID 0")
		}

		_, err = tool.Execute(context.Background(), -1, nil)
		if err == nil {
			t.Fatal("expected error for negative group ID")
		}
	})

	t.Run("returns empty result when search finds no items", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields: []glpi.SearchOption{
						{ID: 1, UID: itemtype + ".name", Field: "name", Name: "Name", DisplayName: "Name"},
						{ID: 71, UID: itemtype + ".Group_Item.Group.completename", Field: "completename", Name: "Group", DisplayName: "Group", Table: "glpi_groups"},
					},
				}, nil
			},
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				if m, ok := result.(*map[string]interface{}); ok {
					*m = map[string]interface{}{
						"totalcount": float64(0),
						"data":       []interface{}{},
					}
				}
				return nil
			},
		}
		tool, err := NewGroupAssetsTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating tool: %v", err)
		}

		result, err := tool.Execute(context.Background(), 5, []string{"Computer"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.GroupID != 5 {
			t.Errorf("GroupID = %d, want 5", result.GroupID)
		}
		if result.Count != 0 {
			t.Errorf("Count = %d, want 0", result.Count)
		}
		if len(result.Assets) != 0 {
			t.Errorf("expected no assets, got %d", len(result.Assets))
		}
	})

	t.Run("returns assets when search finds items", func(t *testing.T) {
		callCount := 0
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields: []glpi.SearchOption{
						{ID: 1, UID: itemtype + ".name", Field: "name", Name: "Name", DisplayName: "Name"},
						{ID: 71, UID: itemtype + ".Group_Item.Group.completename", Field: "completename", Name: "Group", DisplayName: "Group", Table: "glpi_groups"},
					},
				}, nil
			},
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				callCount++
				if m, ok := result.(*map[string]interface{}); ok {
					if callCount == 1 {
						*m = map[string]interface{}{
							"totalcount": float64(2),
							"data": []interface{}{
								map[string]interface{}{
									"id":   float64(101),
									"name": "PC-Office-01",
								},
								map[string]interface{}{
									"id":   float64(102),
									"name": "PC-Office-02",
								},
							},
						}
					} else {
						*m = map[string]interface{}{
							"totalcount": float64(0),
							"data":       []interface{}{},
						}
					}
				}
				return nil
			},
		}
		tool, err := NewGroupAssetsTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating tool: %v", err)
		}

		result, err := tool.Execute(context.Background(), 10, []string{"Computer", "Printer"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.GroupID != 10 {
			t.Errorf("GroupID = %d, want 10", result.GroupID)
		}
		if result.Count != 2 {
			t.Errorf("Count = %d, want 2", result.Count)
		}
		if len(result.Assets) != 2 {
			t.Fatalf("expected 2 assets, got %d", len(result.Assets))
		}

		if result.Assets[0].Itemtype != "Computer" {
			t.Errorf("Assets[0].Itemtype = %q, want %q", result.Assets[0].Itemtype, "Computer")
		}
		if result.Assets[0].ID != 101 {
			t.Errorf("Assets[0].ID = %d, want 101", result.Assets[0].ID)
		}
		if result.Assets[0].Name != "PC-Office-01" {
			t.Errorf("Assets[0].Name = %q, want %q", result.Assets[0].Name, "PC-Office-01")
		}
	})

	t.Run("uses default itemtypes when none provided", func(t *testing.T) {
		searchedItemtypes := []string{}
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields: []glpi.SearchOption{
						{ID: 1, UID: itemtype + ".name", Field: "name", Name: "Name", DisplayName: "Name"},
						{ID: 71, UID: itemtype + ".Group_Item.Group.completename", Field: "completename", Name: "Group", DisplayName: "Group", Table: "glpi_groups"},
					},
				}, nil
			},
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				for _, it := range DefaultItemtypes {
					if containsAll(endpoint, "/search/"+it) {
						searchedItemtypes = append(searchedItemtypes, it)
					}
				}
				if m, ok := result.(*map[string]interface{}); ok {
					*m = map[string]interface{}{
						"totalcount": float64(0),
						"data":       []interface{}{},
					}
				}
				return nil
			},
		}
		tool, err := NewGroupAssetsTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating tool: %v", err)
		}

		_, err = tool.Execute(context.Background(), 1, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(searchedItemtypes) != len(DefaultItemtypes) {
			t.Errorf("searched %d itemtypes, want %d", len(searchedItemtypes), len(DefaultItemtypes))
		}
	})

	t.Run("skips itemtypes that return errors", func(t *testing.T) {
		callCount := 0
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields: []glpi.SearchOption{
						{ID: 1, UID: itemtype + ".name", Field: "name", Name: "Name", DisplayName: "Name"},
						{ID: 71, UID: itemtype + ".Group_Item.Group.completename", Field: "completename", Name: "Group", DisplayName: "Group", Table: "glpi_groups"},
					},
				}, nil
			},
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				callCount++
				if callCount == 1 {
					// First itemtype succeeds
					if m, ok := result.(*map[string]interface{}); ok {
						*m = map[string]interface{}{
							"totalcount": float64(1),
							"data": []interface{}{
								map[string]interface{}{
									"id":   float64(200),
									"name": "Printer-01",
								},
							},
						}
					}
					return nil
				}
				// Second itemtype fails
				return context.Canceled
			},
		}
		tool, err := NewGroupAssetsTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating tool: %v", err)
		}

		result, err := tool.Execute(context.Background(), 3, []string{"Printer", "Computer"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have 1 asset from the successful search
		if result.Count != 1 {
			t.Errorf("Count = %d, want 1", result.Count)
		}
		if len(result.Assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(result.Assets))
		}
		if result.Assets[0].ID != 200 {
			t.Errorf("Assets[0].ID = %d, want 200", result.Assets[0].ID)
		}
	})

	t.Run("extracts name from field 1 when name key missing", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Fields: []glpi.SearchOption{
						{ID: 1, UID: itemtype + ".name", Field: "name", Name: "Name", DisplayName: "Name"},
						{ID: 71, UID: itemtype + ".Group_Item.Group.completename", Field: "completename", Name: "Group", DisplayName: "Group", Table: "glpi_groups"},
					},
				}, nil
			},
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				if m, ok := result.(*map[string]interface{}); ok {
					*m = map[string]interface{}{
						"totalcount": float64(1),
						"data": []interface{}{
							map[string]interface{}{
								"id": float64(300),
								"1":  "Monitor-01",
							},
						},
					}
				}
				return nil
			},
		}
		tool, err := NewGroupAssetsTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating tool: %v", err)
		}

		result, err := tool.Execute(context.Background(), 7, []string{"Monitor"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(result.Assets) != 1 {
			t.Fatalf("expected 1 asset, got %d", len(result.Assets))
		}
		if result.Assets[0].Name != "Monitor-01" {
			t.Errorf("Name = %q, want %q", result.Assets[0].Name, "Monitor-01")
		}
	})
}

func TestNewGroupAssetsTool(t *testing.T) {
	t.Run("creates tool with valid client", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, err := NewGroupAssetsTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tool == nil {
			t.Fatal("expected non-nil tool")
		}
	})

	t.Run("returns error for nil client", func(t *testing.T) {
		tool, err := NewGroupAssetsTool(nil)
		if err == nil {
			t.Fatal("expected error for nil client")
		}
		if tool != nil {
			t.Fatal("expected nil tool on error")
		}
	})
}

func TestGroupAssetsTool_Name(t *testing.T) {
	mockClient := &MockClient{}
	tool, err := NewGroupAssetsTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tool.Name() != "glpi_group_assets" {
		t.Errorf("Name() = %q, want %q", tool.Name(), "glpi_group_assets")
	}
}

func TestGroupAssetsTool_Description(t *testing.T) {
	mockClient := &MockClient{}
	tool, err := NewGroupAssetsTool(mockClient)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tool.Description() != "Get all assets assigned to a specific group" {
		t.Errorf("Description() = %q, want %q", tool.Description(), "Get all assets assigned to a specific group")
	}
}

func TestGroupAssetsTool_ImplementsTool(t *testing.T) {
	var _ Tool = (*GroupAssetsTool)(nil)
}
