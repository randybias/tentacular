package builder

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/randybias/tentacular/pkg/spec"
)

const (
	maxConfigMapKeySize   = 100 * 1024 // 100KB per key
	maxConfigMapTotalSize = 500 * 1024 // 500KB total
)

// GitProvenance holds git state for the tentacle directory.
type GitProvenance struct {
	Commit string `json:"commit"`         // short SHA
	Branch string `json:"branch"`         // current branch
	Repo   string `json:"repo,omitempty"` // remote origin URL
	Dirty  bool   `json:"dirty"`          // uncommitted changes
}

// MetadataBundle holds all metadata extracted from the tentacle directory.
type MetadataBundle struct {
	Annotations     map[string]string
	GitProvenance   *GitProvenance
	Readme          string
	ContractSummary string
	ParamsSchema    string
	Prompts         string
}

// ReadMetadata reads all metadata files from the tentacle directory
// and extracts structural metadata from the parsed workflow.
// Metadata errors never block a deploy — missing or unreadable files are silently skipped.
func ReadMetadata(wf *spec.Workflow, tentacleDir string) *MetadataBundle {
	bundle := &MetadataBundle{
		Annotations: make(map[string]string),
	}

	// --- Tier 1: Structural metadata from workflow spec ---
	buildTier1Annotations(bundle, wf)

	// --- Tier 1: Version from git + workflow.yaml ---
	version := deriveVersion(wf.Version, tentacleDir)
	bundle.Annotations["tentacular.io/version"] = version

	// --- Tier 1: Scaffold name (if .scaffold file present) ---
	scaffoldFile := filepath.Join(tentacleDir, ".scaffold")
	if data, err := os.ReadFile(scaffoldFile); err == nil { //nolint:gosec // reading user-specified files
		name := strings.TrimSpace(string(data))
		if name != "" {
			bundle.Annotations["tentacular.io/scaffold-name"] = name
		}
	}

	// --- Tier 1: metadata-ref always points to the ConfigMap ---
	bundle.Annotations["tentacular.io/metadata-ref"] = wf.Name + "-metadata"

	// --- Tier 2: README.md ---
	readmePath := filepath.Join(tentacleDir, "README.md")
	if data, err := os.ReadFile(readmePath); err == nil { //nolint:gosec // reading user-specified files
		bundle.Readme = string(data)
	}

	// --- Tier 2: params.schema.yaml ---
	paramsPath := filepath.Join(tentacleDir, "params.schema.yaml")
	if data, err := os.ReadFile(paramsPath); err == nil { //nolint:gosec // reading user-specified files
		bundle.ParamsSchema = string(data)
	}

	// --- Tier 2: prompts.yaml ---
	promptsPath := filepath.Join(tentacleDir, "prompts.yaml")
	if data, err := os.ReadFile(promptsPath); err == nil { //nolint:gosec // reading user-specified files
		bundle.Prompts = string(data)
	}
	if bundle.Prompts != "" {
		pc, tc := promptTemplateCounts(bundle.Prompts)
		if pc > 0 {
			bundle.Annotations["tentacular.io/prompt-count"] = strconv.Itoa(pc)
		}
		if tc > 0 {
			bundle.Annotations["tentacular.io/template-count"] = strconv.Itoa(tc)
		}
	}

	// --- Tier 2: Contract summary ---
	summaryPath := filepath.Join(tentacleDir, "contract-summary.md")
	if data, err := os.ReadFile(summaryPath); err == nil { //nolint:gosec // reading user-specified files
		bundle.ContractSummary = string(data)
	} else if wf.Contract != nil {
		bundle.ContractSummary = generateContractSummary(wf)
	}

	// --- Tier 2: Git provenance ---
	bundle.GitProvenance = readGitProvenance(tentacleDir)

	return bundle
}

// buildTier1Annotations extracts structural metadata from the workflow spec
// and stores them as annotations in the bundle.
func buildTier1Annotations(bundle *MetadataBundle, wf *spec.Workflow) {
	// Nodes: sorted list of node names
	nodeNames := make([]string, 0, len(wf.Nodes))
	for name := range wf.Nodes {
		nodeNames = append(nodeNames, name)
	}
	sort.Strings(nodeNames)
	if len(nodeNames) > 0 {
		nodesJSON, err := json.Marshal(nodeNames)
		if err == nil {
			bundle.Annotations["tentacular.io/nodes"] = string(nodesJSON)
		}
	}

	// Edges: array of [from, to] pairs
	if len(wf.Edges) > 0 {
		edges := make([][2]string, 0, len(wf.Edges))
		for _, e := range wf.Edges {
			edges = append(edges, [2]string{e.From, e.To})
		}
		edgesJSON, err := json.Marshal(edges)
		if err == nil {
			bundle.Annotations["tentacular.io/edges"] = string(edgesJSON)
		}
	}

	// Sidecars: array of {name, image, port}
	if len(wf.Sidecars) > 0 {
		type sidecarMeta struct {
			Name  string `json:"name"`
			Image string `json:"image"`
			Port  int    `json:"port"`
		}
		scMeta := make([]sidecarMeta, 0, len(wf.Sidecars))
		for _, sc := range wf.Sidecars {
			scMeta = append(scMeta, sidecarMeta{
				Name:  sc.Name,
				Image: sc.Image,
				Port:  sc.Port,
			})
		}
		scJSON, err := json.Marshal(scMeta)
		if err == nil {
			bundle.Annotations["tentacular.io/sidecars"] = string(scJSON)
		}
	}

	// Dependencies: array of {name, protocol, managed}
	if wf.Contract != nil && len(wf.Contract.Dependencies) > 0 {
		type depMeta struct {
			Name     string `json:"name"`
			Protocol string `json:"protocol"`
			Managed  bool   `json:"managed"`
		}
		deps := make([]depMeta, 0, len(wf.Contract.Dependencies))
		for name, dep := range wf.Contract.Dependencies {
			managed := strings.HasPrefix(name, "tentacular-")
			deps = append(deps, depMeta{
				Name:     name,
				Protocol: dep.Protocol,
				Managed:  managed,
			})
		}
		sort.Slice(deps, func(i, j int) bool { return deps[i].Name < deps[j].Name })
		depsJSON, err := json.Marshal(deps)
		if err == nil {
			bundle.Annotations["tentacular.io/dependencies"] = string(depsJSON)
		}
	}

	// Trigger type: comma-separated list of unique trigger types
	if len(wf.Triggers) > 0 {
		types := make([]string, 0, len(wf.Triggers))
		seen := make(map[string]bool)
		for _, t := range wf.Triggers {
			if !seen[t.Type] {
				types = append(types, t.Type)
				seen[t.Type] = true
			}
		}
		bundle.Annotations["tentacular.io/trigger-type"] = strings.Join(types, ",")
	}
}

// GenerateMetadataConfigMap produces a ConfigMap containing rich Tier 2 metadata.
// Returns an empty Manifest if no metadata content is available.
func GenerateMetadataConfigMap(name, namespace string, bundle *MetadataBundle) (Manifest, error) {
	data := make(map[string]string)

	if bundle.Readme != "" {
		data["readme"] = truncateIfNeeded(bundle.Readme, maxConfigMapKeySize)
	}
	if bundle.ContractSummary != "" {
		data["contract"] = truncateIfNeeded(bundle.ContractSummary, maxConfigMapKeySize)
	}
	if bundle.ParamsSchema != "" {
		data["params_schema"] = truncateIfNeeded(bundle.ParamsSchema, maxConfigMapKeySize)
	}
	if bundle.Prompts != "" {
		data["prompts"] = truncateIfNeeded(bundle.Prompts, maxConfigMapKeySize)
	}
	if bundle.GitProvenance != nil {
		provJSON, err := json.Marshal(bundle.GitProvenance)
		if err == nil {
			data["git_provenance"] = string(provJSON)
		}
	}

	if len(data) == 0 {
		return Manifest{}, nil // no metadata to store
	}

	// Check total size and warn if over limit
	var totalSize int
	for _, v := range data {
		totalSize += len(v)
	}
	if totalSize > maxConfigMapTotalSize {
		slog.Warn("metadata ConfigMap total size exceeds limit",
			"name", name, "totalSize", totalSize, "limit", maxConfigMapTotalSize)
	}

	// Build data section with sorted keys for deterministic output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var dataEntries []string
	for _, k := range keys {
		dataEntries = append(dataEntries, fmt.Sprintf("  %s: |\n%s", k, indentString(data[k], 4)))
	}

	labels := fmt.Sprintf(`app.kubernetes.io/name: %s
    app.kubernetes.io/managed-by: tentacular
    tentacular.io/metadata: "true"`, name)

	content := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: %s-metadata
  namespace: %s
  labels:
    %s
data:
%s
`, name, namespace, labels, strings.Join(dataEntries, "\n"))

	return Manifest{
		Kind:    "ConfigMap",
		Name:    name + "-metadata",
		Content: content,
	}, nil
}

// generateContractSummary auto-generates a markdown contract summary from the workflow spec.
// Called only when contract-summary.md does not exist on disk.
// The result is placed in the metadata ConfigMap only — never written to disk.
func generateContractSummary(wf *spec.Workflow) string {
	if wf.Contract == nil {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Dependencies\n\n")

	depNames := make([]string, 0, len(wf.Contract.Dependencies))
	for name := range wf.Contract.Dependencies {
		depNames = append(depNames, name)
	}
	sort.Strings(depNames)

	for _, name := range depNames {
		dep := wf.Contract.Dependencies[name]
		managed := strings.HasPrefix(name, "tentacular-")
		managedStr := "external"
		if managed {
			managedStr = "managed"
		}
		line := fmt.Sprintf("- **%s** (%s, %s)", name, dep.Protocol, managedStr)
		if dep.Auth != nil && dep.Auth.Secret != "" {
			line += " — requires secret: " + dep.Auth.Secret
		}
		sb.WriteString(line + "\n")
	}

	// Secrets section
	secrets := spec.DeriveSecrets(wf.Contract)
	if len(secrets) > 0 {
		sb.WriteString("\n## Secrets\n\n")
		for _, s := range secrets {
			fmt.Fprintf(&sb, "- %s\n", s)
		}
	}

	// Network Egress section (skip DNS-only entries).
	// DNS egress to kube-dns is infrastructure noise that every tentacle has;
	// it adds no useful signal to a contract summary.
	rules := spec.DeriveEgressRules(wf.Contract)
	var nonDNSRules []spec.EgressRule
	for _, r := range rules {
		if r.Host != "kube-dns.kube-system.svc.cluster.local" {
			nonDNSRules = append(nonDNSRules, r)
		}
	}
	if len(nonDNSRules) > 0 {
		sb.WriteString("\n## Network Egress\n\n")
		for _, r := range nonDNSRules {
			fmt.Fprintf(&sb, "- %s:%d (%s)\n", r.Host, r.Port, r.Protocol)
		}
	}

	return sb.String()
}

// promptTemplateCounts parses prompts.yaml just enough to count entries.
func promptTemplateCounts(raw string) (prompts, templates int) {
	var doc struct {
		Prompts   []any `yaml:"prompts"`
		Templates []any `yaml:"templates"`
	}
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return 0, 0
	}
	return len(doc.Prompts), len(doc.Templates)
}

// truncateIfNeeded truncates value to maxBytes, appending a [truncated] marker.
func truncateIfNeeded(value string, maxBytes int) string {
	if len(value) <= maxBytes {
		return value
	}
	const marker = "\n[truncated]"
	return value[:maxBytes-len(marker)] + marker
}
