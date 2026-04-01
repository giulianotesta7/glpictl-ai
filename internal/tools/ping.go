package tools

import (
	"context"
	"fmt"

	"github.com/giulianotesta7/glpictl-ai/internal/glpi"
)

// ToolClient defines the interface for GLPI client operations needed by tools.
type ToolClient interface {
	InitSession(ctx context.Context) error
	KillSession(ctx context.Context) error
	SessionToken() string
	GLPIURL() string
	GetGLPIVersion(ctx context.Context) (string, error)
	Get(ctx context.Context, endpoint string, result interface{}) error
	Post(ctx context.Context, endpoint string, body interface{}, result interface{}) error
	Put(ctx context.Context, endpoint string, body interface{}, result interface{}) error
	Delete(ctx context.Context, endpoint string, result interface{}) error
}

// PingResult represents the result of a ping operation.
type PingResult struct {
	Status       string `json:"status"`
	SessionToken string `json:"session_token,omitempty"`
	GLPIVersion  string `json:"glpi_version,omitempty"`
	GLPIURL      string `json:"glpi_url,omitempty"`
	Error        string `json:"error,omitempty"`
}

// PingTool provides the ping functionality for testing GLPI connection.
type PingTool struct {
	client ToolClient
}

// NewPingTool creates a new ping tool with the given client.
// Returns an error if the client is nil.
func NewPingTool(client ToolClient) (*PingTool, error) {
	if client == nil {
		return nil, fmt.Errorf("ping tool: client cannot be nil")
	}
	return &PingTool{client: client}, nil
}

// Ping tests the connection to GLPI and returns connection status.
// It initializes a session and returns the session token as proof of connection.
func (p *PingTool) Ping(ctx context.Context) (*PingResult, error) {
	err := p.client.InitSession(ctx)
	if err != nil {
		return &PingResult{
			Status: "error",
			Error:  err.Error(),
		}, err
	}

	sessionToken := p.client.SessionToken()
	if sessionToken == "" {
		return &PingResult{
			Status: "error",
			Error:  "empty session token received",
		}, fmt.Errorf("empty session token")
	}

	// Get GLPI version
	glpiVersion, _ := p.client.GetGLPIVersion(ctx)
	// If GetGLPIVersion fails, we still return success but with "unknown" version

	return &PingResult{
		Status:       "connected",
		SessionToken: sessionToken,
		GLPIURL:      p.client.GLPIURL(),
		GLPIVersion:  glpiVersion,
	}, nil
}

// Name returns the tool name for registration.
func (p *PingTool) Name() string {
	return "ping"
}

// Description returns the tool description.
func (p *PingTool) Description() string {
	return "Test GLPI connection and return session status"
}

// Ensure PingTool implements the Tool interface
var _ Tool = (*PingTool)(nil)

// Tool defines the interface for MCP tools.
type Tool interface {
	Name() string
	Description() string
}

// Execute runs the ping operation (implements mcp.ToolHandler).
func (p *PingTool) Execute(ctx context.Context) (*PingResult, error) {
	return p.Ping(ctx)
}

// IsErrAuthFailed checks if the error is an authentication failure.
func IsErrAuthFailed(err error) bool {
	return glpi.IsErrAuthFailed(err)
}

// IsErrSessionExpired checks if the error is a session expiration.
func IsErrSessionExpired(err error) bool {
	return glpi.IsErrSessionExpired(err)
}
