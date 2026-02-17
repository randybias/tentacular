package spec

import (
	"strings"
	"testing"
)

func TestParseValidSpec(t *testing.T) {
	yaml := `
name: test-workflow
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
  transform:
    path: ./nodes/transform.ts
edges:
  - from: fetch
    to: transform
config:
  timeout: 30s
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Name != "test-workflow" {
		t.Errorf("expected name test-workflow, got %s", wf.Name)
	}
	if len(wf.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(wf.Nodes))
	}
	if len(wf.Edges) != 1 {
		t.Errorf("expected 1 edge, got %d", len(wf.Edges))
	}
}

func TestParseMissingName(t *testing.T) {
	yaml := `
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected errors for missing name")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "name is required") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'name is required' error, got: %v", errs)
	}
}

func TestParseInvalidName(t *testing.T) {
	yaml := `
name: NotKebab
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected errors for invalid name")
	}
}

func TestParseCycleDetection(t *testing.T) {
	yaml := `
name: cyclic
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
  b:
    path: ./b.ts
  c:
    path: ./c.ts
edges:
  - from: a
    to: b
  - from: b
    to: c
  - from: c
    to: a
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected cycle detection error")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "cycle") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected cycle error, got: %v", errs)
	}
}

func TestParseEdgeReferenceIntegrity(t *testing.T) {
	yaml := `
name: bad-refs
version: "1.0"
triggers:
  - type: manual
nodes:
  a:
    path: ./a.ts
edges:
  - from: a
    to: nonexistent
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected reference integrity error")
	}
}

func TestParseConfigExtras(t *testing.T) {
	yaml := `
name: test-workflow
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
config:
  timeout: 30s
  retries: 2
  nats_url: "nats://localhost:4222"
  custom_key: "custom_value"
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// Typed fields still work
	if wf.Config.Timeout != "30s" {
		t.Errorf("expected timeout 30s, got %s", wf.Config.Timeout)
	}
	if wf.Config.Retries != 2 {
		t.Errorf("expected retries 2, got %d", wf.Config.Retries)
	}

	// Custom keys land in Extras
	if wf.Config.Extras == nil {
		t.Fatal("expected Extras to be non-nil")
	}
	if wf.Config.Extras["nats_url"] != "nats://localhost:4222" {
		t.Errorf("expected nats_url in extras, got %v", wf.Config.Extras["nats_url"])
	}
	if wf.Config.Extras["custom_key"] != "custom_value" {
		t.Errorf("expected custom_key in extras, got %v", wf.Config.Extras["custom_key"])
	}
}

func TestConfigToMapMerged(t *testing.T) {
	cfg := WorkflowConfig{
		Timeout: "30s",
		Retries: 2,
		Extras:  map[string]interface{}{"nats_url": "test"},
	}
	m := cfg.ToMap()
	if m["timeout"] != "30s" {
		t.Errorf("expected timeout in map, got %v", m["timeout"])
	}
	if m["retries"] != 2 {
		t.Errorf("expected retries in map, got %v", m["retries"])
	}
	if m["nats_url"] != "test" {
		t.Errorf("expected nats_url in map, got %v", m["nats_url"])
	}
}

func TestConfigToMapOmitsZero(t *testing.T) {
	cfg := WorkflowConfig{
		Extras: map[string]interface{}{"nats_url": "test"},
	}
	m := cfg.ToMap()
	if _, ok := m["timeout"]; ok {
		t.Error("expected timeout to be omitted when zero")
	}
	if _, ok := m["retries"]; ok {
		t.Error("expected retries to be omitted when zero")
	}
	if m["nats_url"] != "test" {
		t.Errorf("expected nats_url in map, got %v", m["nats_url"])
	}
}

func TestConfigToMapNilExtras(t *testing.T) {
	cfg := WorkflowConfig{
		Timeout: "10s",
	}
	m := cfg.ToMap()
	if m["timeout"] != "10s" {
		t.Errorf("expected timeout in map, got %v", m["timeout"])
	}
	if len(m) != 1 {
		t.Errorf("expected 1 entry in map, got %d", len(m))
	}
}

func TestParseTriggerNameValid(t *testing.T) {
	yaml := `
name: test-workflow
version: "1.0"
triggers:
  - type: cron
    name: daily-digest
    schedule: "0 9 * * *"
  - type: cron
    name: hourly-check
    schedule: "0 * * * *"
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Triggers[0].Name != "daily-digest" {
		t.Errorf("expected trigger name daily-digest, got %s", wf.Triggers[0].Name)
	}
	if wf.Triggers[1].Name != "hourly-check" {
		t.Errorf("expected trigger name hourly-check, got %s", wf.Triggers[1].Name)
	}
}

func TestParseTriggerNameDuplicate(t *testing.T) {
	yaml := `
name: test-workflow
version: "1.0"
triggers:
  - type: cron
    name: daily-digest
    schedule: "0 9 * * *"
  - type: cron
    name: daily-digest
    schedule: "0 18 * * *"
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for duplicate trigger names")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "duplicate trigger name") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'duplicate trigger name' error, got: %v", errs)
	}
}

func TestParseTriggerNameInvalid(t *testing.T) {
	yaml := `
name: test-workflow
version: "1.0"
triggers:
  - type: cron
    name: "Invalid Name"
    schedule: "0 9 * * *"
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for invalid trigger name")
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

func TestParseQueueTriggerValid(t *testing.T) {
	yaml := `
name: queue-wf
version: "1.0"
triggers:
  - type: queue
    subject: events.github.push
nodes:
  handler:
    path: ./nodes/handler.ts
edges: []
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Triggers[0].Type != "queue" {
		t.Errorf("expected trigger type queue, got %s", wf.Triggers[0].Type)
	}
	if wf.Triggers[0].Subject != "events.github.push" {
		t.Errorf("expected subject events.github.push, got %s", wf.Triggers[0].Subject)
	}
}

func TestParseQueueTriggerMissingSubject(t *testing.T) {
	yaml := `
name: queue-wf
version: "1.0"
triggers:
  - type: queue
nodes:
  handler:
    path: ./nodes/handler.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for queue trigger missing subject")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "queue trigger requires subject") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'queue trigger requires subject' error, got: %v", errs)
	}
}

func TestParseTriggerValidation(t *testing.T) {
	yaml := `
name: bad-trigger
version: "1.0"
triggers:
  - type: cron
nodes:
  a:
    path: ./a.ts
edges: []
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected cron schedule error")
	}
}

func TestParseDeploymentNamespace(t *testing.T) {
	yaml := `
name: ns-test
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
deployment:
  namespace: pd-custom-ns
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Deployment.Namespace != "pd-custom-ns" {
		t.Errorf("expected deployment namespace pd-custom-ns, got %q", wf.Deployment.Namespace)
	}
}

func TestParseNoDeploymentSection(t *testing.T) {
	yaml := `
name: no-deploy
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Deployment.Namespace != "" {
		t.Errorf("expected empty deployment namespace when no deployment section, got %q", wf.Deployment.Namespace)
	}
}

func TestParseContractHTTPSDependency(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
contract:
  dependencies:
    github:
      protocol: https
      host: api.github.com
      port: 443
      auth:
        secret: github.token
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Contract == nil {
		t.Fatal("expected contract to be parsed")
	}
	if len(wf.Contract.Dependencies) != 1 {
		t.Fatalf("expected 1 dependency, got %d", len(wf.Contract.Dependencies))
	}
	github := wf.Contract.Dependencies["github"]
	if github.Protocol != "https" {
		t.Errorf("expected protocol https, got %s", github.Protocol)
	}
	if github.Host != "api.github.com" {
		t.Errorf("expected host api.github.com, got %s", github.Host)
	}
	if github.Port != 443 {
		t.Errorf("expected port 443, got %d", github.Port)
	}
	if github.Auth == nil || github.Auth.Secret != "github.token" {
		t.Errorf("expected auth.secret github.token")
	}
}

func TestParseContractPostgresDependency(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
contract:
  dependencies:
    postgres:
      protocol: postgresql
      host: postgres.svc.cluster.local
      port: 5432
      database: appdb
      user: postgres
      auth:
        secret: postgres.password
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Contract == nil {
		t.Fatal("expected contract to be parsed")
	}
	pg := wf.Contract.Dependencies["postgres"]
	if pg.Protocol != "postgresql" {
		t.Errorf("expected protocol postgresql, got %s", pg.Protocol)
	}
	if pg.Database != "appdb" {
		t.Errorf("expected database appdb, got %s", pg.Database)
	}
	if pg.User != "postgres" {
		t.Errorf("expected user postgres, got %s", pg.User)
	}
}

func TestParseContractInvalidProtocol(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
contract:
  dependencies:
    api:
      protocol: grpc
      host: api.example.com
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for invalid protocol")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "invalid protocol") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected invalid protocol error, got: %v", errs)
	}
}

func TestParseContractInvalidAuthSecret(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
contract:
  dependencies:
    api:
      protocol: https
      host: api.example.com
      auth:
        secret: invalid_format
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected error for invalid auth secret format")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e, "service.key") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected service.key format error, got: %v", errs)
	}
}

func TestParseContractMissingRequiredFields(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
contract:
  dependencies:
    postgres:
      protocol: postgresql
      host: postgres.svc
`
	_, errs := Parse([]byte(yaml))
	if len(errs) == 0 {
		t.Fatal("expected errors for missing postgresql fields")
	}
	// Should error on missing database and user
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
	if !foundDatabase {
		t.Error("expected error for missing database")
	}
	if !foundUser {
		t.Error("expected error for missing user")
	}
}

func TestParseContractOptional(t *testing.T) {
	yaml := `
name: test-wf
version: "1.0"
triggers:
  - type: manual
nodes:
  fetch:
    path: ./nodes/fetch.ts
edges: []
`
	wf, errs := Parse([]byte(yaml))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if wf.Contract != nil {
		t.Error("expected contract to be nil when not present")
	}
}
