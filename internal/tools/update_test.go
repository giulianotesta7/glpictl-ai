package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestUpdateTool(t *testing.T) {
	t.Run("creates update tool with valid client", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, err := NewUpdateTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating update tool: %v", err)
		}
		if tool == nil {
			t.Fatal("expected non-nil update tool")
		}
	})

	t.Run("returns error on nil client", func(t *testing.T) {
		tool, err := NewUpdateTool(nil)
		if err == nil {
			t.Error("expected error on nil client")
		}
		if tool != nil {
			t.Error("expected nil tool on error")
		}
	})

	t.Run("Name returns correct tool name", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewUpdateTool(mockClient)
		if tool.Name() != "glpi_update" {
			t.Errorf("Name() = %q, want %q", tool.Name(), "glpi_update")
		}
	})

	t.Run("Description returns correct description", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewUpdateTool(mockClient)
		if tool.Description() == "" {
			t.Error("Description() should not be empty")
		}
	})
}

func TestUpdateTool_Execute(t *testing.T) {
	t.Run("updates item successfully", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				// Verify endpoint
				if endpoint != "/Computer/5" {
					t.Errorf("endpoint = %q, want %q", endpoint, "/Computer/5")
				}
				responseBody := []byte(`{"id":5,"message":"Item updated"}`)
				return json.Unmarshal(responseBody, result)
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		data := map[string]interface{}{
			"name": "Updated Computer",
		}
		result, err := tool.Execute(context.Background(), "Computer", 5, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != 5 {
			t.Errorf("ID = %d, want %d", result.ID, 5)
		}
		if result.Message != "Item updated" {
			t.Errorf("Message = %q, want %q", result.Message, "Item updated")
		}
	})

	t.Run("handles array-shaped GLPI update response", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				responseBody := []byte(`[{"id": 5, "message": "Item updated"}]`)
				return json.Unmarshal(responseBody, result)
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		data := map[string]interface{}{"name": "Updated Computer"}

		result, err := tool.Execute(context.Background(), "Computer", 5, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != 5 {
			t.Errorf("ID = %d, want %d", result.ID, 5)
		}
		if result.Message != "Item updated" {
			t.Errorf("Message = %q, want %q", result.Message, "Item updated")
		}
	})

	t.Run("falls back to input id when array response has no id", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				responseBody := []byte(`[{"message": "Item updated"}]`)
				return json.Unmarshal(responseBody, result)
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		data := map[string]interface{}{"name": "Updated Computer"}

		result, err := tool.Execute(context.Background(), "Computer", 9, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != 9 {
			t.Errorf("ID = %d, want %d", result.ID, 9)
		}
		if result.Message != "Item updated" {
			t.Errorf("Message = %q, want %q", result.Message, "Item updated")
		}
	})

	t.Run("handles object payload without message", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				responseBody := []byte(`{"id": 5}`)
				return json.Unmarshal(responseBody, result)
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		data := map[string]interface{}{"name": "Updated Computer"}

		result, err := tool.Execute(context.Background(), "Computer", 5, data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != 5 {
			t.Errorf("ID = %d, want %d", result.ID, 5)
		}
		if result.Message != "" {
			t.Errorf("Message = %q, want empty string", result.Message)
		}
	})

	t.Run("returns error on nil response payload", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				return nil
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5, map[string]interface{}{"name": "Updated Computer"})
		if err == nil {
			t.Fatal("expected error for nil response payload")
		}
	})

	t.Run("returns error on empty array payload", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				responseBody := []byte(`[]`)
				return json.Unmarshal(responseBody, result)
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5, map[string]interface{}{"name": "Updated Computer"})
		if err == nil {
			t.Fatal("expected error for empty array payload")
		}
	})

	t.Run("returns error on non-object array payload", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				responseBody := []byte(`[1, "ok", true]`)
				return json.Unmarshal(responseBody, result)
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5, map[string]interface{}{"name": "Updated Computer"})
		if err == nil {
			t.Fatal("expected error for non-object array payload")
		}
	})

	t.Run("returns error on unsupported scalar payload", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				responseBody := []byte(`123`)
				return json.Unmarshal(responseBody, result)
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5, map[string]interface{}{"name": "Updated Computer"})
		if err == nil {
			t.Fatal("expected error for unsupported scalar payload")
		}
	})

	t.Run("returns error when item type is empty", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "", 5, map[string]interface{}{"name": "test"})
		if err == nil {
			t.Fatal("expected error for empty item type")
		}
	})

	t.Run("returns error when ID is invalid", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 0, map[string]interface{}{"name": "test"})
		if err == nil {
			t.Fatal("expected error for invalid ID")
		}
	})

	t.Run("returns error when data is nil", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5, nil)
		if err == nil {
			t.Fatal("expected error for nil data")
		}
	})

	t.Run("returns error when data is empty", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5, map[string]interface{}{})
		if err == nil {
			t.Fatal("expected error for empty data")
		}
	})

	t.Run("handles not found error", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				return glpi.NewNotFoundError("/Computer/999")
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 999, map[string]interface{}{"name": "test"})
		if err == nil {
			t.Fatal("expected error for not found")
		}
		if !errors.Is(err, glpi.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("handles server error", func(t *testing.T) {
		mockClient := &MockClient{
			PutFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				return glpi.NewServerError(500, "database error")
			},
		}

		tool, _ := NewUpdateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5, map[string]interface{}{"name": "test"})
		if err == nil {
			t.Fatal("expected error for server error")
		}
	})
}

func TestUpdateInput_Validation(t *testing.T) {
	t.Run("validates required fields", func(t *testing.T) {
		input := &UpdateInput{
			ItemType: "Computer",
			ID:       5,
			Data: map[string]interface{}{
				"name": "Updated Computer",
			},
		}

		if input.ItemType == "" {
			t.Error("ItemType is required")
		}
		if input.ID <= 0 {
			t.Error("ID must be positive")
		}
		if len(input.Data) == 0 {
			t.Error("Data is required")
		}
	})
}
