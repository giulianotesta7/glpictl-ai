package config

import (
	"errors"
	"testing"
)

func TestConfigErrors(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		wantMsg string
	}{
		{
			name:    "ErrNotFound has correct message",
			err:     ErrNotFound,
			wantMsg: "config: not found",
		},
		{
			name:    "ErrInvalidType has correct message",
			err:     ErrInvalidType,
			wantMsg: "config: invalid type",
		},
		{
			name:    "ErrMissingRequired has correct message",
			err:     ErrMissingRequired,
			wantMsg: "config: missing required field",
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

func TestMissingRequiredError(t *testing.T) {
	tests := []struct {
		name    string
		field   string
		wantMsg string
	}{
		{
			name:    "missing GLPI URL",
			field:   "glpi.url",
			wantMsg: "config: missing required field: glpi.url",
		},
		{
			name:    "missing app token",
			field:   "glpi.app_token",
			wantMsg: "config: missing required field: glpi.app_token",
		},
		{
			name:    "missing user token",
			field:   "glpi.user_token",
			wantMsg: "config: missing required field: glpi.user_token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewMissingRequiredError(tt.field)
			if !errors.Is(err, ErrMissingRequired) {
				t.Error("error should wrap ErrMissingRequired")
			}
			if err.Error() != tt.wantMsg {
				t.Errorf("error message = %q, want %q", err.Error(), tt.wantMsg)
			}
		})
	}
}
