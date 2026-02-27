package mcp

import (
	"errors"
	"fmt"
)

// Error represents an MCP JSON-RPC error response.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string {
	return fmt.Sprintf("MCP error %d: %s", e.Code, e.Message)
}

// ToolError represents a tool execution error (tool ran but returned isError=true).
type ToolError struct {
	Tool    string
	Message string
}

func (e *ToolError) Error() string {
	return fmt.Sprintf("tool %s failed: %s", e.Tool, e.Message)
}

// ServerUnavailableError indicates the MCP server could not be reached.
type ServerUnavailableError struct {
	Endpoint string
	Cause    error
}

func (e *ServerUnavailableError) Error() string {
	return fmt.Sprintf("MCP server unreachable at %s: %v", e.Endpoint, e.Cause)
}

func (e *ServerUnavailableError) Unwrap() error {
	return e.Cause
}

// IsUnauthorized returns true if err is an HTTP 401 response from the MCP server.
func IsUnauthorized(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == 401
	}
	return false
}

// IsForbidden returns true if err is an HTTP 403 response from the MCP server.
func IsForbidden(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == 403
	}
	return false
}

// IsNotFound returns true if err is an HTTP 404 response from the MCP server.
func IsNotFound(err error) bool {
	var e *Error
	if errors.As(err, &e) {
		return e.Code == 404
	}
	return false
}

// IsServerUnavailable returns true if the MCP server could not be reached.
func IsServerUnavailable(err error) bool {
	var e *ServerUnavailableError
	return errors.As(err, &e)
}

// IsToolError returns true if the MCP tool returned an error result.
func IsToolError(err error) bool {
	var e *ToolError
	return errors.As(err, &e)
}
