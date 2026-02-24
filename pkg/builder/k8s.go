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
	ModuleProxyURL   string // If set and workflow has jsr/npm deps, generates a proxy pre-warm initContainer
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

	// Build conditional import map volume mount and volume (for jsr/npm module proxy deps)
	importMapVolumeMount := ""
	importMapVolume := ""
	if spec.HasModuleProxyDeps(wf) {
		// Mount the merged deno.json (engine + workflow deps) at /app/engine/deno.json.
		// Deno's config discovery walks up from the entrypoint (engine/main.ts) and
		// finds /app/engine/deno.json before /app/deno.json, so we must mount here.
		importMapVolumeMount = `            - name: import-map
              mountPath: /app/engine/deno.json
              subPath: deno.json
              readOnly: true
`
		importMapVolume = fmt.Sprintf(`        - name: import-map
          configMap:
            name: %s-import-map
            optional: true
`, wf.Name)
	}

	// Build command/args block for Deno permission flags (10-space indent for container-level)
	commandArgsBlock := ""
	denoFlags := spec.DeriveDenoFlags(wf.Contract)
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

	// Build proxy pre-warm initContainer when module proxy deps are present.
	// The initContainer runs in-cluster before the engine starts, triggering esm.sh
	// to build and cache each jsr/npm module. This prevents cold-start timeouts where
	// the first build exceeds esm.sh's 60s context deadline at pod startup.
	initContainerBlock := ""
	if opts.ModuleProxyURL != "" && spec.HasModuleProxyDeps(wf) {
		proxyBase := strings.TrimRight(opts.ModuleProxyURL, "/")
		seenURLs := make(map[string]bool)
		var curlURLs []string
		for _, dep := range wf.Contract.Dependencies {
			if dep.Protocol != "jsr" && dep.Protocol != "npm" {
				continue
			}
			var url string
			switch dep.Protocol {
			case "jsr":
				url = proxyBase + "/jsr/" + dep.Host
			case "npm":
				url = proxyBase + "/" + dep.Host
			}
			if dep.Version != "" {
				url += "@" + dep.Version
			}
			if !seenURLs[url] {
				curlURLs = append(curlURLs, url)
				seenURLs[url] = true
			}
		}
		sort.Strings(curlURLs)
		if len(curlURLs) > 0 {
			var cmds []string
			for _, url := range curlURLs {
				// Use '|| true' rather than '|| echo ...' to avoid ': ' in the echo
				// message causing the YAML scalar to be parsed as a map key:value.
				cmds = append(cmds, fmt.Sprintf("curl -sf --retry 3 --retry-delay 10 --max-time 120 %s > /dev/null || true", url))
			}
			shellScript := strings.Join(cmds, "; ")
			// %q wraps in YAML-safe double quotes so '>', '|', '||' inside the
			// shell script are treated as literal characters, not YAML operators.
			initContainerBlock = fmt.Sprintf(`      initContainers:
        - name: proxy-prewarm
          image: curlimages/curl:latest
          command:
            - /bin/sh
            - -c
            - %q
          resources:
            limits:
              memory: "32Mi"
              cpu: "100m"
`, shellScript)
		}
	}

	// Deployment with security hardening
	deployment := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    %s
spec:
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
%s      containers:
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
%s`, wf.Name, namespace, labels, wf.Name, labels, runtimeClassLine, initContainerBlock, imageTag, imagePullPolicy, commandArgsBlock, importMapVolumeMount, wf.Name, strings.Join(configMapItems, "\n"), wf.Name, importMapVolume)

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
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: %s
  ports:
    - port: 8080
      targetPort: 8080
      protocol: TCP
`, wf.Name, namespace, labels, wf.Name)

	manifests = append(manifests, Manifest{
		Kind: "Service", Name: wf.Name, Content: service,
	})

	// CronJobs for cron triggers
	var cronTriggers []cronTriggerInfo
	for i, t := range wf.Triggers {
		if t.Type == "cron" {
			cronTriggers = append(cronTriggers, cronTriggerInfo{index: i, trigger: t})
		}
	}
	for i, ct := range cronTriggers {
		cronName := wf.Name + "-cron"
		if len(cronTriggers) > 1 {
			cronName = fmt.Sprintf("%s-cron-%d", wf.Name, i)
		}
		cronManifest := generateCronJobManifest(wf.Name, cronName, namespace, labels, ct.trigger)
		manifests = append(manifests, cronManifest)
	}

	return manifests
}

type cronTriggerInfo struct {
	index   int
	trigger spec.Trigger
}

func generateCronJobManifest(wfName, cronName, namespace, labels string, trigger spec.Trigger) Manifest {
	postBody := "{}"
	if trigger.Name != "" {
		postBody = fmt.Sprintf(`{\"trigger\":\"%s\"}`, trigger.Name)
	}

	svcURL := fmt.Sprintf("http://%s.%s.svc.cluster.local:8080/run", wfName, namespace)

	content := fmt.Sprintf(`apiVersion: batch/v1
kind: CronJob
metadata:
  name: %s
  namespace: %s
  labels:
    %s
spec:
  schedule: "%s"
  concurrencyPolicy: Forbid
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
            tentacular.dev/role: trigger
        spec:
          restartPolicy: OnFailure
          containers:
            - name: trigger
              image: curlimages/curl:latest
              command:
                - curl
                - -sf
                - -X
                - POST
                - -H
                - "Content-Type: application/json"
                - -d
                - "%s"
                - %s
              resources:
                limits:
                  memory: "32Mi"
                  cpu: "100m"
`, cronName, namespace, labels, trigger.Schedule, postBody, svcURL)

	return Manifest{
		Kind: "CronJob", Name: cronName, Content: content,
	}
}
