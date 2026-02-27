package k8s

import (
	"fmt"

	"github.com/randybias/tentacular/pkg/builder"
)

const (
	// DefaultMCPImage is the default tentacular-mcp server image.
	DefaultMCPImage = "ghcr.io/randybias/tentacular-mcp:latest"

	// DefaultMCPNamespace is the namespace where the MCP server is installed.
	DefaultMCPNamespace = "tentacular-system"

	// DefaultMCPPort is the HTTP port the MCP server listens on.
	DefaultMCPPort = 8080

	// MCPTokenSecretName is the K8s Secret name that holds the MCP bearer token.
	MCPTokenSecretName = "tentacular-mcp-token"

	// MCPServiceName is the K8s Service name for the MCP server.
	MCPServiceName = "tentacular-mcp"
)

// GenerateMCPServerManifests returns the full set of K8s manifests required to
// deploy the tentacular-mcp server:
//
//  1. ServiceAccount
//  2. ClusterRole
//  3. ClusterRoleBinding
//  4. Secret (bearer token)
//  5. Deployment
//  6. Service
func GenerateMCPServerManifests(namespace, image, token string) []builder.Manifest {
	if image == "" {
		image = DefaultMCPImage
	}

	return []builder.Manifest{
		{Kind: "ServiceAccount", Name: "tentacular-mcp", Content: mcpServiceAccount(namespace)},
		{Kind: "ClusterRole", Name: "tentacular-mcp", Content: mcpClusterRole()},
		{Kind: "ClusterRoleBinding", Name: "tentacular-mcp", Content: mcpClusterRoleBinding(namespace)},
		{Kind: "Secret", Name: MCPTokenSecretName, Content: mcpTokenSecret(namespace, token)},
		{Kind: "Deployment", Name: "tentacular-mcp", Content: mcpDeployment(namespace, image)},
		{Kind: "Service", Name: MCPServiceName, Content: mcpService(namespace)},
	}
}

// MCPEndpointInCluster returns the in-cluster URL for the MCP server.
func MCPEndpointInCluster(namespace string) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d", MCPServiceName, namespace, DefaultMCPPort)
}

func mcpServiceAccount(namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: tentacular-mcp
  namespace: %s
  labels:
    app.kubernetes.io/name: tentacular-mcp
    app.kubernetes.io/managed-by: tentacular
`, namespace)
}

func mcpClusterRole() string {
	return `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: tentacular-mcp
  labels:
    app.kubernetes.io/name: tentacular-mcp
    app.kubernetes.io/managed-by: tentacular
rules:
- apiGroups: [""]
  resources: ["pods", "pods/log", "services", "configmaps", "secrets", "events"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["batch"]
  resources: ["jobs", "cronjobs"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: ["networking.k8s.io"]
  resources: ["networkpolicies"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["namespaces"]
  verbs: ["get", "list", "watch", "create", "delete"]
- apiGroups: ["authentication.k8s.io"]
  resources: ["tokenreviews"]
  verbs: ["create"]
`
}

func mcpClusterRoleBinding(namespace string) string {
	return fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: tentacular-mcp
  labels:
    app.kubernetes.io/name: tentacular-mcp
    app.kubernetes.io/managed-by: tentacular
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: tentacular-mcp
subjects:
- kind: ServiceAccount
  name: tentacular-mcp
  namespace: %s
`, namespace)
}

func mcpTokenSecret(namespace, token string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: tentacular-mcp
    app.kubernetes.io/managed-by: tentacular
type: Opaque
stringData:
  token: %q
`, MCPTokenSecretName, namespace, token)
}

func mcpDeployment(namespace, image string) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: tentacular-mcp
  namespace: %s
  labels:
    app.kubernetes.io/name: tentacular-mcp
    app.kubernetes.io/managed-by: tentacular
spec:
  replicas: 1
  strategy:
    type: Recreate
  selector:
    matchLabels:
      app.kubernetes.io/name: tentacular-mcp
  template:
    metadata:
      labels:
        app.kubernetes.io/name: tentacular-mcp
        app.kubernetes.io/managed-by: tentacular
    spec:
      serviceAccountName: tentacular-mcp
      automountServiceAccountToken: true
      volumes:
      - name: mcp-token
        secret:
          secretName: %s
      containers:
      - name: tentacular-mcp
        image: %s
        ports:
        - containerPort: %d
          name: http
        env:
        - name: TOKEN_PATH
          value: /etc/tentacular-mcp/token
        - name: TENTACULAR_MCP_NAMESPACE
          value: %s
        volumeMounts:
        - name: mcp-token
          mountPath: /etc/tentacular-mcp
          readOnly: true
        livenessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 5
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /healthz
            port: http
          initialDelaySeconds: 3
          periodSeconds: 5
        resources:
          requests:
            cpu: "100m"
            memory: "128Mi"
          limits:
            cpu: "500m"
            memory: "512Mi"
`, namespace, MCPTokenSecretName, image, DefaultMCPPort, namespace)
}

func mcpService(namespace string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %s
  namespace: %s
  labels:
    app.kubernetes.io/name: tentacular-mcp
    app.kubernetes.io/managed-by: tentacular
spec:
  selector:
    app.kubernetes.io/name: tentacular-mcp
  ports:
  - name: http
    port: %d
    targetPort: http
  type: ClusterIP
`, MCPServiceName, namespace, DefaultMCPPort)
}
