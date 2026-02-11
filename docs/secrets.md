# Secrets Management

## Local Development

Copy the generated template and fill in values:

```bash
cp .secrets.yaml.example .secrets.yaml
```

```yaml
# .secrets.yaml (gitignored)
github:
  token: "ghp_..."
slack:
  webhook_url: "https://hooks.slack.com/services/..."
anthropic:
  api_key: "sk-ant-..."
```

The engine loads `.secrets.yaml` at startup. Values are available via `ctx.secrets` and used for `ctx.fetch` auth injection.

## Production (Kubernetes)

`tntc deploy` automatically provisions secrets to Kubernetes from:
1. `.secrets/` directory (files as secret entries), or
2. `.secrets.yaml` file (YAML keys as secret entries)

The K8s Secret is mounted read-only at `/app/secrets` inside the container. Secrets are **never** exposed as environment variables.

## Manual Secret Management

To manage secrets manually:

```bash
kubectl create secret generic my-workflow-secrets \
  -n my-namespace \
  --from-file=github=./github-token.json \
  --from-file=slack=./slack-config.json
```

Convention: secrets are named `<workflow-name>-secrets`.

## Cascade Precedence

Secrets are resolved in order, with later sources merging on top of earlier ones:

| Priority | Source | Description |
|----------|--------|-------------|
| 1 (highest) | `--secrets <path>` | Explicit flag — skips all other sources |
| 2 | `/app/secrets` | K8s Secret volume mount (always checked last, merges on top) |
| 3 | `.secrets.yaml` | YAML file in workflow directory |
| 4 (base) | `.secrets/` | Directory of files (K8s volume mount format) |

When no explicit path is given: `.secrets/` provides the base, `.secrets.yaml` merges on top, then `/app/secrets` merges on top of everything.

See [architecture.md](architecture.md) for details on auth injection and deploy-time provisioning.
