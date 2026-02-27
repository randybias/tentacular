package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// --- wf_apply ---

// WfApplyParams are the arguments for the wf_apply MCP tool.
type WfApplyParams struct {
	Namespace string                   `json:"namespace"`
	Name      string                   `json:"name"`
	Manifests []map[string]interface{} `json:"manifests"`
}

// WfApplyResult is the response from wf_apply.
type WfApplyResult struct {
	Status  string   `json:"status"`  // "created" | "updated" | "unchanged"
	Applied []string `json:"applied"` // resource names applied
	Updated bool     `json:"updated"` // true if any resource was updated vs created
}

// WfApply calls the wf_apply MCP tool to apply workflow manifests.
func (c *Client) WfApply(ctx context.Context, namespace, name string, manifests []map[string]interface{}) (*WfApplyResult, error) {
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
	Deleted []string `json:"deleted"` // resource names deleted
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
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   string `json:"version,omitempty"`
	Ready     bool   `json:"ready"`
	Replicas  int32  `json:"replicas"`
	Available int32  `json:"available"`
	// Detail fields (when detail=true)
	Pods   []PodInfo   `json:"pods,omitempty"`
	Events []EventInfo `json:"events,omitempty"`
}

// PodInfo represents a pod in the workflow deployment.
type PodInfo struct {
	Name   string `json:"name"`
	Phase  string `json:"phase"`
	Ready  bool   `json:"ready"`
	NodeName string `json:"nodeName,omitempty"`
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
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Version   string `json:"version,omitempty"`
	Ready     bool   `json:"ready"`
	Replicas  int32  `json:"replicas"`
	Available int32  `json:"available"`
	CreatedAt string `json:"createdAt,omitempty"`
}

// WfList calls the wf_list MCP tool to enumerate deployed workflows.
func (c *Client) WfList(ctx context.Context, namespace string) ([]WfListItem, error) {
	raw, err := c.CallTool(ctx, "wf_list", WfListParams{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	var items []WfListItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parsing wf_list result: %w", err)
	}
	return items, nil
}

// --- wf_logs ---

// WfLogsParams are the arguments for the wf_logs MCP tool.
type WfLogsParams struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	TailLines int64  `json:"tailLines,omitempty"`
}

// WfLogsResult is the response from wf_logs.
type WfLogsResult struct {
	Logs string `json:"logs"`
}

// WfLogs calls the wf_logs MCP tool to retrieve pod logs.
func (c *Client) WfLogs(ctx context.Context, namespace, name string, tailLines int64) (*WfLogsResult, error) {
	raw, err := c.CallTool(ctx, "wf_logs", WfLogsParams{
		Namespace: namespace,
		Name:      name,
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
	Output     json.RawMessage `json:"output"`
	DurationMs int64           `json:"duration_ms"`
	PodName    string          `json:"pod_name,omitempty"`
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
	Passed      bool   `json:"passed"`
	Warning     string `json:"warning,omitempty"`
	Remediation string `json:"remediation,omitempty"`
}

// ClusterPreflightResult is the response from cluster_preflight.
type ClusterPreflightResult struct {
	Results []CheckResult `json:"results"`
	AllPass bool          `json:"allPass"`
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
	Namespace    string                 `json:"namespace"`
	WorkflowName string                 `json:"workflowName"`
	Expected     map[string]interface{} `json:"expected"`
}

// AuditResourcesResult is the response from audit_resources.
type AuditResourcesResult struct {
	NetworkPolicy ResourceAudit `json:"networkPolicy"`
	Secrets       ResourceAudit `json:"secrets"`
	CronJobs      ResourceAudit `json:"cronJobs"`
	Overall       string        `json:"overall"`
}

// ResourceAudit holds audit results for a single resource type.
type ResourceAudit struct {
	Status  string   `json:"status"`
	Details []string `json:"details,omitempty"`
	Missing []string `json:"missing,omitempty"`
	Extra   []string `json:"extra,omitempty"`
}

// AuditResources calls the audit_resources MCP tool.
func (c *Client) AuditResources(ctx context.Context, namespace, workflowName string, expected map[string]interface{}) (*AuditResourcesResult, error) {
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
