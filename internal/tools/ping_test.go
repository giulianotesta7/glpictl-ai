package tools

import (
	"context"
	"testing"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// MockClient implements glpi.Clienter interface for testing
type MockClient struct {
	InitSessionFunc    func(ctx context.Context) error
	killSessionFunc    func(ctx context.Context) error
	sessionTokenFunc   func() string
	SessionTokenResult string
	InitSessionError   error
	GLPIURLResult      string
	GLPIVersionResult  string
	GLPIVersionError   error
	GetFunc            func(ctx context.Context, endpoint string, result interface{}) error
	PostFunc           func(ctx context.Context, endpoint string, body interface{}, result interface{}) error
	PutFunc            func(ctx context.Context, endpoint string, body interface{}, result interface{}) error
	DeleteFunc         func(ctx context.Context, endpoint string, result interface{}) error
}

func (m *MockClient) InitSession(ctx context.Context) error {
	if m.InitSessionFunc != nil {
		return m.InitSessionFunc(ctx)
	}
	return m.InitSessionError
}

func (m *MockClient) KillSession(ctx context.Context) error {
	if m.killSessionFunc != nil {
		return m.killSessionFunc(ctx)
	}
	return nil
}

func (m *MockClient) SessionToken() string {
	if m.sessionTokenFunc != nil {
		return m.sessionTokenFunc()
	}
	return m.SessionTokenResult
}

func (m *MockClient) GLPIURL() string {
	return m.GLPIURLResult
}

func (m *MockClient) GetGLPIVersion(ctx context.Context) (string, error) {
	if m.GLPIVersionError != nil {
		return "", m.GLPIVersionError
	}
	return m.GLPIVersionResult, nil
}

func (m *MockClient) Get(ctx context.Context, endpoint string, result interface{}) error {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, endpoint, result)
	}
	return nil
}

func (m *MockClient) Post(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	if m.PostFunc != nil {
		return m.PostFunc(ctx, endpoint, body, result)
	}
	return nil
}

func (m *MockClient) Put(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	if m.PutFunc != nil {
		return m.PutFunc(ctx, endpoint, body, result)
	}
	return nil
}

func (m *MockClient) Delete(ctx context.Context, endpoint string, result interface{}) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, endpoint, result)
	}
	return nil
}

func TestPing(t *testing.T) {
	t.Run("returns GLPI version and connection status on success", func(t *testing.T) {
		mockClient := &MockClient{
			SessionTokenResult: "test-session-token",
			InitSessionFunc: func(ctx context.Context) error {
				return nil
			},
		}

		pingTool, err := NewPingTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating ping tool: %v", err)
		}
		result, err := pingTool.Ping(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Status != "connected" {
			t.Errorf("status = %q, want %q", result.Status, "connected")
		}
		if result.SessionToken != "test-session-token" {
			t.Errorf("session token = %q, want %q", result.SessionToken, "test-session-token")
		}
	})

	t.Run("returns error status when connection fails", func(t *testing.T) {
		mockClient := &MockClient{
			InitSessionError: glpi.NewAuthFailedError("invalid credentials"),
		}

		pingTool, err := NewPingTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating ping tool: %v", err)
		}
		result, err := pingTool.Ping(context.Background())

		if err == nil {
			t.Fatal("expected error, got nil")
		}

		if result.Status != "error" {
			t.Errorf("status = %q, want %q", result.Status, "error")
		}

		if result.Error == "" {
			t.Error("expected error message to be set")
		}
	})

	t.Run("includes GLPI URL and version in result", func(t *testing.T) {
		mockClient := &MockClient{
			SessionTokenResult: "test-session",
			GLPIURLResult:      "http://localhost/apirest.php",
			GLPIVersionResult:  "9.5.7",
		}

		pingTool, err := NewPingTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating ping tool: %v", err)
		}
		result, err := pingTool.Ping(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// The result should indicate success
		if result.Status != "connected" {
			t.Errorf("status = %q, want %q", result.Status, "connected")
		}

		// GLPI URL should be populated
		if result.GLPIURL != "http://localhost/apirest.php" {
			t.Errorf("GLPIURL = %q, want %q", result.GLPIURL, "http://localhost/apirest.php")
		}

		// GLPI Version should be populated
		if result.GLPIVersion != "9.5.7" {
			t.Errorf("GLPIVersion = %q, want %q", result.GLPIVersion, "9.5.7")
		}
	})

	t.Run("handles missing GLPI version gracefully", func(t *testing.T) {
		mockClient := &MockClient{
			SessionTokenResult: "test-session",
			GLPIURLResult:      "http://localhost/apirest.php",
			GLPIVersionResult:  "unknown",
		}

		pingTool, err := NewPingTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating ping tool: %v", err)
		}
		result, err := pingTool.Ping(context.Background())

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if result.Status != "connected" {
			t.Errorf("status = %q, want %q", result.Status, "connected")
		}

		// Should have "unknown" as version when not available
		if result.GLPIVersion != "unknown" {
			t.Errorf("GLPIVersion = %q, want %q", result.GLPIVersion, "unknown")
		}
	})

	t.Run("context cancellation is respected", func(t *testing.T) {
		mockClient := &MockClient{
			InitSessionFunc: func(ctx context.Context) error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				default:
					return nil
				}
			},
		}

		pingTool, err := NewPingTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating ping tool: %v", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err = pingTool.Ping(ctx)
		if err == nil {
			t.Error("expected error for cancelled context")
		}
	})
}

func TestPingResult(t *testing.T) {
	t.Run("PingResult struct is correctly populated", func(t *testing.T) {
		result := PingResult{
			Status:       "connected",
			SessionToken: "abc123",
			GLPIVersion:  "9.5.5",
			GLPIURL:      "http://localhost/apirest.php",
		}

		if result.Status != "connected" {
			t.Errorf("Status = %q, want %q", result.Status, "connected")
		}
		if result.SessionToken != "abc123" {
			t.Errorf("SessionToken = %q, want %q", result.SessionToken, "abc123")
		}
		if result.GLPIVersion != "9.5.5" {
			t.Errorf("GLPIVersion = %q, want %q", result.GLPIVersion, "9.5.5")
		}
		if result.GLPIURL != "http://localhost/apirest.php" {
			t.Errorf("GLPIURL = %q, want %q", result.GLPIURL, "http://localhost/apirest.php")
		}
	})
}

func TestToolRegistry(t *testing.T) {
	t.Run("registers tools correctly", func(t *testing.T) {
		registry := NewToolRegistry()

		mockClient := &MockClient{}
		pingTool, err := NewPingTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating ping tool: %v", err)
		}

		err = registry.Register("ping", pingTool)
		if err != nil {
			t.Errorf("unexpected error registering tool: %v", err)
		}

		if !registry.Has("ping") {
			t.Error("expected registry to have 'ping' tool")
		}
	})

	t.Run("returns false for unregistered tools", func(t *testing.T) {
		registry := NewToolRegistry()

		if registry.Has("nonexistent") {
			t.Error("expected registry to not have 'nonexistent' tool")
		}
	})

	t.Run("lists all registered tools", func(t *testing.T) {
		registry := NewToolRegistry()

		mockClient := &MockClient{}
		pingTool, err := NewPingTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating ping tool: %v", err)
		}

		err = registry.Register("ping", pingTool)
		if err != nil {
			t.Errorf("unexpected error registering tool: %v", err)
		}

		tools := registry.List()
		if len(tools) != 1 {
			t.Errorf("expected 1 tool, got %d", len(tools))
		}
		if tools[0] != "ping" {
			t.Errorf("expected 'ping', got %q", tools[0])
		}
	})

	t.Run("returns error on duplicate registration", func(t *testing.T) {
		registry := NewToolRegistry()

		mockClient := &MockClient{}
		pingTool, err := NewPingTool(mockClient)
		if err != nil {
			t.Fatalf("unexpected error creating ping tool: %v", err)
		}

		err = registry.Register("ping", pingTool)
		if err != nil {
			t.Errorf("unexpected error on first registration: %v", err)
		}

		err = registry.Register("ping", pingTool)
		if err == nil {
			t.Error("expected error on duplicate registration")
		}
	})
}

func TestNewPingTool(t *testing.T) {
	t.Run("creates ping tool with client", func(t *testing.T) {
		mockClient := &MockClient{}

		pingTool, err := NewPingTool(mockClient)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if pingTool == nil {
			t.Fatal("expected non-nil ping tool")
		}
	})

	t.Run("returns error on nil client", func(t *testing.T) {
		pingTool, err := NewPingTool(nil)
		if err == nil {
			t.Error("expected error on nil client")
		}
		if pingTool != nil {
			t.Error("expected nil ping tool on error")
		}
	})
}

// Verify the Clienter interface is satisfied
func TestClienterInterface(t *testing.T) {
	// This test ensures the Client implements the Clienter interface
	var _ ToolClient = (*MockClient)(nil)
	var _ ToolClient = (*glpi.Client)(nil)

	// Create a real client with minimal config to verify interface compliance
	cfg := &config.Config{
		GLPI: config.GLPIConfig{
			URL:       "http://localhost/apirest.php",
			AppToken:  "test",
			UserToken: "test",
		},
	}
	_, err := glpi.NewClient(cfg)
	// We don't care about connection errors, only that NewClient exists
	// and returns something that implements ToolClient
	_ = err // Acknowledge the error to avoid unused variable warning
}
