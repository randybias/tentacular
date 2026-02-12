## MODIFIED Requirements

### Requirement: Test fixture format
Test fixtures SHALL be JSON files with the structure `{ "input": <any>, "config"?: <record>, "secrets"?: <record>, "expected"?: <any> }`.

#### Scenario: Valid fixture with config and secrets
- **GIVEN** a fixture file containing `{ "input": {}, "config": {"timeout": "30s"}, "secrets": {"slack": {"webhook_url": "https://hooks.slack.com/test"}}, "expected": {} }`
- **WHEN** the fixture is loaded
- **THEN** `fixture.input` SHALL be `{}`
- **AND** `fixture.config` SHALL be `{"timeout": "30s"}`
- **AND** `fixture.secrets` SHALL be `{"slack": {"webhook_url": "https://hooks.slack.com/test"}}`

#### Scenario: Fixture without config/secrets (backwards compatible)
- **GIVEN** a fixture file containing `{ "input": {"x": 1}, "expected": {"x": 2} }`
- **WHEN** the fixture is loaded
- **THEN** `fixture.config` SHALL be undefined
- **AND** `fixture.secrets` SHALL be undefined
- **AND** the test runner SHALL use empty objects as defaults

#### Scenario: Fixture discovery
- **GIVEN** files `tests/fixtures/mynode.json` and `tests/fixtures/mynode-edge.json`
- **WHEN** fixtures are discovered for node "mynode"
- **THEN** both files SHALL be found and returned

### Requirement: Node-level testing with fixtures
The test runner SHALL load fixture files from `tests/fixtures/<nodename>*.json`, import the corresponding node module, execute it with a mock context and the fixture input, and compare the output against the expected value if provided.

#### Scenario: Node test with fixture config/secrets
- **GIVEN** a fixture with `config` and `secrets` fields
- **WHEN** the test runner executes the node
- **THEN** `createMockContext()` SHALL be called with `{ config: fixture.config, secrets: fixture.secrets }`
- **AND** the node function SHALL receive a context with those config and secrets values

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
