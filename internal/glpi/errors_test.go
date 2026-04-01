package glpi

import (
	"errors"
	"testing"
)

func TestGLErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "ErrAuthFailed has correct message",
			err:     ErrAuthFailed,
			wantMsg: "glpi: authentication failed",
		},
		{
			name:    "ErrSessionExpired has correct message",
			err:     ErrSessionExpired,
			wantMsg: "glpi: session expired",
		},
		{
			name:    "ErrNotFound has correct message",
			err:     ErrNotFound,
			wantMsg: "glpi: not found",
		},
		{
			name:    "ErrRateLimited has correct message",
			err:     ErrRateLimited,
			wantMsg: "glpi: rate limited",
		},
		{
			name:    "ErrServerError has correct message",
			err:     ErrServerError,
			wantMsg: "glpi: server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("error message = %q, want %q", tt.err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestAuthFailedError(t *testing.T) {
	tests := []struct {
		name    string
		reason  string
		wantMsg string
	}{
		{
			name:    "auth failed with reason",
			reason:  "invalid credentials",
			wantMsg: "glpi: authentication failed: invalid credentials",
		},
		{
			name:    "auth failed with empty reason",
			reason:  "",
			wantMsg: "glpi: authentication failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewAuthFailedError(tt.reason)
			if !errors.Is(err, ErrAuthFailed) {
				t.Error("error should wrap ErrAuthFailed")
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("error message = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestServerError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantMsg    string
	}{
		{
			name:       "server error 500",
			statusCode: 500,
			body:       "internal error",
			wantMsg:    "glpi: server error (500): internal error",
		},
		{
			name:       "server error 502",
			statusCode: 502,
			body:       "bad gateway",
			wantMsg:    "glpi: server error (502): bad gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewServerError(tt.statusCode, tt.body)
			if !errors.Is(err, ErrServerError) {
				t.Error("error should wrap ErrServerError")
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("error message = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestRateLimitedError(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter int
		wantMsg    string
	}{
		{
			name:       "rate limited with retry",
			retryAfter: 60,
			wantMsg:    "glpi: rate limited, retry after 60s",
		},
		{
			name:       "rate limited without retry",
			retryAfter: 0,
			wantMsg:    "glpi: rate limited",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewRateLimitedError(tt.retryAfter)
			if !errors.Is(err, ErrRateLimited) {
				t.Error("error should wrap ErrRateLimited")
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("error message = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestSessionExpiredError(t *testing.T) {
	err := NewSessionExpiredError()
	if !errors.Is(err, ErrSessionExpired) {
		t.Error("error should wrap ErrSessionExpired")
	}
	if err.Error() != "glpi: session expired" {
		t.Errorf("error message = %q, want %q", err.Error(), "glpi: session expired")
	}
}

func TestNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		resource string
		wantMsg  string
	}{
		{
			name:     "not found with resource",
			resource: "Computer/123",
			wantMsg:  "glpi: not found: Computer/123",
		},
		{
			name:     "not found without resource",
			resource: "",
			wantMsg:  "glpi: not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewNotFoundError(tt.resource)
			if !errors.Is(err, ErrNotFound) {
				t.Error("error should wrap ErrNotFound")
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("error message = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestIsErrAuthFailed(t *testing.T) {
	t.Run("returns true for AuthFailedError", func(t *testing.T) {
		if !IsErrAuthFailed(NewAuthFailedError("test")) {
			t.Error("expected IsErrAuthFailed to be true")
		}
	})

	t.Run("returns true for ErrAuthFailed", func(t *testing.T) {
		if !IsErrAuthFailed(ErrAuthFailed) {
			t.Error("expected IsErrAuthFailed(ErrAuthFailed) to be true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsErrAuthFailed(errors.New("other")) {
			t.Error("expected IsErrAuthFailed(other error) to be false")
		}
	})
}

func TestIsErrSessionExpired(t *testing.T) {
	t.Run("returns true for SessionExpiredError", func(t *testing.T) {
		if !IsErrSessionExpired(NewSessionExpiredError()) {
			t.Error("expected IsErrSessionExpired to be true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsErrSessionExpired(errors.New("other")) {
			t.Error("expected IsErrSessionExpired(other error) to be false")
		}
	})
}

func TestIsErrNotFound(t *testing.T) {
	t.Run("returns true for NotFoundError", func(t *testing.T) {
		if !IsErrNotFound(NewNotFoundError("test")) {
			t.Error("expected IsErrNotFound to be true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsErrNotFound(errors.New("other")) {
			t.Error("expected IsErrNotFound(other error) to be false")
		}
	})
}

func TestIsErrRateLimited(t *testing.T) {
	t.Run("returns true for RateLimitedError", func(t *testing.T) {
		if !IsErrRateLimited(NewRateLimitedError(60)) {
			t.Error("expected IsErrRateLimited to be true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsErrRateLimited(errors.New("other")) {
			t.Error("expected IsErrRateLimited(other error) to be false")
		}
	})
}

func TestIsErrServerError(t *testing.T) {
	t.Run("returns true for ServerError", func(t *testing.T) {
		if !IsErrServerError(NewServerError(500, "error")) {
			t.Error("expected IsErrServerError to be true")
		}
	})

	t.Run("returns false for other errors", func(t *testing.T) {
		if IsErrServerError(errors.New("other")) {
			t.Error("expected IsErrServerError(other error) to be false")
		}
	})
}
