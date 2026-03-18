package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// --- wf_apply ---

// WfApplyParams are the arguments for the wf_apply MCP tool.
type WfApplyParams struct {
	Namespace string           `json:"namespace"`
	Name      string           `json:"name"`
	Manifests []map[string]any `json:"manifests"`
}

// WfApplyResult is the response from wf_apply.
type WfApplyResult struct {
	Status  string   `json:"status"`  // "created" | "updated" | "unchanged"
	Applied []string `json:"applied"` // resource names applied
	Updated int      `json:"updated"` // count of resources updated (vs created)
}

// WfApply calls the wf_apply MCP tool to apply workflow manifests.
func (c *Client) WfApply(ctx context.Context, namespace, name string, manifests []map[string]any) (*WfApplyResult, error) {
	raw, err := c.CallTool(ctx, "wf_apply", WfApplyParams{
		Namespace: namespace,
		Name:      name,
		Manifests: manifests,
	})
	if err != nil {
		return nil, err
	}
	var result WfApplyResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing wf_apply result: %w", err)
	}
	return &result, nil
}

// --- wf_remove ---

// WfRemoveParams are the arguments for the wf_remove MCP tool.
type WfRemoveParams struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// WfRemoveResult is the response from wf_remove.
type WfRemoveResult struct {
	ExoCleanupDetails string   `json:"exo_cleanup_details,omitempty"`
	Deleted           []string `json:"deleted"`
	DeletedCount      int      `json:"deletedCount"`
	ExoCleanedUp      bool     `json:"exo_cleaned_up,omitempty"`
}

func (r *WfRemoveResult) UnmarshalJSON(data []byte) error {
	// Try standard format first (deleted as []string).
	type alias WfRemoveResult
	var a alias
	if err := json.Unmarshal(data, &a); err == nil && len(a.Deleted) > 0 {
		*r = WfRemoveResult(a)
		return nil
	}
	// Server may return deleted as a number (count) plus optional exo fields.
	var alt struct {
		ExoCleanupDetails string `json:"exo_cleanup_details"`
		Deleted           int    `json:"deleted"`
		ExoCleanedUp      bool   `json:"exo_cleaned_up"`
	}
	if err := json.Unmarshal(data, &alt); err == nil {
		r.DeletedCount = alt.Deleted
		r.ExoCleanedUp = alt.ExoCleanedUp
		r.ExoCleanupDetails = alt.ExoCleanupDetails
		return nil
	}
	return fmt.Errorf("cannot parse wf_remove result: %s", string(data))
}

// WfRemove calls the wf_remove MCP tool to delete a workflow's resources.
func (c *Client) WfRemove(ctx context.Context, namespace, name string) (*WfRemoveResult, error) {
	raw, err := c.CallTool(ctx, "wf_remove", WfRemoveParams{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, err
	}
	var result WfRemoveResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing wf_remove result: %w", err)
	}
	return &result, nil
}

// --- wf_status ---

// WfStatusParams are the arguments for the wf_status MCP tool.
type WfStatusParams struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Detail    bool   `json:"detail,omitempty"`
}

// WfStatusResult is the response from wf_status.
type WfStatusResult struct {
	Name      string      `json:"name"`
	Namespace string      `json:"namespace"`
	Version   string      `json:"version,omitempty"`
	Pods      []PodInfo   `json:"pods,omitempty"`
	Events    []EventInfo `json:"events,omitempty"`
	Replicas  int32       `json:"replicas"`
	Available int32       `json:"available"`
	Ready     bool        `json:"ready"`
}

// PodInfo represents a pod in the workflow deployment.
type PodInfo struct {
	Name     string `json:"name"`
	Phase    string `json:"phase"`
	NodeName string `json:"nodeName,omitempty"`
	Ready    bool   `json:"ready"`
}

// EventInfo represents a Kubernetes event.
type EventInfo struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
	Count   int32  `json:"count"`
}

// WfStatus calls the wf_status MCP tool.
func (c *Client) WfStatus(ctx context.Context, namespace, name string, detail bool) (*WfStatusResult, error) {
	raw, err := c.CallTool(ctx, "wf_status", WfStatusParams{
		Namespace: namespace,
		Name:      name,
		Detail:    detail,
	})
	if err != nil {
		return nil, err
	}
	var result WfStatusResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing wf_status result: %w", err)
	}
	return &result, nil
}

// --- wf_list ---

// WfListParams are the arguments for the wf_list MCP tool.
type WfListParams struct {
	Namespace string `json:"namespace"`
}

// WfListItem represents a single workflow in the list response.
type WfListItem struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	Version     string `json:"version,omitempty"`
	Owner       string `json:"owner,omitempty"`
	Team        string `json:"team,omitempty"`
	Environment string `json:"environment,omitempty"`
	DeployedBy  string `json:"deployed_by,omitempty"`
	DeployedVia string `json:"deployed_via,omitempty"`
	Age         string `json:"age,omitempty"`
	CreatedAt   string `json:"createdAt,omitempty"`
	Replicas    int32  `json:"replicas,omitempty"`
	Available   int32  `json:"available,omitempty"`
	Ready       bool   `json:"ready"`
}

// wfListResult is the envelope returned by the MCP server for wf_list.
type wfListResult struct {
	Workflows []WfListItem `json:"workflows"`
}

// WfList calls the wf_list MCP tool to enumerate deployed workflows.
func (c *Client) WfList(ctx context.Context, namespace string) ([]WfListItem, error) {
	raw, err := c.CallTool(ctx, "wf_list", WfListParams{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	// Try envelope format first: {"workflows": [...]}
	var envelope wfListResult
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Workflows != nil {
		return envelope.Workflows, nil
	}
	// Fall back to bare array for older servers.
	var items []WfListItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parsing wf_list result: %w", err)
	}
	return items, nil
}

// --- wf_pods ---

// WfPodsParams are the arguments for the wf_pods MCP tool.
type WfPodsParams struct {
	Namespace string `json:"namespace"`
}

// WfPod represents a single pod in the wf_pods response.
type WfPod struct {
	Name     string   `json:"name"`
	Phase    string   `json:"phase"`
	Age      string   `json:"age"`
	Images   []string `json:"images"`
	Restarts int      `json:"restarts"`
	Ready    bool     `json:"ready"`
}

// WfPodsResult is the response from wf_pods.
type WfPodsResult struct {
	Pods []WfPod `json:"pods"`
}

// WfPods calls the wf_pods MCP tool to list pods in a namespace.
func (c *Client) WfPods(ctx context.Context, namespace string) (*WfPodsResult, error) {
	raw, err := c.CallTool(ctx, "wf_pods", WfPodsParams{
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}
	var result WfPodsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing wf_pods result: %w", err)
	}
	return &result, nil
}

// --- wf_logs ---

// WfLogsParams are the arguments for the wf_logs MCP tool.
type WfLogsParams struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container,omitempty"`
	TailLines int64  `json:"tail_lines,omitempty"`
}

// WfLogsResult is the response from wf_logs.
type WfLogsResult struct {
	Logs      string   `json:"logs"`
	Pod       string   `json:"pod"`
	Container string   `json:"container"`
	Lines     []string `json:"lines"`
}

// LogText returns the log content as a single string.
func (r *WfLogsResult) LogText() string {
	if len(r.Lines) > 0 {
		return strings.Join(r.Lines, "\n")
	}
	return r.Logs
}

// WfLogs calls the wf_logs MCP tool to retrieve pod logs.
func (c *Client) WfLogs(ctx context.Context, namespace, pod string, tailLines int64) (*WfLogsResult, error) {
	raw, err := c.CallTool(ctx, "wf_logs", WfLogsParams{
		Namespace: namespace,
		Pod:       pod,
		TailLines: tailLines,
	})
	if err != nil {
		return nil, err
	}
	var result WfLogsResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing wf_logs result: %w", err)
	}
	return &result, nil
}

// --- wf_run ---

// WfRunParams are the arguments for the wf_run MCP tool.
type WfRunParams struct {
	Namespace      string          `json:"namespace"`
	Name           string          `json:"name"`
	Input          json.RawMessage `json:"input,omitempty"`
	TimeoutSeconds int             `json:"timeout_seconds,omitempty"`
}

// WfRunResult is the response from wf_run.
type WfRunResult struct {
	Name       string          `json:"name"`
	Namespace  string          `json:"namespace"`
	PodName    string          `json:"pod_name,omitempty"`
	Output     json.RawMessage `json:"output"`
	DurationMs int64           `json:"duration_ms"`
}

// WfRun calls the wf_run MCP tool to trigger a workflow execution.
func (c *Client) WfRun(ctx context.Context, namespace, name string, input json.RawMessage, timeoutSeconds int) (*WfRunResult, error) {
	raw, err := c.CallTool(ctx, "wf_run", WfRunParams{
		Namespace:      namespace,
		Name:           name,
		Input:          input,
		TimeoutSeconds: timeoutSeconds,
	})
	if err != nil {
		return nil, err
	}
	var result WfRunResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing wf_run result: %w", err)
	}
	return &result, nil
}

// --- cluster_preflight ---

// ClusterPreflightParams are the arguments for the cluster_preflight MCP tool.
type ClusterPreflightParams struct {
	Namespace string `json:"namespace"`
}

// CheckResult mirrors k8s.CheckResult for deserialization from MCP.
type CheckResult struct {
	Name        string `json:"name"`
	Warning     string `json:"warning,omitempty"`
	Remediation string `json:"remediation,omitempty"`
	Passed      bool   `json:"passed"`
}

// ClusterPreflightResult is the response from cluster_preflight.
type ClusterPreflightResult struct {
	Results []CheckResult `json:"results"`
	AllPass bool          `json:"allPass"`
}

// UnmarshalJSON handles both "results" and "checks" field names from the MCP server.
func (r *ClusterPreflightResult) UnmarshalJSON(data []byte) error {
	// Try canonical form first
	type alias ClusterPreflightResult
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*r = ClusterPreflightResult(a)

	// If Results is empty, try "checks" field
	if len(r.Results) == 0 {
		var alt struct {
			Checks  []CheckResult `json:"checks"`
			AllPass bool          `json:"allPass"`
		}
		if err := json.Unmarshal(data, &alt); err == nil && len(alt.Checks) > 0 {
			r.Results = alt.Checks
			r.AllPass = alt.AllPass
		}
	}
	return nil
}

// ClusterPreflight calls the cluster_preflight MCP tool.
func (c *Client) ClusterPreflight(ctx context.Context, namespace string) (*ClusterPreflightResult, error) {
	raw, err := c.CallTool(ctx, "cluster_preflight", ClusterPreflightParams{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	var result ClusterPreflightResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing cluster_preflight result: %w", err)
	}
	return &result, nil
}

// --- ns_create ---

// NsCreateParams are the arguments for the ns_create MCP tool.
type NsCreateParams struct {
	Name        string `json:"name"`
	QuotaPreset string `json:"quota_preset,omitempty"`
}

// NsCreateResult is the response from ns_create.
type NsCreateResult struct {
	Name    string `json:"name"`
	Created bool   `json:"created"` // false means already existed
}

// NsCreate calls the ns_create MCP tool to ensure a namespace exists.
func (c *Client) NsCreate(ctx context.Context, name, quotaPreset string) (*NsCreateResult, error) {
	raw, err := c.CallTool(ctx, "ns_create", NsCreateParams{
		Name:        name,
		QuotaPreset: quotaPreset,
	})
	if err != nil {
		return nil, err
	}
	var result NsCreateResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing ns_create result: %w", err)
	}
	return &result, nil
}

// --- audit_resources ---

// AuditResourcesParams are the arguments for the audit_resources MCP tool.
type AuditResourcesParams struct {
	Expected     map[string]any `json:"expected"`
	Namespace    string         `json:"namespace"`
	WorkflowName string         `json:"workflowName"`
}

// AuditResourcesResult is the response from audit_resources.
type AuditResourcesResult struct {
	Overall       string        `json:"overall"`
	NetworkPolicy ResourceAudit `json:"networkPolicy"`
	Secrets       ResourceAudit `json:"secrets"`
	CronJobs      ResourceAudit `json:"cronJobs"`
}

// ResourceAudit holds audit results for a single resource type.
type ResourceAudit struct {
	Status  string   `json:"status"`
	Details []string `json:"details,omitempty"`
	Missing []string `json:"missing,omitempty"`
	Extra   []string `json:"extra,omitempty"`
}

// AuditResources calls the audit_resources MCP tool.
func (c *Client) AuditResources(ctx context.Context, namespace, workflowName string, expected map[string]any) (*AuditResourcesResult, error) {
	raw, err := c.CallTool(ctx, "audit_resources", AuditResourcesParams{
		Namespace:    namespace,
		WorkflowName: workflowName,
		Expected:     expected,
	})
	if err != nil {
		return nil, err
	}
	var result AuditResourcesResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing audit_resources result: %w", err)
	}
	return &result, nil
}

// --- cluster_profile ---

// ClusterProfileParams are the arguments for the cluster_profile MCP tool.
type ClusterProfileParams struct {
	Namespace string `json:"namespace,omitempty"`
}

// ClusterProfileResult is the raw JSON response from cluster_profile.
// The exact schema depends on the MCP server version; raw is preserved for
// flexible rendering in the CLI.
type ClusterProfileResult struct {
	Raw json.RawMessage
}

// ClusterProfile calls the cluster_profile MCP tool and returns the raw JSON result.
func (c *Client) ClusterProfile(ctx context.Context, namespace string) (*ClusterProfileResult, error) {
	params := ClusterProfileParams{}
	if namespace != "" {
		params.Namespace = namespace
	}
	raw, err := c.CallTool(ctx, "cluster_profile", params)
	if err != nil {
		return nil, err
	}
	return &ClusterProfileResult{Raw: raw}, nil
}

// --- wf_describe ---

// WfDescribeParams are the arguments for the wf_describe MCP tool.
type WfDescribeParams struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// WfDescribeResult is the response from wf_describe.
type WfDescribeResult struct {
	Annotations map[string]string `json:"annotations,omitempty"`
	Name        string            `json:"name"`
	Namespace   string            `json:"namespace"`
	Image       string            `json:"image,omitempty"`
	Nodes       []PodInfo         `json:"nodes,omitempty"`
	Triggers    []string          `json:"triggers,omitempty"`
	Manifests   []map[string]any  `json:"manifests,omitempty"`
	Replicas    int32             `json:"replicas"`
	Available   int32             `json:"available"`
	Ready       bool              `json:"ready"`
}

// WfDescribe calls the wf_describe MCP tool to get detailed deployment info.
func (c *Client) WfDescribe(ctx context.Context, namespace, name string) (*WfDescribeResult, error) {
	raw, err := c.CallTool(ctx, "wf_describe", WfDescribeParams{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, err
	}
	var result WfDescribeResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing wf_describe result: %w", err)
	}
	return &result, nil
}

// --- exo_status ---

// ExoStatusParams are the arguments for the exo_status MCP tool.
type ExoStatusParams struct{}

// ExoStatusResult is the response from exo_status.
type ExoStatusResult struct {
	AuthIssuer        string `json:"auth_issuer,omitempty"`
	Enabled           bool   `json:"enabled"`
	CleanupOnUndeploy bool   `json:"cleanup_on_undeploy"`
	PostgresAvailable bool   `json:"postgres_available"`
	NATSAvailable     bool   `json:"nats_available"`
	RustFSAvailable   bool   `json:"rustfs_available"`
	SPIREAvailable    bool   `json:"spire_available"`
	NATSSpiffeEnabled bool   `json:"nats_spiffe_enabled"`
	AuthEnabled       bool   `json:"auth_enabled"`
}

// ExoStatus calls the exo_status MCP tool to check exoskeleton feature status.
func (c *Client) ExoStatus(ctx context.Context) (*ExoStatusResult, error) {
	raw, err := c.CallTool(ctx, "exo_status", ExoStatusParams{})
	if err != nil {
		return nil, err
	}
	var result ExoStatusResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing exo_status result: %w", err)
	}
	return &result, nil
}

// --- exo_registration ---

// ExoRegistrationParams are the arguments for the exo_registration MCP tool.
type ExoRegistrationParams struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// ExoRegistrationResult is the response from exo_registration.
type ExoRegistrationResult struct {
	Data      map[string]string `json:"data,omitempty"`
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Found     bool              `json:"found"`
}

// ExoRegistration calls the exo_registration MCP tool to check if a workflow
// has an exoskeleton Secret registered.
func (c *Client) ExoRegistration(ctx context.Context, namespace, name string) (*ExoRegistrationResult, error) {
	raw, err := c.CallTool(ctx, "exo_registration", ExoRegistrationParams{
		Namespace: namespace,
		Name:      name,
	})
	if err != nil {
		return nil, err
	}
	var result ExoRegistrationResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing exo_registration result: %w", err)
	}
	return &result, nil
}
