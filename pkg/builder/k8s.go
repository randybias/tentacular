package builder

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/randybias/tentacular/pkg/spec"
)

// Manifest is a raw K8s manifest as YAML string.
type Manifest struct {
	Kind    string
	Name    string
	Content string
}

// DeployOptions controls optional features in generated manifests.
type DeployOptions struct {
	RuntimeClassName string // If empty, omit runtimeClassName from pod spec
	ImagePullPolicy  string // If empty, defaults to "Always"
	ModuleProxyURL   string // If set, used to scope Deno net flags for jsr/npm proxy host
}

// GenerateCodeConfigMap produces a ConfigMap containing workflow code (workflow.yaml + nodes/*.ts).
// Returns error if total data size exceeds 900KB limit.
func GenerateCodeConfigMap(wf *spec.Workflow, workflowDir, namespace string) (Manifest, error) {
	data := make(map[string]string)
	var totalSize int

	// Read workflow.yaml
	workflowPath := filepath.Join(workflowDir, "workflow.yaml")
	workflowContent, err := os.ReadFile(workflowPath)
	if err != nil {
		return Manifest{}, fmt.Errorf("reading workflow.yaml: %w", err)
	}
	data["workflow.yaml"] = string(workflowContent)
	totalSize += len(workflowContent)

	// Read nodes/*.ts files (if directory exists)
	nodesDir := filepath.Join(workflowDir, "nodes")
	entries, err := os.ReadDir(nodesDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".ts") {
				continue
			}
			nodePath := filepath.Join(nodesDir, entry.Name())
			nodeContent, err := os.ReadFile(nodePath)
			if err != nil {
				return Manifest{}, fmt.Errorf("reading %s: %w", nodePath, err)
			}
			// Use __ instead of / since K8s ConfigMap keys cannot contain slashes
			dataKey := "nodes__" + entry.Name()
			data[dataKey] = string(nodeContent)
			totalSize += len(nodeContent)
		}
	} else if !os.IsNotExist(err) {
		return Manifest{}, fmt.Errorf("reading nodes directory: %w", err)
	}

	// Check size limit (900KB = 921600 bytes)
	const maxSize = 921600
	if totalSize > maxSize {
		return Manifest{}, fmt.Errorf("workflow code size (%d bytes) exceeds ConfigMap limit of %d bytes (900KB)", totalSize, maxSize)
	}

	// Build data section with sorted keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dataEntries []string
	for _, k := range keys {
		v := data[k]
		// Escape the value for YAML
		dataEntries = append(dataEntries, fmt.Sprintf("  %s: |\n%s", k, indentString(v, 4)))
	}

	labels := fmt.Sprintf(`app.kubernetes.io/name: %s
    app.kubernetes.io/version: "%s"
    app.kubernetes.io/managed-by: tentacular`, wf.Name, wf.Version)

	content := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-code
  namespace: %s
  labels:
    %s
data:
%s
`, wf.Name, namespace, labels, strings.Join(dataEntries, "\n"))

	return Manifest{
		Kind:    "ConfigMap",
		Name:    wf.Name + "-code",
		Content: content,
	}, nil
}

// indentString indents each line of s by n spaces.
func indentString(s string, n int) string {
	prefix := strings.Repeat(" ", n)
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// sanitizeAnnotationValue strips characters that could break YAML annotation
// output when values are interpolated directly into a YAML template string.
// Newlines and carriage returns are removed to prevent injection of additional
// annotation keys; leading/trailing whitespace is trimmed.
func sanitizeAnnotationValue(v string) string {
	v = strings.ReplaceAll(v, "\n", "")
	v = strings.ReplaceAll(v, "\r", "")
	return strings.TrimSpace(v)
}

// buildDeployAnnotations converts workflow metadata and cron triggers into a YAML
// annotations block. Returns empty string if all fields are empty.
// Values are sanitized to prevent YAML injection via newlines or special characters.
// If triggers contains cron triggers, a tentacular.dev/cron-schedule annotation is
// added with comma-separated schedules (one per cron trigger).
func buildDeployAnnotations(meta *spec.WorkflowMetadata, triggers []spec.Trigger) string {
	var lines []string
	if meta != nil {
		if meta.Owner != "" {
			if v := sanitizeAnnotationValue(meta.Owner); v != "" {
				lines = append(lines, fmt.Sprintf("    tentacular.dev/owner: %s", v))
			}
		}
		if meta.Team != "" {
			if v := sanitizeAnnotationValue(meta.Team); v != "" {
				lines = append(lines, fmt.Sprintf("    tentacular.dev/team: %s", v))
			}
		}
		if len(meta.Tags) > 0 {
			var cleanTags []string
			for _, tag := range meta.Tags {
				if v := sanitizeAnnotationValue(tag); v != "" {
					cleanTags = append(cleanTags, v)
				}
			}
			if len(cleanTags) > 0 {
				lines = append(lines, fmt.Sprintf("    tentacular.dev/tags: %s", strings.Join(cleanTags, ",")))
			}
		}
		if meta.Environment != "" {
			if v := sanitizeAnnotationValue(meta.Environment); v != "" {
				lines = append(lines, fmt.Sprintf("    tentacular.dev/environment: %s", v))
			}
		}
	}
	// Add cron-schedule annotation for any cron triggers.
	// Multiple schedules are joined with commas. The MCP server reads this annotation
	// to register the workflow with the in-process scheduler (no CronJob pods needed).
	var cronSchedules []string
	for _, t := range triggers {
		if t.Type == "cron" && t.Schedule != "" {
			cronSchedules = append(cronSchedules, t.Schedule)
		}
	}
	if len(cronSchedules) > 0 {
		lines = append(lines, fmt.Sprintf(`    tentacular.dev/cron-schedule: "%s"`, strings.Join(cronSchedules, ",")))
	}
	if len(lines) == 0 {
		return ""
	}
	return "  annotations:\n" + strings.Join(lines, "\n") + "\n"
}

// GenerateK8sManifests produces K8s manifests for deploying a workflow.
func GenerateK8sManifests(wf *spec.Workflow, imageTag, namespace string, opts DeployOptions) []Manifest {
	var manifests []Manifest

	labels := fmt.Sprintf(`app.kubernetes.io/name: %s
    app.kubernetes.io/version: "%s"
    app.kubernetes.io/managed-by: tentacular`, wf.Name, wf.Version)

	// RuntimeClass line (conditional)
	runtimeClassLine := ""
	if opts.RuntimeClassName != "" {
		runtimeClassLine = fmt.Sprintf("      runtimeClassName: %s\n", opts.RuntimeClassName)
	}

	// ImagePullPolicy (default: Always)
	imagePullPolicy := opts.ImagePullPolicy
	if imagePullPolicy == "" {
		imagePullPolicy = "Always"
	}

	// Build ConfigMap items to map flattened keys back to proper paths
	// K8s ConfigMap keys cannot contain slashes, so we use __ as separator
	var configMapItems []string
	configMapItems = append(configMapItems, "              - key: workflow.yaml\n                path: workflow.yaml")

	// Sort node names for deterministic output
	nodeNames := make([]string, 0, len(wf.Nodes))
	for name := range wf.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)

	for _, nodeName := range nodeNames {
		nodeSpec := wf.Nodes[nodeName]
		// Extract filename from path (e.g., "./nodes/foo.ts" -> "foo.ts")
		filename := filepath.Base(nodeSpec.Path)
		flatKey := "nodes__" + filename
		targetPath := "nodes/" + filename
		configMapItems = append(configMapItems, fmt.Sprintf("              - key: %s\n                path: %s", flatKey, targetPath))
	}

	// Always mount the import map — the engine has jsr: deps (e.g. @nats-io/transport-deno)
	// that must route through the in-cluster module proxy. Workflow namespaces cannot reach
	// external registries directly.
	importMapVolumeMount := `            - name: import-map
              mountPath: /app/deno.json
              subPath: deno.json
              readOnly: true
`
	importMapVolume := fmt.Sprintf(`        - name: import-map
          configMap:
            name: %s-import-map
            optional: true
`, wf.Name)

	// Build command/args block for Deno permission flags (10-space indent for container-level)
	commandArgsBlock := ""
	// Extract host:port from ModuleProxyURL for DeriveDenoFlags scoping.
	// If empty, DeriveDenoFlags falls back to the default constant.
	proxyHost := ""
	if opts.ModuleProxyURL != "" {
		proxyHost = strings.TrimPrefix(opts.ModuleProxyURL, "http://")
		proxyHost = strings.TrimRight(proxyHost, "/")
	}
	denoFlags := spec.DeriveDenoFlags(wf.Contract, proxyHost)
	if denoFlags != nil && len(denoFlags) > 0 {
		var lines []string
		lines = append(lines, "          command:")
		lines = append(lines, fmt.Sprintf("            - %s", denoFlags[0])) // "deno"
		if len(denoFlags) > 1 {
			lines = append(lines, "          args:")
			for _, arg := range denoFlags[1:] {
				// Quote numeric values so YAML treats them as strings (K8s args must be strings)
				if _, err := fmt.Sscanf(arg, "%d", new(int)); err == nil {
					lines = append(lines, fmt.Sprintf("            - \"%s\"", arg))
				} else {
					lines = append(lines, fmt.Sprintf("            - %s", arg))
				}
			}
		}
		commandArgsBlock = strings.Join(lines, "\n") + "\n"
	}

	// Deployment with security hardening
	deployment := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    %s
%sspec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: %s
  template:
    metadata:
      labels:
        %s
    spec:
      automountServiceAccountToken: false
%s      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: engine
          image: %s
          imagePullPolicy: %s
%s          env:
            - name: DENO_DIR
              value: /tmp/deno-cache
          ports:
            - containerPort: 8080
              protocol: TCP
          securityContext:
            readOnlyRootFilesystem: true
            allowPrivilegeEscalation: false
            capabilities:
              drop:
                - ALL
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health
              port: 8080
            initialDelaySeconds: 3
            periodSeconds: 5
          volumeMounts:
            - name: code
              mountPath: /app/workflow
              readOnly: true
            - name: secrets
              mountPath: /app/secrets
              readOnly: true
            - name: tmp
              mountPath: /tmp
%s          resources:
            requests:
              memory: "64Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "500m"
      volumes:
        - name: code
          configMap:
            name: %s-code
            items:
%s
        - name: secrets
          secret:
            secretName: %s-secrets
            optional: true
        - name: tmp
          emptyDir:
            sizeLimit: 512Mi
%s`, wf.Name, namespace, labels, buildDeployAnnotations(wf.Metadata, wf.Triggers), wf.Name, labels, runtimeClassLine, imageTag, imagePullPolicy, commandArgsBlock, importMapVolumeMount, wf.Name, strings.Join(configMapItems, "\n"), wf.Name, importMapVolume)

	manifests = append(manifests, Manifest{
		Kind: "Deployment", Name: wf.Name, Content: deployment,
	})

	// Service
	service := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
  labels:
    %s
%sspec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: %s
  ports:
    - port: 8080
      targetPort: 8080
      protocol: TCP
`, wf.Name, namespace, labels, buildDeployAnnotations(wf.Metadata, nil), wf.Name)

	manifests = append(manifests, Manifest{
		Kind: "Service", Name: wf.Name, Content: service,
	})

	return manifests
}

