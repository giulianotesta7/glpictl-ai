package tools

import (
	"context"
	"errors"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

func TestListFieldsTool_Execute(t *testing.T) {
	t.Run("returns normalized fields and cached false", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Cached:   false,
					Fields: []glpi.SearchOption{{
						ID:          1,
						UID:         "Computer.name",
						Name:        "Name",
						DisplayName: "Name",
					}},
				}, nil
			},
		}

		tool, err := NewListFieldsTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating list fields tool: %v", err)
		}

		result, err := tool.Execute(context.Background(), "Computer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ItemType != "Computer" {
			t.Errorf("ItemType = %q, want %q", result.ItemType, "Computer")
		}
		if result.Cached {
			t.Error("Cached should be false")
		}
		if len(result.Fields) != 1 {
			t.Fatalf("len(Fields) = %d, want 1", len(result.Fields))
		}
	})

	t.Run("returns cached true from client", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return &glpi.SearchOptionsResult{
					ItemType: itemtype,
					Cached:   true,
					Fields:   []glpi.SearchOption{{ID: 1, UID: "Computer.name"}},
				}, nil
			},
		}

		tool, _ := NewListFieldsTool(mockClient)
		result, err := tool.Execute(context.Background(), "Computer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result.Cached {
			t.Error("Cached should be true")
		}
	})

	t.Run("returns error for invalid itemtype", func(t *testing.T) {
		mockClient := &MockClient{}
		tool, _ := NewListFieldsTool(mockClient)

		_, err := tool.Execute(context.Background(), "../admin")
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("wraps client error", func(t *testing.T) {
		mockClient := &MockClient{
			searchOptionsFunc: func(ctx context.Context, itemtype string) (*glpi.SearchOptionsResult, error) {
				return nil, glpi.NewServerError(500, "boom")
			},
		}

		tool, _ := NewListFieldsTool(mockClient)
		_, err := tool.Execute(context.Background(), "Computer")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, glpi.ErrServerError) {
			t.Fatalf("expected wrapped server error, got %v", err)
		}
	})
}
