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
%s      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: engine
          image: %s
          imagePullPolicy: %s
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
          resources:
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
          emptyDir: {}
`, wf.Name, namespace, labels, wf.Name, labels, runtimeClassLine, imageTag, imagePullPolicy, wf.Name, strings.Join(configMapItems, "\n"), wf.Name)

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
