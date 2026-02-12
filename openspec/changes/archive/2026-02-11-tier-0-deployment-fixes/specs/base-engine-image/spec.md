## MODIFIED Requirements

### Requirement: Dockerfile caches engine dependencies at build time
The generated Dockerfile SHALL pre-cache engine dependencies at build time via `deno cache` with the `--no-lock` flag.

#### Scenario: Engine deps cached with --no-lock
- **WHEN** `GenerateDockerfile()` is called
- **THEN** the Dockerfile SHALL contain a `RUN` instruction that caches `engine/main.ts` dependencies
- **AND** the `deno cache` command SHALL include the `--no-lock` flag

#### Scenario: Cache command matches ENTRYPOINT lock behavior
- **WHEN** `GenerateDockerfile()` is called
- **THEN** both the `deno cache` RUN instruction and the `deno run` ENTRYPOINT SHALL include `--no-lock`
- **AND** neither SHALL reference a lock file
