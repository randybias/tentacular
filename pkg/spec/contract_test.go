package spec

import (
	"strings"
	"testing"
)

// --- Additional Contract Parsing Tests (Phase 1 Comprehensive Coverage) ---

func TestParseContractEmptyDependencies(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies: {}
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Contract == nil {
		t.Fatal("expected contract to be parsed")
	}
	if len(wf.Contract.Dependencies) != 0 {
		t.Errorf("expected empty dependencies map, got %d entries", len(wf.Contract.Dependencies))
	}
}

func TestParseContractMultipleDependenciesDifferentProtocols(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    github:
      protocol: https
      host: api.github.com
      port: 443
      auth:
        type: bearer-token
        secret: github.token
    postgres:
      protocol: postgresql
      host: postgres.svc
      port: 5432
      database: appdb
      user: postgres
      auth:
        type: bearer-token
        secret: postgres.password
    nats-queue:
      protocol: nats
      host: nats.svc
      port: 4222
      subject: events.workflow
      auth:
        type: password
        secret: nats.token
    azure-storage:
      protocol: blob
      host: storage.blob.core.windows.net
      port: 443
      container: reports
      auth:
        type: bearer-token
        secret: azure.sas_token
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(wf.Contract.Dependencies) != 4 {
		t.Fatalf("expected 4 dependencies, got %d", len(wf.Contract.Dependencies))
	}

	// Verify each protocol
	if wf.Contract.Dependencies["github"].Protocol != "https" {
		t.Error("expected github to be https protocol")
	}
	if wf.Contract.Dependencies["postgres"].Protocol != "postgresql" {
		t.Error("expected postgres to be postgresql protocol")
	}
	if wf.Contract.Dependencies["nats-queue"].Protocol != "nats" {
		t.Error("expected nats-queue to be nats protocol")
	}
	if wf.Contract.Dependencies["azure-storage"].Protocol != "blob" {
		t.Error("expected azure-storage to be blob protocol")
	}
}

func TestParseContractNATSDependency(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    messaging:
      protocol: nats
      host: nats.svc.cluster.local
      port: 4222
      subject: events.workflow
      auth:
        type: bearer-token
        secret: nats.token
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	nats := wf.Contract.Dependencies["messaging"]
	if nats.Protocol != "nats" {
		t.Errorf("expected protocol nats, got %s", nats.Protocol)
	}
	if nats.Subject != "events.workflow" {
		t.Errorf("expected subject events.workflow, got %s", nats.Subject)
	}
}

func TestParseContractBlobDependency(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    storage:
      protocol: blob
      host: storage.blob.core.windows.net
      port: 443
      container: reports
      auth:
        type: bearer-token
        secret: azure.sas_token
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	blob := wf.Contract.Dependencies["storage"]
	if blob.Protocol != "blob" {
		t.Errorf("expected protocol blob, got %s", blob.Protocol)
	}
	if blob.Container != "reports" {
		t.Errorf("expected container reports, got %s", blob.Container)
	}
}

func TestParseContractExtensionFields(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    postgres:
      protocol: postgresql
      host: postgres.svc
      port: 5432
      database: appdb
      user: postgres
      auth:
        type: password
        secret: postgres.password
      sslMode: require
      connectionTimeout: 30s
  x-provider-metadata:
    region: us-west-2
    tier: production
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Check dependency extension fields
	postgres := wf.Contract.Dependencies["postgres"]
	if postgres.Extensions == nil {
		t.Fatal("expected dependency extensions to be non-nil")
	}
	if postgres.Extensions["sslMode"] != "require" {
		t.Error("expected sslMode extension field preserved in dependency")
	}
	if postgres.Extensions["connectionTimeout"] != "30s" {
		t.Error("expected connectionTimeout extension field preserved in dependency")
	}

	// Check contract extension fields
	if wf.Contract.Extensions == nil {
		t.Fatal("expected contract extensions to be non-nil")
	}
}

func TestParseContractNetworkPolicyOverride(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    github:
      protocol: https
      host: api.github.com
      port: 443
  networkPolicyOverride:
    additionalEgress:
      - toCIDR: 10.0.0.0/8
        ports:
          - "8080/TCP"
          - "8443/TCP"
      - toCIDR: 172.16.0.0/12
        ports:
          - "9000/TCP"
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Contract.NetworkPolicyOverride == nil {
		t.Fatal("expected networkPolicyOverride to be non-nil")
	}
	if len(wf.Contract.NetworkPolicyOverride.AdditionalEgress) != 2 {
		t.Fatalf("expected 2 additionalEgress rules, got %d", len(wf.Contract.NetworkPolicyOverride.AdditionalEgress))
	}

	rule1 := wf.Contract.NetworkPolicyOverride.AdditionalEgress[0]
	if rule1.ToCIDR != "10.0.0.0/8" {
		t.Errorf("expected CIDR 10.0.0.0/8, got %s", rule1.ToCIDR)
	}
	if len(rule1.Ports) != 2 {
		t.Errorf("expected 2 ports, got %d", len(rule1.Ports))
	}
}

// --- Error Path Tests ---

func TestParseContractMissingProtocol(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    api:
      host: api.example.com
      port: 443
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for missing protocol")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "protocol is required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'protocol is required' error, got: %v", errs)
	}
}

func TestParseContractHTTPSMissingHost(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    api:
      protocol: https
      port: 443
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for missing host in https dependency")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "https requires host") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'https requires host' error, got: %v", errs)
	}
}

func TestParseContractNATSMissingSubject(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    messaging:
      protocol: nats
      host: nats.svc
      port: 4222
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for missing subject in nats dependency")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "nats requires subject") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'nats requires subject' error, got: %v", errs)
	}
}

func TestParseContractBlobMissingContainer(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    storage:
      protocol: blob
      host: storage.blob.core.windows.net
      port: 443
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for missing container in blob dependency")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "blob requires container") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'blob requires container' error, got: %v", errs)
	}
}

func TestParseContractInvalidDependencyName(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    Invalid-Name:
      protocol: https
      host: api.example.com
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for invalid dependency name")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "name must match") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected name format error, got: %v", errs)
	}
}

func TestParseContractAuthMissingSecret(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    api:
      protocol: https
      host: api.example.com
      auth:
        type: bearer-token
        secret: ""
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for empty auth.secret")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "auth.secret is required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'auth.secret is required' error, got: %v", errs)
	}
}

func TestParseContractMultipleValidationErrors(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    postgres:
      protocol: postgresql
      host: postgres.svc
      # Missing database and user
    invalid-proto:
      protocol: grpc
      host: api.example.com
`
	_, errs := Parse([]byte(yaml))
	// Unknown protocols now log warnings instead of errors, so expect only 2 errors (database, user)
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors (database, user), got %d: %v", len(errs), errs)
	}
	// Verify we get the expected postgresql errors
	foundDatabase := false
	foundUser := false
	for _, e := range errs {
		if strings.Contains(e, "database") {
			foundDatabase = true
		}
		if strings.Contains(e, "user") {
			foundUser = true
		}
	}
	if !foundDatabase || !foundUser {
		t.Errorf("expected database and user errors, got: %v", errs)
	}
}

func TestParseContractDependencyWithoutAuth(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
contract:
  version: "1"
  dependencies:
    public-api:
      protocol: https
      host: api.example.com
      port: 443
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors for dependency without auth: %v", errs)
	}
	dep := wf.Contract.Dependencies["public-api"]
	if dep.Auth != nil {
		t.Error("expected auth to be nil for public dependency")
	}
}
