package glpi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
)

func TestClient_Put(t *testing.T) {
	t.Run("sends PUT request with JSON body", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			// Verify method is PUT
			if r.Method != "PUT" {
				t.Errorf("expected PUT, got %s", r.Method)
			}

			// Verify endpoint path
			if !strings.Contains(r.URL.Path, "/Computer/5") {
				t.Errorf("expected path to contain /Computer/5, got %s", r.URL.Path)
			}

			// Verify Content-Type header
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
			}

			// Verify Session-Token header
			if r.Header.Get("Session-Token") != "test-session" {
				t.Errorf("expected Session-Token header, got %s", r.Header.Get("Session-Token"))
			}

			// Return success response
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      5,
				"message": "Item updated",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       server.URL + "/apirest.php",
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Initialize session first
		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error initializing session: %v", err)
		}

		// Make PUT request
		updateData := map[string]interface{}{
			"input": map[string]interface{}{
				"name": "Updated Computer",
			},
		}

		var result map[string]interface{}
		err = client.Put(context.Background(), "/Computer/5", updateData, &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify response
		if result["id"].(float64) != 5 {
			t.Errorf("expected id 5, got %v", result["id"])
		}
	})

	t.Run("handles 404 not found error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "Item not found"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       server.URL + "/apirest.php",
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error initializing session: %v", err)
		}

		var result map[string]interface{}
		err = client.Put(context.Background(), "/Computer/999", map[string]interface{}{"input": map[string]interface{}{"name": "test"}}, &result)
		if !IsErrNotFound(err) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("auto-reconnects on 401 session expired", func(t *testing.T) {
		initCount := 0
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				initCount++
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "session"})
				return
			}

			requestCount++
			// First request returns 401
			if requestCount == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "session expired"}`))
				return
			}
			// Second request succeeds
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id": 5, "message": "Item updated"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       server.URL + "/apirest.php",
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error initializing session: %v", err)
		}

		var result map[string]interface{}
		err = client.Put(context.Background(), "/Computer/5", map[string]interface{}{"input": map[string]interface{}{"name": "test"}}, &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have reconnected
		if initCount != 2 {
			t.Errorf("expected 2 inits (initial + reconnect), got %d", initCount)
		}
	})
}

func TestClient_Delete(t *testing.T) {
	t.Run("sends DELETE request and returns success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			// Verify method is DELETE
			if r.Method != "DELETE" {
				t.Errorf("expected DELETE, got %s", r.Method)
			}

			// Verify endpoint path
			if !strings.Contains(r.URL.Path, "/Computer/5") {
				t.Errorf("expected path to contain /Computer/5, got %s", r.URL.Path)
			}

			// Verify Session-Token header
			if r.Header.Get("Session-Token") != "test-session" {
				t.Errorf("expected Session-Token header, got %s", r.Header.Get("Session-Token"))
			}

			// Verify App-Token header
			if r.Header.Get("App-Token") != "test-app-token" {
				t.Errorf("expected App-Token header, got %s", r.Header.Get("App-Token"))
			}

			// Return success response
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":      5,
				"message": "Item deleted",
			})
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       server.URL + "/apirest.php",
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error initializing session: %v", err)
		}

		var result map[string]interface{}
		err = client.Delete(context.Background(), "/Computer/5", &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify response
		if result["id"].(float64) != 5 {
			t.Errorf("expected id 5, got %v", result["id"])
		}
	})

	t.Run("handles 404 not found error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error": "Item not found"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       server.URL + "/apirest.php",
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error initializing session: %v", err)
		}

		var result map[string]interface{}
		err = client.Delete(context.Background(), "/Computer/999", &result)
		if !IsErrNotFound(err) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("auto-reconnects on 401 session expired", func(t *testing.T) {
		initCount := 0
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				initCount++
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "session"})
				return
			}

			requestCount++
			// First request returns 401
			if requestCount == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error": "session expired"}`))
				return
			}
			// Second request succeeds
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"id": 5, "message": "Item deleted"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       server.URL + "/apirest.php",
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error initializing session: %v", err)
		}

		var result map[string]interface{}
		err = client.Delete(context.Background(), "/Computer/5", &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have reconnected
		if initCount != 2 {
			t.Errorf("expected 2 inits (initial + reconnect), got %d", initCount)
		}
	})

	t.Run("handles server error on DELETE", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "Database error"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       server.URL + "/apirest.php",
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error initializing session: %v", err)
		}

		var result map[string]interface{}
		err = client.Delete(context.Background(), "/Computer/5", &result)
		if !IsErrServerError(err) {
			t.Errorf("expected ErrServerError, got %v", err)
		}
	})
}
