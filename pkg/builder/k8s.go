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
	workflowContent, err := os.ReadFile(workflowPath) //nolint:gosec // reading user-specified workflow files
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
			nodeContent, readErr := os.ReadFile(nodePath) //nolint:gosec // reading user-specified workflow files
			if readErr != nil {
				return Manifest{}, fmt.Errorf("reading %s: %w", nodePath, readErr)
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

// buildDeployAnnotations converts workflow metadata, description, and cron triggers
// into a YAML annotations block. Returns empty string if all fields are empty.
// Values are sanitized to prevent YAML injection via newlines or special characters.
// If triggers contains cron triggers, a tentacular.io/cron-schedule annotation is
// added with comma-separated schedules (one per cron trigger).
func buildDeployAnnotations(meta *spec.WorkflowMetadata, triggers []spec.Trigger, description string) string {
	var lines []string
	if v := sanitizeAnnotationValue(description); v != "" {
		lines = append(lines, "    tentacular.io/description: "+v)
	}
	if meta != nil {
		if meta.Group != "" {
			if v := sanitizeAnnotationValue(meta.Group); v != "" {
				lines = append(lines, "    tentacular.io/group: "+v)
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
				lines = append(lines, "    tentacular.io/tags: "+strings.Join(cleanTags, ","))
			}
		}
		if meta.Environment != "" {
			if v := sanitizeAnnotationValue(meta.Environment); v != "" {
				lines = append(lines, "    tentacular.io/environment: "+v)
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
		lines = append(lines, fmt.Sprintf(`    tentacular.io/cron-schedule: "%s"`, strings.Join(cronSchedules, ",")))
	}
	if len(lines) == 0 {
		return ""
	}
	return "  annotations:\n" + strings.Join(lines, "\n") + "\n"
}

// buildSidecarContainers generates YAML for sidecar container specs.
// Each sidecar gets the same SecurityContext hardening as the engine container.
// Returns the YAML string to inject into the containers section (8-space base indent).
func buildSidecarContainers(sidecars []spec.SidecarSpec) string {
	if len(sidecars) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, sc := range sidecars {
		// Container name and image
		fmt.Fprintf(&sb, "        - name: %s\n", sc.Name)
		fmt.Fprintf(&sb, "          image: %s\n", sc.Image)
		sb.WriteString("          imagePullPolicy: Always\n")

		// Command (optional)
		if len(sc.Command) > 0 {
			sb.WriteString("          command:\n")
			for _, c := range sc.Command {
				fmt.Fprintf(&sb, "            - %q\n", c)
			}
		}

		// Args (optional — always quote to prevent YAML type coercion)
		if len(sc.Args) > 0 {
			sb.WriteString("          args:\n")
			for _, a := range sc.Args {
				fmt.Fprintf(&sb, "            - %q\n", a)
			}
		}

		// Env (optional)
		if len(sc.Env) > 0 {
			sb.WriteString("          env:\n")
			// Sort keys for deterministic output
			envKeys := make([]string, 0, len(sc.Env))
			for k := range sc.Env {
				envKeys = append(envKeys, k)
			}
			sort.Strings(envKeys)
			for _, k := range envKeys {
				fmt.Fprintf(&sb, "            - name: %s\n", k)
				fmt.Fprintf(&sb, "              value: %s\n", sc.Env[k])
			}
		}

		// Port
		sb.WriteString("          ports:\n")
		fmt.Fprintf(&sb, "            - containerPort: %d\n", sc.Port)
		sb.WriteString("              protocol: TCP\n")

		// SecurityContext (same hardening as engine)
		sb.WriteString("          securityContext:\n")
		sb.WriteString("            readOnlyRootFilesystem: true\n")
		sb.WriteString("            allowPrivilegeEscalation: false\n")
		sb.WriteString("            capabilities:\n")
		sb.WriteString("              drop:\n")
		sb.WriteString("                - ALL\n")

		// Readiness probe
		healthPath := sc.HealthPath
		if healthPath == "" {
			healthPath = "/health"
		}
		sb.WriteString("          readinessProbe:\n")
		sb.WriteString("            httpGet:\n")
		fmt.Fprintf(&sb, "              path: %s\n", healthPath)
		fmt.Fprintf(&sb, "              port: %d\n", sc.Port)
		sb.WriteString("            initialDelaySeconds: 3\n")
		sb.WriteString("            periodSeconds: 5\n")

		// Volume mounts: /shared + /tmp
		sb.WriteString("          volumeMounts:\n")
		sb.WriteString("            - name: shared\n")
		sb.WriteString("              mountPath: /shared\n")
		fmt.Fprintf(&sb, "            - name: tmp-%s\n", sc.Name)
		sb.WriteString("              mountPath: /tmp\n")

		// Resources (optional)
		if sc.Resources != nil {
			sb.WriteString("          resources:\n")
			if sc.Resources.Requests.CPU != "" || sc.Resources.Requests.Memory != "" {
				sb.WriteString("            requests:\n")
				if sc.Resources.Requests.Memory != "" {
					fmt.Fprintf(&sb, "              memory: \"%s\"\n", sc.Resources.Requests.Memory)
				}
				if sc.Resources.Requests.CPU != "" {
					fmt.Fprintf(&sb, "              cpu: \"%s\"\n", sc.Resources.Requests.CPU)
				}
			}
			if sc.Resources.Limits.CPU != "" || sc.Resources.Limits.Memory != "" {
				sb.WriteString("            limits:\n")
				if sc.Resources.Limits.Memory != "" {
					fmt.Fprintf(&sb, "              memory: \"%s\"\n", sc.Resources.Limits.Memory)
				}
				if sc.Resources.Limits.CPU != "" {
					fmt.Fprintf(&sb, "              cpu: \"%s\"\n", sc.Resources.Limits.CPU)
				}
			}
		}
	}
	return sb.String()
}

// buildSidecarVolumes generates volume YAML for sidecar support.
// Creates a shared emptyDir + per-sidecar /tmp emptyDir volumes.
// Returns the YAML string to append to the volumes section (8-space base indent).
func buildSidecarVolumes(sidecars []spec.SidecarSpec) string {
	if len(sidecars) == 0 {
		return ""
	}

	var sb strings.Builder
	// Shared emptyDir for engine <-> sidecar file handoff
	sb.WriteString("        - name: shared\n")
	sb.WriteString("          emptyDir:\n")
	sb.WriteString("            sizeLimit: 1Gi\n")
	// Per-sidecar /tmp volumes (needed for tools like ffmpeg)
	for _, sc := range sidecars {
		fmt.Fprintf(&sb, "        - name: tmp-%s\n", sc.Name)
		sb.WriteString("          emptyDir:\n")
		sb.WriteString("            sizeLimit: 256Mi\n")
	}
	return sb.String()
}

// GenerateK8sManifests produces K8s manifests for deploying a workflow.
func GenerateK8sManifests(wf *spec.Workflow, imageTag, namespace string, opts DeployOptions) []Manifest {
	manifests := make([]Manifest, 0, 2)

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
	configMapItems := make([]string, 0, 1+len(wf.Nodes))
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
	// Mount at /app/engine/deno.json because Deno discovers config by walking up from the
	// entrypoint (/app/engine/main.ts) and the engine image bakes a deno.json at that path.
	// Mounting at /app/deno.json would be shadowed by the closer /app/engine/deno.json.
	importMapVolumeMount := `            - name: import-map
              mountPath: /app/engine/deno.json
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
	denoFlags := spec.DeriveDenoFlags(wf.Contract, wf.Sidecars, proxyHost)
	if len(denoFlags) > 0 {
		var lines []string
		lines = append(lines, "          command:")
		lines = append(lines, "            - "+denoFlags[0]) // "deno"
		if len(denoFlags) > 1 {
			lines = append(lines, "          args:")
			for _, arg := range denoFlags[1:] {
				// Quote numeric values so YAML treats them as strings (K8s args must be strings)
				if _, err := fmt.Sscanf(arg, "%d", new(int)); err == nil {
					lines = append(lines, fmt.Sprintf("            - \"%s\"", arg))
				} else {
					lines = append(lines, "            - "+arg)
				}
			}
		}
		commandArgsBlock = strings.Join(lines, "\n") + "\n"
	}

	// Build sidecar containers block (empty string if no sidecars)
	sidecarContainersBlock := buildSidecarContainers(wf.Sidecars)

	// Engine shared volume mount (only when sidecars declared)
	engineSharedMount := ""
	if len(wf.Sidecars) > 0 {
		engineSharedMount = "            - name: shared\n              mountPath: /shared\n"
	}

	// Build sidecar volumes block (empty string if no sidecars)
	sidecarVolumesBlock := buildSidecarVolumes(wf.Sidecars)

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
%s%s          resources:
            requests:
              memory: "64Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "500m"
%s      volumes:
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
%s%s`, wf.Name, namespace, labels, buildDeployAnnotations(wf.Metadata, wf.Triggers, wf.Description), wf.Name, labels, runtimeClassLine, imageTag, imagePullPolicy, commandArgsBlock, engineSharedMount, importMapVolumeMount, sidecarContainersBlock, wf.Name, strings.Join(configMapItems, "\n"), wf.Name, sidecarVolumesBlock, importMapVolume)

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
`, wf.Name, namespace, labels, buildDeployAnnotations(wf.Metadata, nil, wf.Description), wf.Name)

	manifests = append(manifests, Manifest{
		Kind: "Service", Name: wf.Name, Content: service,
	})

	return manifests
}
