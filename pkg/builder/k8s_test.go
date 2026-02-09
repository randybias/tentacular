package builder

import (
	"strings"
	"testing"

	"github.com/randyb/pipedreamer2/pkg/spec"
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
	if !strings.Contains(dep, "app.kubernetes.io/managed-by: pipedreamer") {
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
	for _, expected := range []string{".engine/", "workflow.yaml", "nodes/", "deno.json"} {
		if !strings.Contains(df, expected) {
			t.Errorf("expected COPY instruction for %s", expected)
		}
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
