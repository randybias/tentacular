## 1. Add Deployment Namespace to workflow.yaml Files

- [x] 1.1 Add `deployment: { namespace: pd-cluster-health }` to `example-workflows/cluster-health-collector/workflow.yaml`
- [x] 1.2 Add `deployment: { namespace: pd-cluster-health }` to `example-workflows/cluster-health-reporter/workflow.yaml`
- [x] 1.3 Add `deployment: { namespace: pd-sep-tracker }` to `example-workflows/sep-tracker/workflow.yaml`
- [x] 1.4 Add `deployment: { namespace: pd-uptime-prober }` to `example-workflows/uptime-prober/workflow.yaml`

## 2. Create Missing .secrets.yaml.example Files

- [x] 2.1 Create `example-workflows/github-digest/.secrets.yaml.example` with `slack: { webhook_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL" }`
- [x] 2.2 Create `example-workflows/pr-digest/.secrets.yaml.example` with `github: { token: "ghp_YOUR_TOKEN" }`, `anthropic: { api_key: "sk-ant-YOUR_KEY" }`, `slack: { webhook_url: "https://hooks.slack.com/services/YOUR/WEBHOOK/URL" }`

## 3. Add Secrets to Test Fixtures

- [x] 3.1 Add `"secrets": {"slack": {"webhook_url": "https://hooks.slack.com/test"}}` to `uptime-prober/tests/fixtures/notify-slack.json`
- [x] 3.2 Add `"secrets": {"slack": {"webhook_url": "https://hooks.slack.com/test"}}` to `github-digest/tests/fixtures/notify.json`
- [x] 3.3 Add `"secrets": {"slack": {"webhook_url": "https://hooks.slack.com/test"}}` to `pr-digest/tests/fixtures/notify-slack.json`
- [x] 3.4 Add `"secrets": {"slack": {"webhook_url": "https://hooks.slack.com/test"}}` to `sep-tracker/tests/fixtures/notify.json`
- [x] 3.5 Add `"secrets": {"postgres": {"password": "test_password"}, "azure": {"sas_token": "test_sas"}}` to `sep-tracker/tests/fixtures/store-report.json`
- [x] 3.6 Add `"secrets": {"postgres": {"password": "test_password"}}` to `sep-tracker/tests/fixtures/diff-seps.json`
- [x] 3.7 Add `"secrets": {"postgres": {"password": "test_password"}}` to `cluster-health-collector/tests/fixtures/store-health-data.json`
- [x] 3.8 Add `"secrets": {"postgres": {"password": "test_password"}}` to `cluster-health-reporter/tests/fixtures/query-health-history.json`
- [x] 3.9 Add `"secrets": {"slack": {"webhook_url": "https://hooks.slack.com/test"}}` to `cluster-health-reporter/tests/fixtures/send-report.json`
- [x] 3.10 Add `"secrets": {"anthropic": {"api_key": "sk-ant-test-key"}}` to `cluster-health-reporter/tests/fixtures/analyze-trends.json`

## 4. Verification

- [x] 4.1 Run `tntc validate` on all 8 example workflow directories -- all pass
- [x] 4.2 Run `tntc secrets check` on all example workflows with secrets -- verify accurate reporting
- [x] 4.3 Run engine test suite -- verify updated fixtures work with new TestFixture interface
