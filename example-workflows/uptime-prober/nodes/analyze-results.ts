import type { Context } from "tentacular";

interface ProbeResult {
  url: string;
  status: number;
  ok: boolean;
  latencyMs: number;
  error?: string;
}

interface AnalysisOutput {
  alert: boolean;
  probedAt: string;
  totalEndpoints: number;
  healthyCount: number;
  unhealthyCount: number;
  unhealthy: ProbeResult[];
  healthy: ProbeResult[];
}

/** Analyze probe results — flag alert if any endpoint is down */
export default async function run(ctx: Context, input: unknown): Promise<AnalysisOutput> {
  const data = input as { results: ProbeResult[]; probedAt: string };
  const results = data.results;

  const healthy = results.filter((r) => r.ok);
  const unhealthy = results.filter((r) => !r.ok);

  const alert = unhealthy.length > 0;

  if (alert) {
    ctx.log.warn(`${unhealthy.length}/${results.length} endpoint(s) DOWN`);
    for (const u of unhealthy) {
      ctx.log.warn(`  DOWN: ${u.url} — status=${u.status} ${u.error ?? ""}`);
    }
  } else {
    ctx.log.info(`All ${results.length} endpoint(s) healthy`);
  }

  return {
    alert,
    probedAt: data.probedAt,
    totalEndpoints: results.length,
    healthyCount: healthy.length,
    unhealthyCount: unhealthy.length,
    unhealthy,
    healthy,
  };
}
