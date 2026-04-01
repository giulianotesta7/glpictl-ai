package config

import (
	"errors"
	"fmt"
)

// Sentinel errors for config package.
var (
	// ErrNotFound indicates the config file was not found.
	ErrNotFound = errors.New("config: not found")
	// ErrInvalidType indicates a config value has an invalid type.
	ErrInvalidType = errors.New("config: invalid type")
	// ErrMissingRequired indicates a required field is missing.
	ErrMissingRequired = errors.New("config: missing required field")
)

// MissingRequiredError wraps ErrMissingRequired with field context.
type MissingRequiredError struct {
	Field string
}

// Error implements the error interface.
func (e *MissingRequiredError) Error() string {
	return fmt.Sprintf("config: missing required field: %s", e.Field)
}

// Unwrap returns the underlying ErrMissingRequired.
func (e *MissingRequiredError) Unwrap() error {
	return ErrMissingRequired
}

// NewMissingRequiredError creates a new MissingRequiredError.
func NewMissingRequiredError(field string) error {
	return &MissingRequiredError{Field: field}
}
