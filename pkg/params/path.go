package params

import (
	"errors"
	"fmt"
	"strings"
)

// Segment represents one step in a path expression.
type Segment struct {
	Key         string // the mapping key to navigate into
	FilterField string // for filtered segments: match on this field
	FilterValue string // for filtered segments: the field must equal this value
}

// ParsePath parses a path expression into a slice of Segments.
//
// Grammar:
//
//	path     = segment ("." segment)*
//	segment  = key filter?
//	key      = [a-zA-Z_][a-zA-Z0-9_-]*
//	filter   = "[" key "=" value "]"
//	value    = [a-zA-Z0-9_-]{1,256}
//
// Examples:
//
//	config.endpoints
//	config.latency_threshold_ms
//	triggers[name=check-endpoints].schedule
//	nodes.probe-endpoints.env.TIMEOUT
func ParsePath(expr string) ([]Segment, error) {
	if expr == "" {
		return nil, errors.New("path expression must not be empty")
	}

	parts := splitPath(expr)
	segs := make([]Segment, 0, len(parts))
	for _, p := range parts {
		seg, err := parseSegment(p)
		if err != nil {
			return nil, fmt.Errorf("invalid path expression '%s': %w", expr, err)
		}
		segs = append(segs, seg)
	}
	return segs, nil
}

// splitPath splits a path expression by "." but respects brackets (so "triggers[name=x.y]"
// does not split on dots inside brackets).
func splitPath(expr string) []string {
	var parts []string
	depth := 0
	start := 0
	for i, ch := range expr {
		switch ch {
		case '[':
			depth++
		case ']':
			depth--
		case '.':
			if depth == 0 {
				parts = append(parts, expr[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, expr[start:])
	return parts
}

// parseSegment parses a single path segment like "key" or "key[field=value]".
func parseSegment(s string) (Segment, error) {
	open := strings.Index(s, "[")
	if open == -1 {
		// Simple key segment
		if err := validateKey(s); err != nil {
			return Segment{}, err
		}
		return Segment{Key: s}, nil
	}

	// Filtered segment: key[field=value]
	closeBracket := strings.LastIndex(s, "]")
	if closeBracket == -1 || closeBracket < open {
		return Segment{}, fmt.Errorf("segment '%s': unmatched '['", s)
	}
	if closeBracket != len(s)-1 {
		return Segment{}, fmt.Errorf("segment '%s': unexpected characters after ']'", s)
	}

	key := s[:open]
	if err := validateKey(key); err != nil {
		return Segment{}, err
	}

	inner := s[open+1 : closeBracket]
	eqIdx := strings.Index(inner, "=")
	if eqIdx == -1 {
		return Segment{}, fmt.Errorf("segment '%s': filter must be field=value", s)
	}

	filterField := inner[:eqIdx]
	filterValue := inner[eqIdx+1:]

	if err := validateKey(filterField); err != nil {
		return Segment{}, fmt.Errorf("segment '%s': filter field: %w", s, err)
	}
	if err := validateFilterValue(filterValue); err != nil {
		return Segment{}, fmt.Errorf("segment '%s': filter value: %w", s, err)
	}

	return Segment{Key: key, FilterField: filterField, FilterValue: filterValue}, nil
}

// maxFilterValueLen is the maximum allowed length for a filter value.
const maxFilterValueLen = 256

func validateKey(key string) error {
	if key == "" {
		return errors.New("key must not be empty")
	}
	for i, ch := range key {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch == '_':
		case i > 0 && (ch >= '0' && ch <= '9'):
		case i > 0 && ch == '-':
		default:
			return fmt.Errorf("key '%s': invalid character '%c' at position %d", key, ch, i)
		}
	}
	return nil
}

// validateFilterValue enforces security constraints on filter values.
// Values must match [a-zA-Z0-9_-]+ and be at most maxFilterValueLen chars.
func validateFilterValue(value string) error {
	if value == "" {
		return errors.New("filter value must not be empty")
	}
	if len(value) > maxFilterValueLen {
		return fmt.Errorf("filter value exceeds maximum length of %d characters", maxFilterValueLen)
	}
	for i, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
		case ch >= 'A' && ch <= 'Z':
		case ch >= '0' && ch <= '9':
		case ch == '_':
		case ch == '-':
		default:
			return fmt.Errorf("filter value contains invalid character '%c' at position %d (only [a-zA-Z0-9_-] allowed)", ch, i)
		}
	}
	return nil
}
