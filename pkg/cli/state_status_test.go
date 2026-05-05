package cli

import (
	"bytes"
	"strings"
	"testing"
)

// fixtureWithDrift returns a slice of DriftEntry values where one tentacle
// has a cluster SHA different from the local SHA.
func fixtureWithDrift() []DriftEntry {
	return []DriftEntry{
		{
			Enclave:    "tentacular-agensys",
			Tentacle:   "ai-news-digest",
			ClusterSHA: "aaa11111",
			LocalSHA:   "bbb22222",
			Drifted:    true,
		},
		{
			Enclave:    "tentacular-agensys",
			Tentacle:   "ai-blog-writer",
			ClusterSHA: "ccc33333",
			LocalSHA:   "ccc33333",
			Drifted:    false,
		},
	}
}

// fixtureInSync returns a slice of DriftEntry values where all tentacles
// are in sync with the cluster.
func fixtureInSync() []DriftEntry {
	return []DriftEntry{
		{
			Enclave:    "tentacular-agensys",
			Tentacle:   "ai-news-digest",
			ClusterSHA: "ccc33333",
			LocalSHA:   "ccc33333",
			Drifted:    false,
		},
	}
}

// renderDriftEntries renders DriftEntry values to a string using the same
// format as the status command.
func renderDriftEntries(entries []DriftEntry) string {
	var buf bytes.Buffer
	writeDriftEntries(&buf, entries)
	return buf.String()
}

func TestStateStatusReportsDriftPerTentacle(t *testing.T) {
	out := renderDriftEntries(fixtureWithDrift())
	if !strings.Contains(out, "DRIFT") {
		t.Fatalf("expected DRIFT marker in output, got:\n%s", out)
	}
	if !strings.Contains(out, "ai-news-digest") {
		t.Fatal("expected drifted tentacle name in output")
	}
	if !strings.Contains(out, "aaa11111") {
		t.Fatal("expected cluster SHA prefix in output")
	}
	if !strings.Contains(out, "bbb22222") {
		t.Fatal("expected local SHA prefix in output")
	}
}

func TestStateStatusReportsInSyncPerTentacle(t *testing.T) {
	out := renderDriftEntries(fixtureInSync())
	if !strings.Contains(out, "IN_SYNC") {
		t.Fatalf("expected IN_SYNC marker in output, got:\n%s", out)
	}
	if !strings.Contains(out, "ai-news-digest") {
		t.Fatal("expected tentacle name in output")
	}
}

func TestStateStatusMixedOutput(t *testing.T) {
	out := renderDriftEntries(fixtureWithDrift())
	if !strings.Contains(out, "IN_SYNC") {
		t.Fatalf("expected IN_SYNC for ai-blog-writer, got:\n%s", out)
	}
	if !strings.Contains(out, "DRIFT") {
		t.Fatalf("expected DRIFT for ai-news-digest, got:\n%s", out)
	}
}
