# Centralize example workflow secrets

## Why

Each example workflow currently has its own `.secrets.yaml.example` file with
duplicated secret references. For example, multiple workflows may reference the
same GitHub token or HN API key. There is no shared secret mechanism beyond the
`$shared.<name>` reference syntax that resolves against a repo-root `.secrets/`
directory.

The current approach has two problems:

1. **Secret duplication**: The same secret value must be provisioned in every
   workflow's `.secrets.yaml` or `.secrets/` directory.
2. **Missing shared infrastructure**: The `$shared.<name>` reference exists in
   `resolveSharedSecrets()` (in `pkg/cli/secrets.go`) but there is no tooling
   to initialize or manage the shared secrets directory.

## What Changes

- **Create a repo-root `.secrets.yaml.example`** that documents all shared
  secrets used across example workflows.
- **Update example workflows** to use `$shared.<name>` references in their
  `.secrets.yaml.example` files instead of duplicating secret values.
- **Add `tntc secrets init --shared`** subcommand that initializes the repo-root
  `.secrets/` directory from `.secrets.yaml.example`.
- **Update `tntc secrets check`** to also verify shared secret availability when
  `$shared.<name>` references are present.
- **Update `pkg/cli/secrets.go`** with the `--shared` flag handler and shared
  secrets validation logic.

## Acceptance Criteria

- `tntc secrets init --shared` creates `.secrets/` directory at repo root with
  placeholder files from `.secrets.yaml.example`.
- `tntc secrets check` reports missing shared secrets when workflows reference
  `$shared.<name>` and the shared secret is not provisioned.
- Example workflows reference shared secrets via `$shared.<name>` instead of
  containing their own copies.

## Non-goals

- Encrypting secrets at rest -- that is a separate concern.
- Cloud secret manager integration (AWS Secrets Manager, Vault, etc.).
