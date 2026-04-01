package glpi

import (
	"errors"
	"fmt"
)

// Sentinel errors for GLPI client package.
var (
	// ErrAuthFailed indicates authentication failure.
	ErrAuthFailed = errors.New("glpi: authentication failed")
	// ErrSessionExpired indicates the session token has expired.
	ErrSessionExpired = errors.New("glpi: session expired")
	// ErrNotFound indicates the requested resource was not found.
	ErrNotFound = errors.New("glpi: not found")
	// ErrRateLimited indicates rate limiting is active.
	ErrRateLimited = errors.New("glpi: rate limited")
	// ErrServerError indicates a server-side error.
	ErrServerError = errors.New("glpi: server error")
)

// AuthFailedError wraps ErrAuthFailed with additional context.
type AuthFailedError struct {
	Reason string
}

// Error implements the error interface.
func (e *AuthFailedError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("glpi: authentication failed: %s", e.Reason)
	}
	return "glpi: authentication failed"
}

// Unwrap returns the underlying ErrAuthFailed.
func (e *AuthFailedError) Unwrap() error {
	return ErrAuthFailed
}

// NewAuthFailedError creates a new AuthFailedError.
func NewAuthFailedError(reason string) error {
	return &AuthFailedError{Reason: reason}
}

// ServerError wraps ErrServerError with status code and body.
type ServerError struct {
	StatusCode int
	Body       string
}

// Error implements the error interface.
func (e *ServerError) Error() string {
	return fmt.Sprintf("glpi: server error (%d): %s", e.StatusCode, e.Body)
}

// Unwrap returns the underlying ErrServerError.
func (e *ServerError) Unwrap() error {
	return ErrServerError
}

// NewServerError creates a new ServerError.
func NewServerError(statusCode int, body string) error {
	return &ServerError{StatusCode: statusCode, Body: body}
}

// RateLimitedError wraps ErrRateLimited with retry-after information.
type RateLimitedError struct {
	RetryAfter int
}

// Error implements the error interface.
func (e *RateLimitedError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("glpi: rate limited, retry after %ds", e.RetryAfter)
	}
	return "glpi: rate limited"
}

// Unwrap returns the underlying ErrRateLimited.
func (e *RateLimitedError) Unwrap() error {
	return ErrRateLimited
}

// NewRateLimitedError creates a new RateLimitedError.
func NewRateLimitedError(retryAfter int) error {
	return &RateLimitedError{RetryAfter: retryAfter}
}

// SessionExpiredError wraps ErrSessionExpired.
type SessionExpiredError struct{}

// Error implements the error interface.
func (e *SessionExpiredError) Error() string {
	return "glpi: session expired"
}

// Unwrap returns the underlying ErrSessionExpired.
func (e *SessionExpiredError) Unwrap() error {
	return ErrSessionExpired
}

// NewSessionExpiredError creates a new SessionExpiredError.
func NewSessionExpiredError() error {
	return &SessionExpiredError{}
}

// NotFoundError wraps ErrNotFound with resource context.
type NotFoundError struct {
	Resource string
}

// Error implements the error interface.
func (e *NotFoundError) Error() string {
	if e.Resource != "" {
		return fmt.Sprintf("glpi: not found: %s", e.Resource)
	}
	return "glpi: not found"
}

// Unwrap returns the underlying ErrNotFound.
func (e *NotFoundError) Unwrap() error {
	return ErrNotFound
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(resource string) error {
	return &NotFoundError{Resource: resource}
}

// IsErrAuthFailed returns true if err is ErrAuthFailed.
func IsErrAuthFailed(err error) bool {
	return errors.Is(err, ErrAuthFailed)
}

// IsErrSessionExpired returns true if err is ErrSessionExpired.
func IsErrSessionExpired(err error) bool {
	return errors.Is(err, ErrSessionExpired)
}

// IsErrNotFound returns true if err is ErrNotFound.
func IsErrNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsErrRateLimited returns true if err is ErrRateLimited.
func IsErrRateLimited(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// IsErrServerError returns true if err is ErrServerError.
func IsErrServerError(err error) bool {
	return errors.Is(err, ErrServerError)
}
