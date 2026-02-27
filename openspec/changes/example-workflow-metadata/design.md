## Context

The `sep-tracker` workflow is a production-grade example with multiple triggers (manual + cron), external dependencies, and a real deployment namespace. It is a practical candidate for demonstrating metadata because it represents a realistic operational workflow.

## Goals / Non-Goals

**Goals:**
- Add a complete `metadata:` section to `sep-tracker/workflow.yaml` showcasing all fields
- Serve as copy-paste template for users adding metadata to their own workflows

**Non-Goals:**
- Adding metadata to all example workflows (one example is sufficient for now)
- Changing any functional behavior of the workflow

## Decisions

### Use sep-tracker as the example
It is a production-like workflow with cron triggers and a real deployment namespace, making the metadata (owner, team, environment) feel natural and realistic. Alternative: hn-digest -- simpler but metadata like "team" and "environment" feel forced.

### Include all metadata fields
Even though all fields are optional, the example should show them all to document what is available.

## Risks / Trade-offs

- **None significant** -- purely additive YAML change to an example file.
