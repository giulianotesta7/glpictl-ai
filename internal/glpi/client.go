package glpi

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/giulianotesta7/glpictl-ai/internal/config"
)

// Client wraps the GLPI REST API for inventory management.
// It implements lazy session initialization with mutex-protected lazy initialization and auto-reconnect on 401.
type Client struct {
	cfg         *config.Config
	httpClient  *http.Client
	baseURL     string
	appToken    string
	userToken   string
	insecureSSL bool

	// Session state protected by mutex
	mu           sync.Mutex
	sessionToken string

	// Lazy initialization - protected by initMu (separate from mu to avoid blocking requests during init)
	initMu      sync.Mutex
	initialized bool

	// Reconnect tracking - use atomic for thread-safe check-and-increment
	reconnectCount atomic.Int32

	// Logger for debug output
	logger *slog.Logger

	searchOptionsTTL   time.Duration
	searchOptionsCache map[string]cachedSearchOptions
}

// SearchOption represents a normalized GLPI search option.
type SearchOption struct {
	ID          int    `json:"id"`
	UID         string `json:"uid,omitempty"`
	Name        string `json:"name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	DataType    string `json:"datatype,omitempty"`
	Table       string `json:"table,omitempty"`
	Field       string `json:"field,omitempty"`
}

// SearchOptionsResult represents listSearchOptions output with cache metadata.
type SearchOptionsResult struct {
	ItemType string         `json:"itemtype"`
	Fields   []SearchOption `json:"fields"`
	Cached   bool           `json:"cached"`
}

type cachedSearchOptions struct {
	fields    []SearchOption
	fetchedAt time.Time
}

// NewClient creates a new GLPI client.
// It validates the configuration and prepares the client for lazy initialization.
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if cfg.GLPI.URL == "" {
		return nil, fmt.Errorf("GLPI URL is required")
	}

	if cfg.GLPI.AppToken == "" {
		return nil, fmt.Errorf("app token is required")
	}

	if cfg.GLPI.UserToken == "" {
		return nil, fmt.Errorf("user token is required")
	}

	// Configure HTTP client with optional insecure SSL
	httpClient := &http.Client{
		Timeout: time.Duration(cfg.Server.Timeout) * time.Second,
	}

	if cfg.GLPI.InsecureSSL {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	// Set up logger based on log level
	var logger *slog.Logger
	if cfg.Server.LogLevel == "debug" {
		logger = slog.Default()
	} else {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	return &Client{
		cfg:                cfg,
		httpClient:         httpClient,
		baseURL:            strings.TrimSuffix(cfg.GLPI.URL, "/"),
		appToken:           cfg.GLPI.AppToken,
		userToken:          cfg.GLPI.UserToken,
		insecureSSL:        cfg.GLPI.InsecureSSL,
		logger:             logger,
		searchOptionsTTL:   time.Hour,
		searchOptionsCache: make(map[string]cachedSearchOptions),
	}, nil
}

// InsecureSSL returns true if the client is configured to skip SSL verification.
func (c *Client) InsecureSSL() bool {
	return c.insecureSSL
}

// SessionToken returns the current session token, or empty if not initialized.
func (c *Client) SessionToken() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionToken
}

// InitSession authenticates with GLPI and obtains a session token.
// After successful authentication, all subsequent requests will include the session token.
// It is safe to call multiple times - subsequent calls are no-ops if already initialized.
func (c *Client) InitSession(ctx context.Context) error {
	c.initMu.Lock()
	defer c.initMu.Unlock()

	// Check again after acquiring initMu (double-checked locking pattern)
	c.mu.Lock()
	if c.initialized {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	err := c.doInitSession(ctx)
	if err != nil {
		return fmt.Errorf("init session: %w", err)
	}

	c.mu.Lock()
	c.initialized = true
	c.mu.Unlock()

	return nil
}

// doInitSession performs the actual session initialization.
func (c *Client) doInitSession(ctx context.Context) error {
	// Build initSession URL
	initURL := c.baseURL + "/initSession"

	req, err := http.NewRequestWithContext(ctx, "GET", initURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("App-Token", c.appToken)
	req.Header.Set("Authorization", "user_token "+c.userToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized {
			return NewAuthFailedError(string(body))
		}
		return NewServerError(resp.StatusCode, string(body))
	}

	// Parse session token from response
	var result struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if result.SessionToken == "" {
		return NewAuthFailedError("empty session token")
	}

	// Only hold lock for the mutation
	c.mu.Lock()
	c.sessionToken = result.SessionToken
	c.mu.Unlock()
	return nil
}

// Get performs a GET request to the GLPI API.
// It lazily initializes the session if needed and handles auto-reconnect on 401.
func (c *Client) Get(ctx context.Context, endpoint string, result interface{}) error {
	return c.doRequest(ctx, "GET", endpoint, nil, result)
}

// Post performs a POST request to the GLPI API.
// It lazily initializes the session if needed and handles auto-reconnect on 401.
func (c *Client) Post(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, "POST", endpoint, body, result)
}

// Put performs a PUT request to the GLPI API.
// It lazily initializes the session if needed and handles auto-reconnect on 401.
func (c *Client) Put(ctx context.Context, endpoint string, body interface{}, result interface{}) error {
	return c.doRequest(ctx, "PUT", endpoint, body, result)
}

// Delete performs a DELETE request to the GLPI API.
// It lazily initializes the session if needed and handles auto-reconnect on 401.
func (c *Client) Delete(ctx context.Context, endpoint string, result interface{}) error {
	return c.doRequest(ctx, "DELETE", endpoint, nil, result)
}

// doRequest performs an HTTP request with session management.
func (c *Client) doRequest(ctx context.Context, method, endpoint string, requestBody, result interface{}) error {
	// Lazy initialization
	c.mu.Lock()
	if !c.initialized {
		c.mu.Unlock()
		if err := c.InitSession(ctx); err != nil {
			return fmt.Errorf("initialize session for %s %s: %w", method, endpoint, err)
		}
		c.mu.Lock()
	}
	c.mu.Unlock()

	// First attempt
	err := c.doHTTPRequest(ctx, method, endpoint, requestBody, result)
	if err == nil {
		return nil
	}

	// Check if we should retry on 401 (only once)
	if IsErrSessionExpired(err) {
		// Use atomic compare-and-swap for thread-safe check-and-increment
		// This ensures only one goroutine gets to reconnect
		if c.reconnectCount.CompareAndSwap(0, 1) {
			// Clear session and reconnect
			c.mu.Lock()
			c.sessionToken = ""
			c.clearSearchOptionsCacheLocked()
			c.mu.Unlock()

			if err := c.doInitSession(ctx); err != nil {
				return fmt.Errorf("reconnect session for %s %s: %w", method, endpoint, err)
			}

			// Retry the request
			retryErr := c.doHTTPRequest(ctx, method, endpoint, requestBody, result)
			// Reset counter after reconnect attempt regardless of outcome
			c.reconnectCount.Store(0)
			if retryErr != nil {
				return fmt.Errorf("retry request %s %s: %w", method, endpoint, retryErr)
			}
			return nil
		}
		// Another goroutine already reconnected, just return the error
		return fmt.Errorf("request %s %s: %w", method, endpoint, NewSessionExpiredError())
	}

	return fmt.Errorf("request %s %s: %w", method, endpoint, err)
}

// doHTTPRequest performs the actual HTTP request without session management.
func (c *Client) doHTTPRequest(ctx context.Context, method, endpoint string, requestBody, result interface{}) error {
	c.mu.Lock()
	sessionToken := c.sessionToken
	c.mu.Unlock()

	// Build URL
	requestURL := c.baseURL + endpoint

	// Build request body
	var bodyReader io.Reader
	if requestBody != nil {
		bodyBytes, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("App-Token", c.appToken)
	req.Header.Set("Session-Token", sessionToken)
	req.Header.Set("Content-Type", "application/json")

	// Debug logging - redact sensitive tokens
	if c.logger.Enabled(ctx, slog.LevelDebug) {
		redactedToken := ""
		if len(sessionToken) > 8 {
			redactedToken = sessionToken[:4] + "..." + sessionToken[len(sessionToken)-4:]
		}
		c.logger.Debug("HTTP request",
			"method", method,
			"url", requestURL,
			"session_token", redactedToken,
			"has_body", requestBody != nil,
		)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Debug logging for response
	if c.logger.Enabled(ctx, slog.LevelDebug) {
		bodyPreview := string(body)
		if len(bodyPreview) > 500 {
			bodyPreview = bodyPreview[:500] + "... (truncated)"
		}
		c.logger.Debug("HTTP response",
			"status", resp.StatusCode,
			"body", bodyPreview,
		)
	}

	// Handle GLPI's &nbsp; quirk (replace with space for cleaner JSON)
	bodyStr := strings.ReplaceAll(string(body), "&nbsp;", " ")
	body = []byte(bodyStr)

	if resp.StatusCode == http.StatusUnauthorized {
		return NewSessionExpiredError()
	}

	if resp.StatusCode == http.StatusNotFound {
		return NewNotFoundError(endpoint)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return NewRateLimitedError(0) // Could parse Retry-After header
	}

	if resp.StatusCode >= 500 {
		return NewServerError(resp.StatusCode, string(body))
	}

	// GLPI returns 206 for partial content(which is OK)
	if resp.StatusCode >= 400 {
		return NewServerError(resp.StatusCode, string(body))
	}

	// Parse response if result is provided
	if result != nil && len(body) > 0 {
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
	}

	return nil
}

// KillSession terminates the current GLPI session.
// After calling KillSession, the client is no longer authenticated.
func (c *Client) KillSession(ctx context.Context) error {
	c.mu.Lock()
	sessionToken := c.sessionToken
	c.mu.Unlock()

	if sessionToken == "" {
		return fmt.Errorf("no active session to kill")
	}

	killURL := c.baseURL + "/killSession"

	req, err := http.NewRequestWithContext(ctx, "GET", killURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("App-Token", c.appToken)
	req.Header.Set("Session-Token", sessionToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return NewServerError(resp.StatusCode, string(body))
	}

	// Clear session token and reset state
	c.mu.Lock()
	c.sessionToken = ""
	c.initialized = false
	c.clearSearchOptionsCacheLocked()
	c.mu.Unlock()

	// Reset reconnect counter for next session
	c.reconnectCount.Store(0)

	return nil
}

// GetSearchOptions returns normalized searchable fields for the given itemtype.
func (c *Client) GetSearchOptions(ctx context.Context, itemtype string) (*SearchOptionsResult, error) {
	if itemtype == "" {
		return nil, fmt.Errorf("itemtype is required")
	}

	cacheKey := strings.ToLower(itemtype)

	c.mu.Lock()
	if cached, ok := c.searchOptionsCache[cacheKey]; ok && time.Since(cached.fetchedAt) < c.searchOptionsTTL {
		fields := cloneSearchOptions(cached.fields)
		c.mu.Unlock()
		return &SearchOptionsResult{ItemType: itemtype, Fields: fields, Cached: true}, nil
	}
	c.mu.Unlock()

	var raw map[string]interface{}
	endpoint := fmt.Sprintf("/listSearchOptions/%s", itemtype)
	if err := c.Get(ctx, endpoint, &raw); err != nil {
		return nil, fmt.Errorf("get search options for %s: %w", itemtype, err)
	}

	fields := normalizeSearchOptions(raw)

	c.mu.Lock()
	c.searchOptionsCache[cacheKey] = cachedSearchOptions{
		fields:    cloneSearchOptions(fields),
		fetchedAt: time.Now(),
	}
	c.mu.Unlock()

	return &SearchOptionsResult{ItemType: itemtype, Fields: fields, Cached: false}, nil
}

func normalizeSearchOptions(raw map[string]interface{}) []SearchOption {
	fields := make([]SearchOption, 0, len(raw))

	for key, value := range raw {
		id, err := strconv.Atoi(key)
		if err != nil {
			continue
		}

		entry, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		name := getStringValue(entry, "name")
		displayName := name
		if displayName == "" {
			displayName = getStringValue(entry, "field")
		}

		fields = append(fields, SearchOption{
			ID:          id,
			UID:         getStringValue(entry, "uid"),
			Name:        name,
			DisplayName: displayName,
			DataType:    getStringValue(entry, "datatype"),
			Table:       getStringValue(entry, "table"),
			Field:       getStringValue(entry, "field"),
		})
	}

	sort.Slice(fields, func(i, j int) bool {
		return fields[i].ID < fields[j].ID
	})

	return fields
}

func getStringValue(entry map[string]interface{}, key string) string {
	v, ok := entry[key]
	if !ok {
		return ""
	}
	str, ok := v.(string)
	if !ok {
		return ""
	}
	return str
}

func cloneSearchOptions(src []SearchOption) []SearchOption {
	if len(src) == 0 {
		return nil
	}
	dst := make([]SearchOption, len(src))
	copy(dst, src)
	return dst
}

func (c *Client) clearSearchOptionsCacheLocked() {
	if len(c.searchOptionsCache) == 0 {
		return
	}
	c.searchOptionsCache = make(map[string]cachedSearchOptions)
}

// GetGLPIVersion fetches the GLPI version from the API.
// It tries getGlpiConfig first, then falls back to getFullSession.
// Returns "unknown" if the version cannot be determined.
func (c *Client) GetGLPIVersion(ctx context.Context) (string, error) {
	// Try getGlpiConfig endpoint first
	var result struct {
		GLPIVersion string `json:"glpi_version"`
	}

	err := c.Get(ctx, "/getGlpiConfig", &result)
	if err == nil && result.GLPIVersion != "" {
		return result.GLPIVersion, nil
	}

	// Fallback to getFullSession endpoint
	var sessionResult struct {
		GLPIVersion string `json:"glpi_version"`
	}

	err = c.Get(ctx, "/getFullSession", &sessionResult)
	if err == nil && sessionResult.GLPIVersion != "" {
		return sessionResult.GLPIVersion, nil
	}

	// If we can't determine the version, return "unknown"
	return "unknown", nil
}

// GetFullSession fetches the full session information from GLPI.
func (c *Client) GetFullSession(ctx context.Context) (map[string]interface{}, error) {
	var result map[string]interface{}
	err := c.Get(ctx, "/getFullSession", &result)
	if err != nil {
		return nil, fmt.Errorf("failed to get full session: %w", err)
	}
	return result, nil
}

// GLPIURL returns the base GLPI URL.
func (c *Client) GLPIURL() string {
	return c.baseURL
}
