## Context

All trigger implementation is complete. Documentation needs to cover the full trigger model: manual, cron, queue, and webhook (roadmap).

## Goals / Non-Goals

**Goals:**
- Document all trigger types with clear examples
- Explain config block as open (arbitrary keys)
- Cover NATS connection setup and deployment
- Show CronJob lifecycle end-to-end

**Non-Goals:**
- Tutorial-style walkthroughs (keep as reference docs)
- Video or interactive docs

## Decisions

### Trigger table as primary reference
A single table summarizing all trigger types, their mechanism, required fields, K8s resources, and status. This appears in both architecture.md and SKILL.md.

### Roadmap as separate doc
Future enhancements (webhook bridge, JetStream, ConfigMap) go in a dedicated roadmap.md rather than cluttering architecture.md.

## Risks / Trade-offs

- Documentation may drift from implementation over time. Mitigation: keep docs close to code, reference spec types directly.
