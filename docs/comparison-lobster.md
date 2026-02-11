# Tentacular vs. Lobster

A deep comparison between Tentacular and [openclaw/lobster](https://github.com/openclaw/lobster), two TypeScript-based workflow engines that solve adjacent but distinct problems.

## Shared Concepts

Both projects turn multi-step tasks into deterministic, composable pipelines rather than ad-hoc sequential execution. Both:

- Define workflows as stages where structured data flows between steps
- Use TypeScript as the node/stage authoring language
- Provide a CLI as the primary interface
- Pass JSON-structured data between stages (not raw text)
- Include fixture-based testing patterns
- Are early-stage projects (~3,500-8,500 lines of code)

## At a Glance

| Dimension | Tentacular | Lobster |
|-----------|---------------|---------|
| **Target user** | DevOps/platform engineers building scheduled data pipelines | AI agent ("Clawdbot") orchestrating user-facing automations |
| **Runtime** | Deno (in-container) | Node.js (local machine) |
| **CLI language** | Go (Cobra) | TypeScript |
| **Deployment** | Kubernetes (Deployments, CronJobs, ConfigMaps) | Local-only (`~/.lobster/state/`) |
| **Trigger model** | Cron, manual HTTP, NATS queue | AI agent tool call, CLI invocation |
| **Workflow format** | YAML DAG (`nodes:` + `edges:`) | YAML steps (linear), pipeline strings (`cmd \| cmd`), or SDK fluent API |
| **Execution topology** | True DAG with fan-out, fan-in, parallel stages | Primarily linear pipelines |
| **Security model** | 5-layer defense-in-depth (gVisor, Deno perms, distroless, SecurityContext, secret mounts) | No container sandbox; delegates auth to Clawdbot |
| **Secrets** | 4-level cascade with K8s Secret volume mounts | None; Clawdbot owns all auth |
| **Human-in-the-loop** | None (fully automated once deployed) | Core feature: `approve` gates halt execution until confirmed |
| **Statefulness** | Stateless (each run is independent) | Stateful: `diff.last` tracks changes, cursor-based resumption |
| **Resume/halt** | Not supported | First-class: base64url resume tokens |
| **Container model** | Distroless Deno container per workflow | No containers; local Node.js process |
| **Dependencies** | Go modules + Deno stdlib | 2 runtime deps (`yaml` + `ajv`) |

## Architectural Philosophy

### Tentacular: Kubernetes-native workflow engine

Tentacular assumes:

- Workflows run unattended on infrastructure you control
- Security is paramount (defense-in-depth, gVisor, distroless)
- Code is authored by trusted developers and deployed to clusters
- Scheduling via K8s CronJobs, triggered via HTTP or NATS
- Each workflow lives in its own hardened pod

### Lobster: AI agent execution layer

Lobster assumes:

- Workflows are invoked by an LLM (Clawdbot) as a "tool"
- Safety comes from approval gates, not container sandboxing
- Runs locally on the user's machine with no infrastructure to manage
- The AI agent must not take irreversible actions without human approval
- Token savings matter (1 Lobster call replaces 10+ LLM tool calls)

## Concepts Lobster Has That Tentacular Lacks

### Approval gates

Hard pipeline halts requiring human confirmation before side effects. The pipeline physically cannot continue until explicit approval. This is Lobster's core safety primitive for AI-driven automation.

### Resume tokens

Serialized pipeline state (base64url-encoded JSON) that allows halted workflows to continue after approval. Enables async human-in-the-loop patterns where the agent pauses, presents choices, and resumes.

### Change detection (`diff.last`)

Stateful comparisons between runs. Stores the current value under a key and compares it to the previously stored value, enabling "only react to changes" patterns (e.g., a PR monitor that only notifies when PR state actually changed).

### LLM task invocation

Built-in command to call an LLM with caching, JSON schema validation, and retries. Includes run-state caching (resumes don't re-invoke the LLM) and file-based persistent cache keyed by SHA-256 hash.

### Dual output modes

Human-readable (TTY) vs. structured JSON envelope (`{ protocolVersion, ok, status, output, requiresApproval }`) for programmatic consumption by AI agents.

### Recipe system

Pre-built composable workflow templates with attached `.meta` for schema/description, enabling dynamic discovery by AI agents.

## Concepts Tentacular Has That Lobster Lacks

### True DAG execution

Fan-out, fan-in, and diamond patterns with parallel stage execution via `Promise.all()`. Nodes within a stage run concurrently; stages execute sequentially. Lobster pipelines are primarily linear chains.

### Kubernetes deployment

Full lifecycle from `build` to `deploy` to `undeploy`. Generates Deployments, Services, ConfigMaps, Secrets, and CronJobs. No kubectl knowledge required.

### Container security (5 layers)

1. Distroless base image (no shell, no package manager)
2. Deno permission locking (`--allow-net`, `--allow-read=/app`, `--allow-write=/tmp`)
3. gVisor sandbox (kernel-level syscall interception)
4. K8s SecurityContext (`runAsNonRoot`, `readOnlyRootFilesystem`, `drop: ALL`)
5. Secrets as volumes (never environment variables)

### Secrets management

4-level cascade with merge semantics:

1. Explicit `--secrets <path>` flag (highest priority)
2. `/app/secrets` K8s Secret volume mount
3. `.secrets.yaml` file
4. `.secrets/` directory (base)

Auth injection via `ctx.fetch("github", "/user/repos")` automatically resolves service URLs and injects bearer tokens or API keys from secrets.

### Hot-reload dev server

File watcher monitors the workflow directory. On change: clear module cache, reload nodes, swap references atomically so in-flight requests complete with old code.

### DAG visualization

Generates Mermaid diagrams from workflow topology via `tntc visualize`.

### Cluster operations

Preflight checks with auto-remediation, status monitoring, log streaming, and deployment listing across namespaces.

## Cross-Pollination Opportunities

### Ideas Tentacular could adopt from Lobster

- **Approval gates** for workflows with destructive side effects (e.g., "send this notification", "deploy this change"). Could be implemented as a webhook-based approval step that pauses execution.
- **Change detection / `diff.last`** to help cron workflows avoid duplicate notifications. A persistent state store per workflow could track last-seen values.
- **Resume tokens** for long-running workflows that need human intervention mid-flight, enabling async approval patterns.

### Ideas Lobster could adopt from Tentacular

- **True DAG execution** with fan-out/fan-in for more powerful parallel pipelines.
- **Container isolation** to address the security gap when running untrusted pipeline stages.
- **Secrets cascade** to remove the hard dependency on Clawdbot for all authentication.

## Summary

These projects solve adjacent but distinct problems:

- **Tentacular** = "I have recurring data pipelines that should run on a schedule in my cluster with strong isolation."
- **Lobster** = "My AI agent needs to safely orchestrate multi-step tasks on my local machine, pausing for human approval before irreversible actions."

The overlap is in the pipeline/DAG abstraction and TypeScript node authoring. The divergence is in deployment target (Kubernetes vs. local), security model (container sandboxing vs. approval gates), and invocation model (cron/HTTP vs. AI agent tool call).
