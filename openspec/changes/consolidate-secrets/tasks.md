# Tasks

## Implementation

- [ ] Create `.secrets.yaml.example` at repo root documenting shared secrets
- [ ] Update example workflow `.secrets.yaml.example` files to use `$shared.<name>` references
- [ ] Add `--shared` flag to `tntc secrets init` in `pkg/cli/secrets.go`
- [ ] Implement shared secrets initialization (create `.secrets/` from `.secrets.yaml.example`)
- [ ] Update `tntc secrets check` to validate shared secret availability
- [ ] Ensure `.secrets/` directory is in `.gitignore`

## Testing

- [ ] Test `tntc secrets init --shared` creates `.secrets/` directory with correct files
- [ ] Test `tntc secrets check` reports missing shared secrets
- [ ] Test `$shared.<name>` resolution still works in `resolveSharedSecrets()`
- [ ] Test backwards compatibility: workflows without `$shared` references work as before
- [ ] `go build ./...` passes
- [ ] `go test -count=1 ./...` passes
