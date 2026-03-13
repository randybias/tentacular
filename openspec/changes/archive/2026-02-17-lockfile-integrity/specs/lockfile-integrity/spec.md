## ADDED Requirements

### Requirement: Dockerfile copies deno.lock into image
The generated Dockerfile SHALL include a `COPY deno.lock /app/deno.lock` instruction before the `deno cache` step.

#### Scenario: deno.lock is copied into the image
- **WHEN** `GenerateDockerfile` is called
- **THEN** the output SHALL contain `COPY deno.lock /app/deno.lock`

### Requirement: Build-time cache uses lock file for integrity verification
The `deno cache` RUN instruction SHALL use `--lock=deno.lock` instead of `--no-lock`.

#### Scenario: deno cache uses lock flag
- **WHEN** `GenerateDockerfile` is called
- **THEN** the `deno cache` instruction SHALL contain `--lock=deno.lock`
- **AND** the `deno cache` instruction SHALL NOT contain `--no-lock`

### Requirement: Runtime ENTRYPOINT keeps no-lock
The runtime ENTRYPOINT SHALL continue to use `--no-lock` since the cache is already verified and the filesystem is read-only.

#### Scenario: Runtime entrypoint uses no-lock
- **WHEN** `GenerateDockerfile` is called
- **THEN** the ENTRYPOINT instruction SHALL contain `--no-lock`
