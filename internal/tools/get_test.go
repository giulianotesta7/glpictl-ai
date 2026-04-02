package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestGetTool(t *testing.T) {
	t.Run("creates get tool with valid client", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, err := NewGetTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating get tool: %v", err)
		}
		if tool == nil {
			t.Fatal("expected non-nil get tool")
		}
	})

	t.Run("returns error on nil client", func(t *testing.T) {
		tool, err := NewGetTool(nil)
		if err == nil {
			t.Error("expected error on nil client")
		}
		if tool != nil {
			t.Error("expected nil tool on error")
		}
	})

	t.Run("Name returns correct tool name", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewGetTool(mockClient)
		if tool.Name() != "glpi_get" {
			t.Errorf("Name() = %q, want %q", tool.Name(), "glpi_get")
		}
	})

	t.Run("Description returns correct description", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewGetTool(mockClient)
		if tool.Description() == "" {
			t.Error("Description() should not be empty")
		}
	})
}

func TestGetTool_Execute(t *testing.T) {
	t.Run("gets item by type and id successfully", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				// Verify endpoint
				if endpoint != "/Computer/5" {
					t.Errorf("endpoint = %q, want %q", endpoint, "/Computer/5")
				}
				// Populate result
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"id":   float64(5),
					"name": "Computer-001",
				}
				return nil
			},
		}

		tool, _ := NewGetTool(mockClient)
		result, err := tool.Execute(context.Background(), "Computer", 5, nil, nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != 5 {
			t.Errorf("ID = %d, want %d", result.ID, 5)
		}
		if result.Name != "Computer-001" {
			t.Errorf("Name = %q, want %q", result.Name, "Computer-001")
		}
	})

	t.Run("gets item with specific fields", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				// Verify endpoint has query parameters
				expectedEndpoint := "/Computer/5?fields[0]=name&fields[1]=serial"
				if endpoint != expectedEndpoint {
					t.Errorf("endpoint = %q, want %q", endpoint, expectedEndpoint)
				}
				resMap := result.(*map[string]interface{})
				*resMap = map[string]interface{}{
					"id":     float64(5),
					"name":   "Computer-001",
					"serial": "SN12345",
				}
				return nil
			},
		}

		tool, _ := NewGetTool(mockClient)
		fields := []string{"name", "serial"}
		result, err := tool.Execute(context.Background(), "Computer", 5, fields, nil, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ID != 5 {
			t.Errorf("ID = %d, want %d", result.ID, 5)
		}
	})

	t.Run("returns error when item not found", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewNotFoundError("/Computer/999")
			},
		}

		tool, _ := NewGetTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 999, nil, nil, false)
		if err == nil {
			t.Fatal("expected error for not found item")
		}
		if !errors.Is(err, glpi.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("returns error when item type has invalid characters", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return nil
			},
		}

		tool, _ := NewGetTool(mockClient)
		_, err := tool.Execute(context.Background(), "../admin", 1, nil, nil, false)
		if err == nil {
			t.Fatal("expected error for path traversal itemtype")
		}
	})

	t.Run("returns error on server error", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewServerError(500, "internal server error")
			},
		}

		tool, _ := NewGetTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 1, nil, nil, false)
		if err == nil {
			t.Fatal("expected error for server error")
		}
	})

	t.Run("returns error on session expired", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewSessionExpiredError()
			},
		}

		tool, _ := NewGetTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 1, nil, nil, false)
		if err == nil {
			t.Fatal("expected error for session expired")
		}
		if !errors.Is(err, glpi.ErrSessionExpired) {
			t.Errorf("expected ErrSessionExpired, got %v", err)
		}
	})

	t.Run("returns error on auth failed", func(t *testing.T) {
		mockClient := &MockClient{
			GetFunc: func(ctx context.Context, endpoint string, result interface{}) error {
				return glpi.NewAuthFailedError("invalid credentials")
			},
		}

		tool, _ := NewGetTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer", 1, nil, nil, false)
		if err == nil {
			t.Fatal("expected error for auth failed")
		}
		if !errors.Is(err, glpi.ErrAuthFailed) {
			t.Errorf("expected ErrAuthFailed, got %v", err)
		}
	})
}

func TestGetTool_GetInput(t *testing.T) {
	t.Run("returns correct input structure", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewGetTool(mockClient)
		input := tool.GetInput()

		if input.ItemType != "" {
			t.Errorf("ItemType should be empty by default, got %q", input.ItemType)
		}
		if input.ID != 0 {
			t.Errorf("ID should be 0 by default, got %d", input.ID)
		}
		if input.Fields != nil {
			t.Errorf("Fields should be nil by default, got %v", input.Fields)
		}
	})
}

func TestGetInput_Validation(t *testing.T) {
	t.Run("validates required fields", func(t *testing.T) {
		input := &GetInput{
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

	t.Run("allows empty fields array", func(t *testing.T) {
		input := &GetInput{
			ItemType: "Computer",
			ID:       5,
			Fields:   []string{},
		}

		if len(input.Fields) != 0 {
			t.Errorf("Fields should be empty, got %d", len(input.Fields))
		}
	})

	t.Run("allows specific fields", func(t *testing.T) {
		input := &GetInput{
			ItemType: "Computer",
			ID:       5,
			Fields:   []string{"name", "serial"},
		}

		if len(input.Fields) != 2 {
			t.Errorf("Fields should have 2 elements, got %d", len(input.Fields))
		}
	})
}
