package k8s

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/randybias/tentacular/pkg/builder"
	"github.com/randybias/tentacular/pkg/spec"
)

// DefaultModuleProxyURL is the in-cluster URL of the esm.sh module proxy service.
// Installed by `tntc cluster install` in the tentacular-system namespace.
const DefaultModuleProxyURL = "http://esm-sh.tentacular-system.svc.cluster.local:8080"

// GenerateImportMap produces a ConfigMap manifest containing a Deno import_map.json
// that rewrites all jsr: and npm: specifiers to the in-cluster module proxy URL.
// Returns nil if the workflow has no jsr/npm dependencies.
func GenerateImportMap(wf *spec.Workflow, proxyURL string) *builder.Manifest {
	if wf.Contract == nil || len(wf.Contract.Dependencies) == 0 {
		return nil
	}

	if proxyURL == "" {
		proxyURL = DefaultModuleProxyURL
	}
	proxyURL = strings.TrimRight(proxyURL, "/")

	// Collect jsr/npm deps in sorted order for deterministic output
	type entry struct {
		specifier string
		url       string
	}
	var entries []entry

	for _, dep := range wf.Contract.Dependencies {
		if dep.Protocol != "jsr" && dep.Protocol != "npm" {
			continue
		}
		if dep.Host == "" {
			continue
		}

		var specifier, proxyPath string
		switch dep.Protocol {
		case "jsr":
			// jsr:@scope/package → proxy/jsr/@scope/package[@version]
			specifier = "jsr:" + dep.Host
			proxyPath = "/jsr/" + dep.Host
		case "npm":
			// npm:package → proxy/package[@version]
			specifier = "npm:" + dep.Host
			proxyPath = "/" + dep.Host
		}

		if dep.Version != "" {
			proxyPath += "@" + dep.Version
		}

		entries = append(entries, entry{
			specifier: specifier,
			url:       proxyURL + proxyPath,
		})
	}

	if len(entries) == 0 {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].specifier < entries[j].specifier
	})

	imports := make(map[string]string, len(entries))
	for _, e := range entries {
		imports[e.specifier] = e.url
	}

	importMap := map[string]interface{}{
		"imports": imports,
	}
	importMapJSON, err := json.MarshalIndent(importMap, "", "  ")
	if err != nil {
		return nil
	}

	configMap := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-import-map
  namespace: %s
  labels:
    app.kubernetes.io/name: %s
    app.kubernetes.io/managed-by: tentacular
  annotations:
    tentacular.dev/proxy-url: %s
data:
  import_map.json: |
%s
`,
		wf.Name,
		"__NAMESPACE__", // replaced by caller with actual namespace
		wf.Name,
		proxyURL,
		indentLines(string(importMapJSON), "    "),
	)

	return &builder.Manifest{
		Kind:    "ConfigMap",
		Name:    wf.Name + "-import-map",
		Content: configMap,
	}
}

// GenerateImportMapWithNamespace produces the import map ConfigMap with the given namespace.
func GenerateImportMapWithNamespace(wf *spec.Workflow, namespace, proxyURL string) *builder.Manifest {
	m := GenerateImportMap(wf, proxyURL)
	if m == nil {
		return nil
	}
	m.Content = strings.ReplaceAll(m.Content, "__NAMESPACE__", namespace)
	return m
}

// HasModuleProxyDeps returns true if the workflow has any jsr or npm dependencies.
// Delegates to spec.HasModuleProxyDeps.
func HasModuleProxyDeps(wf *spec.Workflow) bool {
	return spec.HasModuleProxyDeps(wf)
}

// indentLines prepends each line with the given prefix.
func indentLines(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = prefix + line
		}
	}
	return strings.Join(lines, "\n")
}

// GenerateModuleProxyManifests returns the set of K8s manifests for the esm.sh
// module proxy service, deployed into tentacular-system by `tntc cluster install`.
func GenerateModuleProxyManifests(image, namespace, storage, pvcSize string) []builder.Manifest {
	if image == "" {
		image = "ghcr.io/esm-dev/esm.sh:v135"
	}
	if namespace == "" {
		namespace = "tentacular-system"
	}

	var volumeSpec, volumeMountSpec, pvcManifest string

	if storage == "pvc" {
		if pvcSize == "" {
			pvcSize = "5Gi"
		}
		volumeSpec = `      volumes:
        - name: cache
          persistentVolumeClaim:
            claimName: esm-sh-cache`
		volumeMountSpec = `          volumeMounts:
            - name: cache
              mountPath: /esm.sh/cache`
		pvcManifest = fmt.Sprintf(`---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: esm-sh-cache
  namespace: %s
  labels:
    app.kubernetes.io/name: esm-sh
    app.kubernetes.io/managed-by: tentacular
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: %s
`, namespace, pvcSize)
	} else {
		// emptyDir (default) — cache is lost on pod restart but no PVC needed
		volumeSpec = `      volumes:
        - name: cache
          emptyDir:
            sizeLimit: 2Gi`
		volumeMountSpec = `          volumeMounts:
            - name: cache
              mountPath: /esm.sh/cache`
	}

	deployment := fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: esm-sh
  namespace: %s
  labels:
    app.kubernetes.io/name: esm-sh
    app.kubernetes.io/managed-by: tentacular
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: esm-sh
  template:
    metadata:
      labels:
        app.kubernetes.io/name: esm-sh
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534
      containers:
        - name: esm-sh
          image: %s
          ports:
            - containerPort: 8080
              protocol: TCP
          env:
            - name: ESM_ORIGIN
              value: http://esm-sh.%s.svc.cluster.local:8080
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "500m"
%s
%s
`, namespace, image, namespace, volumeMountSpec, volumeSpec)

	service := fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: esm-sh
  namespace: %s
  labels:
    app.kubernetes.io/name: esm-sh
    app.kubernetes.io/managed-by: tentacular
spec:
  selector:
    app.kubernetes.io/name: esm-sh
  ports:
    - name: http
      protocol: TCP
      port: 8080
      targetPort: 8080
`, namespace)

	networkPolicy := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: esm-sh-netpol
  namespace: %s
  labels:
    app.kubernetes.io/name: esm-sh
    app.kubernetes.io/managed-by: tentacular
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: esm-sh
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 8080
  egress:
  - to:
    - podSelector:
        matchLabels:
          k8s-app: kube-dns
      namespaceSelector:
        matchLabels:
          kubernetes.io/metadata.name: kube-system
    ports:
    - protocol: UDP
      port: 53
    - protocol: TCP
      port: 53
  - to:
    - ipBlock:
        cidr: 0.0.0.0/0
        except:
        - 10.0.0.0/8
        - 172.16.0.0/12
        - 192.168.0.0/16
    ports:
    - protocol: TCP
      port: 443
`, namespace)

	manifests := []builder.Manifest{
		{Kind: "Deployment", Name: "esm-sh", Content: deployment},
		{Kind: "Service", Name: "esm-sh", Content: service},
		{Kind: "NetworkPolicy", Name: "esm-sh-netpol", Content: networkPolicy},
	}

	if pvcManifest != "" {
		manifests = append(manifests, builder.Manifest{
			Kind:    "PersistentVolumeClaim",
			Name:    "esm-sh-cache",
			Content: pvcManifest,
		})
	}

	return manifests
}
