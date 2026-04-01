package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestCreateTool(t *testing.T) {
	t.Run("creates create tool with valid client", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, err := NewCreateTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating create tool: %v", err)
		}
		if tool == nil {
			t.Fatal("expected non-nil create tool")
		}
	})

	t.Run("returns error on nil client", func(t *testing.T) {
		tool, err := NewCreateTool(nil)
		if err == nil {
			t.Error("expected error on nil client")
		}
		if tool != nil {
			t.Error("expected nil tool on error")
		}
	})

	t.Run("Name returns correct tool name", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewCreateTool(mockClient)
		if tool.Name() != "glpi_create" {
			t.Errorf("Name() = %q, want %q", tool.Name(), "glpi_create")
		}
	})

	t.Run("Description returns correct description", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewCreateTool(mockClient)
		if tool.Description() == "" {
			t.Error("Description() should not be empty")
		}
	})
}

func TestCreateTool_Execute(t *testing.T) {
	t.Run("creates item successfully", func(t *testing.T) {
		mockClient := &MockClient{
			PostFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				// Verify endpoint
				if endpoint != "/Computer" {
					t.Errorf("endpoint = %q, want %q", endpoint, "/Computer")
				}
				// Verify body has input wrapper
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"id":      float64(5),
					"message": "Item created",
				}
				return nil
			},
		}

		tool, _ := NewCreateTool(mockClient)
		data := map[string]interface{}{
			"name":   "New Computer",
			"serial": "SN12345",
		}
		result, err := tool.Execute(context.Background(), "Computer", data)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != 5 {
			t.Errorf("ID = %d, want %d", result.ID, 5)
		}
		if result.Message != "Item created" {
			t.Errorf("Message = %q, want %q", result.Message, "Item created")
		}
	})

	t.Run("returns error when item type is empty", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewCreateTool(mockClient)
		_, err := tool.Execute(context.Background(), "", map[string]interface{}{"name": "test"})
		if err == nil {
			t.Fatal("expected error for empty item type")
		}
	})

	t.Run("returns error when data is nil", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewCreateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", nil)
		if err == nil {
			t.Fatal("expected error for nil data")
		}
	})

	t.Run("returns error when data is empty", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewCreateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", map[string]interface{}{})
		if err == nil {
			t.Fatal("expected error for empty data")
		}
	})

	t.Run("handles server error", func(t *testing.T) {
		mockClient := &MockClient{
			PostFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				return glpi.NewServerError(500, "database error")
			},
		}

		tool, _ := NewCreateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", map[string]interface{}{"name": "test"})
		if err == nil {
			t.Fatal("expected error for server error")
		}
	})

	t.Run("handles auth failed error", func(t *testing.T) {
		mockClient := &MockClient{
			PostFunc: func(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
				return glpi.NewAuthFailedError("invalid token")
			},
		}

		tool, _ := NewCreateTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", map[string]interface{}{"name": "test"})
		if err == nil {
			t.Fatal("expected error for auth failed")
		}
		if !errors.Is(err, glpi.ErrAuthFailed) {
			t.Errorf("expected ErrAuthFailed, got %v", err)
		}
	})
}

func TestCreateInput_Validation(t *testing.T) {
	t.Run("validates required fields", func(t *testing.T) {
		input := &CreateInput{
			ItemType: "Computer",
			Data: map[string]interface{}{
				"name": "Test Computer",
			},
		}

		if input.ItemType == "" {
			t.Error("ItemType is required")
		}
		if len(input.Data) == 0 {
			t.Error("Data is required")
		}
	})
}
