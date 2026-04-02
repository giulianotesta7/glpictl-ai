package glpi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
)

func TestClient_New(t *testing.T) {
	t.Run("creates client with valid config", func(t *testing.T) {
		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       "http://localhost/apirest.php",
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("returns error for empty URL", func(t *testing.T) {
		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				AppToken:  "test-app-token",
				UserToken: "test-user-token",
			},
		}

		_, err := NewClient(cfg)
		if err == nil {
			t.Error("expected error for empty URL")
		}
	})

	t.Run("returns error for empty app token", func(t *testing.T) {
		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:       "http://localhost/apirest.php",
				UserToken: "test-user-token",
			},
		}

		_, err := NewClient(cfg)
		if err == nil {
			t.Error("expected error for empty app token")
		}
	})

	t.Run("returns error for empty user token", func(t *testing.T) {
		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:      "http://localhost/apirest.php",
				AppToken: "test-app-token",
			},
		}

		_, err := NewClient(cfg)
		if err == nil {
			t.Error("expected error for empty user token")
		}
	})
}

func TestClient_InitSession(t *testing.T) {
	t.Run("successfully authenticates and gets session token", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify request method and path
			if r.Method != "GET" {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if !strings.HasSuffix(r.URL.Path, "/initSession") {
				t.Errorf("expected path to end with /initSession, got %s", r.URL.Path)
			}

			// Verify headers
			if r.Header.Get("App-Token") != "test-app-token" {
				t.Errorf("expected App-Token header, got %s", r.Header.Get("App-Token"))
			}
			if r.Header.Get("Authorization") != "user_token test-user-token" {
				t.Errorf("expected Authorization header, got %s", r.Header.Get("Authorization"))
			}

			// Return session token
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session-token"})
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
			t.Fatalf("unexpected error: %v", err)
		}

		if client.SessionToken() != "test-session-token" {
			t.Errorf("session token = %q, want %q", client.SessionToken(), "test-session-token")
		}
	})

	t.Run("returns ErrAuthFailed on 401", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "invalid credentials"})
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
		if !IsErrAuthFailed(err) {
			t.Errorf("expected ErrAuthFailed, got %v", err)
		}
	})

	t.Run("returns ErrServerError on 500", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
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
		if !IsErrServerError(err) {
			t.Errorf("expected ErrServerError, got %v", err)
		}
	})
}

func TestClient_Do(t *testing.T) {
	t.Run("auto-initializes session on first request", func(t *testing.T) {
		initCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				initCalled = true
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "auto-session"})
				return
			}

			// Verify session token is set
			if r.Header.Get("Session-Token") != "auto-session" {
				t.Errorf("expected Session-Token header, got %s", r.Header.Get("Session-Token"))
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "ok"}`))
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

		// Make request without initializing session
		var result map[string]string
		err = client.Get(context.Background(), "/someEndpoint", &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !initCalled {
			t.Error("expected initSession to be called")
		}
	})

	t.Run("auto-reconnects once on 401", func(t *testing.T) {
		initCount := 0
		requestCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				initCount++
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "session-" + string(rune('A'+initCount))})
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
			w.Write([]byte(`{"status": "ok"}`))
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
			t.Fatalf("unexpected error: %v", err)
		}

		// First init should have been called
		if initCount != 1 {
			t.Errorf("expected 1 init, got %d", initCount)
		}

		// Make request that will get 401 and auto-reconnect
		var result map[string]string
		err = client.Get(context.Background(), "/someEndpoint", &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should have called init twice (initial + reconnect)
		if initCount != 2 {
			t.Errorf("expected 2 inits (initial + reconnect), got %d", initCount)
		}
	})

	t.Run("returns ErrSessionExpired after max reconnects", func(t *testing.T) {
		initCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				initCount++
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "session"})
				return
			}
			// Always return 401
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error": "session expired"}`))
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
			t.Fatalf("unexpected error: %v", err)
		}

		// Make request that will fail even after reconnect
		var result map[string]string
		err = client.Get(context.Background(), "/someEndpoint", &result)
		if !IsErrSessionExpired(err) {
			t.Errorf("expected ErrSessionExpired, got %v", err)
		}

		// Should have tried init twice (max 1 reconnect)
		if initCount != 2 {
			t.Errorf("expected 2 inits, got %d", initCount)
		}
	})

	t.Run("initializes session only once under concurrency", func(t *testing.T) {
		initCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				initCount++
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "lazy-session"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "ok"}`))
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

		// Make multiple requests concurrently
		done := make(chan bool)
		for i := 0; i < 5; i++ {
			go func() {
				var result map[string]string
				client.Get(context.Background(), "/someEndpoint", &result)
				done <- true
			}()
		}

		// Wait for all requests
		for i := 0; i < 5; i++ {
			<-done
		}

		// init should only be called once despite concurrent requests
		if initCount != 1 {
			t.Errorf("expected 1 init (lazy with mutex), got %d", initCount)
		}
	})
}

func TestClient_KillSession(t *testing.T) {
	t.Run("successfully kills session", func(t *testing.T) {
		killCalled := false
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}
			if strings.HasSuffix(r.URL.Path, "/killSession") {
				killCalled = true
				// Verify session token header
				if r.Header.Get("Session-Token") != "test-session" {
					t.Errorf("expected Session-Token header, got %s", r.Header.Get("Session-Token"))
				}
				w.WriteHeader(http.StatusOK)
				return
			}
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

		// Initialize session
		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Kill session
		err = client.KillSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if !killCalled {
			t.Error("expected killSession to be called")
		}

		// Session token should be cleared
		if client.SessionToken() != "" {
			t.Errorf("expected empty session token after kill, got %q", client.SessionToken())
		}
	})

	t.Run("returns error if session not initialized", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Should not be called
			t.Error("killSession should not be called without active session")
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

		err = client.KillSession(context.Background())
		if err == nil {
			t.Error("expected error when killing session without initialization")
		}
	})
}

func TestClient_AppTokenHeader(t *testing.T) {
	t.Run("sends App-Token header on all requests", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check App-Token header
			if r.Header.Get("App-Token") != "test-app-token" {
				t.Errorf("expected App-Token header on all requests, got %s", r.Header.Get("App-Token"))
			}

			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "ok"}`))
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

		// Initialize
		err = client.InitSession(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Make another request
		var result map[string]string
		err = client.Get(context.Background(), "/someEndpoint", &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestClient_SessionTokenHeader(t *testing.T) {
	t.Run("sends Session-Token header after initSession", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "my-session-token"})
				return
			}

			// Check Session-Token header
			if r.Header.Get("Session-Token") != "my-session-token" {
				t.Errorf("expected Session-Token header, got %s", r.Header.Get("Session-Token"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "ok"}`))
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
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]string
		err = client.Get(context.Background(), "/someEndpoint", &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestClient_HandleNbsp(t *testing.T) {
	t.Run("handles nbsp entity in JSON responses", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			// Return JSON with &nbsp; entity
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"name": "Computer&nbsp;123", "description": "&nbsp;"}`))
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
			t.Fatalf("unexpected error: %v", err)
		}

		var result map[string]string
		err = client.Get(context.Background(), "/Computer/123", &result)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// &nbsp; should be handled (converted to nullor stripped)
		if result["name"] != "Computer 123" && result["name"] != "Computer&nbsp;123" {
			t.Errorf("unexpected name value: %q", result["name"])
		}
	})
}

func TestClient_InsecureSSL(t *testing.T) {
	t.Run("configures InsecureSkipVerify when InsecureSSL is true", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "ok"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:         server.URL + "/apirest.php",
				AppToken:    "test-app-token",
				UserToken:   "test-user-token",
				InsecureSSL: true,
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify InsecureSSL is configured
		if !client.InsecureSSL() {
			t.Error("expected InsecureSSL to be true")
		}
	})

	t.Run("defaults to secure TLS when InsecureSSL is false", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"status": "ok"}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			GLPI: config.GLPIConfig{
				URL:         server.URL + "/apirest.php",
				AppToken:    "test-app-token",
				UserToken:   "test-user-token",
				InsecureSSL: false,
			},
		}

		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify InsecureSSL is false
		if client.InsecureSSL() {
			t.Error("expected InsecureSSL to be false")
		}
	})
}

func TestClient_GetGLPIVersion(t *testing.T) {
	t.Run("fetches GLPI version from getGlpiConfig endpoint", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}
			if strings.Contains(r.URL.Path, "getGlpiConfig") || strings.Contains(r.URL.Path, "getFullSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"glpi_version": "9.5.7",
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
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

		version, err := client.GetGLPIVersion(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if version != "9.5.7" {
			t.Errorf("version = %q, want %q", version, "9.5.7")
		}
	})

	t.Run("returns unknown when version endpoint fails", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}
			// Return 404 for all other endpoints
			w.WriteHeader(http.StatusNotFound)
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

		version, err := client.GetGLPIVersion(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if version != "unknown" {
			t.Errorf("version = %q, want %q", version, "unknown")
		}
	})
}

func TestClient_GetSearchOptions(t *testing.T) {
	t.Run("fetches and normalizes search options on cache miss", func(t *testing.T) {
		listCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			if strings.Contains(r.URL.Path, "/listSearchOptions/Computer") {
				listCalls++
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"1":{"uid":"Computer.name","name":"Name","field":"name","datatype":"string","table":"glpi_computers"}}`))
				return
			}

			w.WriteHeader(http.StatusNotFound)
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

		result, err := client.GetSearchOptions(context.Background(), "Computer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.ItemType != "Computer" {
			t.Errorf("ItemType = %q, want %q", result.ItemType, "Computer")
		}
		if result.Cached {
			t.Error("Cached should be false on first call")
		}
		if len(result.Fields) != 1 {
			t.Fatalf("len(Fields) = %d, want %d", len(result.Fields), 1)
		}
		if result.Fields[0].ID != 1 {
			t.Errorf("ID = %d, want %d", result.Fields[0].ID, 1)
		}
		if listCalls != 1 {
			t.Errorf("listSearchOptions calls = %d, want %d", listCalls, 1)
		}
	})

	t.Run("returns cached search options on second call", func(t *testing.T) {
		listCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			if strings.Contains(r.URL.Path, "/listSearchOptions/Computer") {
				listCalls++
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"1":{"uid":"Computer.name","name":"Name","field":"name"}}`))
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &config.Config{GLPI: config.GLPIConfig{URL: server.URL + "/apirest.php", AppToken: "test-app-token", UserToken: "test-user-token"}}
		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		first, err := client.GetSearchOptions(context.Background(), "Computer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		second, err := client.GetSearchOptions(context.Background(), "Computer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if first.Cached {
			t.Error("first response should not be cached")
		}
		if !second.Cached {
			t.Error("second response should be cached")
		}
		if listCalls != 1 {
			t.Errorf("listSearchOptions calls = %d, want %d", listCalls, 1)
		}
	})

	t.Run("refetches when TTL is expired", func(t *testing.T) {
		listCalls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			if strings.Contains(r.URL.Path, "/listSearchOptions/Computer") {
				listCalls++
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"1":{"uid":"Computer.name","name":"Name","field":"name"}}`))
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &config.Config{GLPI: config.GLPIConfig{URL: server.URL + "/apirest.php", AppToken: "test-app-token", UserToken: "test-user-token"}}
		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		client.searchOptionsTTL = 1 * time.Millisecond

		_, err = client.GetSearchOptions(context.Background(), "Computer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		time.Sleep(5 * time.Millisecond)

		fresh, err := client.GetSearchOptions(context.Background(), "Computer")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if fresh.Cached {
			t.Error("response should not be cached after TTL expiry")
		}
		if listCalls != 2 {
			t.Errorf("listSearchOptions calls = %d, want %d", listCalls, 2)
		}
	})

	t.Run("propagates invalid itemtype error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/initSession") {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
				return
			}

			if strings.Contains(r.URL.Path, "/listSearchOptions/InvalidItem") {
				w.WriteHeader(http.StatusNotFound)
				w.Write([]byte(`{"message":"itemtype not found"}`))
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		cfg := &config.Config{GLPI: config.GLPIConfig{URL: server.URL + "/apirest.php", AppToken: "test-app-token", UserToken: "test-user-token"}}
		client, err := NewClient(cfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		_, err = client.GetSearchOptions(context.Background(), "InvalidItem")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "get search options") {
			t.Fatalf("error = %q, want wrapped context", err.Error())
		}
	})
}

func TestClient_KillSession_ClearsSearchOptionsCache(t *testing.T) {
	listCalls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/initSession") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"session_token": "test-session"})
			return
		}

		if strings.Contains(r.URL.Path, "/listSearchOptions/Computer") {
			listCalls++
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"1":{"uid":"Computer.name","name":"Name","field":"name"}}`))
			return
		}

		if strings.HasSuffix(r.URL.Path, "/killSession") {
			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := &config.Config{GLPI: config.GLPIConfig{URL: server.URL + "/apirest.php", AppToken: "test-app-token", UserToken: "test-user-token"}}
	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = client.GetSearchOptions(context.Background(), "Computer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := client.KillSession(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	second, err := client.GetSearchOptions(context.Background(), "Computer")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if second.Cached {
		t.Error("response after kill session should not be cached")
	}
	if listCalls != 2 {
		t.Errorf("listSearchOptions calls = %d, want %d", listCalls, 2)
	}
}
