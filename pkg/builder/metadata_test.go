package builder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

// --- Helpers ---

func makeMetadataTestWorkflow(name string) *spec.Workflow {
	return &spec.Workflow{
		Name:    name,
		Version: "1.2.0",
		Nodes: map[string]spec.NodeSpec{
			"fetch":   {Path: "./nodes/fetch.ts"},
			"analyze": {Path: "./nodes/analyze.ts"},
			"report":  {Path: "./nodes/report.ts"},
		},
		Edges: []spec.Edge{
			{From: "fetch", To: "analyze"},
			{From: "analyze", To: "report"},
		},
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 9 * * *"},
		},
		Sidecars: []spec.SidecarSpec{
			{Name: "chromium", Image: "chromium:latest", Port: 9222},
		},
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"tentacular-postgres": {Protocol: "postgresql"},
				"openai-api": {
					Protocol: "https",
					Host:     "api.openai.com",
					Port:     443,
					Auth:     &spec.DependencyAuth{Type: "apikey", Secret: "openai-api.api-key"},
				},
			},
		},
	}
}

// --- Tests: buildTier1Annotations ---

func TestBuildTier1AnnotationsNodes(t *testing.T) {
	wf := &spec.Workflow{
		Name: "test-nodes",
		Nodes: map[string]spec.NodeSpec{
			"fetch":  {Path: "./nodes/fetch.ts"},
			"output": {Path: "./nodes/output.ts"},
		},
	}
	bundle := &MetadataBundle{Annotations: make(map[string]string)}
	buildTier1Annotations(bundle, wf)

	val, ok := bundle.Annotations["tentacular.io/nodes"]
	if !ok {
		t.Fatal("expected tentacular.io/nodes annotation")
	}
	var nodes []string
	if err := json.Unmarshal([]byte(val), &nodes); err != nil {
		t.Fatalf("nodes annotation is not valid JSON: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(nodes))
	}
	// Should be sorted
	if nodes[0] != "fetch" || nodes[1] != "output" {
		t.Errorf("expected sorted nodes [fetch output], got %v", nodes)
	}
}

func TestBuildTier1AnnotationsEdges(t *testing.T) {
	wf := &spec.Workflow{
		Name: "test-edges",
		Nodes: map[string]spec.NodeSpec{
			"a": {Path: "./nodes/a.ts"},
			"b": {Path: "./nodes/b.ts"},
		},
		Edges: []spec.Edge{
			{From: "a", To: "b"},
		},
	}
	bundle := &MetadataBundle{Annotations: make(map[string]string)}
	buildTier1Annotations(bundle, wf)

	val, ok := bundle.Annotations["tentacular.io/edges"]
	if !ok {
		t.Fatal("expected tentacular.io/edges annotation")
	}
	var edges [][2]string
	if err := json.Unmarshal([]byte(val), &edges); err != nil {
		t.Fatalf("edges annotation is not valid JSON: %v", err)
	}
	if len(edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(edges))
	}
	if edges[0][0] != "a" || edges[0][1] != "b" {
		t.Errorf("expected edge [a b], got %v", edges[0])
	}
}

func TestBuildTier1AnnotationsSidecars(t *testing.T) {
	wf := &spec.Workflow{
		Name: "test-sidecars",
		Sidecars: []spec.SidecarSpec{
			{Name: "chrome", Image: "chrome:latest", Port: 9222},
		},
	}
	bundle := &MetadataBundle{Annotations: make(map[string]string)}
	buildTier1Annotations(bundle, wf)

	val, ok := bundle.Annotations["tentacular.io/sidecars"]
	if !ok {
		t.Fatal("expected tentacular.io/sidecars annotation")
	}
	var sidecars []struct {
		Name  string `json:"name"`
		Image string `json:"image"`
		Port  int    `json:"port"`
	}
	if err := json.Unmarshal([]byte(val), &sidecars); err != nil {
		t.Fatalf("sidecars annotation is not valid JSON: %v", err)
	}
	if len(sidecars) != 1 {
		t.Fatalf("expected 1 sidecar, got %d", len(sidecars))
	}
	if sidecars[0].Name != "chrome" || sidecars[0].Port != 9222 {
		t.Errorf("unexpected sidecar: %+v", sidecars[0])
	}
}

func TestBuildTier1AnnotationsDependencies(t *testing.T) {
	wf := &spec.Workflow{
		Name: "test-deps",
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"tentacular-pg": {Protocol: "postgresql"},
				"openai-api":    {Protocol: "https"},
			},
		},
	}
	bundle := &MetadataBundle{Annotations: make(map[string]string)}
	buildTier1Annotations(bundle, wf)

	val, ok := bundle.Annotations["tentacular.io/dependencies"]
	if !ok {
		t.Fatal("expected tentacular.io/dependencies annotation")
	}
	var deps []struct {
		Name     string `json:"name"`
		Protocol string `json:"protocol"`
		Managed  bool   `json:"managed"`
	}
	if err := json.Unmarshal([]byte(val), &deps); err != nil {
		t.Fatalf("dependencies annotation is not valid JSON: %v", err)
	}
	if len(deps) != 2 {
		t.Fatalf("expected 2 deps, got %d", len(deps))
	}
	// sorted by name: openai-api, tentacular-pg
	if deps[0].Name != "openai-api" || deps[0].Managed {
		t.Errorf("expected openai-api not managed, got %+v", deps[0])
	}
	if deps[1].Name != "tentacular-pg" || !deps[1].Managed {
		t.Errorf("expected tentacular-pg managed, got %+v", deps[1])
	}
}

func TestBuildTier1AnnotationsTriggerType(t *testing.T) {
	wf := &spec.Workflow{
		Name: "test-triggers",
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 9 * * *"},
			{Type: "manual"},
			{Type: "cron", Schedule: "0 18 * * *"}, // duplicate type
		},
	}
	bundle := &MetadataBundle{Annotations: make(map[string]string)}
	buildTier1Annotations(bundle, wf)

	val, ok := bundle.Annotations["tentacular.io/trigger-type"]
	if !ok {
		t.Fatal("expected tentacular.io/trigger-type annotation")
	}
	// cron appears once, manual appears once
	if !strings.Contains(val, "cron") || !strings.Contains(val, "manual") {
		t.Errorf("unexpected trigger-type: %q", val)
	}
	// Verify cron appears only once (deduped)
	if strings.Count(val, "cron") != 1 {
		t.Errorf("expected cron to appear once, got: %q", val)
	}
}

func TestBuildTier1AnnotationsEmptyWorkflow(t *testing.T) {
	wf := &spec.Workflow{Name: "empty-wf"}
	bundle := &MetadataBundle{Annotations: make(map[string]string)}
	buildTier1Annotations(bundle, wf)

	if _, ok := bundle.Annotations["tentacular.io/nodes"]; ok {
		t.Error("expected no nodes annotation for empty workflow")
	}
	if _, ok := bundle.Annotations["tentacular.io/edges"]; ok {
		t.Error("expected no edges annotation for empty workflow")
	}
	if _, ok := bundle.Annotations["tentacular.io/sidecars"]; ok {
		t.Error("expected no sidecars annotation for empty workflow")
	}
	if _, ok := bundle.Annotations["tentacular.io/dependencies"]; ok {
		t.Error("expected no dependencies annotation for empty workflow")
	}
	if _, ok := bundle.Annotations["tentacular.io/trigger-type"]; ok {
		t.Error("expected no trigger-type annotation for empty workflow")
	}
}

// --- Tests: ReadMetadata ---

func TestReadMetadataReadmePresent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# My Workflow\nDoes stuff."), 0o600); err != nil {
		t.Fatal(err)
	}
	wf := &spec.Workflow{Name: "my-wf", Version: "1.0.0"}

	bundle, err := ReadMetadata(wf, dir)
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if bundle.Readme != "# My Workflow\nDoes stuff." {
		t.Errorf("unexpected readme: %q", bundle.Readme)
	}
}

func TestReadMetadataReadmeMissing(t *testing.T) {
	dir := t.TempDir()
	wf := &spec.Workflow{Name: "no-readme-wf", Version: "1.0.0"}

	bundle, err := ReadMetadata(wf, dir)
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if bundle.Readme != "" {
		t.Errorf("expected empty readme, got: %q", bundle.Readme)
	}
}

func TestReadMetadataParamsSchemaPresent(t *testing.T) {
	dir := t.TempDir()
	schema := "type: object\nproperties:\n  url:\n    type: string\n"
	if err := os.WriteFile(filepath.Join(dir, "params.schema.yaml"), []byte(schema), 0o600); err != nil {
		t.Fatal(err)
	}
	wf := &spec.Workflow{Name: "schema-wf", Version: "1.0.0"}

	bundle, err := ReadMetadata(wf, dir)
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if bundle.ParamsSchema != schema {
		t.Errorf("unexpected params_schema: %q", bundle.ParamsSchema)
	}
}

func TestReadMetadataContractSummaryOnDiskTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	summary := "## My Contract\nHuman-authored content."
	if err := os.WriteFile(filepath.Join(dir, "contract-summary.md"), []byte(summary), 0o600); err != nil {
		t.Fatal(err)
	}
	wf := &spec.Workflow{
		Name:    "contract-wf",
		Version: "1.0.0",
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"openai": {Protocol: "https"},
			},
		},
	}

	bundle, err := ReadMetadata(wf, dir)
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if bundle.ContractSummary != summary {
		t.Errorf("expected on-disk contract summary to take precedence, got: %q", bundle.ContractSummary)
	}
}

func TestReadMetadataContractAutoGenerated(t *testing.T) {
	dir := t.TempDir() // no contract-summary.md
	wf := &spec.Workflow{
		Name:    "auto-contract-wf",
		Version: "1.0.0",
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"openai-api": {Protocol: "https", Host: "api.openai.com", Port: 443},
			},
		},
	}

	bundle, err := ReadMetadata(wf, dir)
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if bundle.ContractSummary == "" {
		t.Error("expected auto-generated contract summary")
	}
	if !strings.Contains(bundle.ContractSummary, "## Dependencies") {
		t.Error("expected auto-generated summary to contain ## Dependencies")
	}
	if !strings.Contains(bundle.ContractSummary, "openai-api") {
		t.Error("expected auto-generated summary to contain dependency name")
	}
}

func TestReadMetadataNoContractNoSummary(t *testing.T) {
	dir := t.TempDir()
	wf := &spec.Workflow{Name: "no-contract-wf", Version: "1.0.0"}

	bundle, err := ReadMetadata(wf, dir)
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if bundle.ContractSummary != "" {
		t.Errorf("expected empty contract summary when no contract and no file, got: %q", bundle.ContractSummary)
	}
}

func TestReadMetadataAlwaysSetsTier1Annotations(t *testing.T) {
	dir := t.TempDir()
	wf := &spec.Workflow{
		Name:    "tier1-wf",
		Version: "1.0.0",
		Nodes:   map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
	}

	bundle, err := ReadMetadata(wf, dir)
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if bundle.Annotations == nil {
		t.Fatal("expected non-nil annotations")
	}
	// metadata-ref is always set
	if ref := bundle.Annotations["tentacular.io/metadata-ref"]; ref != "tier1-wf-metadata" {
		t.Errorf("expected metadata-ref 'tier1-wf-metadata', got %q", ref)
	}
	// version is always set
	if v := bundle.Annotations["tentacular.io/version"]; v == "" {
		t.Error("expected non-empty version annotation")
	}
	// nodes should be set
	if _, ok := bundle.Annotations["tentacular.io/nodes"]; !ok {
		t.Error("expected tentacular.io/nodes annotation")
	}
}

func TestReadMetadataScaffoldFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".scaffold"), []byte("video-content-analyzer\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	wf := &spec.Workflow{Name: "scaffold-wf", Version: "1.0.0"}

	bundle, err := ReadMetadata(wf, dir)
	if err != nil {
		t.Fatalf("ReadMetadata failed: %v", err)
	}
	if v := bundle.Annotations["tentacular.io/scaffold-name"]; v != "video-content-analyzer" {
		t.Errorf("expected scaffold-name 'video-content-analyzer', got %q", v)
	}
}

// --- Tests: Version derivation ---

func TestDeriveVersionWithSpecVersionNoGit(t *testing.T) {
	dir := t.TempDir() // not a git repo
	result := deriveVersion("2.3.1", dir)
	if result != "2.3.1" {
		t.Errorf("expected '2.3.1' for non-git with spec version, got %q", result)
	}
}

func TestDeriveVersionNoSpecVersionNoGit(t *testing.T) {
	dir := t.TempDir() // not a git repo
	result := deriveVersion("", dir)
	if result != "0.1.0" {
		t.Errorf("expected '0.1.0' for non-git with no spec version, got %q", result)
	}
}

func TestDeriveVersionWithGit(t *testing.T) {
	dir := initTestGitRepo(t)
	result := deriveVersion("1.0.0", dir)
	// Should be "1.0.0+<shortSHA>" or "1.0.0+<shortSHA>.dirty"
	if !strings.HasPrefix(result, "1.0.0+") {
		t.Errorf("expected result to start with '1.0.0+', got %q", result)
	}
}

func TestDeriveVersionNoSpecVersionWithGit(t *testing.T) {
	dir := initTestGitRepo(t)
	result := deriveVersion("", dir)
	// Should be "0.1.0+<count>.<shortSHA>" or with .dirty
	if !strings.HasPrefix(result, "0.1.0+") {
		t.Errorf("expected result to start with '0.1.0+', got %q", result)
	}
	// Should contain a dot separating count and SHA
	suffix := strings.TrimPrefix(result, "0.1.0+")
	parts := strings.SplitN(suffix, ".", 2)
	if len(parts) < 2 {
		t.Errorf("expected count.sha format in suffix, got %q", suffix)
	}
}

// initTestGitRepo creates a minimal git repository with one commit for testing.
func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := append([]string{"git"}, args...)
		_ = cmd // avoid unused import; exec via gitCommand
		out := gitCommand(dir, args...)
		_ = out
	}
	run("-c", "init.defaultBranch=main", "init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "Test")
	// Create a commit
	testFile := filepath.Join(dir, "README.md")
	if err := os.WriteFile(testFile, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "init")
	return dir
}

// --- Tests: generateContractSummary ---

func TestGenerateContractSummaryNilContract(t *testing.T) {
	wf := &spec.Workflow{Name: "nil-contract-wf"}
	result := generateContractSummary(wf)
	if result != "" {
		t.Errorf("expected empty string for nil contract, got %q", result)
	}
}

func TestGenerateContractSummaryWithDependencies(t *testing.T) {
	wf := &spec.Workflow{
		Name: "contract-wf",
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"tentacular-pg": {Protocol: "postgresql"},
				"openai-api": {
					Protocol: "https",
					Host:     "api.openai.com",
					Port:     443,
					Auth:     &spec.DependencyAuth{Type: "apikey", Secret: "openai-api.api-key"},
				},
			},
		},
	}

	result := generateContractSummary(wf)

	if !strings.Contains(result, "## Dependencies") {
		t.Error("expected ## Dependencies section")
	}
	if !strings.Contains(result, "tentacular-pg") {
		t.Error("expected tentacular-pg in summary")
	}
	if !strings.Contains(result, "managed") {
		t.Error("expected 'managed' label for tentacular- prefixed dep")
	}
	if !strings.Contains(result, "openai-api") {
		t.Error("expected openai-api in summary")
	}
	if !strings.Contains(result, "external") {
		t.Error("expected 'external' label for non-managed dep")
	}
	if !strings.Contains(result, "## Secrets") {
		t.Error("expected ## Secrets section when dep has auth secret")
	}
	if !strings.Contains(result, "openai-api.api-key") {
		t.Error("expected secret key in summary")
	}
}

func TestGenerateContractSummaryDeterministic(t *testing.T) {
	wf := &spec.Workflow{
		Name: "det-wf",
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"zzz": {Protocol: "https"},
				"aaa": {Protocol: "postgresql"},
			},
		},
	}

	result1 := generateContractSummary(wf)
	result2 := generateContractSummary(wf)

	if result1 != result2 {
		t.Error("generateContractSummary is not deterministic")
	}
	// aaa should come before zzz (sorted)
	aIdx := strings.Index(result1, "aaa")
	zIdx := strings.Index(result1, "zzz")
	if aIdx > zIdx {
		t.Error("expected dependencies to be sorted alphabetically")
	}
}

// --- Tests: GenerateMetadataConfigMap ---

func TestGenerateMetadataConfigMapAllKeys(t *testing.T) {
	bundle := &MetadataBundle{
		Readme:          "# My Workflow",
		ContractSummary: "## Dependencies\n- foo",
		ParamsSchema:    "type: object",
		GitProvenance:   &GitProvenance{Commit: "abc1234", Branch: "main", Dirty: false},
	}

	cm, err := GenerateMetadataConfigMap("my-wf", "my-ns", bundle)
	if err != nil {
		t.Fatalf("GenerateMetadataConfigMap failed: %v", err)
	}
	if cm.Kind != "ConfigMap" {
		t.Errorf("expected Kind ConfigMap, got %q", cm.Kind)
	}
	if cm.Name != "my-wf-metadata" {
		t.Errorf("expected name 'my-wf-metadata', got %q", cm.Name)
	}

	content := cm.Content
	if !strings.Contains(content, "name: my-wf-metadata") {
		t.Error("expected ConfigMap name in content")
	}
	if !strings.Contains(content, "namespace: my-ns") {
		t.Error("expected namespace in content")
	}
	if !strings.Contains(content, "readme:") {
		t.Error("expected readme key")
	}
	if !strings.Contains(content, "contract:") {
		t.Error("expected contract key")
	}
	if !strings.Contains(content, "params_schema:") {
		t.Error("expected params_schema key")
	}
	if !strings.Contains(content, "git_provenance:") {
		t.Error("expected git_provenance key")
	}
	if !strings.Contains(content, "abc1234") {
		t.Error("expected commit SHA in git_provenance")
	}
	if !strings.Contains(content, `tentacular.io/metadata: "true"`) {
		t.Error("expected tentacular.io/metadata label")
	}
}

func TestGenerateMetadataConfigMapEmptyBundle(t *testing.T) {
	bundle := &MetadataBundle{}
	cm, err := GenerateMetadataConfigMap("empty-wf", "ns", bundle)
	if err != nil {
		t.Fatalf("GenerateMetadataConfigMap failed: %v", err)
	}
	// Empty bundle should return empty Manifest
	if cm.Kind != "" {
		t.Errorf("expected empty Manifest for empty bundle, got Kind=%q", cm.Kind)
	}
}

func TestGenerateMetadataConfigMapTruncation(t *testing.T) {
	// Create a value larger than maxConfigMapKeySize (100KB)
	bigReadme := strings.Repeat("x", maxConfigMapKeySize+100)
	bundle := &MetadataBundle{Readme: bigReadme}

	cm, err := GenerateMetadataConfigMap("big-wf", "ns", bundle)
	if err != nil {
		t.Fatalf("GenerateMetadataConfigMap failed: %v", err)
	}
	if !strings.Contains(cm.Content, "[truncated]") {
		t.Error("expected [truncated] marker in content")
	}
	// Verify the content doesn't contain the full big readme
	if strings.Contains(cm.Content, bigReadme) {
		t.Error("expected readme to be truncated")
	}
}

// --- Tests: GenerateK8sManifests with metadata ---

func TestGenerateK8sManifestsWithMetadataBundle(t *testing.T) {
	wf := makeMetadataTestWorkflow("meta-bundle-wf")
	bundle := &MetadataBundle{
		Annotations: map[string]string{
			"tentacular.io/nodes":        `["analyze","fetch","report"]`,
			"tentacular.io/version":      "1.2.0+abc1234",
			"tentacular.io/metadata-ref": "meta-bundle-wf-metadata",
		},
		Readme:        "# Test",
		GitProvenance: &GitProvenance{Commit: "abc1234", Branch: "main"},
	}

	opts := DeployOptions{Metadata: bundle}
	manifests := GenerateK8sManifests(wf, "engine:latest", "default", opts)

	// Should have 3 manifests: metadata ConfigMap, Deployment, Service
	if len(manifests) != 3 {
		t.Fatalf("expected 3 manifests with metadata, got %d", len(manifests))
	}

	// Find each manifest by Kind/Name
	var metaCM, dep *Manifest
	for i := range manifests {
		switch {
		case manifests[i].Kind == "ConfigMap" && manifests[i].Name == "meta-bundle-wf-metadata":
			metaCM = &manifests[i]
		case manifests[i].Kind == "Deployment":
			dep = &manifests[i]
		}
	}

	if metaCM == nil {
		t.Error("expected metadata ConfigMap in manifests")
	}
	if dep == nil {
		t.Fatal("expected Deployment manifest")
	}

	// ConfigMap should be before Deployment
	cmIdx, depIdx := -1, -1
	for i, m := range manifests {
		if m.Kind == "ConfigMap" && m.Name == "meta-bundle-wf-metadata" {
			cmIdx = i
		}
		if m.Kind == "Deployment" {
			depIdx = i
		}
	}
	if cmIdx >= depIdx {
		t.Errorf("expected metadata ConfigMap (idx %d) to appear before Deployment (idx %d)", cmIdx, depIdx)
	}

	// Deployment should have the Tier 1 annotations
	depContent := dep.Content
	if !strings.Contains(depContent, "tentacular.io/nodes") {
		t.Error("expected tentacular.io/nodes in Deployment annotations")
	}
	if !strings.Contains(depContent, "1.2.0+abc1234") {
		t.Error("expected version annotation in Deployment")
	}
	if !strings.Contains(depContent, "tentacular.io/metadata-ref") {
		t.Error("expected metadata-ref annotation in Deployment")
	}
}

func TestGenerateK8sManifestsWithoutMetadataBundle(t *testing.T) {
	wf := makeMetadataTestWorkflow("no-bundle-wf")
	manifests := GenerateK8sManifests(wf, "engine:latest", "default", DeployOptions{})

	// Should have 2 manifests: Deployment, Service (no metadata ConfigMap)
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests without metadata, got %d", len(manifests))
	}
}

func TestGenerateK8sManifestsMetadataAnnotationsEscaped(t *testing.T) {
	wf := makeTestWorkflow("escape-test-wf")
	bundle := &MetadataBundle{
		Annotations: map[string]string{
			"tentacular.io/nodes": `["fetch","output"]`,
		},
	}

	opts := DeployOptions{Metadata: bundle}
	manifests := GenerateK8sManifests(wf, "engine:latest", "default", opts)

	// Find the Deployment manifest
	var depContent string
	for _, m := range manifests {
		if m.Kind == "Deployment" {
			depContent = m.Content
			break
		}
	}
	if depContent == "" {
		t.Fatal("no Deployment manifest found")
	}

	// JSON values should be single-quoted in the deployment YAML
	if !strings.Contains(depContent, `tentacular.io/nodes: '["fetch","output"]'`) {
		t.Errorf("expected JSON annotation to be single-quoted, got deployment:\n%s", depContent)
	}
}

// --- Tests: truncateIfNeeded ---

func TestTruncateIfNeededUnderLimit(t *testing.T) {
	v := "hello world"
	result := truncateIfNeeded(v, 100)
	if result != v {
		t.Errorf("expected unchanged value, got %q", result)
	}
}

func TestTruncateIfNeededOverLimit(t *testing.T) {
	v := strings.Repeat("x", 200)
	result := truncateIfNeeded(v, 100)
	if len(result) > 100 {
		t.Errorf("expected truncated to <=100 bytes, got %d", len(result))
	}
	if !strings.HasSuffix(result, "[truncated]") {
		t.Error("expected [truncated] suffix")
	}
}

func TestTruncateIfNeededExactLimit(t *testing.T) {
	v := strings.Repeat("x", 100)
	result := truncateIfNeeded(v, 100)
	if result != v {
		t.Errorf("expected unchanged at exact limit")
	}
}
