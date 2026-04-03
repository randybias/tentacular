package mcp

import (
	"encoding/json"
	"testing"
)

// 2.14: EnclaveProvisionParams default_mode serialization.
func TestEnclaveProvisionParams_DefaultModeSerialization(t *testing.T) {
	params := EnclaveProvisionParams{
		Name:        "test-enc",
		Owner:       "alice@example.com",
		DefaultMode: "rwxr-x---",
	}
	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(data)
	if !containsJSON(s, `"default_mode":"rwxr-x---"`) {
		t.Errorf("expected default_mode in JSON, got: %s", s)
	}
}

// 2.15: Response type deserialization for all 5 enclave tools.

func TestEnclaveProvisionResult_Deserialization(t *testing.T) {
	raw := `{"name":"my-enc","status":"active","quota_preset":"medium","owner":"alice@example.com","members":["bob@example.com"],"resources_created":["namespace/my-enc"]}`
	var result EnclaveProvisionResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if result.Name != "my-enc" {
		t.Errorf("Name = %q", result.Name)
	}
	if result.Status != "active" {
		t.Errorf("Status = %q", result.Status)
	}
	if result.QuotaPreset != "medium" {
		t.Errorf("QuotaPreset = %q", result.QuotaPreset)
	}
	if result.Owner != "alice@example.com" {
		t.Errorf("Owner = %q", result.Owner)
	}
	if len(result.Members) != 1 || result.Members[0] != "bob@example.com" {
		t.Errorf("Members = %v", result.Members)
	}
	if len(result.ResourcesCreated) != 1 {
		t.Errorf("ResourcesCreated = %v", result.ResourcesCreated)
	}
}

func TestEnclaveInfoResult_Deserialization(t *testing.T) {
	raw := `{"name":"my-enc","owner":"alice@example.com","owner_sub":"sub-123","platform":"slack","channel_id":"C123","channel_name":"eng","quota_preset":"large","status":"active","created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-02T00:00:00Z","members":["bob@example.com"],"exo_services":[{"name":"postgres","available":true}],"tentacle_count":3}`
	var result EnclaveInfoResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if result.Name != "my-enc" {
		t.Errorf("Name = %q", result.Name)
	}
	if result.Owner != "alice@example.com" {
		t.Errorf("Owner = %q", result.Owner)
	}
	if result.OwnerSub != "sub-123" {
		t.Errorf("OwnerSub = %q", result.OwnerSub)
	}
	if result.Platform != "slack" {
		t.Errorf("Platform = %q", result.Platform)
	}
	if result.ChannelID != "C123" {
		t.Errorf("ChannelID = %q", result.ChannelID)
	}
	if result.ChannelName != "eng" {
		t.Errorf("ChannelName = %q", result.ChannelName)
	}
	if result.QuotaPreset != "large" {
		t.Errorf("QuotaPreset = %q", result.QuotaPreset)
	}
	if result.Status != "active" {
		t.Errorf("Status = %q", result.Status)
	}
	if result.CreatedAt != "2026-01-01T00:00:00Z" {
		t.Errorf("CreatedAt = %q", result.CreatedAt)
	}
	if result.UpdatedAt != "2026-01-02T00:00:00Z" {
		t.Errorf("UpdatedAt = %q", result.UpdatedAt)
	}
	if len(result.Members) != 1 {
		t.Errorf("Members = %v", result.Members)
	}
	if len(result.ExoServices) != 1 || result.ExoServices[0].Name != "postgres" || !result.ExoServices[0].Available {
		t.Errorf("ExoServices = %v", result.ExoServices)
	}
	if result.TentacleCount != 3 {
		t.Errorf("TentacleCount = %d", result.TentacleCount)
	}
}

func TestEnclaveListItem_Deserialization(t *testing.T) {
	raw := `{"enclaves":[{"name":"enc-1","owner":"alice@example.com","status":"active","platform":"slack","channel_name":"eng","created_at":"2026-01-01T00:00:00Z","members":["bob@example.com"]}]}`
	var envelope enclaveListResult
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if len(envelope.Enclaves) != 1 {
		t.Fatalf("expected 1 enclave, got %d", len(envelope.Enclaves))
	}
	item := envelope.Enclaves[0]
	if item.Name != "enc-1" {
		t.Errorf("Name = %q", item.Name)
	}
	if item.Owner != "alice@example.com" {
		t.Errorf("Owner = %q", item.Owner)
	}
	if item.Status != "active" {
		t.Errorf("Status = %q", item.Status)
	}
	if item.Platform != "slack" {
		t.Errorf("Platform = %q", item.Platform)
	}
	if item.ChannelName != "eng" {
		t.Errorf("ChannelName = %q", item.ChannelName)
	}
	if len(item.Members) != 1 {
		t.Errorf("Members = %v", item.Members)
	}
}

func TestEnclaveSyncResult_Deserialization(t *testing.T) {
	raw := `{"name":"my-enc","updated":["members","owner"],"enclave":{"name":"my-enc","owner":"bob@example.com","status":"active","members":["alice@example.com"],"tentacle_count":0}}`
	var result EnclaveSyncResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if result.Name != "my-enc" {
		t.Errorf("Name = %q", result.Name)
	}
	if len(result.Updated) != 2 {
		t.Errorf("Updated = %v", result.Updated)
	}
	if result.Enclave.Owner != "bob@example.com" {
		t.Errorf("Enclave.Owner = %q", result.Enclave.Owner)
	}
}

func TestEnclaveDeprovisionResult_Deserialization(t *testing.T) {
	raw := `{"name":"my-enc","deleted":true,"tentacles_removed":5}`
	var result EnclaveDeprovisionResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if result.Name != "my-enc" {
		t.Errorf("Name = %q", result.Name)
	}
	if !result.Deleted {
		t.Error("expected Deleted=true")
	}
	if result.TentaclesRemoved != 5 {
		t.Errorf("TentaclesRemoved = %d", result.TentaclesRemoved)
	}
}

// containsJSON checks if a JSON string contains the expected key:value substring.
func containsJSON(json, substr string) bool {
	return len(json) > 0 && len(substr) > 0 && jsonContains(json, substr)
}

func jsonContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
