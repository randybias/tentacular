import type { Context } from "tentacular";

interface ProbeResult {
  url: string;
  status: number;
  ok: boolean;
  latencyMs: number;
  error?: string;
}

/** Probe each configured endpoint and collect status + latency */
export default async function run(ctx: Context, _input: unknown): Promise<{ results: ProbeResult[]; probedAt: string }> {
  const endpoints = ctx.config.endpoints as string[] | undefined;
  if (!endpoints || endpoints.length === 0) {
    ctx.log.warn("No endpoints configured in config.endpoints");
    return { results: [], probedAt: new Date().toISOString() };
  }

  ctx.log.info(`Probing ${endpoints.length} endpoint(s)`);
  const results: ProbeResult[] = [];

  for (const url of endpoints) {
    const start = Date.now();
    try {
      const resp = await ctx.fetch("probe", url, { method: "GET" });
      const latencyMs = Date.now() - start;
      results.push({
        url,
        status: resp.status,
        ok: resp.ok,
        latencyMs,
      });
      // Consume the body to free resources
      await resp.text();
      ctx.log.info(`${url} => ${resp.status} (${latencyMs}ms)`);
    } catch (err) {
      const latencyMs = Date.now() - start;
      const message = err instanceof Error ? err.message : String(err);
      results.push({
        url,
        status: 0,
        ok: false,
        latencyMs,
        error: message,
      });
      ctx.log.error(`${url} => ERROR: ${message} (${latencyMs}ms)`);
    }
  }

  return { results, probedAt: new Date().toISOString() };
}
