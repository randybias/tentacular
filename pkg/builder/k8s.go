package builder

import (
	"fmt"

	"github.com/randyb/pipedreamer2/pkg/spec"
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
}

// GenerateK8sManifests produces K8s manifests for deploying a workflow.
func GenerateK8sManifests(wf *spec.Workflow, imageTag, namespace string, opts DeployOptions) []Manifest {
	var manifests []Manifest

	labels := fmt.Sprintf(`app.kubernetes.io/name: %s
    app.kubernetes.io/managed-by: pipedreamer`, wf.Name)

	// RuntimeClass line (conditional)
	runtimeClassLine := ""
	if opts.RuntimeClassName != "" {
		runtimeClassLine = fmt.Sprintf("      runtimeClassName: %s\n", opts.RuntimeClassName)
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
        - name: secrets
          secret:
            secretName: %s-secrets
            optional: true
        - name: tmp
          emptyDir: {}
`, wf.Name, namespace, labels, wf.Name, labels, runtimeClassLine, imageTag, wf.Name)

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

	return manifests
}
