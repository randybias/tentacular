# Agent Workflow Guide

Non-interactive workflow for coding agents deploying and
testing tentacular workflows. All commands support
`-o json` for structured output; when active, progress
messages go to stderr and only the JSON envelope goes
to stdout.

## Develop a Plan in Advance for New or Updated Workflows

Before writing or changing workflow code, the agent MUST run a
planning loop with the user. The goal is to eliminate hidden
dependencies (especially secrets and runtime config) before
implementation starts.

### Planning Objectives

1. Confirm the user's intent and expected outcome.
2. Author the `contract.dependencies` block first.
3. Derive secrets and network intent from the contract.
4. Pre-validate the environment, credentials, and
   connectivity.
5. Define explicit dev-to-prod promotion gates.

### Step 1: Confirm User Intent (Do Not Assume)

Ask targeted questions and restate the intent before coding:

1. What business outcome should this workflow produce?
2. What triggers are required (`manual`, `cron`, `queue`)?
3. What are the expected inputs and outputs?
4. What systems are read-only vs. write targets?
5. What constitutes success vs. acceptable degradation?

The agent should summarize the intent back to the user and get
confirmation before proceeding.

### Step 2: Author the Contract

The planning loop starts with contract authoring.
Enumerate all external dependencies and write the
`contract.dependencies` block in `workflow.yaml`:

1. APIs/services (GitHub, Slack, NATS, cloud storage)
2. Data stores (Postgres, Redis, object stores)
3. For each: protocol, host, port, auth type, secret ref
4. Environment-specific behavior (`dev` vs. `prod`)
5. Required trigger scheduling and runtime behavior

If any dependency is uncertain, stop and ask. Do not
proceed with guessed endpoints, guessed credentials,
or placeholder resources unless explicitly requested
for mock-only testing.

Once the contract is authored, run `tntc validate` and
`tntc visualize --rich` to verify derived artifacts:

- Derived secrets: confirm all `auth.secret` refs match
  provisioned keys in `.secrets.yaml`
- Derived NetworkPolicy: confirm egress rules match
  expected network access
- Review the rich visualization with the user

### Step 3: Define Config and Secrets Sources

The `config` section now holds only business-logic
parameters (e.g., `target_repo`, `sep_label`).
Connection metadata belongs in contract dependencies.

Secrets handling policy:

1. Secret keys are derived from contract `auth.secret`
   refs -- do not define them separately
2. Current source: local workflow secrets
   (`<workflow>/.secrets.yaml` or `<workflow>/.secrets/`)
3. Future source: external secrets vault (**FUTURE**)
4. Never use environment variables for workflow secrets
5. Never print secret values in terminal output
6. Validate key presence and format before live runs

Minimum to confirm with user:

1. Secret key names match contract `auth.secret` refs
2. Real target endpoints match contract host/port values
3. Environment mapping (`dev` namespace/context/image
   and `prod` namespace/context/image)
4. Expected side effects per environment

### Step 4: Planning Loop With User (Round-Trip Until Stable)

The agent should propose a concrete plan and iterate with the
user until there are no open questions.

Plan must include:

1. Workflow changes to make
2. Secrets/config values required per environment
3. Pre-validation checks to run first
4. Test sequence (unit/mock, pipeline, live dev)
5. Promotion gate for prod deploy
6. Rollback/cleanup plan (`tntc undeploy ... -y`)

The plan should be repeated back in detailed form after each
user clarification. Do not start implementation until the plan
is explicitly confirmed.

### Step 5: Pre-Validate Before Implementation Work

Run lightweight checks before coding/deploying:

```bash
# Validate cluster targets
tntc cluster check --fix -n <dev-namespace>
tntc cluster check --fix -n <prod-namespace>

# Validate workflow spec and contract
tntc validate <workflow-dir>

# Review derived artifacts with user
tntc visualize --rich <workflow-dir>

# Check secrets provisioning vs contract
tntc secrets check <workflow-dir>

# Confirm config points to expected image and envs
cat .tentacular/config.yaml
```

Also pre-validate critical credentials/connectivity where
possible (without exposing secret values), for example:

1. GitHub token format and API reachability
2. Database credentials vs. expected DB target
3. Slack webhook format and safe test delivery target
4. Storage URL/container existence and write permissions
5. Queue URL/auth/subject validity for queue triggers

If validation fails, fix inputs first; do not continue blindly.

### Step 6: Enforce Dev E2E Gate Before Prod

Every new or updated workflow must pass the full
contract and testing pipeline before production:

1. `tntc validate` (spec + contract)
2. `tntc visualize --rich` (review with user)
3. `tntc test` (mock tests + drift detection)
4. `tntc test --pipeline`
5. `tntc test --live --env dev`
6. Review outputs/logs and confirm side effects
7. Only then `tntc deploy --env prod` (with `--verify`)

Using separate dev creds is fine and encouraged. The key
requirement is that dev is a real environment with real
integration behavior, not mock-only.

### Non-Negotiable Agent Rules

1. Do not deploy to prod without a successful live dev run
   unless the user explicitly overrides this decision.
2. Do not use placeholder credentials for live testing.
3. Do not proceed when secret keys/config targets are ambiguous.
4. Always state resolved image, context, and namespace before
   running live tests or deploy.
5. Always clean up temporary deployments after live testing
   unless `--keep` is intentionally requested.

## Full E2E Cycle (Non-Interactive)

```bash
# 1. Validate spec + contract
tntc validate example-workflows/my-wf -o json

# 2. Persist and review contract artifacts with user
tntc visualize --rich --write example-workflows/my-wf

# 3. Mock tests + drift detection (no cluster needed)
tntc test example-workflows/my-wf -o json

# 4. Build container image
tntc build example-workflows/my-wf \
  -t tentacular-engine:my-wf

# 5. Live test on dev (deploy -> run -> validate -> cleanup)
tntc test --live --env dev \
  example-workflows/my-wf -o json

# 6. Deploy to target environment
tntc deploy --env prod \
  example-workflows/my-wf -o json

# 7. Post-deploy run
tntc run my-wf -n <namespace> -o json

# 8. Cleanup when done
tntc undeploy my-wf -n <namespace> -y
```

## Image Tag Cascade

The CLI resolves the engine image in this order:

1. `--image` flag (highest priority)
2. Environment config `image` field (`--env`)
3. `<workflow-dir>/.tentacular/base-image.txt` (written
   by `tntc build`)
4. `tentacular-engine:latest` (fallback)

Repository default: configure `environments.<name>.image`
to `ghcr.io/randybias/tentacular-engine:latest` so deploys
do not rely on the unqualified fallback tag.

`tntc build` writes the built tag to the workflow directory,
not the current working directory. Both `deploy` and
`test --live` read from the workflow directory automatically.

## Build + Registry Interaction

The config `registry` value is prepended to the `-t` tag only
when the tag does not already contain a registry prefix (a `/`
before the `:`). This prevents double-prefixing:

```bash
# Config has registry: reg.io
tntc build -t my-image:v1          # builds reg.io/my-image:v1
tntc build -t reg.io/my-image:v1   # builds reg.io/my-image:v1 (no double prefix)
```

When pushing to a remote registry for ARM64 clusters:

```bash
tntc build -t tentacular-engine:my-wf \
  -r reg.io --push --platform linux/arm64
```

## Deploy with --env

The `--env` flag resolves context, namespace, runtime-class,
and image from the named environment in config. CLI flags
override environment values:

```bash
# Use everything from prod environment config
tntc deploy --env prod example-workflows/my-wf

# Override namespace from environment
tntc deploy --env prod -n custom-ns example-workflows/my-wf

# Force deploy (skip live test gate)
tntc deploy --env prod --force example-workflows/my-wf
```

## Fresh Deploy vs. Update

On fresh deployments (all resources created, none updated),
the CLI skips the rollout restart since Kubernetes already
starts pods for new Deployments. On updates to existing
resources, a rollout restart is triggered automatically.

## JSON Output Behavior

When `-o json` is active:

- **stdout**: Only the structured JSON envelope
- **stderr**: All progress messages (preflight checks,
  manifest application, rollout status, cleanup)

This lets agents parse stdout as JSON while still seeing
progress in stderr. Parse the `status` field ("pass" or
"fail") to determine success. On failure, check `hints`
for remediation steps.

## Common Gotchas for Agents

1. **undeploy needs confirmation**: Always pass `-y`
   in non-interactive scripts. Without it, the command
   blocks waiting for stdin.

2. **Namespace must exist**: `deploy` does not create
   namespaces. Create them first with `kubectl create
   namespace <name>`.

3. **kubeconfig context**: When targeting multiple
   clusters, use `--env` to switch contexts automatically.
   Without `--env`, the CLI uses the current kubeconfig
   context.

4. **kind clusters**: Auto-detected by context name
   prefix `kind-`. gVisor and imagePullPolicy are
   adjusted automatically. After `tntc build`, images
   are loaded into kind via `kind load docker-image`.

5. **Secrets**: Local secrets come from
   `<workflow-dir>/.secrets.yaml`. The CLI generates a
   K8s Secret manifest from this file during deploy.
   If `.secrets/` directory exists, it takes precedence
   over `.secrets.yaml`.

6. **Deploy gate**: When a `dev` environment is
   configured, `deploy` auto-runs a live test first.
   Use `--force` to skip. This only triggers when the
   CLI detects a dev environment in config.

7. **Contract required**: All workflows need a
   `contract` section (even if `dependencies: {}`).
   Deploy fails in strict mode without a valid
   contract. See [Contract Model](contract.md).

8. **NetworkPolicy auto-generated**: `deploy`
   generates NetworkPolicy from contract dependencies.
   Verify with `kubectl get networkpolicy` after
   deploy. Use `contract.networkPolicy.additionalEgress`
   for edge cases not derivable from dependencies.

9. **Drift detection**: `tntc test` compares runtime
   behavior against contract declarations. Direct
   `ctx.fetch()` or `ctx.secrets` usage is flagged
   as a contract violation. Use `ctx.dependency()`.
