package mcp

import (
	"context"
	"encoding/json"
	"testing"
)

// makeToolServer creates a test server with a single named tool that returns
// the given result as JSON text content.
func makeToolServer(t *testing.T, toolName string, result interface{}, isError bool) *testServerHandle {
	t.Helper()
	resultJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}

	srv, client := makeTestServer(t, map[string]func(map[string]any) (string, bool){
		toolName: func(args map[string]any) (string, bool) {
			return string(resultJSON), isError
		},
	})
	return &testServerHandle{srv: srv, client: client}
}

// testServerHandle bundles server and client for cleanup.
type testServerHandle struct {
	srv    interface{ Close() }
	client *Client
}

func (h *testServerHandle) Close() {
	h.client.Close()
	h.srv.Close()
}

// makeInvalidTextServer returns a test server with tools that return text
// that is NOT a valid JSON object/struct, causing unmarshal errors.
func makeInvalidTextServer(t *testing.T) (*testServerHandle, *Client) {
	t.Helper()
	tools := map[string]func(map[string]any) (string, bool){}
	for _, name := range []string{
		"wf_apply", "wf_remove", "wf_status", "wf_list", "wf_logs",
		"wf_run", "cluster_preflight", "ns_create", "audit_resources",
	} {
		tools[name] = func(args map[string]any) (string, bool) {
			return "not-a-json-object", false
		}
	}
	srv, client := makeTestServer(t, tools)
	return &testServerHandle{srv: srv, client: client}, client
}

func TestWfApply(t *testing.T) {
	h := makeToolServer(t, "wf_apply", WfApplyResult{
		Status:  "created",
		Applied: []string{"ConfigMap/my-wf", "Deployment/my-wf"},
		Updated: 0,
	}, false)
	defer h.Close()

	manifests := []map[string]interface{}{
		{"apiVersion": "v1", "kind": "ConfigMap"},
	}
	result, err := h.client.WfApply(context.Background(), "default", "my-wf", manifests)
	if err != nil {
		t.Fatalf("WfApply: %v", err)
	}
	if result.Status != "created" {
		t.Errorf("expected status=created, got %q", result.Status)
	}
	if len(result.Applied) != 2 {
		t.Errorf("expected 2 applied resources, got %d", len(result.Applied))
	}
}

func TestWfRemove(t *testing.T) {
	h := makeToolServer(t, "wf_remove", WfRemoveResult{
		Deleted: []string{"Deployment/my-wf", "Service/my-wf"},
	}, false)
	defer h.Close()

	result, err := h.client.WfRemove(context.Background(), "default", "my-wf")
	if err != nil {
		t.Fatalf("WfRemove: %v", err)
	}
	if len(result.Deleted) != 2 {
		t.Errorf("expected 2 deleted resources, got %d", len(result.Deleted))
	}
}

func TestWfStatus(t *testing.T) {
	h := makeToolServer(t, "wf_status", WfStatusResult{
		Name:      "my-wf",
		Namespace: "default",
		Ready:     true,
		Replicas:  1,
		Available: 1,
	}, false)
	defer h.Close()

	result, err := h.client.WfStatus(context.Background(), "default", "my-wf", false)
	if err != nil {
		t.Fatalf("WfStatus: %v", err)
	}
	if !result.Ready {
		t.Error("expected ready=true")
	}
}

func TestWfList(t *testing.T) {
	h := makeToolServer(t, "wf_list", []WfListItem{
		{Name: "wf-a", Namespace: "default", Ready: true, Replicas: 1, Available: 1},
		{Name: "wf-b", Namespace: "default", Ready: false, Replicas: 1, Available: 0},
	}, false)
	defer h.Close()

	items, err := h.client.WfList(context.Background(), "default")
	if err != nil {
		t.Fatalf("WfList: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "wf-a" {
		t.Errorf("expected first item wf-a, got %q", items[0].Name)
	}
}

func TestWfLogs(t *testing.T) {
	h := makeToolServer(t, "wf_logs", WfLogsResult{Logs: "line1\nline2\n"}, false)
	defer h.Close()

	result, err := h.client.WfLogs(context.Background(), "default", "my-wf", 100)
	if err != nil {
		t.Fatalf("WfLogs: %v", err)
	}
	if result.Logs != "line1\nline2\n" {
		t.Errorf("unexpected logs: %q", result.Logs)
	}
}

func TestWfRun(t *testing.T) {
	h := makeToolServer(t, "wf_run", WfRunResult{
		Name:       "my-wf",
		Namespace:  "default",
		Output:     []byte(`{"success":true}`),
		DurationMs: 1234,
	}, false)
	defer h.Close()

	result, err := h.client.WfRun(context.Background(), "default", "my-wf", nil, 120)
	if err != nil {
		t.Fatalf("WfRun: %v", err)
	}
	if result.DurationMs != 1234 {
		t.Errorf("expected duration=1234, got %d", result.DurationMs)
	}
}

func TestClusterPreflight(t *testing.T) {
	h := makeToolServer(t, "cluster_preflight", ClusterPreflightResult{
		Results: []CheckResult{
			{Name: "Namespace", Passed: true},
			{Name: "RBAC", Passed: true},
		},
		AllPass: true,
	}, false)
	defer h.Close()

	result, err := h.client.ClusterPreflight(context.Background(), "default")
	if err != nil {
		t.Fatalf("ClusterPreflight: %v", err)
	}
	if !result.AllPass {
		t.Error("expected all pass")
	}
	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}
}

func TestNsCreate(t *testing.T) {
	h := makeToolServer(t, "ns_create", NsCreateResult{Name: "my-ns", Created: true}, false)
	defer h.Close()

	result, err := h.client.NsCreate(context.Background(), "my-ns", "")
	if err != nil {
		t.Fatalf("NsCreate: %v", err)
	}
	if !result.Created {
		t.Error("expected created=true")
	}
}

func TestNsCreate_AlreadyExists(t *testing.T) {
	h := makeToolServer(t, "ns_create", NsCreateResult{Name: "existing-ns", Created: false}, false)
	defer h.Close()

	result, err := h.client.NsCreate(context.Background(), "existing-ns", "small")
	if err != nil {
		t.Fatalf("NsCreate: %v", err)
	}
	if result.Created {
		t.Error("expected created=false for existing namespace")
	}
}

func TestAuditResources(t *testing.T) {
	h := makeToolServer(t, "audit_resources", AuditResourcesResult{
		NetworkPolicy: ResourceAudit{Status: "match"},
		Secrets:       ResourceAudit{Status: "match"},
		CronJobs:      ResourceAudit{Status: "match"},
		Overall:       "pass",
	}, false)
	defer h.Close()

	expected := map[string]interface{}{"networkPolicy": true, "cronJobs": 1}
	result, err := h.client.AuditResources(context.Background(), "default", "my-wf", expected)
	if err != nil {
		t.Fatalf("AuditResources: %v", err)
	}
	if result.Overall != "pass" {
		t.Errorf("expected overall=pass, got %q", result.Overall)
	}
}

func TestAuditResources_Mismatch(t *testing.T) {
	h := makeToolServer(t, "audit_resources", AuditResourcesResult{
		Secrets: ResourceAudit{Status: "mismatch", Missing: []string{"postgres.password"}},
		Overall: "fail",
	}, false)
	defer h.Close()

	result, err := h.client.AuditResources(context.Background(), "staging", "my-wf", nil)
	if err != nil {
		t.Fatalf("AuditResources: %v", err)
	}
	if result.Overall != "fail" {
		t.Errorf("expected overall=fail, got %q", result.Overall)
	}
	if len(result.Secrets.Missing) != 1 {
		t.Errorf("expected 1 missing secret, got %d", len(result.Secrets.Missing))
	}
}

func TestAuditResources_ToolError(t *testing.T) {
	h := makeToolServer(t, "audit_resources", map[string]string{"error": "namespace not managed"}, true)
	defer h.Close()

	_, err := h.client.AuditResources(context.Background(), "default", "my-wf", nil)
	if !IsToolError(err) {
		t.Errorf("expected tool error, got: %v", err)
	}
}

func TestWfApply_Updated(t *testing.T) {
	h := makeToolServer(t, "wf_apply", WfApplyResult{
		Status:  "updated",
		Applied: []string{"Deployment/my-wf"},
		Updated: 1,
	}, false)
	defer h.Close()

	result, err := h.client.WfApply(context.Background(), "staging", "my-wf", nil)
	if err != nil {
		t.Fatalf("WfApply: %v", err)
	}
	if result.Updated == 0 {
		t.Error("expected updated > 0")
	}
}

func TestWfStatus_WithPods(t *testing.T) {
	h := makeToolServer(t, "wf_status", WfStatusResult{
		Name: "my-wf", Namespace: "default", Ready: true, Replicas: 2, Available: 2,
		Pods: []PodInfo{
			{Name: "pod-1", Phase: "Running", Ready: true},
			{Name: "pod-2", Phase: "Running", Ready: true},
		},
	}, false)
	defer h.Close()

	result, err := h.client.WfStatus(context.Background(), "default", "my-wf", true)
	if err != nil {
		t.Fatalf("WfStatus: %v", err)
	}
	if len(result.Pods) != 2 {
		t.Errorf("expected 2 pods, got %d", len(result.Pods))
	}
}

func TestWfRun_WithInput(t *testing.T) {
	h := makeToolServer(t, "wf_run", WfRunResult{
		Name: "my-wf", Namespace: "default", Output: []byte(`{"result":"ok"}`), DurationMs: 500,
	}, false)
	defer h.Close()

	input := []byte(`{"key":"value"}`)
	result, err := h.client.WfRun(context.Background(), "default", "my-wf", input, 60)
	if err != nil {
		t.Fatalf("WfRun: %v", err)
	}
	if result.DurationMs != 500 {
		t.Errorf("expected duration=500, got %d", result.DurationMs)
	}
}

func TestClusterPreflight_Failure(t *testing.T) {
	h := makeToolServer(t, "cluster_preflight", ClusterPreflightResult{
		Results: []CheckResult{{Name: "Namespace", Passed: false, Remediation: "create namespace first"}},
		AllPass: false,
	}, false)
	defer h.Close()

	result, err := h.client.ClusterPreflight(context.Background(), "new-ns")
	if err != nil {
		t.Fatalf("ClusterPreflight: %v", err)
	}
	if result.AllPass {
		t.Error("expected allPass=false")
	}
}

// --- Unmarshal error path tests ---

func TestWfApply_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.WfApply(context.Background(), "ns", "wf", nil)
	if err == nil {
		t.Error("expected unmarshal error from WfApply")
	}
}

func TestWfRemove_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.WfRemove(context.Background(), "ns", "wf")
	if err == nil {
		t.Error("expected unmarshal error from WfRemove")
	}
}

func TestWfStatus_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.WfStatus(context.Background(), "ns", "wf", false)
	if err == nil {
		t.Error("expected unmarshal error from WfStatus")
	}
}

func TestWfList_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.WfList(context.Background(), "ns")
	if err == nil {
		t.Error("expected unmarshal error from WfList")
	}
}

func TestWfLogs_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.WfLogs(context.Background(), "ns", "wf", 100)
	if err == nil {
		t.Error("expected unmarshal error from WfLogs")
	}
}

func TestWfRun_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.WfRun(context.Background(), "ns", "wf", nil, 0)
	if err == nil {
		t.Error("expected unmarshal error from WfRun")
	}
}

func TestClusterPreflight_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.ClusterPreflight(context.Background(), "ns")
	if err == nil {
		t.Error("expected unmarshal error from ClusterPreflight")
	}
}

func TestNsCreate_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.NsCreate(context.Background(), "ns", "")
	if err == nil {
		t.Error("expected unmarshal error from NsCreate")
	}
}

func TestAuditResources_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()
	_, err := client.AuditResources(context.Background(), "ns", "wf", nil)
	if err == nil {
		t.Error("expected unmarshal error from AuditResources")
	}
}
