import type { Context } from "tentacular";

interface ProbeResult {
  url: string;
  status: number;
  ok: boolean;
  latencyMs: number;
  error?: string;
}

interface AnalysisInput {
  alert: boolean;
  probedAt: string;
  totalEndpoints: number;
  healthyCount: number;
  unhealthyCount: number;
  unhealthy: ProbeResult[];
  healthy: ProbeResult[];
}

interface ReportOutput {
  alert: boolean;
  probedAt: string;
  summary: string;
  sections: { down: string; healthy: string };
  totalEndpoints: number;
  unhealthyCount: number;
  healthyCount: number;
}

/** Clean up verbose Deno fetch error messages for display */
function cleanError(error: string): string {
  // "error sending request for url (https://...): client error (Connect): dns error: ..."
  // → "DNS error: ..."
  const dnsMatch = error.match(/dns error:\s*(.+)/i);
  if (dnsMatch) return `DNS: ${dnsMatch[1]}`;
  const connectMatch = error.match(/client error \(Connect\):\s*(.+)/i);
  if (connectMatch) return connectMatch[1];
  const tlsMatch = error.match(/client error \(SendRequest\):\s*(.+?)(?:\s*http|$)/i);
  if (tlsMatch) return `TLS: ${tlsMatch[1]}`;
  return error.length > 80 ? error.substring(0, 80) + "..." : error;
}

/** Format probe results into structured report data for Slack */
export default async function run(ctx: Context, input: unknown): Promise<ReportOutput> {
  const data = input as AnalysisInput;

  const downLines = data.unhealthy.map((r) => {
    const reason = r.error ? cleanError(r.error) : `HTTP ${r.status}`;
    return `*${r.url}*\n    ${reason} _(${r.latencyMs}ms)_`;
  }).join("\n");

  const healthyLines = data.healthy.map((r) =>
    `${r.url} — \`${r.status}\` _(${r.latencyMs}ms)_`
  ).join("\n");

  const summary = data.alert
    ? `⚠️ ${data.unhealthyCount} of ${data.totalEndpoints} endpoint(s) unreachable`
    : `✅ All ${data.totalEndpoints} endpoint(s) responding normally`;

  ctx.log.info(data.alert ? "Alert report generated" : "All-clear report generated");

  return {
    alert: data.alert,
    probedAt: data.probedAt,
    summary,
    sections: { down: downLines, healthy: healthyLines },
    totalEndpoints: data.totalEndpoints,
    unhealthyCount: data.unhealthyCount,
    healthyCount: data.healthyCount,
  };
}
