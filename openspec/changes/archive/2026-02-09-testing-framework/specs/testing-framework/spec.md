## ADDED Requirements

### Requirement: Node-level testing with fixtures
The test runner SHALL load fixture files from `tests/fixtures/<nodename>*.json`, import the corresponding node module, execute it with a mock context and the fixture input, and compare the output against the expected value if provided.

#### Scenario: Node test with matching expected output
- **GIVEN** a workflow with a node named "transform" and a fixture file `tests/fixtures/transform.json` containing `{ "input": {"x": 1}, "expected": {"x": 2} }`
- **WHEN** the test runner is invoked with `--node transform`
- **THEN** the node function SHALL be called with the fixture input
- **AND** the output SHALL be compared to the expected value
- **AND** the test SHALL pass if they match

#### Scenario: Node test with mismatched output
- **GIVEN** a fixture with expected output that does not match the actual output
- **WHEN** the test runner executes the node
- **THEN** the test SHALL fail with an error showing expected vs actual values

#### Scenario: Node test without expected field
- **GIVEN** a fixture file with `{ "input": {"x": 1} }` and no `expected` field
- **WHEN** the test runner executes the node
- **THEN** the test SHALL pass as long as the node does not throw an error

#### Scenario: Node not found in workflow
- **WHEN** the test runner is invoked with `--node nonexistent`
- **THEN** the test SHALL fail with an error indicating the node is not defined

#### Scenario: No fixtures found
- **GIVEN** a node with no matching fixture files in `tests/fixtures/`
- **WHEN** the test runner is invoked for that node
- **THEN** the result SHALL indicate no fixtures and count as passed (skip)

### Requirement: Pipeline testing with mock context
The test runner SHALL compile the full workflow DAG, load all node modules, and execute the complete pipeline using the SimpleExecutor with a mock context.

#### Scenario: Successful pipeline test
- **GIVEN** a valid workflow with all nodes loadable
- **WHEN** the test runner is invoked with `--pipeline`
- **THEN** the DAG SHALL be compiled and executed end-to-end
- **AND** the result SHALL report pass if all nodes succeed

#### Scenario: Pipeline test with node failure
- **GIVEN** a workflow where one node throws an error
- **WHEN** the test runner is invoked with `--pipeline`
- **THEN** the result SHALL report failure with the error details

### Requirement: Mock Context
The `createMockContext()` function SHALL return a `Context` object where `fetch` returns mock responses (default: `{ mock: true }`) and `log` captures log entries without printing to console.

#### Scenario: Mock fetch returns default response
- **WHEN** a node calls `ctx.fetch("service", "/path")` during a test
- **THEN** the response SHALL have status 200 and body `{ "mock": true, "service": "service", "path": "/path" }`

#### Scenario: Mock context accepts overrides
- **WHEN** `createMockContext({ config: { key: "value" } })` is called
- **THEN** the returned context SHALL have `config.key` equal to `"value"`

### Requirement: Test fixture format
Test fixtures SHALL be JSON files with the structure `{ "input": <any>, "expected"?: <any> }`.

#### Scenario: Valid fixture
- **GIVEN** a file `tests/fixtures/mynode.json` containing `{ "input": { "name": "test" }, "expected": { "greeting": "hello test" } }`
- **WHEN** the fixture is loaded
- **THEN** `fixture.input` SHALL equal `{ "name": "test" }`
- **AND** `fixture.expected` SHALL equal `{ "greeting": "hello test" }`

#### Scenario: Fixture discovery
- **GIVEN** files `tests/fixtures/mynode.json` and `tests/fixtures/mynode-edge.json`
- **WHEN** fixtures are discovered for node "mynode"
- **THEN** both files SHALL be found and returned

### Requirement: CLI test command
The `pipedreamer test [dir][/<node>]` command SHALL spawn `deno run engine/testing/runner.ts` with the appropriate flags.

#### Scenario: Test all nodes
- **WHEN** `pipedreamer test .` is executed in a workflow directory
- **THEN** the CLI SHALL invoke the Deno test runner with `--workflow workflow.yaml`

#### Scenario: Test specific node
- **WHEN** `pipedreamer test myworkflow/fetch-data` is executed
- **THEN** the CLI SHALL invoke the Deno test runner with `--workflow myworkflow/workflow.yaml --node fetch-data`

#### Scenario: Pipeline test
- **WHEN** `pipedreamer test . --pipeline` is executed
- **THEN** the CLI SHALL invoke the Deno test runner with `--workflow workflow.yaml --pipeline`

#### Scenario: Missing workflow.yaml
- **WHEN** `pipedreamer test /nonexistent` is executed
- **THEN** the command SHALL fail with an error indicating no workflow.yaml was found

### Requirement: Test report output
The test runner SHALL output a structured report showing pass/fail status and timing for each test.

#### Scenario: Report format
- **WHEN** tests complete
- **THEN** output SHALL show each test with a pass/fail indicator, test name, and duration in milliseconds
- **AND** a summary line showing `N/M tests passed`

#### Scenario: Exit code on failure
- **WHEN** any test fails
- **THEN** the process SHALL exit with code 1

#### Scenario: Exit code on success
- **WHEN** all tests pass
- **THEN** the process SHALL exit with code 0
