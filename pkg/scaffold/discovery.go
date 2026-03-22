package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultBaseURL is the raw GitHub URL for the public scaffolds repo.
	DefaultBaseURL = "https://raw.githubusercontent.com/randybias/tentacular-scaffolds/main"
	// DefaultCacheTTL is the string form used for default config (parsed by client).
	DefaultCacheTTL = "1h"

	// maxScaffoldNameLen is the maximum allowed length for a scaffold name.
	maxScaffoldNameLen = 64
)

// scaffoldNameRe matches valid scaffold names: lowercase alphanumeric with hyphens,
// must start and end with alphanumeric, at least 2 characters.
var scaffoldNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$`)

// ValidateScaffoldName enforces security constraints on scaffold names.
// Names must match ^[a-z0-9][a-z0-9-]*[a-z0-9]$ and be at most 64 characters.
// Path separators and traversal sequences are explicitly rejected.
func ValidateScaffoldName(name string) error {
	if name == "" {
		return errors.New("scaffold name must not be empty")
	}
	if strings.ContainsAny(name, `/\`) {
		return errors.New("scaffold name must not contain path separators")
	}
	if strings.Contains(name, "..") {
		return errors.New("scaffold name must not contain '..'")
	}
	if len(name) > maxScaffoldNameLen {
		return fmt.Errorf("scaffold name exceeds maximum length of %d characters", maxScaffoldNameLen)
	}
	if !scaffoldNameRe.MatchString(name) {
		return fmt.Errorf("scaffold name '%s' is invalid: must match ^[a-z0-9][a-z0-9-]*[a-z0-9]$ (lowercase, hyphens allowed, must start and end with alphanumeric)", name)
	}
	return nil
}

// PrivateScaffoldsDir returns the path to the user's private scaffolds directory.
func PrivateScaffoldsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tentacular", "scaffolds"), nil
}

// EnsurePrivateScaffoldsDir creates ~/.tentacular/scaffolds/ with 0700 permissions
// if it does not already exist. Using 0700 ensures other users on shared systems
// cannot read private scaffold content.
func EnsurePrivateScaffoldsDir() (string, error) {
	dir, err := PrivateScaffoldsDir()
	if err != nil {
		return "", err
	}
	if mkErr := os.MkdirAll(dir, 0o700); mkErr != nil {
		return "", fmt.Errorf("creating private scaffolds directory: %w", mkErr)
	}
	return dir, nil
}

// QuickstartsDir returns the path to the cached public quickstarts directory.
func QuickstartsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".tentacular", "quickstarts"), nil
}

// TentaclesDir returns ~/tentacles, the canonical tentacle workspace root.
func TentaclesDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "tentacles"), nil
}

// ReadPrivateScaffolds scans the private scaffolds directory and returns all valid
// scaffolds (directories containing a scaffold.yaml file).
func ReadPrivateScaffolds() ([]ScaffoldEntry, error) {
	dir, err := PrivateScaffoldsDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading private scaffolds dir: %w", err)
	}

	var scaffolds []ScaffoldEntry
	for _, e := range entries {
		// Skip non-directories and symlinks — do not follow symlinks to prevent
		// a local attacker from pointing a symlink at an arbitrary file path.
		if e.Type()&os.ModeSymlink != 0 {
			continue
		}
		if !e.IsDir() {
			continue
		}
		scaffoldYAML := filepath.Join(dir, e.Name(), "scaffold.yaml")
		data, err := os.ReadFile(scaffoldYAML) //nolint:gosec // reading user scaffold metadata
		if err != nil {
			continue // not a scaffold directory, skip
		}
		var s ScaffoldEntry
		if err := yaml.Unmarshal(data, &s); err != nil {
			continue // malformed scaffold.yaml, skip
		}
		s.Source = "private"
		s.Path = filepath.Join(dir, e.Name())
		scaffolds = append(scaffolds, s)
	}
	return scaffolds, nil
}

// ReadPublicScaffolds reads the cached quickstarts index file.
// Returns nil without error if the cache does not yet exist.
func ReadPublicScaffolds(indexPath string) ([]ScaffoldEntry, error) {
	data, err := os.ReadFile(indexPath) //nolint:gosec // reading user-controlled cache file
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading quickstarts index: %w", err)
	}

	var idx ScaffoldIndex
	if err := yaml.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("parsing quickstarts index: %w", err)
	}

	for i := range idx.Scaffolds {
		idx.Scaffolds[i].Source = "public"
	}
	return idx.Scaffolds, nil
}

// FindScaffold searches private then public scaffolds for the given name.
// If source is "private" or "public", only that source is searched.
// Returns the matched entry and an error if not found.
func FindScaffold(name, source, indexPath string) (*ScaffoldEntry, error) {
	var privates []ScaffoldEntry
	var publics []ScaffoldEntry
	var err error

	if source != "public" {
		privates, err = ReadPrivateScaffolds()
		if err != nil {
			return nil, fmt.Errorf("reading private scaffolds: %w", err)
		}
		for i := range privates {
			if privates[i].Name == name {
				return &privates[i], nil
			}
		}
		if source == "private" {
			return nil, fmt.Errorf("scaffold '%s' not found in private scaffolds", name)
		}
	}

	if source != "private" {
		publics, err = ReadPublicScaffolds(indexPath)
		if err != nil {
			return nil, fmt.Errorf("reading public scaffolds: %w", err)
		}
		for i := range publics {
			if publics[i].Name == name {
				return &publics[i], nil
			}
		}
	}

	_ = privates
	return nil, fmt.Errorf("scaffold '%s' not found", name)
}

// ListScaffolds returns all scaffolds from both sources merged, private first.
// Filters are applied if non-empty.
func ListScaffolds(source, category, tag, indexPath string) ([]ScaffoldEntry, error) {
	var result []ScaffoldEntry

	if source != "public" {
		privates, err := ReadPrivateScaffolds()
		if err != nil {
			return nil, fmt.Errorf("reading private scaffolds: %w", err)
		}
		result = append(result, privates...)
	}

	if source != "private" {
		publics, err := ReadPublicScaffolds(indexPath)
		if err != nil {
			return nil, fmt.Errorf("reading public scaffolds: %w", err)
		}
		result = append(result, publics...)
	}

	// Apply filters
	if category == "" && tag == "" {
		return result, nil
	}

	var filtered []ScaffoldEntry
	for _, s := range result {
		if category != "" && !strings.EqualFold(s.Category, category) {
			continue
		}
		if tag != "" && !hasTag(s.Tags, tag) {
			continue
		}
		filtered = append(filtered, s)
	}
	return filtered, nil
}

// SearchScaffolds searches for scaffolds matching query across name, displayName, description, and tags.
func SearchScaffolds(query, source, indexPath string) ([]ScaffoldEntry, error) {
	all, err := ListScaffolds(source, "", "", indexPath)
	if err != nil {
		return nil, err
	}

	q := strings.ToLower(query)
	var matches []ScaffoldEntry
	for _, s := range all {
		if matchesQuery(s, q) {
			matches = append(matches, s)
		}
	}
	return matches, nil
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

func matchesQuery(s ScaffoldEntry, query string) bool {
	if strings.Contains(strings.ToLower(s.Name), query) {
		return true
	}
	if strings.Contains(strings.ToLower(s.DisplayName), query) {
		return true
	}
	if strings.Contains(strings.ToLower(s.Description), query) {
		return true
	}
	for _, tag := range s.Tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
