## Context

NATS runs on the host at `nats.ospo-dev.miralabs.dev:18453`. TLS with Let's Encrypt cert (trusted by system CA store). Token auth required. The engine needs to subscribe to NATS subjects for queue triggers and execute the workflow when messages arrive.

## Goals / Non-Goals

**Goals:**
- Engine subscribes to NATS subjects when queue triggers are defined
- Messages trigger workflow execution with message payload as input
- Support NATS request-reply (send result back if `msg.reply` set)
- Graceful shutdown: drain NATS connections on SIGTERM/SIGINT
- Dynamic import: NATS library only loaded when queue triggers exist
- Graceful degradation: warn and skip if nats_url or nats.token missing

**Non-Goals:**
- JetStream durable subscriptions (future enhancement)
- Dead letter queue for failed executions
- Rate limiting / concurrency control

## Decisions

### Dynamic import of NATS library
Only import `@nats-io/transport-deno` when queue triggers exist. This avoids loading the NATS library for workflows that don't use queue triggers.

### Config-driven connection
NATS URL from `config.nats_url` (via opened config block from Phase 1), token from `secrets.nats.token`. Both are optional — engine warns and skips NATS if either is missing.

### Request-reply support
If a NATS message has a `reply` subject set, the workflow result is sent back. This enables synchronous request-reply patterns over NATS.

### Signal-based graceful shutdown
Register SIGTERM/SIGINT handlers that drain NATS subscriptions before exiting. This ensures in-flight messages complete processing.

## Risks / Trade-offs

- **NATS library stability**: `@nats-io/transport-deno@3.3.0` is the latest on JSR. Core API (connect, subscribe, drain) is stable.
- **TLS**: Let's Encrypt cert on the NATS server. Valid through Mar 31 2026. System trust store handles it — no special config.
- **At-most-once delivery**: Core NATS subscriptions are at-most-once. JetStream (at-least-once) is a future enhancement.
