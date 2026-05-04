package cli

import (
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// resolveEnclaveName resolves the target enclave name using a 4-step priority cascade:
//  1. flagValue — the --enclave flag value (empty string treated as unset)
//  2. TENTACULAR_ENCLAVE environment variable
//  3. enclave: field in .tentacular/config.yaml found by walking up from startDir
//  4. Error: "no enclave specified: ..."
//
// startDir is the directory to begin the config file walk-up from (typically cwd or
// the workflow directory). Pass "" to skip the config file walk.
func resolveEnclaveName(flagValue, startDir string) (string, error) {
	// 1. --enclave flag
	if flagValue != "" {
		return flagValue, nil
	}

	// 2. TENTACULAR_ENCLAVE env var
	if v := os.Getenv("TENTACULAR_ENCLAVE"); v != "" {
		return v, nil
	}

	// 3. Walk up from startDir looking for .tentacular/config.yaml with enclave: field
	if startDir != "" {
		if name := walkEnclaveConfig(startDir); name != "" {
			return name, nil
		}
	}

	return "", errors.New("no enclave specified: pass --enclave, set TENTACULAR_ENCLAVE, or add enclave: <name> to .tentacular/config.yaml")
}

// projectEnclaveConfig holds just the enclave field from a project-level config file.
type projectEnclaveConfig struct {
	Enclave string `yaml:"enclave,omitempty"`
}

// walkEnclaveConfig walks up the directory tree from dir, looking for a
// .tentacular/config.yaml file that contains an enclave: field.
// Returns the enclave name if found, or "" if not found.
// The walk stops at filesystem root or when it can no longer traverse upward.
func walkEnclaveConfig(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}

	for {
		cfgPath := filepath.Join(abs, ".tentacular", "config.yaml")
		if data, readErr := os.ReadFile(cfgPath); readErr == nil { //nolint:gosec // path is user-controlled cwd walk
			var cfg projectEnclaveConfig
			if unmarshalErr := yaml.Unmarshal(data, &cfg); unmarshalErr == nil && cfg.Enclave != "" {
				return cfg.Enclave
			}
		}

		parent := filepath.Dir(abs)
		if parent == abs {
			// Reached filesystem root.
			break
		}
		abs = parent
	}
	return ""
}
