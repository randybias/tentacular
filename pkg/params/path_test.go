// Unit tests for the path expression parser (ParsePath).
//
// Covers all 13 cases from design doc Section 12.1:
//   - Simple key
//   - Dotted path (2 segments)
//   - Deep dotted path (4 segments)
//   - Filtered segment
//   - Filtered with continuation
//   - Key with hyphens
//   - Key with underscores
//   - Empty string (error)
//   - Missing closing bracket (error)
//   - Missing equals in filter (error)
//   - Leading dot (error)
//   - Trailing dot (error)
//   - Double dot / empty segment (error)

package params

import (
	"testing"
)

// TestParsePathSimpleKey verifies that a single key produces one segment
// with no filter.
func TestParsePathSimpleKey(t *testing.T) {
	segs, err := ParsePath("config")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Key != "config" {
		t.Errorf("Key: got %q, want %q", segs[0].Key, "config")
	}
	if segs[0].FilterField != "" {
		t.Errorf("FilterField: got %q, want empty", segs[0].FilterField)
	}
}

// TestParsePathDottedPath verifies that a two-segment dotted path is parsed
// into exactly 2 segments.
func TestParsePathDottedPath(t *testing.T) {
	segs, err := ParsePath("config.endpoints")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if segs[0].Key != "config" {
		t.Errorf("segment[0].Key: got %q, want %q", segs[0].Key, "config")
	}
	if segs[1].Key != "endpoints" {
		t.Errorf("segment[1].Key: got %q, want %q", segs[1].Key, "endpoints")
	}
}

// TestParsePathDeepDottedPath verifies that a four-segment path is parsed
// into exactly 4 segments with correct keys.
func TestParsePathDeepDottedPath(t *testing.T) {
	segs, err := ParsePath("config.storage.s3.bucket")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 4 {
		t.Fatalf("expected 4 segments, got %d", len(segs))
	}
	expected := []string{"config", "storage", "s3", "bucket"}
	for i, want := range expected {
		if segs[i].Key != want {
			t.Errorf("segment[%d].Key: got %q, want %q", i, segs[i].Key, want)
		}
	}
}

// TestParsePathFilteredSegment verifies that a filtered segment is parsed with
// Key, FilterField, and FilterValue populated correctly.
func TestParsePathFilteredSegment(t *testing.T) {
	segs, err := ParsePath("triggers[name=check-endpoints]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Key != "triggers" {
		t.Errorf("Key: got %q, want %q", segs[0].Key, "triggers")
	}
	if segs[0].FilterField != "name" {
		t.Errorf("FilterField: got %q, want %q", segs[0].FilterField, "name")
	}
	if segs[0].FilterValue != "check-endpoints" {
		t.Errorf("FilterValue: got %q, want %q", segs[0].FilterValue, "check-endpoints")
	}
}

// TestParsePathFilteredWithContinuation verifies that a filtered segment
// followed by a dot-path segment produces 2 segments with the filter on the first.
func TestParsePathFilteredWithContinuation(t *testing.T) {
	segs, err := ParsePath("triggers[name=check-endpoints].schedule")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segs))
	}
	if segs[0].Key != "triggers" {
		t.Errorf("segment[0].Key: got %q, want %q", segs[0].Key, "triggers")
	}
	if segs[0].FilterField != "name" {
		t.Errorf("segment[0].FilterField: got %q, want %q", segs[0].FilterField, "name")
	}
	if segs[0].FilterValue != "check-endpoints" {
		t.Errorf("segment[0].FilterValue: got %q, want %q", segs[0].FilterValue, "check-endpoints")
	}
	if segs[1].Key != "schedule" {
		t.Errorf("segment[1].Key: got %q, want %q", segs[1].Key, "schedule")
	}
	if segs[1].FilterField != "" {
		t.Errorf("segment[1].FilterField: got %q, want empty", segs[1].FilterField)
	}
}

// TestParsePathKeyWithHyphens verifies that a key containing hyphens is valid.
func TestParsePathKeyWithHyphens(t *testing.T) {
	segs, err := ParsePath("probe-endpoints")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Key != "probe-endpoints" {
		t.Errorf("Key: got %q, want %q", segs[0].Key, "probe-endpoints")
	}
}

// TestParsePathKeyWithUnderscores verifies that a key containing underscores is valid.
func TestParsePathKeyWithUnderscores(t *testing.T) {
	segs, err := ParsePath("latency_threshold_ms")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(segs) != 1 {
		t.Fatalf("expected 1 segment, got %d", len(segs))
	}
	if segs[0].Key != "latency_threshold_ms" {
		t.Errorf("Key: got %q, want %q", segs[0].Key, "latency_threshold_ms")
	}
}

// TestParsePathEmptyStringError verifies that an empty path expression
// returns an error.
func TestParsePathEmptyStringError(t *testing.T) {
	_, err := ParsePath("")
	if err == nil {
		t.Fatal("expected error for empty path, got nil")
	}
}

// TestParsePathMissingClosingBracketError verifies that an unclosed filter
// bracket returns an error.
func TestParsePathMissingClosingBracketError(t *testing.T) {
	_, err := ParsePath("triggers[name=foo")
	if err == nil {
		t.Fatal("expected error for missing closing bracket, got nil")
	}
}

// TestParsePathMissingEqualsInFilterError verifies that a filter missing
// the '=' separator returns an error.
func TestParsePathMissingEqualsInFilterError(t *testing.T) {
	_, err := ParsePath("triggers[name]")
	if err == nil {
		t.Fatal("expected error for filter missing '=', got nil")
	}
}

// TestParsePathLeadingDotError verifies that a path starting with '.'
// returns an error.
func TestParsePathLeadingDotError(t *testing.T) {
	_, err := ParsePath(".config")
	if err == nil {
		t.Fatal("expected error for leading dot, got nil")
	}
}

// TestParsePathTrailingDotError verifies that a path ending with '.'
// returns an error.
func TestParsePathTrailingDotError(t *testing.T) {
	_, err := ParsePath("config.")
	if err == nil {
		t.Fatal("expected error for trailing dot, got nil")
	}
}

// TestParsePathDoubleDotError verifies that a path with a double dot
// (empty segment between dots) returns an error.
func TestParsePathDoubleDotError(t *testing.T) {
	_, err := ParsePath("config..endpoints")
	if err == nil {
		t.Fatal("expected error for double dot (empty segment), got nil")
	}
}
