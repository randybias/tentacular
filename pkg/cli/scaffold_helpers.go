package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/randybias/tentacular/pkg/version"
)

// checkMinVersion warns if the current tntc version is older than minVer.
func checkMinVersion(minVer string) {
	if minVer == "" {
		return
	}
	current := version.Version
	if current == "dev" {
		return
	}
	if compareSemver(current, minVer) < 0 {
		fmt.Fprintf(os.Stderr, "Warning: this scaffold requires tntc %s or later (you have %s)\n", minVer, current)
	}
}

// compareSemver returns -1, 0, or 1 comparing two semver strings.
// Returns 0 if either is unparseable.
func compareSemver(a, b string) int {
	aParts := parseSemver(a)
	bParts := parseSemver(b)
	if aParts == nil || bParts == nil {
		return 0
	}
	for i := range 3 {
		if aParts[i] < bParts[i] {
			return -1
		}
		if aParts[i] > bParts[i] {
			return 1
		}
	}
	return 0
}

func parseSemver(v string) []int {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) != 3 {
		return nil
	}
	nums := make([]int, 3)
	for i, p := range parts {
		// Strip any pre-release suffix (e.g., "1-beta")
		if idx := strings.IndexAny(p, "-+"); idx >= 0 {
			p = p[:idx]
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil
		}
		nums[i] = n
	}
	return nums
}
