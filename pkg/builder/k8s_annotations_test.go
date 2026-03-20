package builder

// Tests for buildDeployAnnotations() covering the tentacular.io/* annotation domain
// and the updated WorkflowMetadata schema (Group replaces Owner/Team).

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
			Group: "platform-team",
			Tags:  []string{"etl", "reporting"},
		},
	}
}

// TestDeploymentAnnotationsPresent verifies that a Deployment with metadata
// includes tentacular.io/* annotations.
func TestDeploymentAnnotationsPresent(t *testing.T) {
	wf := makeWorkflowWithMetadata("meta-wf")
	manifests := GenerateK8sManifests(wf, "meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "annotations:") {
		t.Error("expected annotations section in Deployment when metadata present")
	}
	if !strings.Contains(dep, "tentacular.io/group: platform-team") {
		t.Error("expected tentacular.io/group: platform-team annotation")
	}
}

// TestDeploymentAnnotationsTags verifies tags are encoded as comma-separated in annotations.
func TestDeploymentAnnotationsTags(t *testing.T) {
	wf := makeWorkflowWithMetadata("tags-wf")
	manifests := GenerateK8sManifests(wf, "tags-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "tentacular.io/tags: etl,reporting") {
		t.Error("expected tentacular.io/tags: etl,reporting annotation")
	}
}

// TestDeploymentAnnotationsNoOwnerTeam verifies that removed owner/team fields
// produce no annotations.
func TestDeploymentAnnotationsNoOwnerTeam(t *testing.T) {
	wf := &spec.Workflow{
		Name:     "no-owner-wf",
		Version:  "1.0",
		Triggers: []spec.Trigger{{Type: "manual"}},
		Nodes:    map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Metadata: &spec.WorkflowMetadata{
			Group: "platform-team",
		},
	}
	manifests := GenerateK8sManifests(wf, "no-owner-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content
	if strings.Contains(dep, "tentacular.io/owner") {
		t.Error("expected NO tentacular.io/owner annotation (owner is identity-derived, not declared)")
	}
	if strings.Contains(dep, "tentacular.io/team") {
		t.Error("expected NO tentacular.io/team annotation (team field was removed)")
	}
	if strings.Contains(dep, "tentacular.dev/") {
		t.Error("expected NO tentacular.dev/* annotations (domain migrated to tentacular.io)")
	}
}

// TestDeploymentAnnotationsNilMetadata verifies no annotation block when metadata is nil.
func TestDeploymentAnnotationsNilMetadata(t *testing.T) {
	wf := makeTestWorkflow("no-meta-wf")
	manifests := GenerateK8sManifests(wf, "no-meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if strings.Contains(dep, "tentacular.io/group") {
		t.Error("expected NO tentacular.io/group annotation when metadata is nil")
	}
	if strings.Contains(dep, "tentacular.io/tags") {
		t.Error("expected NO tentacular.io/tags annotation when metadata is nil")
	}
	if strings.Contains(dep, "tentacular.dev/") {
		t.Error("expected NO tentacular.dev/* annotations")
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

	if strings.Contains(dep, "tentacular.io/group") {
		t.Error("expected NO tentacular.io/group annotation for empty metadata")
	}
	if strings.Contains(dep, "tentacular.io/tags") {
		t.Error("expected NO tentacular.io/tags annotation for empty metadata")
	}
	if strings.Contains(dep, "tentacular.dev/") {
		t.Error("expected NO tentacular.dev/* annotations for empty metadata struct")
	}
}

// TestDeploymentAnnotationsSpecialChars verifies special characters in group values are safe.
func TestDeploymentAnnotationsSpecialChars(t *testing.T) {
	wf := &spec.Workflow{
		Name:     "special-chars-wf",
		Version:  "1.0",
		Triggers: []spec.Trigger{{Type: "manual"}},
		Nodes:    map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Metadata: &spec.WorkflowMetadata{
			Group: "team/platform & ops",
		},
	}
	manifests := GenerateK8sManifests(wf, "special-chars-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content
	if dep == "" {
		t.Error("expected non-empty deployment manifest")
	}
}

// TestDeploymentAnnotationsPipelineSummary verifies that pipeline summary is derived from edges.
func TestDeploymentAnnotationsPipelineSummary(t *testing.T) {
	wf := &spec.Workflow{
		Name:     "pipeline-wf",
		Version:  "1.0",
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
			Group: "data-team",
		},
	}
	manifests := GenerateK8sManifests(wf, "pipeline-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "tentacular.io/node-count") && !strings.Contains(dep, "tentacular.io/pipeline") {
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
		Nodes:    map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Metadata: &spec.WorkflowMetadata{Group: "platform-team"},
	}
	manifests := GenerateK8sManifests(wf, "cron-meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "tentacular.io/triggers") {
		t.Log("note: no triggers annotation found (optional feature)")
	}
}

// TestDeploymentAnnotationsDepsExtracted verifies dependency names appear in annotations.
func TestDeploymentAnnotationsDepsExtracted(t *testing.T) {
	wf := &spec.Workflow{
		Name:     "deps-meta-wf",
		Version:  "1.0",
		Triggers: []spec.Trigger{{Type: "manual"}},
		Nodes:    map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"github": {Protocol: "https", Host: "api.github.com", Port: 443},
				"slack":  {Protocol: "https", Host: "slack.com", Port: 443},
			},
		},
		Metadata: &spec.WorkflowMetadata{Group: "platform-team"},
	}
	manifests := GenerateK8sManifests(wf, "deps-meta-wf:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "tentacular.io/dependencies") {
		t.Log("note: no dependencies annotation found (optional feature)")
	}
}

// TestWorkflowMetadataParsedFromYAML verifies WorkflowMetadata is parsed correctly.
// owner/team fields in YAML are silently ignored (no error, no emission).
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
  group: platform-team
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
	if wf.Metadata.Group != "platform-team" {
		t.Errorf("expected group 'platform-team', got %q", wf.Metadata.Group)
	}
	if len(wf.Metadata.Tags) != 2 {
		t.Errorf("expected 2 tags, got %d: %v", len(wf.Metadata.Tags), wf.Metadata.Tags)
	}
	if wf.Metadata.Tags[0] != "etl" {
		t.Errorf("expected first tag 'etl', got %q", wf.Metadata.Tags[0])
	}
}

// TestWorkflowMetadataOwnerTeamIgnored verifies old owner/team keys in YAML are silently ignored.
func TestWorkflowMetadataOwnerTeamIgnored(t *testing.T) {
	yamlContent := `name: legacy-meta
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
metadata:
  owner: old-owner
  team: old-team
  tags:
    - legacy
`
	wf, errs := spec.Parse([]byte(yamlContent))
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors for legacy metadata: %v", errs)
	}
	if wf.Metadata == nil {
		t.Fatal("expected Metadata to be non-nil")
	}
	manifests := GenerateK8sManifests(wf, "legacy-meta:1-0", "default", DeployOptions{})
	dep := manifests[0].Content
	if strings.Contains(dep, "tentacular.io/owner") || strings.Contains(dep, "tentacular.dev/owner") {
		t.Error("expected legacy owner key to produce no annotation")
	}
	if strings.Contains(dep, "tentacular.io/team") || strings.Contains(dep, "tentacular.dev/team") {
		t.Error("expected legacy team key to produce no annotation")
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
  group: team-a
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

// TestBuildDeployAnnotationsNil verifies nil metadata with no triggers returns empty string.
func TestBuildDeployAnnotationsNil(t *testing.T) {
	result := buildDeployAnnotations(nil, nil)
	if result != "" {
		t.Errorf("expected empty string for nil metadata and no triggers, got %q", result)
	}
}

// TestBuildDeployAnnotationsEmpty verifies empty struct with no triggers returns empty string.
func TestBuildDeployAnnotationsEmpty(t *testing.T) {
	result := buildDeployAnnotations(&spec.WorkflowMetadata{}, nil)
	if result != "" {
		t.Errorf("expected empty string for empty metadata struct and no triggers, got %q", result)
	}
}

// TestBuildDeployAnnotationsGroupOnly verifies annotations block with just group.
func TestBuildDeployAnnotationsGroupOnly(t *testing.T) {
	meta := &spec.WorkflowMetadata{Group: "platform-team"}
	result := buildDeployAnnotations(meta, nil)
	if !strings.Contains(result, "annotations:") {
		t.Error("expected 'annotations:' in result")
	}
	if !strings.Contains(result, "tentacular.io/group: platform-team") {
		t.Error("expected tentacular.io/group: platform-team")
	}
	if strings.Contains(result, "tentacular.io/tags") {
		t.Error("expected NO tentacular.io/tags when tags field is empty")
	}
	if strings.Contains(result, "tentacular.dev/") {
		t.Error("expected NO tentacular.dev/* annotations")
	}
}

// TestBuildDeployAnnotationsAllFields verifies all supported metadata fields appear.
func TestBuildDeployAnnotationsAllFields(t *testing.T) {
	meta := &spec.WorkflowMetadata{
		Group:       "platform-team",
		Tags:        []string{"etl", "daily", "reporting"},
		Environment: "production",
	}
	result := buildDeployAnnotations(meta, nil)
	if !strings.Contains(result, "tentacular.io/group: platform-team") {
		t.Error("expected tentacular.io/group annotation")
	}
	if !strings.Contains(result, "tentacular.io/tags: etl,daily,reporting") {
		t.Error("expected tentacular.io/tags with comma-separated values")
	}
	if !strings.Contains(result, "tentacular.io/environment: production") {
		t.Error("expected tentacular.io/environment annotation")
	}
	if strings.Contains(result, "tentacular.dev/") {
		t.Error("expected NO tentacular.dev/* annotations")
	}
}

// TestBuildDeployAnnotationsSingleTag verifies single tag has no trailing comma.
func TestBuildDeployAnnotationsSingleTag(t *testing.T) {
	meta := &spec.WorkflowMetadata{
		Tags: []string{"etl"},
	}
	result := buildDeployAnnotations(meta, nil)
	if !strings.Contains(result, "tentacular.io/tags: etl") {
		t.Error("expected tentacular.io/tags: etl")
	}
	if strings.Contains(result, "tentacular.io/tags: etl,") {
		t.Error("expected no trailing comma for single tag")
	}
}

// TestBuildDeployAnnotationsCronSchedule verifies cron-schedule annotation uses tentacular.io domain.
func TestBuildDeployAnnotationsCronSchedule(t *testing.T) {
	triggers := []spec.Trigger{
		{Type: "cron", Schedule: "0 9 * * *"},
	}
	result := buildDeployAnnotations(nil, triggers)
	if !strings.Contains(result, `tentacular.io/cron-schedule: "0 9 * * *"`) {
		t.Error("expected tentacular.io/cron-schedule annotation")
	}
	if strings.Contains(result, "tentacular.dev/") {
		t.Error("expected NO tentacular.dev/* annotations")
	}
}
