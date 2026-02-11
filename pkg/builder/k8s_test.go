package builder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/randybias/tentacular/pkg/spec"
)

func makeTestWorkflow(name string) *spec.Workflow {
	return &spec.Workflow{
		Name:    name,
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "manual"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
		Edges: nil,
	}
}

func TestGenerateK8sManifestsReturnsTwoManifests(t *testing.T) {
	wf := makeTestWorkflow("test-wf")
	manifests := GenerateK8sManifests(wf, "test-wf:1-0", "default", DeployOptions{})

	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}
	if manifests[0].Kind != "Deployment" {
		t.Errorf("expected first manifest kind Deployment, got %s", manifests[0].Kind)
	}
	if manifests[1].Kind != "Service" {
		t.Errorf("expected second manifest kind Service, got %s", manifests[1].Kind)
	}
	if manifests[0].Name != "test-wf" {
		t.Errorf("expected first manifest name test-wf, got %s", manifests[0].Name)
	}
	if manifests[1].Name != "test-wf" {
		t.Errorf("expected second manifest name test-wf, got %s", manifests[1].Name)
	}
}

func TestK8sManifestPodSecurityContext(t *testing.T) {
	wf := makeTestWorkflow("sec-test")
	manifests := GenerateK8sManifests(wf, "sec-test:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "runAsNonRoot: true") {
		t.Error("expected runAsNonRoot: true in pod security context")
	}
	if !strings.Contains(dep, "runAsUser: 65534") {
		t.Error("expected runAsUser: 65534 in pod security context")
	}
	if !strings.Contains(dep, "type: RuntimeDefault") {
		t.Error("expected seccompProfile type RuntimeDefault")
	}
}

func TestK8sManifestContainerSecurityContext(t *testing.T) {
	wf := makeTestWorkflow("container-sec")
	manifests := GenerateK8sManifests(wf, "container-sec:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "readOnlyRootFilesystem: true") {
		t.Error("expected readOnlyRootFilesystem: true")
	}
	if !strings.Contains(dep, "allowPrivilegeEscalation: false") {
		t.Error("expected allowPrivilegeEscalation: false")
	}
	if !strings.Contains(dep, "- ALL") {
		t.Error("expected capabilities drop ALL")
	}
}

func TestK8sManifestLivenessProbe(t *testing.T) {
	wf := makeTestWorkflow("probe-test")
	manifests := GenerateK8sManifests(wf, "probe-test:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "livenessProbe:") {
		t.Fatal("expected livenessProbe section")
	}
	if !strings.Contains(dep, "path: /health") {
		t.Error("expected liveness path /health")
	}
	if !strings.Contains(dep, "port: 8080") {
		t.Error("expected liveness port 8080")
	}
	if !strings.Contains(dep, "initialDelaySeconds: 5") {
		t.Error("expected liveness initialDelaySeconds 5")
	}
	if !strings.Contains(dep, "periodSeconds: 10") {
		t.Error("expected liveness periodSeconds 10")
	}
}

func TestK8sManifestReadinessProbe(t *testing.T) {
	wf := makeTestWorkflow("readiness-test")
	manifests := GenerateK8sManifests(wf, "readiness-test:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "readinessProbe:") {
		t.Fatal("expected readinessProbe section")
	}
	if !strings.Contains(dep, "initialDelaySeconds: 3") {
		t.Error("expected readiness initialDelaySeconds 3")
	}
	if !strings.Contains(dep, "periodSeconds: 5") {
		t.Error("expected readiness periodSeconds 5")
	}
}

func TestK8sManifestRuntimeClassIncluded(t *testing.T) {
	wf := makeTestWorkflow("gvisor-test")
	opts := DeployOptions{RuntimeClassName: "gvisor"}
	manifests := GenerateK8sManifests(wf, "gvisor-test:1-0", "default", opts)
	dep := manifests[0].Content

	if !strings.Contains(dep, "runtimeClassName: gvisor") {
		t.Error("expected runtimeClassName: gvisor when RuntimeClassName is set")
	}
}

func TestK8sManifestRuntimeClassOmitted(t *testing.T) {
	wf := makeTestWorkflow("no-gvisor")
	opts := DeployOptions{RuntimeClassName: ""}
	manifests := GenerateK8sManifests(wf, "no-gvisor:1-0", "default", opts)
	dep := manifests[0].Content

	if strings.Contains(dep, "runtimeClassName") {
		t.Error("expected runtimeClassName to be absent when RuntimeClassName is empty")
	}
}

func TestK8sManifestLabels(t *testing.T) {
	wf := makeTestWorkflow("label-test")
	manifests := GenerateK8sManifests(wf, "label-test:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "app.kubernetes.io/name: label-test") {
		t.Error("expected app.kubernetes.io/name label")
	}
	if !strings.Contains(dep, "app.kubernetes.io/managed-by: tentacular") {
		t.Error("expected app.kubernetes.io/managed-by label")
	}
}

func TestK8sManifestVolumes(t *testing.T) {
	wf := makeTestWorkflow("vol-test")
	manifests := GenerateK8sManifests(wf, "vol-test:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "mountPath: /app/secrets") {
		t.Error("expected /app/secrets volume mount")
	}
	if !strings.Contains(dep, "readOnly: true") {
		t.Error("expected readOnly: true on secrets mount")
	}
	if !strings.Contains(dep, "mountPath: /tmp") {
		t.Error("expected /tmp volume mount")
	}
	if !strings.Contains(dep, "emptyDir: {}") {
		t.Error("expected emptyDir for tmp volume")
	}
	if !strings.Contains(dep, "secretName: vol-test-secrets") {
		t.Error("expected secretName: vol-test-secrets")
	}
	if !strings.Contains(dep, "optional: true") {
		t.Error("expected optional: true on secret volume")
	}
}

func TestK8sManifestResources(t *testing.T) {
	wf := makeTestWorkflow("res-test")
	manifests := GenerateK8sManifests(wf, "res-test:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, `memory: "64Mi"`) {
		t.Error("expected memory request 64Mi")
	}
	if !strings.Contains(dep, `memory: "256Mi"`) {
		t.Error("expected memory limit 256Mi")
	}
	if !strings.Contains(dep, `cpu: "100m"`) {
		t.Error("expected cpu request 100m")
	}
	if !strings.Contains(dep, `cpu: "500m"`) {
		t.Error("expected cpu limit 500m")
	}
}

func TestK8sManifestService(t *testing.T) {
	wf := makeTestWorkflow("svc-test")
	manifests := GenerateK8sManifests(wf, "svc-test:1-0", "default", DeployOptions{})
	svc := manifests[1].Content

	if !strings.Contains(svc, "kind: Service") {
		t.Error("expected kind: Service")
	}
	if !strings.Contains(svc, "type: ClusterIP") {
		t.Error("expected type: ClusterIP")
	}
	if !strings.Contains(svc, "port: 8080") {
		t.Error("expected port: 8080")
	}
}

func TestK8sManifestImageTag(t *testing.T) {
	wf := makeTestWorkflow("img-test")
	tag := "registry.local/img-test:1-0"
	manifests := GenerateK8sManifests(wf, tag, "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "image: "+tag) {
		t.Errorf("expected image field to contain %s", tag)
	}
}

func TestK8sManifestNamespace(t *testing.T) {
	wf := makeTestWorkflow("ns-test")
	manifests := GenerateK8sManifests(wf, "ns-test:1-0", "production", DeployOptions{})

	for _, m := range manifests {
		if !strings.Contains(m.Content, "namespace: production") {
			t.Errorf("expected namespace: production in %s manifest", m.Kind)
		}
	}
}

func TestK8sManifestCronTriggerSingle(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "cron-wf",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "manual"},
			{Type: "cron", Schedule: "0 9 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
	}
	manifests := GenerateK8sManifests(wf, "cron-wf:1-0", "default", DeployOptions{})

	if len(manifests) != 3 {
		t.Fatalf("expected 3 manifests (Deployment, Service, CronJob), got %d", len(manifests))
	}
	if manifests[2].Kind != "CronJob" {
		t.Errorf("expected third manifest kind CronJob, got %s", manifests[2].Kind)
	}
	if manifests[2].Name != "cron-wf-cron" {
		t.Errorf("expected CronJob name cron-wf-cron, got %s", manifests[2].Name)
	}
	if !strings.Contains(manifests[2].Content, `schedule: "0 9 * * *"`) {
		t.Error("expected schedule in CronJob manifest")
	}
	if !strings.Contains(manifests[2].Content, "concurrencyPolicy: Forbid") {
		t.Error("expected concurrencyPolicy: Forbid")
	}
	if !strings.Contains(manifests[2].Content, "successfulJobsHistoryLimit: 3") {
		t.Error("expected successfulJobsHistoryLimit: 3")
	}
	if !strings.Contains(manifests[2].Content, "failedJobsHistoryLimit: 3") {
		t.Error("expected failedJobsHistoryLimit: 3")
	}
}

func TestK8sManifestCronTriggerMultiple(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "multi-cron",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Name: "daily", Schedule: "0 9 * * *"},
			{Type: "cron", Name: "hourly", Schedule: "0 * * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
	}
	manifests := GenerateK8sManifests(wf, "multi-cron:1-0", "default", DeployOptions{})

	if len(manifests) != 4 {
		t.Fatalf("expected 4 manifests (Deployment, Service, 2 CronJobs), got %d", len(manifests))
	}
	if manifests[2].Name != "multi-cron-cron-0" {
		t.Errorf("expected first CronJob name multi-cron-cron-0, got %s", manifests[2].Name)
	}
	if manifests[3].Name != "multi-cron-cron-1" {
		t.Errorf("expected second CronJob name multi-cron-cron-1, got %s", manifests[3].Name)
	}
}

func TestK8sManifestCronTriggerNamedPostBody(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "named-cron",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Name: "daily-digest", Schedule: "0 9 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
	}
	manifests := GenerateK8sManifests(wf, "named-cron:1-0", "default", DeployOptions{})

	cronContent := manifests[2].Content
	if !strings.Contains(cronContent, `daily-digest`) {
		t.Error("expected trigger name in POST body")
	}
}

func TestK8sManifestCronTriggerUnnamedPostBody(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "unnamed-cron",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 9 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
	}
	manifests := GenerateK8sManifests(wf, "unnamed-cron:1-0", "default", DeployOptions{})

	cronContent := manifests[2].Content
	// Should contain {} (not a trigger name)
	if strings.Contains(cronContent, `"trigger"`) {
		t.Error("unnamed trigger should not include trigger field in POST body")
	}
}

func TestK8sManifestCronTriggerLabels(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "label-cron",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 9 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
	}
	manifests := GenerateK8sManifests(wf, "label-cron:1-0", "default", DeployOptions{})

	cronContent := manifests[2].Content
	if !strings.Contains(cronContent, "app.kubernetes.io/name: label-cron") {
		t.Error("expected app.kubernetes.io/name label in CronJob")
	}
	if !strings.Contains(cronContent, "app.kubernetes.io/managed-by: tentacular") {
		t.Error("expected app.kubernetes.io/managed-by label in CronJob")
	}
}

func TestK8sManifestManualOnlyNoRegression(t *testing.T) {
	wf := makeTestWorkflow("manual-only")
	manifests := GenerateK8sManifests(wf, "manual-only:1-0", "default", DeployOptions{})

	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests for manual-only (no CronJobs), got %d", len(manifests))
	}
	for _, m := range manifests {
		if m.Kind == "CronJob" {
			t.Error("manual-only workflow should not have CronJob manifests")
		}
	}
}

func TestK8sManifestCronTriggerServiceURL(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "svc-url-test",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "cron", Schedule: "0 9 * * *"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
	}
	manifests := GenerateK8sManifests(wf, "svc-url-test:1-0", "prod", DeployOptions{})

	cronContent := manifests[2].Content
	if !strings.Contains(cronContent, "http://svc-url-test.prod.svc.cluster.local:8080/run") {
		t.Error("expected correct service URL in CronJob manifest")
	}
}

func TestDockerfileDistrolessBase(t *testing.T) {
	df := GenerateDockerfile()
	if !strings.Contains(df, "FROM denoland/deno:distroless") {
		t.Error("expected distroless base image")
	}
}

func TestDockerfileWorkdir(t *testing.T) {
	df := GenerateDockerfile()
	if !strings.Contains(df, "WORKDIR /app") {
		t.Error("expected WORKDIR /app")
	}
}

func TestDockerfileCopyInstructions(t *testing.T) {
	df := GenerateDockerfile()
	// Assert engine and deno.json are present
	for _, expected := range []string{".engine/", "deno.json"} {
		if !strings.Contains(df, expected) {
			t.Errorf("expected COPY instruction for %s", expected)
		}
	}
	// Assert workflow.yaml and nodes/ COPY instructions are ABSENT
	if strings.Contains(df, "COPY workflow.yaml") {
		t.Error("expected NO COPY workflow.yaml instruction (engine-only image)")
	}
	if strings.Contains(df, "COPY nodes/") {
		t.Error("expected NO COPY nodes/ instruction (engine-only image)")
	}
}

func TestDockerfileCacheAndEntrypoint(t *testing.T) {
	df := GenerateDockerfile()
	if !strings.Contains(df, `"deno", "cache"`) {
		t.Error("expected deno cache instruction")
	}
	if !strings.Contains(df, "--allow-net") {
		t.Error("expected --allow-net in entrypoint")
	}
	if !strings.Contains(df, "--allow-read=/app") {
		t.Error("expected --allow-read=/app in entrypoint")
	}
	if !strings.Contains(df, "--allow-write=/tmp") {
		t.Error("expected --allow-write=/tmp in entrypoint")
	}
	if !strings.Contains(df, "--workflow") || !strings.Contains(df, "/app/workflow/workflow.yaml") {
		t.Error("expected --workflow /app/workflow/workflow.yaml in entrypoint")
	}
	if !strings.Contains(df, "EXPOSE 8080") {
		t.Error("expected EXPOSE 8080")
	}
}

func TestDockerfileNoCLIArtifacts(t *testing.T) {
	df := GenerateDockerfile()
	if strings.Contains(df, "COPY cmd/") || strings.Contains(df, "COPY pkg/") {
		t.Error("Dockerfile should not copy cmd/ or pkg/ directories")
	}
}

func TestConfigMapGeneration(t *testing.T) {
	// Create temp directory with workflow.yaml and nodes/
	tmpDir := t.TempDir()
	workflowContent := `name: test-workflow
version: 1.0
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
`
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0o644); err != nil {
		t.Fatal(err)
	}
	nodesDir := filepath.Join(tmpDir, "nodes")
	if err := os.Mkdir(nodesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	nodeContent := `export default async function run(ctx, input) { return input; }`
	if err := os.WriteFile(filepath.Join(nodesDir, "fetch.ts"), []byte(nodeContent), 0o644); err != nil {
		t.Fatal(err)
	}

	wf := makeTestWorkflow("test-workflow")
	cm, err := GenerateCodeConfigMap(wf, tmpDir, "default")
	if err != nil {
		t.Fatalf("GenerateCodeConfigMap failed: %v", err)
	}

	if cm.Kind != "ConfigMap" {
		t.Errorf("expected kind ConfigMap, got %s", cm.Kind)
	}
	if cm.Name != "test-workflow-code" {
		t.Errorf("expected name test-workflow-code, got %s", cm.Name)
	}
	if !strings.Contains(cm.Content, "namespace: default") {
		t.Error("expected namespace: default")
	}
	if !strings.Contains(cm.Content, "app.kubernetes.io/name: test-workflow") {
		t.Error("expected app.kubernetes.io/name label")
	}
	if !strings.Contains(cm.Content, "app.kubernetes.io/managed-by: tentacular") {
		t.Error("expected app.kubernetes.io/managed-by label")
	}
	if !strings.Contains(cm.Content, "workflow.yaml:") {
		t.Error("expected workflow.yaml data key")
	}
	if !strings.Contains(cm.Content, "nodes__fetch.ts:") {
		t.Error("expected nodes__fetch.ts data key (flattened)")
	}
}

func TestConfigMapSizeValidation(t *testing.T) {
	tmpDir := t.TempDir()
	// Create a workflow.yaml that's > 900KB
	largeContent := strings.Repeat("x", 950000)
	workflowContent := "name: test\nversion: 1.0\ntriggers:\n  - type: manual\ndata: " + largeContent
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0o644); err != nil {
		t.Fatal(err)
	}

	wf := makeTestWorkflow("test")
	_, err := GenerateCodeConfigMap(wf, tmpDir, "default")
	if err == nil {
		t.Error("expected error for oversized ConfigMap")
	}
	if !strings.Contains(err.Error(), "exceeds ConfigMap limit") {
		t.Errorf("expected size limit error, got: %v", err)
	}
}

func TestConfigMapMissingNodesDir(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `name: test-workflow
version: 1.0
triggers:
  - type: manual
`
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0o644); err != nil {
		t.Fatal(err)
	}

	wf := makeTestWorkflow("test-workflow")
	cm, err := GenerateCodeConfigMap(wf, tmpDir, "default")
	if err != nil {
		t.Fatalf("GenerateCodeConfigMap failed when nodes/ missing: %v", err)
	}

	if !strings.Contains(cm.Content, "workflow.yaml:") {
		t.Error("expected workflow.yaml data key")
	}
	if strings.Contains(cm.Content, "nodes/") {
		t.Error("expected NO nodes/ keys when directory doesn't exist")
	}
}

func TestConfigMapSkipsNonTsFiles(t *testing.T) {
	tmpDir := t.TempDir()
	workflowContent := `name: test
version: 1.0
triggers:
  - type: manual
`
	if err := os.WriteFile(filepath.Join(tmpDir, "workflow.yaml"), []byte(workflowContent), 0o644); err != nil {
		t.Fatal(err)
	}
	nodesDir := filepath.Join(tmpDir, "nodes")
	if err := os.Mkdir(nodesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Create .ts and non-.ts files
	if err := os.WriteFile(filepath.Join(nodesDir, "valid.ts"), []byte("// ts file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodesDir, "README.md"), []byte("# readme"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nodesDir, "config.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	wf := makeTestWorkflow("test")
	cm, err := GenerateCodeConfigMap(wf, tmpDir, "default")
	if err != nil {
		t.Fatalf("GenerateCodeConfigMap failed: %v", err)
	}

	if !strings.Contains(cm.Content, "nodes__valid.ts:") {
		t.Error("expected nodes__valid.ts data key (flattened)")
	}
	if strings.Contains(cm.Content, "README.md") {
		t.Error("expected README.md to be skipped")
	}
	if strings.Contains(cm.Content, "config.json") {
		t.Error("expected config.json to be skipped")
	}
}

func TestDeploymentHasCodeVolumeMount(t *testing.T) {
	wf := makeTestWorkflow("vol-test")
	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "name: code") {
		t.Error("expected code volume")
	}
	if !strings.Contains(dep, "configMap:") {
		t.Error("expected configMap volume source")
	}
	if !strings.Contains(dep, "name: vol-test-code") {
		t.Error("expected ConfigMap name vol-test-code")
	}
	if !strings.Contains(dep, "mountPath: /app/workflow") {
		t.Error("expected code volume mount at /app/workflow")
	}
	if !strings.Contains(dep, "readOnly: true") {
		t.Error("expected readOnly: true on code volume mount")
	}
	if !strings.Contains(dep, "items:") {
		t.Error("expected items field in ConfigMap volume")
	}
	if !strings.Contains(dep, "key: nodes__fetch.ts") {
		t.Error("expected flattened key nodes__fetch.ts in items")
	}
	if !strings.Contains(dep, "path: nodes/fetch.ts") {
		t.Error("expected path nodes/fetch.ts in items")
	}
}

func TestDeploymentNoContainerArgs(t *testing.T) {
	wf := makeTestWorkflow("args-test")
	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content

	// Verify NO args field in container spec (relies on ENTRYPOINT defaults)
	if strings.Contains(dep, "args:") {
		t.Error("expected NO container args field (ENTRYPOINT provides defaults)")
	}
}
