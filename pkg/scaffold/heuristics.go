package scaffold

import (
	"regexp"
	"strings"
)

// wellKnownHosts is the set of API provider hostnames that should NOT be
// parameterized during extraction. These are fixed by the API provider and
// do not change across organizations.
var wellKnownHosts = map[string]bool{
	"api.anthropic.com":         true,
	"api.openai.com":            true,
	"hooks.slack.com":           true,
	"slack.com":                 true,
	"api.slack.com":             true,
	"github.com":                true,
	"api.github.com":            true,
	"raw.githubusercontent.com": true,
	"api.linear.app":            true,
	"api.pagerduty.com":         true,
	"api.sendgrid.com":          true,
	"api.twilio.com":            true,
	"api.stripe.com":            true,
	"api.braintreegateway.com":  true,
	"googleapis.com":            true,
	"storage.googleapis.com":    true,
	"s3.amazonaws.com":          true,
	"sns.amazonaws.com":         true,
	"sqs.amazonaws.com":         true,
}

// exoskeletonDeps is the set of well-known tentacular exoskeleton dependency names.
// These must not be parameterized -- the `tentacular-` prefix is required for auto-provisioning.
var exoskeletonDeps = map[string]bool{
	"tentacular-postgres": true,
	"tentacular-rustfs":   true,
	"tentacular-nats":     true,
}

// urlRe matches http/https URLs in string values.
var urlRe = regexp.MustCompile(`https?://([^/\s"']+)`)

// orgSpecificPatterns are regex patterns that indicate org-specific values.
// These signal that a config value should be parameterized.
var orgSpecificPatterns = []*regexp.Regexp{
	// URLs with custom domains (non-well-known)
	regexp.MustCompile(`https?://[^/\s"']+\.[a-zA-Z]{2,}`),
	// Slack channel names
	regexp.MustCompile(`^#[a-z0-9_-]+$`),
	// Email addresses
	regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`),
	// S3/GCS bucket names (non-tentacular)
	regexp.MustCompile(`^[a-z0-9][a-z0-9\-]{1,61}[a-z0-9]$`),
	// GitHub org/repo references
	regexp.MustCompile(`^[a-zA-Z0-9_\-]+/[a-zA-Z0-9_\-\.]+$`),
}

// tunablePatterns matches values that should be parameterized with the current value as default.
var tunablePatterns = []*regexp.Regexp{
	// Cron schedule expressions
	regexp.MustCompile(`^(\*|[0-9,\-\*/]+)\s+(\*|[0-9,\-\*/]+)\s+(\*|[0-9,\-\*/]+)\s+(\*|[0-9,\-\*/]+)\s+(\*|[0-9,\-\*/]+)$`),
}

// secretPatterns are patterns that indicate a value looks like a secret.
// Used to warn/block during extraction (security Finding 4).
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`sk-[a-zA-Z0-9]{20,}`),                                            // Anthropic/OpenAI API keys
	regexp.MustCompile(`xoxb-[a-zA-Z0-9\-]+`),                                            // Slack bot tokens
	regexp.MustCompile(`xoxp-[a-zA-Z0-9\-]+`),                                            // Slack user tokens
	regexp.MustCompile(`ghp_[a-zA-Z0-9]{36}`),                                            // GitHub PAT classic
	regexp.MustCompile(`ghs_[a-zA-Z0-9]{36}`),                                            // GitHub app token
	regexp.MustCompile(`github_pat_[a-zA-Z0-9_]{82}`),                                    // GitHub fine-grained PAT
	regexp.MustCompile(`(?i)bearer\s+[a-zA-Z0-9\-._~+/]{20,}`),                           // Bearer tokens
	regexp.MustCompile(`(?i)basic\s+[a-zA-Z0-9+/]{20,}={0,2}`),                           // Basic auth
	regexp.MustCompile(`[A-Za-z0-9+/]{40,}={0,2}`),                                       // Base64 blobs >=40 chars
	regexp.MustCompile(`(?i)(password|passwd|secret|token|apikey|api_key)\s*[:=]\s*\S+`), // key=value secrets
}

// ClassifyValue classifies a string value for extraction purposes.
// Returns one of: "parameterize", "parameterize-with-default", "keep", "secret".
func ClassifyValue(key, value string) string {
	if value == "" {
		return "keep"
	}

	// Security check first: does it look like a secret?
	if looksLikeSecret(value) {
		return "secret"
	}

	// Cron schedule? Parameterize with current as default.
	for _, p := range tunablePatterns {
		if p.MatchString(strings.TrimSpace(value)) {
			return "parameterize-with-default"
		}
	}

	// Numeric-ish threshold keys? Parameterize with current as default.
	if isThresholdKey(key) {
		return "parameterize-with-default"
	}

	// URL with well-known host? Keep.
	if m := urlRe.FindStringSubmatch(value); m != nil {
		host := m[1]
		// Strip port if present
		if idx := strings.Index(host, ":"); idx >= 0 {
			host = host[:idx]
		}
		if isWellKnownHost(host) {
			return "keep"
		}
		// URL with non-well-known host: parameterize
		return "parameterize"
	}

	// Org-specific string patterns?
	for _, p := range orgSpecificPatterns {
		if p.MatchString(value) {
			return "parameterize"
		}
	}

	return "keep"
}

// LooksLikeSecret returns true if the value matches a known secret pattern.
// Used for scanning workflow.yaml config values and node code during extraction.
func LooksLikeSecret(value string) bool {
	return looksLikeSecret(value)
}

func looksLikeSecret(value string) bool {
	for _, p := range secretPatterns {
		if p.MatchString(value) {
			return true
		}
	}
	return false
}

// IsExoskeletonDep returns true if the dependency name is a well-known tentacular exoskeleton dep.
func IsExoskeletonDep(name string) bool {
	return exoskeletonDeps[name]
}

// SafeExampleValue returns a safe example value for a given string value.
// Replaces org-specific parts with placeholder examples.
func SafeExampleValue(value string) string {
	// Replace URLs: keep scheme and well-known hosts, replace others with example.com
	result := urlRe.ReplaceAllStringFunc(value, func(u string) string {
		m := urlRe.FindStringSubmatch(u)
		if m == nil {
			return u
		}
		host := m[1]
		portSuffix := ""
		if idx := strings.Index(host, ":"); idx >= 0 {
			portSuffix = host[idx:]
			host = host[:idx]
		}
		// Keep path after host stripped; replace whole URL with example
		scheme := "https"
		if strings.HasPrefix(u, "http://") {
			scheme = "http"
		}
		if isWellKnownHost(host) {
			return u // keep well-known API URLs as-is
		}
		return scheme + "://example.com" + portSuffix
	})
	return result
}

func isWellKnownHost(host string) bool {
	if wellKnownHosts[host] {
		return true
	}
	// Check suffix matches (e.g., anything ending in .googleapis.com)
	for known := range wellKnownHosts {
		if strings.HasSuffix(host, "."+known) {
			return true
		}
	}
	return false
}

// isThresholdKey returns true if the config key looks like a numeric threshold.
func isThresholdKey(key string) bool {
	lower := strings.ToLower(key)
	keywords := []string{
		"timeout", "threshold", "retry", "retries", "interval",
		"limit", "max", "min", "ttl", "delay", "batch", "concurrency",
		"count", "size", "duration", "period", "ms", "seconds", "minutes",
	}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// InferParamType infers the params.schema.yaml type from a Go value.
func InferParamType(v any) string {
	switch v.(type) {
	case string:
		return "string"
	case int, int64, float64, float32:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "list"
	case map[string]any:
		return "map"
	default:
		return "string"
	}
}
