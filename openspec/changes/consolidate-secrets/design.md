# Design: Centralize example workflow secrets

## Shared Secrets Directory Structure

```
<repo-root>/
  .secrets.yaml.example    # Documents all shared secrets (checked in)
  .secrets/                 # Actual shared secret values (git-ignored)
    github_token
    hn_api_key
  examples/
    hn-digest/
      .secrets.yaml.example  # References: $shared.github_token
      .secrets.yaml          # Resolved at deploy time (git-ignored)
    pr-review/
      .secrets.yaml.example  # References: $shared.github_token
```

## Secret Resolution Flow

The existing `resolveSharedSecrets()` in `pkg/cli/secrets.go` already handles
`$shared.<name>` references. The flow:

1. User runs `tntc secrets init --shared` at repo root.
2. This creates `.secrets/` directory with files from `.secrets.yaml.example`.
3. User edits `.secrets/<name>` files with actual values.
4. `tntc deploy` calls `buildSecretManifest()` which calls
   `resolveSharedSecrets()` which reads `$shared.<name>` from `.secrets/`.

## New CLI Changes

### `tntc secrets init --shared`

When `--shared` is passed:
1. Look for `.secrets.yaml.example` at repo root (found via `findRepoRoot()`).
2. Parse the YAML to extract secret names.
3. Create `.secrets/` directory at repo root.
4. For each secret, create an empty file at `.secrets/<name>`.
5. Print instructions to edit the files.

### `tntc secrets check` enhancement

When checking a workflow that uses `$shared.<name>` references:
1. Resolve the repo root.
2. For each `$shared.<name>` reference, check if `.secrets/<name>` exists and
   is non-empty.
3. Report missing or empty shared secrets.

## Example `.secrets.yaml.example` at repo root

```yaml
# Shared secrets for all tentacular example workflows.
# Run: tntc secrets init --shared
# Then edit the files in .secrets/ with actual values.

github_token: ""     # GitHub personal access token
hn_api_key: ""       # Hacker News API key (if applicable)
```

## Example workflow `.secrets.yaml.example`

```yaml
# Secrets for hn-digest workflow.
# Uses shared secrets from repo root.
github: $shared.github_token
```
