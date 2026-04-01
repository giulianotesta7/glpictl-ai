package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestDeleteTool(t *testing.T) {
	t.Run("creates delete tool with valid client", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, err := NewDeleteTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating delete tool: %v", err)
		}
		if tool == nil {
			t.Fatal("expected non-nil delete tool")
		}
	})

	t.Run("returns error on nil client", func(t *testing.T) {
		tool, err := NewDeleteTool(nil)
		if err == nil {
			t.Error("expected error on nil client")
		}
		if tool != nil {
			t.Error("expected nil tool on error")
		}
	})

	t.Run("Name returns correct tool name", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewDeleteTool(mockClient)
		if tool.Name() != "glpi_delete" {
			t.Errorf("Name() = %q, want %q", tool.Name(), "glpi_delete")
		}
	})

	t.Run("Description returns correct description", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewDeleteTool(mockClient)
		if tool.Description() == "" {
			t.Error("Description() should not be empty")
		}
	})
}

func TestDeleteTool_Execute(t *testing.T) {
	t.Run("deletes item successfully", func(t *testing.T) {
		mockClient := &MockClient{
			DeleteFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				// Verify endpoint
				if endpoint != "/Computer/5" {
					t.Errorf("endpoint = %q, want %q", endpoint, "/Computer/5")
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"id":      float64(5),
					"message": "Item deleted",
				}
				return nil
			},
		}

		tool, _ := NewDeleteTool(mockClient)
		result, err := tool.Execute(context.Background(), "Computer", 5)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != 5 {
			t.Errorf("ID = %d, want %d", result.ID, 5)
		}
		if result.Message != "Item deleted" {
			t.Errorf("Message = %q, want %q", result.Message, "Item deleted")
		}
	})

	t.Run("returns error when item type is empty", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewDeleteTool(mockClient)
		_, err := tool.Execute(context.Background(), "", 5)
		if err == nil {
			t.Fatal("expected error for empty item type")
		}
	})

	t.Run("returns error when ID is invalid", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewDeleteTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 0)
		if err == nil {
			t.Fatal("expected error for invalid ID")
		}
	})

	t.Run("handles not found error", func(t *testing.T) {
		mockClient := &MockClient{
			DeleteFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewNotFoundError("/Computer/999")
			},
		}

		tool, _ := NewDeleteTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 999)
		if err == nil {
			t.Fatal("expected error for not found")
		}
		if !errors.Is(err, glpi.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("handles server error", func(t *testing.T) {
		mockClient := &MockClient{
			DeleteFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewServerError(500, "database error")
			},
		}

		tool, _ := NewDeleteTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5)
		if err == nil {
			t.Fatal("expected error for server error")
		}
	})

	t.Run("handles auth failed error", func(t *testing.T) {
		mockClient := &MockClient{
			DeleteFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewAuthFailedError("invalid token")
			},
		}

		tool, _ := NewDeleteTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 5)
		if err == nil {
			t.Fatal("expected error for auth failed")
		}
		if !errors.Is(err, glpi.ErrAuthFailed) {
			t.Errorf("expected ErrAuthFailed, got %v", err)
		}
	})
}

func TestDeleteInput_Validation(t *testing.T) {
	t.Run("validates required fields", func(t *testing.T) {
		input := &DeleteInput{
			ItemType: "Computer",
			ID:       5,
		}

		if input.ItemType == "" {
			t.Error("ItemType is required")
		}
		if input.ID <= 0 {
			t.Error("ID must be positive")
		}
	})
}
