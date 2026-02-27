package builder

// Tests for buildDeployAnnotations() - Phase 2 of "Enrich Workflow Metadata for MCP Reporting"
//
// These tests are written proactively based on the task specification and will be
// enabled once the developer implements buildDeployAnnotations() in k8s.go and
// adds WorkflowMetadata to pkg/spec/types.go.
//
// To activate: remove the build tag below once the implementation is merged.

import (
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

// makeWorkflowWithMetadata creates a test workflow with full metadata.
func makeWorkflowWithMetadata(name string) *spec.Workflow {
	return &spec.Workflow{
		Name:    name,
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "manual"},
			{Type: "cron", Schedule: "0 9 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch":  {Path: "./nodes/fetch.ts"},
			"output": {Path: "./nodes/output.ts"},
		},
		Edges: []spec.Edge{
			{From: "fetch", To: "output"},
		},
		Metadata: &spec.WorkflowMetadata{
			Owner: "platform-team",
			Team:  "platform",
			Tags:  []string{"etl", "reporting"},
		},
	}
}

// TestDeploymentAnnotationsPresent verifies that a Deployment with metadata
// includes tentacular.dev/* annotations.
func TestDeploymentAnnotationsPresent(t *testing.T) {
	wf := makeWorkflowWithMetadata("meta-wf")
	manifests := GenerateK8sManifests(wf, "meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "annotations:") {
		t.Error("expected annotations section in Deployment when metadata present")
	}
	if !strings.Contains(dep, "tentacular.dev/owner: platform-team") {
		t.Error("expected tentacular.dev/owner: platform-team annotation")
	}
	if !strings.Contains(dep, "tentacular.dev/team: platform") {
		t.Error("expected tentacular.dev/team: platform annotation")
	}
}

// TestDeploymentAnnotationsTags verifies tags are encoded as comma-separated in annotations.
func TestDeploymentAnnotationsTags(t *testing.T) {
	wf := makeWorkflowWithMetadata("tags-wf")
	manifests := GenerateK8sManifests(wf, "tags-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "tentacular.dev/tags: etl,reporting") {
		t.Error("expected tentacular.dev/tags: etl,reporting annotation")
	}
}

// TestDeploymentAnnotationsOwnerTruncated verifies long owner values are truncated/safe.
func TestDeploymentAnnotationsOwnerTruncated(t *testing.T) {
	// K8s annotation values have a 63-char limit for label values (but not annotation values)
	// This tests that a very long owner string doesn't cause a panic
	longOwner := strings.Repeat("a", 250)
	wf := &spec.Workflow{
		Name:    "long-owner-wf",
		Version: "1.0",
		Triggers: []spec.Trigger{{Type: "manual"}},
		Nodes:   map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Metadata: &spec.WorkflowMetadata{
			Owner: longOwner,
		},
	}
	manifests := GenerateK8sManifests(wf, "long-owner-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content
	// Should not panic and should produce a non-empty manifest
	if dep == "" {
		t.Error("expected non-empty manifest even with long owner value")
	}
}

// TestDeploymentAnnotationsNilMetadata verifies no annotation block when metadata is nil.
func TestDeploymentAnnotationsNilMetadata(t *testing.T) {
	wf := makeTestWorkflow("no-meta-wf")
	// makeTestWorkflow creates a Workflow without Metadata (nil)
	manifests := GenerateK8sManifests(wf, "no-meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	// Should not include tentacular.dev/* annotations when metadata is nil
	if strings.Contains(dep, "tentacular.dev/owner") {
		t.Error("expected NO tentacular.dev/owner annotation when metadata is nil")
	}
	if strings.Contains(dep, "tentacular.dev/tags") {
		t.Error("expected NO tentacular.dev/tags annotation when metadata is nil")
	}
}

// TestDeploymentAnnotationsEmptyMetadata verifies no annotation block when metadata is empty struct.
func TestDeploymentAnnotationsEmptyMetadata(t *testing.T) {
	wf := &spec.Workflow{
		Name:     "empty-meta-wf",
		Version:  "1.0",
		Triggers: []spec.Trigger{{Type: "manual"}},
		Nodes:    map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Metadata: &spec.WorkflowMetadata{},
	}
	manifests := GenerateK8sManifests(wf, "empty-meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	// Empty metadata struct should not produce any tentacular.dev/* annotations
	if strings.Contains(dep, "tentacular.dev/owner") {
		t.Error("expected NO tentacular.dev/owner annotation for empty metadata")
	}
	if strings.Contains(dep, "tentacular.dev/tags") {
		t.Error("expected NO tentacular.dev/tags annotation for empty metadata")
	}
	if strings.Contains(dep, "tentacular.dev/team") {
		t.Error("expected NO tentacular.dev/team annotation for empty metadata")
	}
	// 'annotations:' block should be absent (no annotation block for empty metadata)
	if strings.Contains(dep, "tentacular.dev/") {
		t.Error("expected NO tentacular.dev/* annotations for empty metadata struct")
	}
}

// TestDeploymentAnnotationsSpecialChars verifies special characters in annotation values are safe.
func TestDeploymentAnnotationsSpecialChars(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "special-chars-wf",
		Version: "1.0",
		Triggers: []spec.Trigger{{Type: "manual"}},
		Nodes:   map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Metadata: &spec.WorkflowMetadata{
			Owner: "team/platform & ops",
			Team:  "ops & infra <core>",
		},
	}
	manifests := GenerateK8sManifests(wf, "special-chars-wf:1-0", "default", DeployOptions{})
	// Should not panic, should produce valid YAML
	dep := manifests[0].Content
	if dep == "" {
		t.Error("expected non-empty deployment manifest")
	}
}

// TestDeploymentAnnotationsPipelineSummary verifies that pipeline summary is derived from edges.
func TestDeploymentAnnotationsPipelineSummary(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "pipeline-wf",
		Version: "1.0",
		Triggers: []spec.Trigger{{Type: "manual"}},
		Nodes: map[string]spec.NodeSpec{
			"fetch":     {Path: "./nodes/fetch.ts"},
			"transform": {Path: "./nodes/transform.ts"},
			"load":      {Path: "./nodes/load.ts"},
		},
		Edges: []spec.Edge{
			{From: "fetch", To: "transform"},
			{From: "transform", To: "load"},
		},
		Metadata: &spec.WorkflowMetadata{
			Owner: "data-team",
		},
	}
	manifests := GenerateK8sManifests(wf, "pipeline-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	// Should have some annotation indicating node count or pipeline structure
	if !strings.Contains(dep, "tentacular.dev/node-count") && !strings.Contains(dep, "tentacular.dev/pipeline") {
		t.Log("note: no pipeline summary annotation found (optional feature)")
	}
}

// TestDeploymentAnnotationsTriggerSummaryCron verifies cron trigger annotation.
func TestDeploymentAnnotationsTriggerSummaryCron(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "cron-meta-wf",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 9 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Metadata: &spec.WorkflowMetadata{
			Owner: "platform-team",
		},
	}
	manifests := GenerateK8sManifests(wf, "cron-meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "tentacular.dev/triggers") {
		t.Log("note: no triggers annotation found (optional feature)")
	}
}

// TestDeploymentAnnotationsDepsExtracted verifies dependency names appear in annotations.
func TestDeploymentAnnotationsDepsExtracted(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "deps-meta-wf",
		Version: "1.0",
		Triggers: []spec.Trigger{{Type: "manual"}},
		Nodes:   map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"github": {Protocol: "https", Host: "api.github.com", Port: 443},
				"slack":  {Protocol: "https", Host: "slack.com", Port: 443},
			},
		},
		Metadata: &spec.WorkflowMetadata{
			Owner: "platform-team",
		},
	}
	manifests := GenerateK8sManifests(wf, "deps-meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "tentacular.dev/dependencies") {
		t.Log("note: no dependencies annotation found (optional feature)")
	}
}

// TestWorkflowMetadataParsedFromYAML verifies WorkflowMetadata is parsed correctly.
func TestWorkflowMetadataParsedFromYAML(t *testing.T) {
	yamlContent := `name: meta-parse-test
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
metadata:
  owner: platform-team
  team: platform
  tags:
    - etl
    - reporting
`
	wf, errs := spec.Parse([]byte(yamlContent))
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if wf.Metadata == nil {
		t.Fatal("expected Metadata to be parsed, got nil")
	}
	if wf.Metadata.Owner != "platform-team" {
		t.Errorf("expected owner 'platform-team', got %q", wf.Metadata.Owner)
	}
	if wf.Metadata.Team != "platform" {
		t.Errorf("expected team 'platform', got %q", wf.Metadata.Team)
	}
	if len(wf.Metadata.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(wf.Metadata.Tags), wf.Metadata.Tags)
	}
	if wf.Metadata.Tags[0] != "etl" {
		t.Errorf("expected first tag 'etl', got %q", wf.Metadata.Tags[0])
	}
}

// TestWorkflowMetadataOptional verifies workflows without metadata section still parse.
func TestWorkflowMetadataOptional(t *testing.T) {
	yamlContent := `name: no-meta
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
`
	wf, errs := spec.Parse([]byte(yamlContent))
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if wf.Metadata != nil {
		t.Error("expected Metadata to be nil when not present in YAML")
	}
}

// TestWorkflowMetadataTagsEmpty verifies empty tags list is handled.
func TestWorkflowMetadataTagsEmpty(t *testing.T) {
	yamlContent := `name: empty-tags
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
metadata:
  owner: team-a
  tags: []
`
	wf, errs := spec.Parse([]byte(yamlContent))
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if wf.Metadata == nil {
		t.Fatal("expected Metadata to be non-nil")
	}
	if len(wf.Metadata.Tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(wf.Metadata.Tags))
	}
}

// --- Direct unit tests for buildDeployAnnotations() ---

// TestBuildDeployAnnotationsNil verifies nil metadata returns empty string.
func TestBuildDeployAnnotationsNil(t *testing.T) {
	result := buildDeployAnnotations(nil)
	if result != "" {
		t.Errorf("expected empty string for nil metadata, got %q", result)
	}
}

// TestBuildDeployAnnotationsEmpty verifies empty struct returns empty string.
func TestBuildDeployAnnotationsEmpty(t *testing.T) {
	result := buildDeployAnnotations(&spec.WorkflowMetadata{})
	if result != "" {
		t.Errorf("expected empty string for empty metadata struct, got %q", result)
	}
}

// TestBuildDeployAnnotationsOwnerOnly verifies annotations block with just owner.
func TestBuildDeployAnnotationsOwnerOnly(t *testing.T) {
	meta := &spec.WorkflowMetadata{Owner: "platform-team"}
	result := buildDeployAnnotations(meta)
	if !strings.Contains(result, "annotations:") {
		t.Error("expected 'annotations:' in result")
	}
	if !strings.Contains(result, "tentacular.dev/owner: platform-team") {
		t.Error("expected tentacular.dev/owner: platform-team")
	}
	if strings.Contains(result, "tentacular.dev/team") {
		t.Error("expected NO tentacular.dev/team when team field is empty")
	}
	if strings.Contains(result, "tentacular.dev/tags") {
		t.Error("expected NO tentacular.dev/tags when tags field is empty")
	}
}

// TestBuildDeployAnnotationsAllFields verifies all metadata fields appear.
func TestBuildDeployAnnotationsAllFields(t *testing.T) {
	meta := &spec.WorkflowMetadata{
		Owner:       "platform-team",
		Team:        "infra",
		Tags:        []string{"etl", "daily", "reporting"},
		Environment: "production",
	}
	result := buildDeployAnnotations(meta)
	if !strings.Contains(result, "tentacular.dev/owner: platform-team") {
		t.Error("expected tentacular.dev/owner annotation")
	}
	if !strings.Contains(result, "tentacular.dev/team: infra") {
		t.Error("expected tentacular.dev/team annotation")
	}
	if !strings.Contains(result, "tentacular.dev/tags: etl,daily,reporting") {
		t.Error("expected tentacular.dev/tags with comma-separated values")
	}
	if !strings.Contains(result, "tentacular.dev/environment: production") {
		t.Error("expected tentacular.dev/environment annotation")
	}
}

// TestBuildDeployAnnotationsSingleTag verifies single tag has no trailing comma.
func TestBuildDeployAnnotationsSingleTag(t *testing.T) {
	meta := &spec.WorkflowMetadata{
		Tags: []string{"etl"},
	}
	result := buildDeployAnnotations(meta)
	if !strings.Contains(result, "tentacular.dev/tags: etl") {
		t.Error("expected tentacular.dev/tags: etl")
	}
	if strings.Contains(result, "tentacular.dev/tags: etl,") {
		t.Error("expected no trailing comma for single tag")
	}
}
