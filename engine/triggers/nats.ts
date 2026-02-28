/**
 * NATS trigger manager for queue-type triggers.
 * Dynamically imports @nats-io/transport-deno so the library is only loaded
 * when queue triggers are actually configured.
 */

import type { CompiledDAG, Context, Trigger } from "../types.ts";
import type { NodeRunner } from "../executor/types.ts";
import { SimpleExecutor } from "../executor/simple.ts";
import type { TelemetrySink } from "../telemetry/mod.ts";
import { NoopSink } from "../telemetry/mod.ts";

export interface NATSTriggerOptions {
  /** NATS server URL (e.g. "nats.example.com:4222") */
  url: string;
  /** Authentication token */
  token: string;
  /** Queue triggers from the workflow spec */
  triggers: Trigger[];
  /** Compiled workflow DAG */
  graph: CompiledDAG;
  /** Node runner for executing workflows */
  runner: NodeRunner;
  /** Workflow context */
  ctx: Context;
  /** Execution timeout in ms */
  timeoutMs?: number;
  /** Max retries per node */
  maxRetries?: number;
  /** Telemetry sink for runtime observability (default: NoopSink) */
  sink?: TelemetrySink;
}

export interface NATSTriggerHandle {
  /** Drain all subscriptions and close the NATS connection */
  close(): Promise<void>;
}

/**
 * Validate NATS trigger options. Returns an error message if invalid, null if valid.
 */
export function validateOptions(opts: Partial<NATSTriggerOptions>): string | null {
  if (!opts.url || opts.url.trim() === "") {
    return "NATS URL is required (set config.nats_url in workflow.yaml)";
  }
  if (!opts.token || opts.token.trim() === "") {
    return "NATS token is required (set secrets.nats.token)";
  }
  if (!opts.triggers || opts.triggers.length === 0) {
    return "At least one queue trigger is required";
  }
  const missing = opts.triggers.filter((t) => !t.subject);
  if (missing.length > 0) {
    return "All queue triggers must have a subject";
  }
  return null;
}

/**
 * Start NATS trigger subscriptions for all queue triggers.
 * Dynamically imports the NATS library to avoid loading it when not needed.
 */
export async function startNATSTriggers(opts: NATSTriggerOptions): Promise<NATSTriggerHandle> {
  const validationError = validateOptions(opts);
  if (validationError) {
    throw new Error(validationError);
  }

  // Dynamic import — only loaded when queue triggers exist
  const { connect } = await import("@nats-io/transport-deno");

  const nc = await connect({
    servers: opts.url,
    token: opts.token,
    tls: {},
  });

  console.log(`NATS connected to ${opts.url}`);

  const sink: TelemetrySink = opts.sink ?? new NoopSink();

  const executor = new SimpleExecutor({
    timeoutMs: opts.timeoutMs,
    maxRetries: opts.maxRetries,
    sink,
  });

  // Subscribe to each queue trigger's subject
  for (const trigger of opts.triggers) {
    if (!trigger.subject) continue;

    const sub = nc.subscribe(trigger.subject);
    console.log(`  NATS subscribed to: ${trigger.subject}`);

    // Process messages in background (non-blocking)
    (async () => {
      for await (const msg of sub) {
        try {
          // Parse message payload as JSON input
          let input: unknown = {};
          const payload = msg.data;
          if (payload && payload.length > 0) {
            try {
              const text = new TextDecoder().decode(payload);
              input = JSON.parse(text);
            } catch {
              // Non-JSON payload — wrap as { data: "<raw>" }
              input = { data: new TextDecoder().decode(payload) };
            }
          }

          // Add trigger metadata
          if (typeof input === "object" && input !== null) {
            (input as Record<string, unknown>).trigger = trigger.name ?? trigger.subject;
          }

          console.log(`NATS message on ${trigger.subject} — executing workflow`);
          sink.record({ type: "nats-message", timestamp: Date.now(), metadata: { subject: trigger.subject } });
          const result = await executor.execute(opts.graph, opts.runner, opts.ctx, input);

          // Request-reply: send result back if reply subject is set
          if (msg.reply) {
            const resultBytes = new TextEncoder().encode(JSON.stringify(result));
            msg.respond(resultBytes);
          }

          if (!result.success) {
            console.error(`NATS-triggered execution failed on ${trigger.subject}:`, result.errors);
          }
        } catch (err) {
          console.error(`Error processing NATS message on ${trigger.subject}:`, err);
        }
      }
    })();
  }

  return {
    async close() {
      console.log("NATS draining subscriptions...");
      await nc.drain();
      console.log("NATS connection closed");
    },
  };
}
