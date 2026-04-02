package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
	"github.com/giulianotesta7/glpictl-ai/internal/tools"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestCreateSearchHandler_AcceptsFieldName(t *testing.T) {
	searchCalled := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/initSession") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
			return
		}

		if strings.Contains(r.URL.Path, "/listSearchOptions/Computer") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"1":{"uid":"Computer.name","name":"Name","field":"name"}}`))
			return
		}

		if strings.Contains(r.URL.Path, "/search/Computer") {
			searchCalled = true
			if !strings.Contains(r.URL.RawQuery, "criteria%5B0%5D%5Bfield%5D=1") {
				t.Errorf("expected translated field query, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"totalcount":0,"data":[]}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := glpi.NewClient(&config.Config{
		GLPI: config.GLPIConfig{URL: server.URL + "/apirest.php", AppToken: "test-app-token", UserToken: "test-user-token"},
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	handler := createSearchHandler(client)
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "glpi_search",
			Arguments: map[string]interface{}{
				"itemtype": "Computer",
				"criteria": []interface{}{
					map[string]interface{}{
						"field_name": "Computer.name",
						"searchtype": "contains",
						"value":      "pc",
					},
				},
			},
		},
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got error: %+v", result)
	}
	if !searchCalled {
		t.Fatal("expected search endpoint to be called")
	}
}

func TestCreateListFieldsHandler_RegistersAndExecutes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/initSession") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
			return
		}

		if strings.Contains(r.URL.Path, "/listSearchOptions/Computer") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"1":{"uid":"Computer.name","name":"Name","field":"name"}}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := glpi.NewClient(&config.Config{
		GLPI: config.GLPIConfig{URL: server.URL + "/apirest.php", AppToken: "test-app-token", UserToken: "test-user-token"},
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	handler := createListFieldsHandler(client)
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "glpi_list_fields",
			Arguments: map[string]interface{}{
				"itemtype": "Computer",
			},
		},
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got error: %+v", result)
	}

	payload, ok := result.StructuredContent.(*tools.ListFieldsResult)
	if !ok {
		t.Fatalf("expected structured payload type *tools.ListFieldsResult, got %T", result.StructuredContent)
	}

	if payload.ItemType != "Computer" {
		t.Fatalf("expected itemtype Computer, got %q", payload.ItemType)
	}

	if len(payload.Fields) != 1 {
		t.Fatalf("expected 1 field in payload, got %d", len(payload.Fields))
	}

	if payload.Fields[0].ID != 1 {
		t.Fatalf("expected field id 1, got %d", payload.Fields[0].ID)
	}
	if payload.Fields[0].UID != "Computer.name" {
		t.Fatalf("expected field uid Computer.name, got %q", payload.Fields[0].UID)
	}
	if payload.Fields[0].DisplayName != "Name" {
		t.Fatalf("expected display_name Name, got %q", payload.Fields[0].DisplayName)
	}

	if payload.Cached {
		t.Fatalf("expected cached false on first fetch")
	}
}

func TestCreateListFieldsHandler_ReturnsStructuredPayloadOnCacheHit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/initSession") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
			return
		}

		if strings.Contains(r.URL.Path, "/listSearchOptions/Computer") {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"1":{"uid":"Computer.name","name":"Name","field":"name"}}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := glpi.NewClient(&config.Config{
		GLPI: config.GLPIConfig{URL: server.URL + "/apirest.php", AppToken: "test-app-token", UserToken: "test-user-token"},
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	handler := createListFieldsHandler(client)
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "glpi_list_fields",
			Arguments: map[string]interface{}{
				"itemtype": "Computer",
			},
		},
	}

	firstResult, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected first handler error: %v", err)
	}
	if firstResult.IsError {
		t.Fatalf("expected first success result, got error: %+v", firstResult)
	}

	secondResult, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected second handler error: %v", err)
	}
	if secondResult.IsError {
		t.Fatalf("expected second success result, got error: %+v", secondResult)
	}

	payload, ok := secondResult.StructuredContent.(*tools.ListFieldsResult)
	if !ok {
		t.Fatalf("expected structured payload type *tools.ListFieldsResult, got %T", secondResult.StructuredContent)
	}

	if !payload.Cached {
		t.Fatalf("expected cached true on second fetch")
	}
}

func TestCreateSearchHandler_PreservesNumericFieldFastPath(t *testing.T) {
	listOptionsCalls := 0
	searchCalled := false

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/initSession") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
			return
		}

		if strings.Contains(r.URL.Path, "/listSearchOptions/Computer") {
			listOptionsCalls++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"1":{"uid":"Computer.name","name":"Name","field":"name"}}`))
			return
		}

		if strings.Contains(r.URL.Path, "/search/Computer") {
			searchCalled = true
			if !strings.Contains(r.URL.RawQuery, "criteria%5B0%5D%5Bfield%5D=1") {
				t.Errorf("expected numeric field query, got %q", r.URL.RawQuery)
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"totalcount":0,"data":[]}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := glpi.NewClient(&config.Config{
		GLPI: config.GLPIConfig{URL: server.URL + "/apirest.php", AppToken: "test-app-token", UserToken: "test-user-token"},
	})
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	handler := createSearchHandler(client)
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "glpi_search",
			Arguments: map[string]interface{}{
				"itemtype": "Computer",
				"criteria": []interface{}{
					map[string]interface{}{
						"field":      float64(1),
						"field_name": "Computer.name",
						"searchtype": "contains",
						"value":      "pc",
					},
				},
			},
		},
	}

	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	if result.IsError {
		t.Fatalf("expected success result, got error: %+v", result)
	}
	if !searchCalled {
		t.Fatal("expected search endpoint to be called")
	}
	if listOptionsCalls != 0 {
		t.Fatalf("expected zero listSearchOptions calls for numeric fast path, got %d", listOptionsCalls)
	}
}

func TestToolSchemas_IncludeInventoryQueryContracts(t *testing.T) {
	searchTool := newSearchMCPTool()
	criteriaRaw, ok := searchTool.InputSchema.Properties["criteria"]
	if !ok {
		t.Fatalf("expected criteria property in glpi_search schema")
	}

	criteria, ok := criteriaRaw.(map[string]any)
	if !ok {
		t.Fatalf("expected criteria schema object, got %T", criteriaRaw)
	}

	items, ok := criteria["items"].(map[string]any)
	if !ok {
		t.Fatalf("expected criteria.items schema object, got %T", criteria["items"])
	}

	properties, ok := items["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected criteria.items.properties schema object, got %T", items["properties"])
	}

	if _, ok := properties["field_name"]; !ok {
		t.Fatalf("expected field_name property in glpi_search criteria schema")
	}

	listFieldsTool := newListFieldsMCPTool()
	if listFieldsTool.Name != "glpi_list_fields" {
		t.Fatalf("expected glpi_list_fields tool name, got %q", listFieldsTool.Name)
	}

	if _, ok := listFieldsTool.InputSchema.Properties["itemtype"]; !ok {
		t.Fatalf("expected itemtype property in glpi_list_fields schema")
	}
}

func TestGLPIInventorySkill_ContainsCoreGuidance(t *testing.T) {
	skillPath := filepath.Join("..", "..", "skills", "glpi-inventory", "SKILL.md")
	contentBytes, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("expected skill file at %s: %v", skillPath, err)
	}

	content := string(contentBytes)

	requiredPhrases := []string{
		"Discover fields first",
		"glpi_list_fields",
		"Prefer `uid`",
		"glpi_search",
		"field_name",
	}

	for _, phrase := range requiredPhrases {
		if !strings.Contains(content, phrase) {
			t.Fatalf("expected skill to contain phrase %q", phrase)
		}
	}
}
