## 1. Contract Schema and Parser (Go)

- [x] 1.1 Define `Contract` struct in Go types: `Version string`, `Dependencies map[string]Dependency`, `NetworkPolicy *NetworkPolicyOverrides`, `Extensions map[string]interface{} \`yaml:",inline"\`` for x-* fields
- [x] 1.2 Define `Dependency` struct: `Protocol`, `Host`, `Port`, `Auth DependencyAuth`, plus protocol-specific fields (database, user, subject, container, etc.). Supported protocols: `https`, `postgresql`, `nats`, `blob` with protocol-specific default ports (443, 5432, 4222, 443)
- [x] 1.3 Define `DependencyAuth` struct: `Type string` (bearer-token, api-key, sas-token, password, webhook-url), `Secret string` (key reference, e.g. "postgres.password")
- [x] 1.4 Define `NetworkPolicyOverrides` struct: `AdditionalEgress []EgressRule`
- [x] 1.5 Add `Contract *Contract` field to `Workflow` struct as top-level peer of Nodes/Edges/Config
- [x] 1.6 Extend `spec.Parse()` to parse `contract` block with protocol-specific validation per dependency type, wildcard host rejection, and default port application
- [x] 1.7 Add derivation functions: `DeriveSecrets(contract) []string`, `DeriveEgressRules(contract) []EgressRule`, `DeriveIngressRules(triggers) []IngressRule`
- [x] 1.8 Add referential integrity validation (auth secret refs, extension field preservation)
- [x] 1.9 Add strict enforcement by default; support environment-level `enforcement: audit` override
- [x] 1.10 Unit tests: valid contracts, invalid protocols, missing required fields, secret value rejection, empty dependencies, extension round-trip, derivation correctness, wildcard host rejection, default port application, invalid auth types

## 2. Engine: ctx.dependency() API (TypeScript)

Depends on: group 1 (contract schema must be parseable so engine can receive it)

- [x] 2.1 Add `dependency(name: string): DependencyConnection` to Context interface in `engine/types.ts`
- [x] 2.2 Define `DependencyConnection` type: protocol, host, port, protocol-specific fields, secret (eagerly resolved value), authType, and `fetch(path, init?)` convenience method for HTTPS deps (builds URL, no auth injection; nodes handle auth explicitly)
- [x] 2.3 Implement `dependency()` in production context (`engine/context/mod.ts`): resolve from contract metadata + mounted secrets, create fetch wrapper for HTTPS deps
- [x] 2.4 Implement `dependency()` in mock context (`engine/testing/mocks.ts`): return mock values with mock fetch, record dependency access and fetch calls for tracing
- [x] 2.5 Thread contract metadata from Go CLI → engine execution (workflow spec already passes to engine; extend with contract)
- [x] 2.6 Unit tests: dependency resolution, undeclared dependency error, mock recording, secret resolution, dep.fetch() URL construction, mock fetch recording

## 3. Runtime-Tracing Drift Detection

Depends on: group 2 (mock context must record accesses)

- [x] 3.1 Extend mock context to record all `ctx.dependency()` calls with names
- [x] 3.2 Extend mock context to record all `ctx.fetch()` calls with service/host
- [x] 3.3 Extend mock context to record all `ctx.secrets` accesses with service/key
- [x] 3.4 Implement drift comparator: recorded usage vs contract declarations (missing deps, dead deps, direct ctx.fetch/ctx.secrets bypass)
- [x] 3.5 Integrate drift report into `tntc test` output: strict mode (fail) / audit mode (warn)
- [x] 3.6 Add deterministic drift report format in text and JSON outputs
- [x] 3.7 Unit tests: no drift, missing declaration, dead declaration, bypass detection, audit vs strict behavior

## 4. NetworkPolicy Generation (Go)

Depends on: group 1 (derivation functions from task 1.7)

- [x] 4.0 Add `"NetworkPolicy": "networkpolicies"` to `findResource()` in `pkg/k8s/client.go` (group: `networking.k8s.io`, version: `v1`)
- [x] 4.1 Implement K8s NetworkPolicy manifest generation from derived egress rules in `pkg/builder/k8s.go`
- [x] 4.2 Implement mandatory DNS egress rule (UDP/TCP 53 to kube-dns)
- [x] 4.3 Implement ingress rule derivation from trigger types (webhook → allow, else deny-all)
- [x] 4.4 Implement `additionalEgress` override merging
- [x] 4.5 Integrate NetworkPolicy emission into `tntc deploy` manifest generation
- [x] 4.6 Add deploy preflight gate: fail on contract validation errors in strict mode
- [x] 4.7 Unit tests: egress per dependency, DNS rule always present, webhook ingress, no-ingress for cron, CIDR override merge, strict-mode abort

## 5. Visualization and Planning Artifacts

Depends on: group 1 (contract parsed), group 4 (derived policy available)

- [x] 5.1 Extend `tntc visualize` with `--rich` mode: DAG + dependency nodes with protocol/host labels
- [x] 5.2 Include derived secret inventory and derived network policy summary in rich output
- [x] 5.3 Add write mode: emit co-resident Mermaid + contract-summary artifacts in workflow dir
- [x] 5.4 Ensure artifact output is deterministic and stable for PR diffs
- [x] 5.5 Unit tests: rich visualization content, file output, deterministic output

## 6. Example Workflow Updates

Depends on: groups 1-3 (schema, ctx.dependency(), drift detection all working)

### 6a. word-counter (no external deps)

- [x] 6a.1 Add minimal contract block: `contract: {version: "1", dependencies: {}}`
- [x] 6a.2 Verify `tntc test` passes with empty contract (no drift — no deps to declare)
- [x] 6a.3 Verify `tntc validate` shows valid contract with empty derived artifacts

### 6b. sep-tracker (full external deps)

- [x] 6b.1 Add contract block with all dependencies: github-api (https), postgres (postgresql), azure-blob (https), slack-webhook (https)
- [x] 6b.2 Move pg_host, pg_port, pg_database, pg_user from `config` to `contract.dependencies.postgres`
- [x] 6b.3 Update fetch-seps node: use `ctx.dependency("github-api").fetch(path)` instead of `ctx.fetch("github", path)`
- [x] 6b.4 Update diff-seps node: use `ctx.dependency("postgres")` for connection info
- [x] 6b.5 Update store-report node: use `ctx.dependency("postgres")` and `ctx.dependency("azure-blob")`
- [x] 6b.6 Update render-report node: no external deps (pure transform), verify no contract violations
- [x] 6b.7 Update notify node: use `ctx.dependency("slack-webhook")`
- [x] 6b.8 Update all test fixtures for new ctx.dependency() mock patterns
- [x] 6b.9 Verify `tntc test` passes with strict contract enforcement (0 drift)
- [x] 6b.10 Verify `tntc validate` shows correct derived secrets, derived policy

## 7. SKILL Integration and Agent Workflow Changes

Depends on: groups 1-6 (all CLI commands and example workflows must exist)

- [x] 7.1 Update `tentacular-skill/SKILL.md` to require pre-build contract review gate with rich visualization
- [x] 7.2 Add SKILL checklist: confirm dependency targets, derived secrets, derived network intent before build/deploy
- [x] 7.3 Add SKILL examples for `validate` + `visualize --rich` + review loop with user
- [x] 7.4 Update SKILL references to treat co-resident diagrams as mandatory planning artifacts
- [x] 7.5 Add regression checklist: agents fail closed when contract/diagram artifacts are missing or stale

## 8. Testing: Unit and Integration

Runs throughout, but final validation here.

- [x] 8.1 Parser unit tests: all contract schema variations (task 1.10)
- [x] 8.2 Engine unit tests: ctx.dependency() resolution and mocking (task 2.6)
- [x] 8.3 Drift detection unit tests: all violation types (task 3.7)
- [x] 8.4 NetworkPolicy unit tests: all derivation scenarios (task 4.7)
- [x] 8.5 Visualization unit tests: rich output and artifact generation (task 5.5)
- [x] 8.6 Integration test: word-counter mock test passes with empty contract
- [x] 8.7 Integration test: sep-tracker mock test passes with full contract, 0 drift
- [x] 8.8 Integration test: sep-tracker `tntc validate` shows correct derived artifacts
- [x] 8.9 Integration test: deliberate contract mismatch triggers strict failure
- [x] 8.10 Integration test: audit mode downgrades errors to warnings

## 9. E2E Testing

Depends on: all above groups complete

### 9a. Infrastructure setup

- [x] 9a.1 Build tntc from worktree with contract support
- [x] 9a.2 Create/verify kind-tentacular-dev cluster
- [x] 9a.3 Verify prod cluster access (context: Default, nats-admin kubeconfig)
- [x] 9a.4 Create .tentacular/config.yaml with dev/prod environments
- [x] 9a.5 Create sep-tracker .secrets.yaml from ~/dev-secrets

### 9b. word-counter E2E on kind (dev)

- [x] 9b.1 `tntc test example-workflows/word-counter` — mock test passes, empty contract valid
- [x] 9b.2 `tntc validate example-workflows/word-counter -o json` — JSON shows empty derived artifacts
- [x] 9b.3 `tntc visualize --rich example-workflows/word-counter` — rich output with no dependencies
- [x] 9b.4 `tntc build` + `tntc test --live --env dev` — live test on kind passes
- [x] 9b.5 `tntc deploy --env dev` — deploys with NetworkPolicy (deny-all egress except DNS, deny-all ingress for manual trigger)
- [x] 9b.6 Verify generated NetworkPolicy on cluster: `kubectl get networkpolicy -n dev-workflows -o yaml`
- [x] 9b.7 `tntc run word-counter -n dev-workflows -o json` — workflow executes successfully with policy applied
- [x] 9b.8 Cleanup: `tntc undeploy -n dev-workflows word-counter`

### 9c. sep-tracker E2E on prod

- [x] 9c.1 `tntc test example-workflows/sep-tracker` — mock test passes, full contract valid, 0 drift
- [x] 9c.2 `tntc validate example-workflows/sep-tracker -o json` — JSON shows derived secrets (4), derived egress rules (4 deps + DNS), no ingress (cron trigger)
- [x] 9c.3 `tntc visualize --rich example-workflows/sep-tracker` — rich output with dependency graph
- [x] 9c.4 `tntc build --push` to registry
- [x] 9c.5 `tntc deploy --force -n tentacular` — deploys with generated NetworkPolicy
- [x] 9c.6 Verify generated NetworkPolicy on prod: egress allows github (443), postgres (5432), azure blob (443), slack (443), DNS (53); denies all other egress; denies all ingress
- [x] 9c.7 `tntc run sep-tracker -n tentacular -o json` — full workflow executes (or degrades gracefully on expired creds)
- [x] 9c.8 `tntc logs -n tentacular sep-tracker` — verify node execution sequence
- [x] 9c.9 Cleanup: `tntc undeploy -n tentacular sep-tracker`

### 9d. Negative E2E tests

- [x] 9d.1 Deliberately remove a dependency from sep-tracker contract → `tntc test` fails strict mode
- [x] 9d.2 Add a dead dependency to word-counter contract → `tntc test` fails strict mode
- [x] 9d.3 Set env override `enforcement: audit` → same tests produce warnings, not failures
- [x] 9d.4 Deploy with contract validation failure in strict mode → deploy aborts before manifest apply
- [x] 9d.5 Verify `tntc test -o json` drift report is valid parseable JSON with actionable details
