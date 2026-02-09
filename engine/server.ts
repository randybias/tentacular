import type { CompiledDAG, Context, ExecutionResult } from "./types.ts";
import type { NodeRunner } from "./executor/types.ts";
import { SimpleExecutor } from "./executor/simple.ts";

export interface ServerOptions {
  port: number;
  graph: CompiledDAG;
  runner: NodeRunner;
  ctx: Context;
  timeoutMs?: number;
  maxRetries?: number;
}

/**
 * Start the HTTP trigger server for a workflow.
 * Exposes POST /run to trigger workflow execution.
 */
export function startServer(opts: ServerOptions): Deno.HttpServer {
  const executor = new SimpleExecutor({
    timeoutMs: opts.timeoutMs,
    maxRetries: opts.maxRetries,
  });

  const handler = async (req: Request): Promise<Response> => {
    const url = new URL(req.url);

    if (url.pathname === "/health") {
      return new Response(JSON.stringify({ status: "ok" }), {
        headers: { "content-type": "application/json" },
      });
    }

    if (url.pathname === "/run" && (req.method === "POST" || req.method === "GET")) {
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

        return new Response(JSON.stringify(result, null, 2), {
          status: result.success ? 200 : 500,
          headers: { "content-type": "application/json" },
        });
      } catch (err) {
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
  console.log(`  POST /run    — trigger workflow execution`);
  console.log(`  GET  /health — health check`);

  return server;
}
