package version

import "fmt"

// Injected at build time via GoReleaser ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func String() string {
	return fmt.Sprintf("%s (commit %s, built %s)", Version, Commit, Date)
}
