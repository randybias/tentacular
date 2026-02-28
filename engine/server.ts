import type { CompiledDAG, Context, ExecutionResult } from "./types.ts";
import type { NodeRunner } from "./executor/types.ts";
import { SimpleExecutor } from "./executor/simple.ts";
import { handleGitHubWebhook, validateOptions as validateWebhookOptions } from "./triggers/webhook.ts";
import type { TelemetrySink } from "./telemetry/mod.ts";
import { NoopSink } from "./telemetry/mod.ts";

export interface ServerOptions {
  port: number;
  graph: CompiledDAG;
  runner: NodeRunner;
  ctx: Context;
  timeoutMs?: number;
  maxRetries?: number;
  /** GitHub webhook secret for HMAC-SHA256 signature validation */
  webhookSecret?: string;
  /** Telemetry sink for runtime observability (default: NoopSink) */
  sink?: TelemetrySink;
}

/**
 * Start the HTTP trigger server for a workflow.
 * Exposes POST /run to trigger workflow execution.
 */
export function startServer(opts: ServerOptions): Deno.HttpServer {
  // Validate webhook triggers if any are configured
  const webhookTriggers = (opts.graph.workflow.triggers ?? []).filter((t) => t.type === "webhook");
  if (webhookTriggers.length > 0) {
    const err = validateWebhookOptions({ triggers: webhookTriggers });
    if (err) throw new Error(`Webhook trigger configuration error: ${err}`);
  }

  const sink: TelemetrySink = opts.sink ?? new NoopSink();

  const executor = new SimpleExecutor({
    timeoutMs: opts.timeoutMs,
    maxRetries: opts.maxRetries,
    sink,
  });

  const handler = async (req: Request): Promise<Response> => {
    const url = new URL(req.url);

    if (url.pathname === "/health") {
      if (url.searchParams.get("detail") === "1") {
        // snapshot() already includes status: "ok"
        return new Response(JSON.stringify(sink.snapshot(), null, 2), {
          headers: { "content-type": "application/json" },
        });
      }
      return new Response(JSON.stringify({ status: "ok" }), {
        headers: { "content-type": "application/json" },
      });
    }

    // Webhook trigger: POST /webhook/:provider (exact match only)
    const webhookMatch = url.pathname.match(/^\/webhook\/([a-z][a-z0-9-]*)$/);
    if (webhookMatch && req.method === "POST") {
      const provider = webhookMatch[1];
      const webhookTriggers = (opts.graph.workflow.triggers ?? []).filter(
        (t) => t.type === "webhook",
      );

      if (webhookTriggers.length === 0) {
        return new Response(JSON.stringify({ error: "No webhook triggers configured" }), {
          status: 404,
          headers: { "content-type": "application/json" },
        });
      }

      if (provider === "github") {
        return await handleGitHubWebhook(req, {
          triggers: webhookTriggers,
          graph: opts.graph,
          runner: opts.runner,
          ctx: opts.ctx,
          executor, // shared — created once at server startup
          secret: opts.webhookSecret,
        });
      }

      return new Response(JSON.stringify({ error: `Unknown webhook provider: ${provider}` }), {
        status: 400,
        headers: { "content-type": "application/json" },
      });
    }

    if (url.pathname === "/run" && (req.method === "POST" || req.method === "GET")) {
      sink.record({ type: "request-in", timestamp: Date.now(), metadata: { path: "/run" } });
      try {
        // Parse POST body as initial input for root nodes
        let input: unknown = {};
        if (req.method === "POST") {
          try {
            const body = await req.text();
            if (body.trim()) {
              input = JSON.parse(body);
            }
          } catch {
            // Invalid JSON or empty body — use default empty object
          }
        }

        const result = await executor.execute(opts.graph, opts.runner, opts.ctx, input);
        sink.record({ type: "request-out", timestamp: Date.now(), metadata: { path: "/run" } });

        return new Response(JSON.stringify(result, null, 2), {
          status: result.success ? 200 : 500,
          headers: { "content-type": "application/json" },
        });
      } catch (err) {
        // Always emit request-out on unhandled throw so inFlight does not stick.
        sink.record({ type: "request-out", timestamp: Date.now(), metadata: { path: "/run" } });
        const msg = err instanceof Error ? err.message : String(err);
        return new Response(JSON.stringify({ error: msg }), {
          status: 500,
          headers: { "content-type": "application/json" },
        });
      }
    }

    return new Response("Not Found", { status: 404 });
  };

  const server = Deno.serve({ port: opts.port }, handler);
  console.log(`Workflow server listening on http://localhost:${opts.port}`);
  console.log(`  POST /run              — trigger workflow execution`);
  console.log(`  GET  /health           — health check`);
  console.log(`  GET  /health?detail=1  — telemetry snapshot`);
  const hasWebhookTrigger = (opts.graph.workflow.triggers ?? []).some((t) => t.type === "webhook");
  if (hasWebhookTrigger) {
    console.log(`  POST /webhook/github   — GitHub webhook receiver`);
  }

  return server;
}
