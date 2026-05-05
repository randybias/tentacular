package cli

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/randybias/tentacular/pkg/mcp"
)

// DriftEntry holds the drift comparison result for a single tentacle.
type DriftEntry struct {
	Enclave    string `json:"enclave"`
	Tentacle   string `json:"tentacle"`
	ClusterSHA string `json:"cluster_sha"`
	LocalSHA   string `json:"local_sha"`
	Drifted    bool   `json:"drifted"`
}

// detectDrift enumerates deployed tentacles for the given enclave via MCP,
// reads the tentacular.io/git-sha annotation per tentacle, and compares
// against the HEAD commit SHA of the tentacle's local path in the git-state
// repo. Returns one DriftEntry per tentacle.
//
// MCP errors for individual tentacles are surfaced as drifted entries with
// ClusterSHA="(unknown)" so that other entries still report correctly.
func detectDrift(ctx context.Context, mcpClient *mcp.Client, enclave, repoPath string) ([]DriftEntry, error) {
	wfs, err := mcpClient.WfList(ctx, enclave)
	if err != nil {
		return nil, fmt.Errorf("wf_list %s: %w", enclave, err)
	}

	entries := make([]DriftEntry, 0, len(wfs))
	for _, wf := range wfs {
		tentaclePath := filepath.Join("enclaves", enclave, wf.Name)

		// Read the cluster annotation via wf_describe.
		desc, descErr := mcpClient.WfDescribe(ctx, enclave, wf.Name)
		clusterSHA := "(unknown)"
		if descErr == nil && desc != nil {
			if sha, ok := desc.Annotations["tentacular.io/git-sha"]; ok {
				clusterSHA = sha
			}
		}

		// Read local HEAD SHA for this tentacle in the git-state repo.
		localSHA := localHeadForPath(ctx, repoPath, tentaclePath)

		drifted := clusterSHA != localSHA || clusterSHA == "(unknown)" || localSHA == "(unknown)"

		entries = append(entries, DriftEntry{
			Enclave:    enclave,
			Tentacle:   wf.Name,
			ClusterSHA: clusterSHA,
			LocalSHA:   localSHA,
			Drifted:    drifted,
		})
	}
	return entries, nil
}

// localHeadForPath resolves the HEAD commit SHA for a path in the git-state
// repo using `git log -1 --format=%H -- <path>`. Returns "(unknown)" if the
// path doesn't exist in git history.
func localHeadForPath(ctx context.Context, repoPath, path string) string {
	cmd := exec.CommandContext(ctx, "git", "-C", repoPath, "log", "-1", "--format=%H", "--", path) //nolint:gosec // repoPath is config-controlled
	out, err := cmd.Output()
	if err != nil {
		return "(unknown)"
	}
	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "(unknown)"
	}
	return sha
}

// writeDriftEntries writes drift status lines to w in the format:
//
//	DRIFT   <enclave>  <tentacle>  cluster=<sha8> local=<sha8>
//	IN_SYNC <enclave>  <tentacle>  <sha8>
func writeDriftEntries(w io.Writer, entries []DriftEntry) {
	for _, e := range entries {
		if e.Drifted {
			clusterShort := shortSHA(e.ClusterSHA)
			localShort := shortSHA(e.LocalSHA)
			_, _ = fmt.Fprintf(w, "DRIFT\t%s\t%s\tcluster=%s local=%s\n",
				e.Enclave, e.Tentacle, clusterShort, localShort)
		} else {
			_, _ = fmt.Fprintf(w, "IN_SYNC\t%s\t%s\t%s\n",
				e.Enclave, e.Tentacle, shortSHA(e.ClusterSHA))
		}
	}
}

// shortSHA returns the first 8 chars of a SHA, or the full string if shorter.
func shortSHA(sha string) string {
	if len(sha) <= 8 {
		return sha
	}
	return sha[:8]
}
