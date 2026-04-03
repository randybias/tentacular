package mcp

import (
	"context"
	"encoding/json"
	"fmt"
)

// --- enclave_provision ---

// EnclaveProvisionParams are the arguments for the enclave_provision MCP tool.
type EnclaveProvisionParams struct {
	Name        string   `json:"name"`
	Owner       string   `json:"owner_email,omitempty"`
	OwnerSub    string   `json:"owner_sub,omitempty"`
	Platform    string   `json:"platform,omitempty"`
	ChannelID   string   `json:"channel_id,omitempty"`
	ChannelName string   `json:"channel_name,omitempty"`
	Quota       string   `json:"quota_preset,omitempty"`
	DefaultMode string   `json:"default_mode,omitempty"`
	Members     []string `json:"members,omitempty"`
}

// EnclaveProvisionResult is the response from enclave_provision.
type EnclaveProvisionResult struct {
	Name             string   `json:"name"`
	Status           string   `json:"status"`
	QuotaPreset      string   `json:"quota_preset,omitempty"`
	Owner            string   `json:"owner"`
	Members          []string `json:"members,omitempty"`
	ResourcesCreated []string `json:"resources_created,omitempty"`
}

// EnclaveProvision calls the enclave_provision MCP tool to provision a new enclave.
func (c *Client) EnclaveProvision(ctx context.Context, params EnclaveProvisionParams) (*EnclaveProvisionResult, error) {
	raw, err := c.CallTool(ctx, "enclave_provision", params)
	if err != nil {
		return nil, err
	}
	var result EnclaveProvisionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing enclave_provision result: %w", err)
	}
	return &result, nil
}

// --- enclave_info ---

// EnclaveInfoParams are the arguments for the enclave_info MCP tool.
type EnclaveInfoParams struct {
	Name string `json:"name"`
}

// EnclaveExoService describes the availability of a single exoskeleton service for an enclave.
type EnclaveExoService struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
}

// EnclaveInfoResult is the response from enclave_info.
type EnclaveInfoResult struct {
	Name          string              `json:"name"`
	Owner         string              `json:"owner"`
	OwnerSub      string              `json:"owner_sub,omitempty"`
	Platform      string              `json:"platform,omitempty"`
	ChannelID     string              `json:"channel_id,omitempty"`
	ChannelName   string              `json:"channel_name,omitempty"`
	QuotaPreset   string              `json:"quota_preset,omitempty"`
	Status        string              `json:"status,omitempty"`
	CreatedAt     string              `json:"created_at,omitempty"`
	UpdatedAt     string              `json:"updated_at,omitempty"`
	Members       []string            `json:"members,omitempty"`
	ExoServices   []EnclaveExoService `json:"exo_services,omitempty"`
	TentacleCount int                 `json:"tentacle_count"`
}

// EnclaveInfo calls the enclave_info MCP tool to retrieve enclave details.
func (c *Client) EnclaveInfo(ctx context.Context, name string) (*EnclaveInfoResult, error) {
	raw, err := c.CallTool(ctx, "enclave_info", EnclaveInfoParams{Name: name})
	if err != nil {
		return nil, err
	}
	var result EnclaveInfoResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing enclave_info result: %w", err)
	}
	return &result, nil
}

// --- enclave_list ---

// EnclaveListParams are the arguments for the enclave_list MCP tool.
type EnclaveListParams struct {
	CallerEmail string `json:"caller_email,omitempty"`
}

// EnclaveListItem represents a single enclave in the list response.
type EnclaveListItem struct {
	Name        string   `json:"name"`
	Owner       string   `json:"owner"`
	Status      string   `json:"status,omitempty"`
	Platform    string   `json:"platform,omitempty"`
	ChannelName string   `json:"channel_name,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	Members     []string `json:"members,omitempty"`
}

// enclaveListResult is the envelope returned by the MCP server for enclave_list.
type enclaveListResult struct {
	Enclaves []EnclaveListItem `json:"enclaves"`
}

// EnclaveList calls the enclave_list MCP tool to enumerate enclaves.
// If callerEmail is non-empty, filters to only enclaves the caller belongs to.
func (c *Client) EnclaveList(ctx context.Context, callerEmail string) ([]EnclaveListItem, error) {
	raw, err := c.CallTool(ctx, "enclave_list", EnclaveListParams{CallerEmail: callerEmail})
	if err != nil {
		return nil, err
	}
	// Try envelope format first: {"enclaves": [...]}
	var envelope enclaveListResult
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Enclaves != nil {
		return envelope.Enclaves, nil
	}
	// Fall back to bare array for servers that return it directly.
	var items []EnclaveListItem
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil, fmt.Errorf("parsing enclave_list result: %w", err)
	}
	return items, nil
}

// --- enclave_sync ---

// EnclaveSyncParams are the arguments for the enclave_sync MCP tool.
type EnclaveSyncParams struct {
	Name          string   `json:"name"`
	NewOwner      string   `json:"new_owner,omitempty"`
	ChannelName   string   `json:"new_channel_name,omitempty"`
	Status        string   `json:"new_status,omitempty"`
	AddMembers    []string `json:"add_members,omitempty"`
	RemoveMembers []string `json:"remove_members,omitempty"`
}

// EnclaveSyncResult is the response from enclave_sync.
type EnclaveSyncResult struct {
	Name    string            `json:"name"`
	Updated []string          `json:"updated,omitempty"`
	Enclave EnclaveInfoResult `json:"enclave"`
}

// EnclaveSync calls the enclave_sync MCP tool to update enclave membership or metadata.
func (c *Client) EnclaveSync(ctx context.Context, params EnclaveSyncParams) (*EnclaveSyncResult, error) {
	raw, err := c.CallTool(ctx, "enclave_sync", params)
	if err != nil {
		return nil, err
	}
	var result EnclaveSyncResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing enclave_sync result: %w", err)
	}
	return &result, nil
}

// --- enclave_deprovision ---

// EnclaveDeprovisionParams are the arguments for the enclave_deprovision MCP tool.
type EnclaveDeprovisionParams struct {
	Name string `json:"name"`
}

// EnclaveDeprovisionResult is the response from enclave_deprovision.
type EnclaveDeprovisionResult struct {
	Name             string `json:"name"`
	Deleted          bool   `json:"deleted"`
	TentaclesRemoved int    `json:"tentacles_removed"`
}

// EnclaveDeprovision calls the enclave_deprovision MCP tool to destroy an enclave.
func (c *Client) EnclaveDeprovision(ctx context.Context, name string) (*EnclaveDeprovisionResult, error) {
	raw, err := c.CallTool(ctx, "enclave_deprovision", EnclaveDeprovisionParams{Name: name})
	if err != nil {
		return nil, err
	}
	var result EnclaveDeprovisionResult
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parsing enclave_deprovision result: %w", err)
	}
	return &result, nil
}
