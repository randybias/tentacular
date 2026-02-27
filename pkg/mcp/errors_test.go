package mcp

import (
	"errors"
	"testing"
)

func TestErrorString(t *testing.T) {
	e := &Error{Code: 500, Message: "internal error"}
	want := "MCP error 500: internal error"
	if e.Error() != want {
		t.Errorf("got %q, want %q", e.Error(), want)
	}
}

func TestToolErrorString(t *testing.T) {
	e := &ToolError{Tool: "wf_apply", Message: "namespace not found"}
	want := "tool wf_apply failed: namespace not found"
	if e.Error() != want {
		t.Errorf("got %q, want %q", e.Error(), want)
	}
}

func TestServerUnavailableErrorString(t *testing.T) {
	cause := errors.New("connection refused")
	e := &ServerUnavailableError{Endpoint: "http://mcp:8080", Cause: cause}
	s := e.Error()
	if s == "" {
		t.Error("expected non-empty error string")
	}
	if !containsString(s, "http://mcp:8080") {
		t.Errorf("expected endpoint in error string, got %q", s)
	}
	if !containsString(s, "connection refused") {
		t.Errorf("expected cause in error string, got %q", s)
	}
}

func TestServerUnavailableErrorUnwrap(t *testing.T) {
	cause := errors.New("dial tcp: connection refused")
	e := &ServerUnavailableError{Endpoint: "http://mcp:8080", Cause: cause}
	if !errors.Is(e, cause) {
		t.Error("expected errors.Is to match through Unwrap")
	}
}

func TestIsNotFound_True(t *testing.T) {
	err := &Error{Code: 404, Message: "not found"}
	if !IsNotFound(err) {
		t.Error("expected IsNotFound to return true for 404 Error")
	}
}

func TestIsNotFound_False_WrongCode(t *testing.T) {
	err := &Error{Code: 403, Message: "forbidden"}
	if IsNotFound(err) {
		t.Error("expected IsNotFound to return false for 403 Error")
	}
}

func TestIsNotFound_False_WrongType(t *testing.T) {
	err := errors.New("some other error")
	if IsNotFound(err) {
		t.Error("expected IsNotFound to return false for non-Error type")
	}
}

func TestIsUnauthorized_True(t *testing.T) {
	err := &Error{Code: 401, Message: "unauthorized"}
	if !IsUnauthorized(err) {
		t.Error("expected IsUnauthorized to return true for 401")
	}
}

func TestIsUnauthorized_False(t *testing.T) {
	if IsUnauthorized(errors.New("random error")) {
		t.Error("expected IsUnauthorized false for non-Error type")
	}
}

func TestIsForbidden_True(t *testing.T) {
	err := &Error{Code: 403, Message: "forbidden"}
	if !IsForbidden(err) {
		t.Error("expected IsForbidden to return true for 403")
	}
}

func TestIsForbidden_False(t *testing.T) {
	if IsForbidden(errors.New("random error")) {
		t.Error("expected IsForbidden false for non-Error type")
	}
}

func TestIsServerUnavailable_True(t *testing.T) {
	err := &ServerUnavailableError{Endpoint: "x", Cause: errors.New("refused")}
	if !IsServerUnavailable(err) {
		t.Error("expected IsServerUnavailable true")
	}
}

func TestIsServerUnavailable_False(t *testing.T) {
	if IsServerUnavailable(errors.New("other")) {
		t.Error("expected IsServerUnavailable false for other errors")
	}
}

func TestIsToolError_True(t *testing.T) {
	err := &ToolError{Tool: "wf_run", Message: "failed"}
	if !IsToolError(err) {
		t.Error("expected IsToolError true")
	}
}

func TestIsToolError_False(t *testing.T) {
	if IsToolError(errors.New("other")) {
		t.Error("expected IsToolError false for other errors")
	}
}

func TestIsNilError(t *testing.T) {
	// All Is* functions must handle nil gracefully
	if IsUnauthorized(nil) {
		t.Error("IsUnauthorized(nil) should be false")
	}
	if IsForbidden(nil) {
		t.Error("IsForbidden(nil) should be false")
	}
	if IsNotFound(nil) {
		t.Error("IsNotFound(nil) should be false")
	}
	if IsServerUnavailable(nil) {
		t.Error("IsServerUnavailable(nil) should be false")
	}
	if IsToolError(nil) {
		t.Error("IsToolError(nil) should be false")
	}
}
