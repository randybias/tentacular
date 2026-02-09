## 1. Test Fixture System

- [x] 1.1 Create `engine/testing/fixtures.ts` with `TestFixture` interface (`{ input, expected? }`)
- [x] 1.2 Implement `loadFixture(path)` to read and parse JSON fixture files
- [x] 1.3 Implement `findFixtures(testDir, nodeName)` to discover fixture files matching node name prefix

## 2. Mock Context

- [x] 2.1 Create `engine/testing/mocks.ts` with `createMockContext(overrides?)` returning a `Context`
- [x] 2.2 Implement mock `fetch` that returns default JSON responses and supports pre-configured responses
- [x] 2.3 Implement mock `log` that captures log entries to an array
- [x] 2.4 Add `mockFetchResponse(body, status)` helper for creating mock Response objects

## 3. Test Runner

- [x] 3.1 Create `engine/testing/runner.ts` with CLI flag parsing (`--workflow`, `--node`, `--pipeline`)
- [x] 3.2 Implement `runNodeTests(nodeName)` — load fixtures, import node, execute with mock context, compare outputs
- [x] 3.3 Implement `runPipelineTest()` — compile DAG, load all nodes, execute through SimpleExecutor
- [x] 3.4 Implement test result collection and report output (pass/fail indicator, timing, summary)
- [x] 3.5 Default mode: run all nodes when no `--node` or `--pipeline` flag given

## 4. CLI Integration

- [x] 4.1 Implement `pkg/cli/test.go` `NewTestCmd()` with cobra command and `--pipeline` flag
- [x] 4.2 Implement `runTest()` — parse `[dir][/<node>]` argument, resolve workflow path, spawn Deno runner
- [x] 4.3 Support `pipedreamer test .` (all nodes), `pipedreamer test dir/node` (specific node), `pipedreamer test . --pipeline`

## 5. Verification

- [x] 5.1 Create a temporary test workflow with a simple node and fixture
- [x] 5.2 Run `pipedreamer test` and verify pass/fail output format
- [x] 5.3 Run `pipedreamer test <workflow>/<node>` for single-node testing
- [x] 5.4 Run `pipedreamer test --pipeline` for full DAG testing
- [x] 5.5 Verify exit code 0 on all-pass, exit code 1 on failure
