package mcp

import (
	"context"
	"testing"
)

// TestWfRemove_WithExoCleanupDetails verifies WfRemoveResult parses
// exo cleanup fields from the numeric deleted format.
func TestWfRemove_WithExoCleanupDetails(t *testing.T) {
	h := makeToolServer(t, "wf_remove", map[string]any{
		"deleted":             5,
		"exo_cleaned_up":      true,
		"exo_cleanup_details": "postgres: schema api_gw dropped, role api_gw_user dropped; rustfs: bucket api-gw removed, IAM user removed; nats: auth entries cleared",
	}, false)
	defer h.Close()

	result, err := h.client.WfRemove(context.Background(), "prod", "api-gw")
	if err != nil {
		t.Fatalf("WfRemove: %v", err)
	}
	if result.DeletedCount != 5 {
		t.Errorf("expected deletedCount=5, got %d", result.DeletedCount)
	}
	if !result.ExoCleanedUp {
		t.Error("expected ExoCleanedUp=true")
	}
	if result.ExoCleanupDetails == "" {
		t.Error("expected non-empty ExoCleanupDetails")
	}
}

// TestWfRemove_WithoutExoCleanup verifies WfRemoveResult when exoskeleton
// cleanup did not run.
func TestWfRemove_WithoutExoCleanup(t *testing.T) {
	h := makeToolServer(t, "wf_remove", map[string]any{
		"deleted": 2,
	}, false)
	defer h.Close()

	result, err := h.client.WfRemove(context.Background(), "default", "simple-wf")
	if err != nil {
		t.Fatalf("WfRemove: %v", err)
	}
	if result.DeletedCount != 2 {
		t.Errorf("expected deletedCount=2, got %d", result.DeletedCount)
	}
	if result.ExoCleanedUp {
		t.Error("expected ExoCleanedUp=false when not present")
	}
	if result.ExoCleanupDetails != "" {
		t.Errorf("expected empty ExoCleanupDetails, got %q", result.ExoCleanupDetails)
	}
}

// TestWfDescribe_FullResponse verifies WfDescribe parses all fields.
func TestWfDescribe_FullResponse(t *testing.T) {
	h := makeToolServer(t, "wf_describe", map[string]any{
		"name":      "my-wf",
		"enclave":   "staging",
		"ready":     true,
		"replicas":  2,
		"available": 2,
		"image":     "ghcr.io/example/my-wf:v1.2.0",
		"annotations": map[string]string{
			"tentacular.io/deployer": "alice@example.com",
		},
		"nodes": []map[string]any{
			{"name": "pod-abc", "phase": "Running", "ready": true, "nodeName": "node-1"},
		},
		"triggers": []string{"cron:*/5 * * * *"},
	}, false)
	defer h.Close()

	result, err := h.client.WfDescribe(context.Background(), "staging", "my-wf")
	if err != nil {
		t.Fatalf("WfDescribe: %v", err)
	}
	if result.Name != "my-wf" {
		t.Errorf("expected name=my-wf, got %q", result.Name)
	}
	if result.Image != "ghcr.io/example/my-wf:v1.2.0" {
		t.Errorf("expected image, got %q", result.Image)
	}
	if result.Annotations["tentacular.io/deployer"] != "alice@example.com" {
		t.Errorf("expected deployer annotation, got %v", result.Annotations)
	}
	if len(result.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(result.Nodes))
	}
	if len(result.Triggers) != 1 {
		t.Errorf("expected 1 trigger, got %d", len(result.Triggers))
	}
}

// TestWfDescribe_MinimalResponse verifies WfDescribe handles minimal response.
func TestWfDescribe_MinimalResponse(t *testing.T) {
	h := makeToolServer(t, "wf_describe", map[string]any{
		"name":      "bare-wf",
		"enclave":   "default",
		"ready":     false,
		"replicas":  1,
		"available": 0,
	}, false)
	defer h.Close()

	result, err := h.client.WfDescribe(context.Background(), "default", "bare-wf")
	if err != nil {
		t.Fatalf("WfDescribe: %v", err)
	}
	if result.Ready {
		t.Error("expected ready=false")
	}
	if result.Image != "" {
		t.Errorf("expected empty image, got %q", result.Image)
	}
}

// TestWfDescribe_ToolError verifies tool errors are propagated.
func TestWfDescribe_ToolError(t *testing.T) {
	h := makeToolServer(t, "wf_describe", map[string]string{
		"error": "workflow not found",
	}, true)
	defer h.Close()

	_, err := h.client.WfDescribe(context.Background(), "default", "missing")
	if !IsToolError(err) {
		t.Errorf("expected tool error, got: %v", err)
	}
}

// TestWfDescribe_UnmarshalError verifies unmarshal errors are returned.
func TestWfDescribe_UnmarshalError(t *testing.T) {
	h, client := makeInvalidTextServer(t)
	defer h.Close()

	// wf_describe isn't in the invalid server tool list, so add it specially
	srv, client2 := makeTestServer(t, map[string]func(map[string]any) (string, bool){
		"wf_describe": func(args map[string]any) (string, bool) {
			return "not-valid-json", false
		},
	})
	defer srv.Close()
	defer func() { _ = client2.Close() }()
	_ = client // silence unused

	_, err := client2.WfDescribe(context.Background(), "ns", "wf")
	if err == nil {
		t.Error("expected unmarshal error from WfDescribe")
	}
}
