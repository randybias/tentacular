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
	if !strings.Contains(dep, `app.kubernetes.io/version: "1.0"`) {
		t.Error("expected app.kubernetes.io/version label")
	}
}

func TestK8sManifestImagePullPolicy(t *testing.T) {
	wf := makeTestWorkflow("pull-test")
	manifests := GenerateK8sManifests(wf, "pull-test:1-0", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "imagePullPolicy: Always") {
		t.Error("expected imagePullPolicy: Always in Deployment")
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
	if !strings.Contains(dep, "emptyDir:") {
		t.Error("expected emptyDir for tmp volume")
	}
	if !strings.Contains(dep, "sizeLimit: 512Mi") {
		t.Error("expected sizeLimit: 512Mi on emptyDir volume")
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

func TestK8sManifestServiceLabels(t *testing.T) {
	wf := makeTestWorkflow("svc-label-test")
	manifests := GenerateK8sManifests(wf, "svc-label-test:1-0", "default", DeployOptions{})
	svc := manifests[1].Content

	if !strings.Contains(svc, "app.kubernetes.io/name: svc-label-test") {
		t.Error("expected app.kubernetes.io/name label in Service")
	}
	if !strings.Contains(svc, "app.kubernetes.io/managed-by: tentacular") {
		t.Error("expected app.kubernetes.io/managed-by label in Service")
	}
	if !strings.Contains(svc, `app.kubernetes.io/version: "1.0"`) {
		t.Error("expected app.kubernetes.io/version label in Service")
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

	// Cron triggers are encoded as an annotation on the Deployment, not as CronJob manifests.
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests (Deployment, Service), got %d", len(manifests))
	}
	for _, m := range manifests {
		if m.Kind == "CronJob" {
			t.Errorf("unexpected CronJob manifest: cron triggers are now annotations on the Deployment")
		}
	}
	dep := manifests[0].Content
	if !strings.Contains(dep, "tentacular.dev/cron-schedule: 0 9 * * *") {
		t.Error("expected cron-schedule annotation on Deployment with schedule")
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

	// Multiple cron triggers are comma-joined in the cron-schedule annotation.
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests (Deployment, Service), got %d", len(manifests))
	}
	for _, m := range manifests {
		if m.Kind == "CronJob" {
			t.Errorf("unexpected CronJob manifest: cron triggers are now annotations on the Deployment")
		}
	}
	dep := manifests[0].Content
	if !strings.Contains(dep, "tentacular.dev/cron-schedule:") {
		t.Error("expected cron-schedule annotation on Deployment")
	}
	// Both schedules should appear in the annotation (comma-joined)
	if !strings.Contains(dep, "0 9 * * *") {
		t.Error("expected daily schedule in cron-schedule annotation")
	}
	if !strings.Contains(dep, "0 * * * *") {
		t.Error("expected hourly schedule in cron-schedule annotation")
	}
}

func TestK8sManifestCronTriggerNamedScheduleAnnotation(t *testing.T) {
	// Named triggers should still appear in the cron-schedule annotation.
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

	dep := manifests[0].Content
	if !strings.Contains(dep, "tentacular.dev/cron-schedule: 0 9 * * *") {
		t.Error("expected cron-schedule annotation with schedule on Deployment")
	}
}

func TestK8sManifestCronTriggerNoCronJobGenerated(t *testing.T) {
	// Verify no CronJob manifests are generated regardless of trigger type.
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

	for _, m := range manifests {
		if m.Kind == "CronJob" {
			t.Errorf("unexpected CronJob manifest: cron triggers are now annotations on the Deployment")
		}
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
	// Manual-only workflows should not have the cron-schedule annotation
	dep := manifests[0].Content
	if strings.Contains(dep, "tentacular.dev/cron-schedule") {
		t.Error("expected no cron-schedule annotation for manual-only workflow")
	}
}

func TestK8sManifestCronTriggerAnnotationOnlyTwoManifests(t *testing.T) {
	// Cron workflows now produce only Deployment+Service (no CronJob) with the
	// schedule encoded as a Deployment annotation for the MCP scheduler.
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

	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests (Deployment+Service), got %d", len(manifests))
	}
	dep := manifests[0].Content
	if !strings.Contains(dep, "tentacular.dev/cron-schedule: 0 9 * * *") {
		t.Error("expected cron-schedule annotation on Deployment")
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
	if !strings.Contains(cm.Content, `app.kubernetes.io/version: "1.0"`) {
		t.Error("expected app.kubernetes.io/version label in ConfigMap")
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

func TestDeploymentAutomountServiceAccountTokenFalse(t *testing.T) {
	wf := makeTestWorkflow("sa-token-test")
	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "automountServiceAccountToken: false") {
		t.Error("expected automountServiceAccountToken: false in Deployment spec")
	}
}

func TestDeploymentEmptyDirSizeLimit(t *testing.T) {
	wf := makeTestWorkflow("size-limit-test")
	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content

	if !strings.Contains(dep, "sizeLimit: 512Mi") {
		t.Error("expected sizeLimit: 512Mi on emptyDir volume")
	}
}

func TestDeploymentScopedNetWithContract(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "scoped-net-test",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "manual"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"github": {
					Protocol: "https",
					Host:     "api.github.com",
					Port:     443,
				},
			},
		},
	}

	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content

	// Should have command and args with scoped --allow-net
	if !strings.Contains(dep, "command:") {
		t.Error("expected command field when contract exists")
	}
	if !strings.Contains(dep, "args:") {
		t.Error("expected args field when contract exists")
	}
	if !strings.Contains(dep, "--allow-net=0.0.0.0:8080,api.github.com:443") {
		t.Error("expected scoped --allow-net with specific hosts in args")
	}
	if !strings.Contains(dep, "--allow-env=DENO_DIR,HOME") {
		t.Error("expected scoped --allow-env=DENO_DIR,HOME in args")
	}
}

func TestDeploymentBroadNetWithDynamicTarget(t *testing.T) {
	wf := &spec.Workflow{
		Name:    "broad-net-test",
		Version: "1.0",
		Triggers: []spec.Trigger{
			{Type: "manual"},
		},
		Nodes: map[string]spec.NodeSpec{
			"fetch": {Path: "./nodes/fetch.ts"},
		},
		Contract: &spec.Contract{
			Dependencies: map[string]spec.Dependency{
				"external-api": {
					Protocol: "https",
					Type:     "dynamic-target",
					CIDR:     "0.0.0.0/0",
					DynPorts: []string{"443/TCP"},
				},
			},
		},
	}

	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content

	// Should have command and args with broad --allow-net
	if !strings.Contains(dep, "command:") {
		t.Error("expected command field when contract exists")
	}
	if !strings.Contains(dep, "args:") {
		t.Error("expected args field when contract exists")
	}
	// For dynamic-target, should use broad --allow-net (not scoped)
	if !strings.Contains(dep, "- --allow-net") {
		t.Error("expected broad --allow-net flag in args for dynamic-target")
	}
	// Should NOT have scoped form like --allow-net=host:port
	lines := strings.Split(dep, "\n")
	for _, line := range lines {
		if strings.Contains(line, "--allow-net=") {
			t.Error("expected NO scoped --allow-net= for dynamic-target, should use broad --allow-net")
		}
	}
}

func TestDeploymentNoArgsWithoutContract(t *testing.T) {
	wf := makeTestWorkflow("no-contract-test")
	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content

	// Without contract, should NOT inject command/args (rely on ENTRYPOINT)
	if strings.Contains(dep, "command:") {
		t.Error("expected NO command field when contract is nil")
	}
	if strings.Contains(dep, "args:") {
		t.Error("expected NO args field when contract is nil")
	}
}

func TestDeploymentDENODIRAlwaysSet(t *testing.T) {
	// DENO_DIR=/tmp/deno-cache must be in every Deployment so Deno's cache
	// stays in /tmp (writable even with gVisor's read-only /deno-dir).
	wf := makeTestWorkflow("deno-dir-test")
	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content
	if !strings.Contains(dep, "DENO_DIR") {
		t.Error("expected DENO_DIR env var in Deployment")
	}
	if !strings.Contains(dep, "/tmp/deno-cache") {
		t.Error("expected DENO_DIR=/tmp/deno-cache in Deployment")
	}
}

func TestDeploymentNoInitContainers(t *testing.T) {
	// The proxy pre-warm initContainer has been removed. Module pre-warming is now
	// handled server-side by the MCP server's PrewarmModules function after wf_apply.
	wf := &spec.Workflow{
		Name:    "prewarm-test",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Triggers: []spec.Trigger{{Type: "manual"}},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"pg": {Protocol: "jsr", Host: "@db/postgres", Version: "0.19.5"},
			},
		},
	}

	t.Run("no initContainers even with ModuleProxyURL set and jsr deps", func(t *testing.T) {
		opts := DeployOptions{
			ModuleProxyURL: "http://esm-sh.tentacular-support.svc.cluster.local:8080",
		}
		manifests := GenerateK8sManifests(wf, "test:latest", "default", opts)
		dep := manifests[0].Content
		if strings.Contains(dep, "initContainers:") {
			t.Error("expected no initContainers block (pre-warming moved to MCP server)")
		}
		if strings.Contains(dep, "proxy-prewarm") {
			t.Error("expected no proxy-prewarm reference in Deployment")
		}
		if strings.Contains(dep, "curlimages") {
			t.Error("expected no curlimages reference in Deployment")
		}
	})

	t.Run("no initContainers without ModuleProxyURL", func(t *testing.T) {
		manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
		dep := manifests[0].Content
		if strings.Contains(dep, "initContainers:") {
			t.Error("expected no initContainers block")
		}
	})
}

func TestDeploymentImportMapMountPath(t *testing.T) {
	// deno.json must be mounted at /app/deno.json so Deno's config discovery
	// finds it when the entrypoint is /app/mod.ts and "tentacular" resolves to ./mod.ts.
	wf := &spec.Workflow{
		Name:    "mount-path-test",
		Version: "1.0",
		Nodes:   map[string]spec.NodeSpec{"n": {Path: "./nodes/n.ts"}},
		Triggers: []spec.Trigger{{Type: "manual"}},
		Contract: &spec.Contract{
			Version: "1",
			Dependencies: map[string]spec.Dependency{
				"pg": {Protocol: "jsr", Host: "@db/postgres", Version: "0.19.5"},
			},
		},
	}
	manifests := GenerateK8sManifests(wf, "test:latest", "default", DeployOptions{})
	dep := manifests[0].Content
	if !strings.Contains(dep, "mountPath: /app/deno.json") {
		t.Error("expected import map mounted at /app/deno.json")
	}
	if strings.Contains(dep, "/app/engine/deno.json") {
		t.Error("unexpected mount at /app/engine/deno.json — entrypoint is now /app/mod.ts")
	}
}

func TestBuildDeployAnnotationsNilMetadata(t *testing.T) {
	result := buildDeployAnnotations(nil, nil)
	if result != "" {
		t.Errorf("expected empty string for nil metadata and no triggers, got %q", result)
	}
}

func TestBuildDeployAnnotationsAllEmpty(t *testing.T) {
	result := buildDeployAnnotations(&spec.WorkflowMetadata{}, nil)
	if result != "" {
		t.Errorf("expected empty string for empty metadata struct and no triggers, got %q", result)
	}
}

func TestBuildDeployAnnotationsFullMetadata(t *testing.T) {
	meta := &spec.WorkflowMetadata{
		Owner:       "platform-team",
		Team:        "infra",
		Tags:        []string{"production", "critical"},
		Environment: "prod",
	}
	result := buildDeployAnnotations(meta, nil)
	if !strings.Contains(result, "tentacular.dev/owner: platform-team") {
		t.Error("expected owner annotation")
	}
	if !strings.Contains(result, "tentacular.dev/team: infra") {
		t.Error("expected team annotation")
	}
	if !strings.Contains(result, "tentacular.dev/tags: production,critical") {
		t.Error("expected tags annotation")
	}
	if !strings.Contains(result, "tentacular.dev/environment: prod") {
		t.Error("expected environment annotation")
	}
	if !strings.HasPrefix(result, "  annotations:\n") {
		t.Error("expected result to start with annotations block")
	}
}

func TestBuildDeployAnnotationsPartialMetadata(t *testing.T) {
	meta := &spec.WorkflowMetadata{
		Owner: "data-team",
		// Team, Tags, Environment intentionally omitted
	}
	result := buildDeployAnnotations(meta, nil)
	if !strings.Contains(result, "tentacular.dev/owner: data-team") {
		t.Error("expected owner annotation")
	}
	if strings.Contains(result, "tentacular.dev/team:") {
		t.Error("expected no team annotation when Team is empty")
	}
	if strings.Contains(result, "tentacular.dev/tags:") {
		t.Error("expected no tags annotation when Tags is nil")
	}
	if strings.Contains(result, "tentacular.dev/environment:") {
		t.Error("expected no environment annotation when Environment is empty")
	}
}

func TestBuildDeployAnnotationsCronScheduleSingle(t *testing.T) {
	triggers := []spec.Trigger{
		{Type: "cron", Schedule: "0 9 * * *"},
	}
	result := buildDeployAnnotations(nil, triggers)
	if !strings.Contains(result, "tentacular.dev/cron-schedule: 0 9 * * *") {
		t.Error("expected cron-schedule annotation with single schedule")
	}
}

func TestBuildDeployAnnotationsCronScheduleMultiple(t *testing.T) {
	triggers := []spec.Trigger{
		{Type: "cron", Schedule: "0 9 * * *"},
		{Type: "manual"},
		{Type: "cron", Schedule: "0 * * * *"},
	}
	result := buildDeployAnnotations(nil, triggers)
	if !strings.Contains(result, "tentacular.dev/cron-schedule: 0 9 * * *,0 * * * *") {
		t.Errorf("expected comma-joined schedules in cron-schedule annotation, got: %q", result)
	}
}

func TestBuildDeployAnnotationsNewlineInjectionBlocked(t *testing.T) {
	meta := &spec.WorkflowMetadata{
		Owner: "foo\n    injected.key: evil-value",
		Team:  "bar\r\nbaz",
	}
	result := buildDeployAnnotations(meta, nil)

	// Verify no additional YAML lines were injected. After sanitization the
	// newlines are removed, so the only annotation lines should be
	// tentacular.dev/owner and tentacular.dev/team.
	lines := strings.Split(strings.TrimSpace(result), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "annotations:" {
			continue
		}
		if !strings.HasPrefix(trimmed, "tentacular.dev/") {
			t.Errorf("unexpected annotation line (possible injection): %q", line)
		}
	}

	// The tentacular.dev/owner line should still be present (value is sanitized but non-empty)
	if !strings.Contains(result, "tentacular.dev/owner:") {
		t.Error("expected tentacular.dev/owner annotation to be present")
	}

	// No bare newlines should appear inside an annotation value
	if strings.Contains(result, "tentacular.dev/owner: foo\n") {
		t.Error("expected no trailing newline within annotation value")
	}
}

func TestK8sManifestMetadataAnnotations(t *testing.T) {
	wf := makeTestWorkflow("meta-test")
	wf.Metadata = &spec.WorkflowMetadata{
		Owner: "platform-team",
		Team:  "infra",
		Tags:  []string{"production", "critical"},
	}
	manifests := GenerateK8sManifests(wf, "meta-test:1-0", "default", DeployOptions{})
	dep := manifests[0].Content
	svc := manifests[1].Content

	if !strings.Contains(dep, "tentacular.dev/owner: platform-team") {
		t.Error("expected owner annotation in Deployment")
	}
	if !strings.Contains(dep, "tentacular.dev/team: infra") {
		t.Error("expected team annotation in Deployment")
	}
	if !strings.Contains(dep, "tentacular.dev/tags: production,critical") {
		t.Error("expected tags annotation in Deployment")
	}
	if !strings.Contains(svc, "tentacular.dev/owner: platform-team") {
		t.Error("expected owner annotation in Service")
	}
}

func TestK8sManifestNoMetadataNoAnnotations(t *testing.T) {
	wf := makeTestWorkflow("no-meta")
	manifests := GenerateK8sManifests(wf, "no-meta:1-0", "default", DeployOptions{})
	dep := manifests[0].Content
	svc := manifests[1].Content

	if strings.Contains(dep, "annotations:") {
		t.Error("expected no annotations block in Deployment when metadata is nil")
	}
	if strings.Contains(svc, "annotations:") {
		t.Error("expected no annotations block in Service when metadata is nil")
	}
}
