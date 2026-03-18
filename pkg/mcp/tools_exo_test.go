package mcp

import (
	"context"
	"testing"
)

// TestExoStatus_AllFieldsParsed verifies all ExoStatusResult fields including
// SPIREAvailable and NATSSpiffeEnabled are correctly parsed.
func TestExoStatus_AllFieldsParsed(t *testing.T) {
	h := makeToolServer(t, "exo_status", map[string]any{
		"enabled":             true,
		"cleanup_on_undeploy": true,
		"postgres_available":  true,
		"nats_available":      true,
		"rustfs_available":    true,
		"spire_available":     true,
		"nats_spiffe_enabled": true,
		"auth_enabled":        true,
		"auth_issuer":         "https://auth.example.com/realms/tentacular",
	}, false)
	defer h.Close()

	result, err := h.client.ExoStatus(context.Background())
	if err != nil {
		t.Fatalf("ExoStatus: %v", err)
	}
	if !result.Enabled {
		t.Error("expected Enabled=true")
	}
	if !result.CleanupOnUndeploy {
		t.Error("expected CleanupOnUndeploy=true")
	}
	if !result.PostgresAvailable {
		t.Error("expected PostgresAvailable=true")
	}
	if !result.NATSAvailable {
		t.Error("expected NATSAvailable=true")
	}
	if !result.RustFSAvailable {
		t.Error("expected RustFSAvailable=true")
	}
	if !result.SPIREAvailable {
		t.Error("expected SPIREAvailable=true")
	}
	if !result.NATSSpiffeEnabled {
		t.Error("expected NATSSpiffeEnabled=true")
	}
	if !result.AuthEnabled {
		t.Error("expected AuthEnabled=true")
	}
	if result.AuthIssuer != "https://auth.example.com/realms/tentacular" {
		t.Errorf("expected AuthIssuer=https://auth.example.com/realms/tentacular, got %q", result.AuthIssuer)
	}
}

// TestExoStatus_DisabledCluster verifies parsing when exoskeleton is not enabled.
func TestExoStatus_DisabledCluster(t *testing.T) {
	h := makeToolServer(t, "exo_status", map[string]any{
		"enabled":             false,
		"cleanup_on_undeploy": false,
		"postgres_available":  false,
		"nats_available":      false,
		"rustfs_available":    false,
		"spire_available":     false,
		"nats_spiffe_enabled": false,
		"auth_enabled":        false,
	}, false)
	defer h.Close()

	result, err := h.client.ExoStatus(context.Background())
	if err != nil {
		t.Fatalf("ExoStatus: %v", err)
	}
	if result.Enabled {
		t.Error("expected Enabled=false")
	}
	if result.SPIREAvailable {
		t.Error("expected SPIREAvailable=false")
	}
	if result.NATSSpiffeEnabled {
		t.Error("expected NATSSpiffeEnabled=false")
	}
}

// TestExoStatus_ToolError verifies tool errors are propagated.
func TestExoStatus_ToolError(t *testing.T) {
	h := makeToolServer(t, "exo_status", map[string]string{
		"error": "exoskeleton not installed",
	}, true)
	defer h.Close()

	_, err := h.client.ExoStatus(context.Background())
	if !IsToolError(err) {
		t.Errorf("expected tool error, got: %v", err)
	}
}

// TestExoRegistration_FoundWithData verifies a registered workflow with data fields.
func TestExoRegistration_FoundWithData(t *testing.T) {
	h := makeToolServer(t, "exo_registration", map[string]any{
		"found":     true,
		"namespace": "prod",
		"name":      "api-gateway",
		"data": map[string]string{
			"postgres.host":     "pg.cluster.local",
			"postgres.database": "api_gateway",
			"rustfs.bucket":     "api-gateway-data",
			"nats.subject":      "api.gateway.>",
		},
	}, false)
	defer h.Close()

	result, err := h.client.ExoRegistration(context.Background(), "prod", "api-gateway")
	if err != nil {
		t.Fatalf("ExoRegistration: %v", err)
	}
	if !result.Found {
		t.Error("expected Found=true")
	}
	if result.Namespace != "prod" {
		t.Errorf("expected namespace=prod, got %q", result.Namespace)
	}
	if result.Name != "api-gateway" {
		t.Errorf("expected name=api-gateway, got %q", result.Name)
	}
	if len(result.Data) != 4 {
		t.Errorf("expected 4 data entries, got %d", len(result.Data))
	}
	if result.Data["rustfs.bucket"] != "api-gateway-data" {
		t.Errorf("expected rustfs.bucket=api-gateway-data, got %q", result.Data["rustfs.bucket"])
	}
}

// TestExoRegistration_ToolError verifies tool errors are propagated.
func TestExoRegistration_ToolError(t *testing.T) {
	h := makeToolServer(t, "exo_registration", map[string]string{
		"error": "namespace not found",
	}, true)
	defer h.Close()

	_, err := h.client.ExoRegistration(context.Background(), "missing-ns", "wf")
	if !IsToolError(err) {
		t.Errorf("expected tool error, got: %v", err)
	}
}

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
		"namespace": "staging",
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
		"namespace": "default",
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
