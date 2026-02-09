# Pipedreamer v2 Roadmap

## Webhook Triggers via NATS Bridge

A single gateway workflow subscribes to HTTP webhooks and publishes events to NATS subjects. Downstream workflows subscribe to those subjects via queue triggers. This avoids per-workflow ingress configuration and centralizes webhook handling.

```
GitHub Webhook → Gateway Workflow → NATS publish("events.github.push") → Queue Trigger Workflows
```

Benefits: no per-workflow Ingress resources, single TLS termination point, centralized webhook verification.

## ConfigMap-Mounted Runtime Config Overrides

Mount a K8s ConfigMap at `/app/config` to override workflow config values at runtime without rebuilding the container. The engine merges ConfigMap values on top of workflow.yaml config.

## NATS JetStream Durable Subscriptions

Upgrade from core NATS (at-most-once) to JetStream (at-least-once delivery) for queue triggers. This provides:

- **Durable subscriptions**: messages persist if the workflow engine is offline
- **Acknowledgment**: messages are redelivered if not acknowledged within a timeout
- **Replay**: ability to replay historical messages for debugging or reprocessing

## Message Payload Passthrough as Workflow Input

Currently queue trigger messages are parsed as JSON and passed to root nodes. Future enhancement: support binary payloads, content-type negotiation, and schema validation for incoming messages.

## Rate Limiting / Concurrency Control for Queue Triggers

Add configurable concurrency limits for NATS-triggered executions:

- **Max concurrent executions**: prevent resource exhaustion from message bursts
- **Rate limiting**: token bucket or sliding window rate limiting
- **Backpressure**: slow down NATS subscription when at capacity

## Dead Letter Queue for Failed Executions

Failed NATS-triggered executions publish the original message to a configurable dead letter subject (e.g., `{subject}.dlq`). This enables:

- Retry from DLQ after fixing issues
- Alerting on DLQ depth
- Forensic analysis of failed messages

## Multi-Cluster Deployment

Support deploying workflows across multiple K8s clusters with a single command. The CLI discovers available clusters from kubeconfig contexts and generates manifests for each.

## Workflow Versioning and Canary Deploys

Support running multiple versions of a workflow simultaneously with traffic splitting. CronJobs and NATS subscriptions route to the active version. Canary deploys send a percentage of traffic to the new version.
