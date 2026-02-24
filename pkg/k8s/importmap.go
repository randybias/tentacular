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

// engineDenoImports holds the engine's base import map entries from engine/deno.json.
// These are merged with workflow jsr/npm entries in the generated deno.json ConfigMap.
// TODO: auto-sync from engine/deno.json at build time instead of hardcoding here.
// Keep in sync with engine/deno.json whenever engine deps change.
var engineDenoImports = map[string]string{
	"tentacular":              "./engine/mod.ts",
	"std/":                    "https://deno.land/std@0.224.0/",
	"std/yaml":                "https://deno.land/std@0.224.0/yaml/mod.ts",
	"std/path":                "https://deno.land/std@0.224.0/path/mod.ts",
	"std/flags":               "https://deno.land/std@0.224.0/flags/mod.ts",
	"std/assert":              "https://deno.land/std@0.224.0/assert/mod.ts",
	"@nats-io/transport-deno": "jsr:@nats-io/transport-deno@3.3.0",
	"@std/fmt/colors":         "https://deno.land/std@0.224.0/fmt/colors.ts",
	"@std/io":                 "https://deno.land/std@0.224.0/io/mod.ts",
	"@std/bytes":              "https://deno.land/std@0.224.0/bytes/mod.ts",
}

// GenerateImportMap produces a ConfigMap manifest containing a merged deno.json that
// combines engine import entries with jsr:/npm: rewrites pointing to the in-cluster
// module proxy. Mounted at /app/deno.json, it overrides the image-baked deno.json so
// Deno auto-discovers it without needing --import-map (which would break engine imports).
// Returns nil if the workflow has no jsr/npm dependencies.
func GenerateImportMap(wf *spec.Workflow, proxyURL string) *builder.Manifest {
	if wf.Contract == nil || len(wf.Contract.Dependencies) == 0 {
		return nil
	}

	if proxyURL == "" {
		proxyURL = DefaultModuleProxyURL
	}
	proxyURL = strings.TrimRight(proxyURL, "/")

	// Build merged imports: engine entries + workflow jsr/npm entries
	imports := make(map[string]string, len(engineDenoImports))
	for k, v := range engineDenoImports {
		imports[k] = v
	}

	hasProxyDeps := false
	for _, dep := range wf.Contract.Dependencies {
		if dep.Protocol != "jsr" && dep.Protocol != "npm" {
			continue
		}
		if dep.Host == "" {
			continue
		}

		switch dep.Protocol {
		case "jsr":
			// Emit two keys when a version is specified:
			//   "jsr:@scope/pkg@version" (exact match for versioned imports in code)
			//   "jsr:@scope/pkg"         (fallback for bare/unversioned imports)
			// Both point to the versioned proxy URL to keep the pinned version.
			// Deno's import map does exact-key lookup — the unversioned key alone
			// would never intercept "jsr:@db/postgres@0.19.5".
			baseSpec := "jsr:" + dep.Host
			proxyPath := "/jsr/" + dep.Host
			if dep.Version != "" {
				proxyPath += "@" + dep.Version
				imports[baseSpec+"@"+dep.Version] = proxyURL + proxyPath
			}
			imports[baseSpec] = proxyURL + proxyPath
		case "npm":
			// Same dual-key strategy for npm: specifiers.
			baseSpec := "npm:" + dep.Host
			proxyPath := "/" + dep.Host
			if dep.Version != "" {
				proxyPath += "@" + dep.Version
				imports[baseSpec+"@"+dep.Version] = proxyURL + proxyPath
			}
			imports[baseSpec] = proxyURL + proxyPath
		}
		hasProxyDeps = true
	}

	if !hasProxyDeps {
		return nil
	}

	// Build sorted imports for deterministic output
	keys := make([]string, 0, len(imports))
	for k := range imports {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	orderedImports := make(map[string]string, len(imports))
	for _, k := range keys {
		orderedImports[k] = imports[k]
	}

	denoConfig := map[string]interface{}{
		"compilerOptions": map[string]interface{}{
			"strict":                 true,
			"noUncheckedIndexedAccess": true,
		},
		"imports": orderedImports,
	}
	denoConfigJSON, err := json.MarshalIndent(denoConfig, "", "  ")
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
  deno.json: |
%s
`,
		wf.Name,
		"__NAMESPACE__",
		wf.Name,
		proxyURL,
		indentLines(string(denoConfigJSON), "    "),
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
		image = "ghcr.io/esm-dev/esm.sh:v136"
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
              mountPath: /.esmd`
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
		// emptyDir (default) — cache is lost on pod restart but no PVC needed.
		// Mounted at /.esmd so esm.sh (v136+) can write its data directory even
		// as runAsUser: 65534 (nobody), since emptyDir is world-writable on creation.
		volumeSpec = `      volumes:
        - name: cache
          emptyDir:
            sizeLimit: 2Gi`
		volumeMountSpec = `          volumeMounts:
            - name: cache
              mountPath: /.esmd`
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
