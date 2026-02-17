import type { Context } from "tentacular";

interface HealthRecord {
  id: number;
  collectedAt: string;
  totalNodes: number;
  readyNodes: number;
  totalPods: number;
  healthyPods: number;
  problemPods: number;
  nodesJson: unknown;
  problemPodsJson: unknown;
  namespacesJson: unknown;
}

interface HealthHistory {
  records: HealthRecord[];
  periodStart: string;
  periodEnd: string;
  snapshotCount: number;
}

export interface HealthAnalysis {
  periodStart: string;
  periodEnd: string;
  snapshotCount: number;
  aiSummary: string;
  stats: {
    avgHealthyPodPct: number;
    minHealthyPodPct: number;
    maxProblemPods: number;
    nodeDownEvents: number;
    totalPodRange: { min: number; max: number };
  };
}

/** Analyze health trends using Claude and basic statistics */
export default async function run(ctx: Context, input: unknown): Promise<HealthAnalysis> {
  const history = input as HealthHistory;

  if (history.records.length === 0) {
    ctx.log.warn("No health records found for analysis period");
    return {
      periodStart: history.periodStart,
      periodEnd: history.periodEnd,
      snapshotCount: 0,
      aiSummary: "No health data available for the last 24 hours. The collector may not be running.",
      stats: {
        avgHealthyPodPct: 0,
        minHealthyPodPct: 0,
        maxProblemPods: 0,
        nodeDownEvents: 0,
        totalPodRange: { min: 0, max: 0 },
      },
    };
  }

  // Compute basic stats
  const healthyPcts = history.records.map((r) =>
    r.totalPods > 0 ? (r.healthyPods / r.totalPods) * 100 : 100
  );
  const avgHealthyPodPct = healthyPcts.reduce((a, b) => a + b, 0) / healthyPcts.length;
  const minHealthyPodPct = Math.min(...healthyPcts);
  const maxProblemPods = Math.max(...history.records.map((r) => r.problemPods));
  const nodeDownEvents = history.records.filter((r) => r.readyNodes < r.totalNodes).length;
  const totalPods = history.records.map((r) => r.totalPods);

  const stats = {
    avgHealthyPodPct: Math.round(avgHealthyPodPct * 10) / 10,
    minHealthyPodPct: Math.round(minHealthyPodPct * 10) / 10,
    maxProblemPods,
    nodeDownEvents,
    totalPodRange: { min: Math.min(...totalPods), max: Math.max(...totalPods) },
  };

  ctx.log.info(`Stats: avg healthy=${stats.avgHealthyPodPct}%, node-down events=${nodeDownEvents}`);

  // Build a data summary for Claude
  const latest = history.records[history.records.length - 1];
  const earliest = history.records[0];

  // Sample problem pods from the most recent snapshot for context
  const recentProblems = latest.problemPodsJson as { namespace: string; name: string; phase: string; restarts: number }[];
  const problemSample = (recentProblems ?? []).slice(0, 10).map((p) =>
    `  ${p.namespace}/${p.name}: ${p.phase} (restarts: ${p.restarts})`
  ).join("\n");

  const prompt = `You are a Kubernetes operations expert analyzing cluster health data.

Period: ${history.periodStart} to ${history.periodEnd} (${history.snapshotCount} snapshots, every 5 minutes)

Summary statistics:
- Nodes: ${latest.totalNodes} total, ${stats.nodeDownEvents} snapshots with a node not ready
- Pods: ${stats.totalPodRange.min}-${stats.totalPodRange.max} total across period
- Healthy pod rate: avg ${stats.avgHealthyPodPct}%, min ${stats.minHealthyPodPct}%
- Peak problem pods: ${stats.maxProblemPods}

Current state (latest snapshot):
- ${latest.readyNodes}/${latest.totalNodes} nodes ready
- ${latest.healthyPods}/${latest.totalPods} pods healthy
- ${latest.problemPods} problem pods

${problemSample ? `Recent problem pods:\n${problemSample}` : "No problem pods in latest snapshot."}

Write a concise daily health report (3-5 bullet points). Flag any concerning trends. If everything looks healthy, say so briefly. Be specific with numbers. Do not use markdown headers.`;

  const anthropic = ctx.dependency("anthropic");
  if (!anthropic.secret) {
    ctx.log.warn("No anthropic.api_key in secrets — falling back to stats-only summary");
    const fallback = [
      `Period: ${history.periodStart} to ${history.periodEnd} (${history.snapshotCount} snapshots)`,
      `Nodes: ${latest.readyNodes}/${latest.totalNodes} ready, ${stats.nodeDownEvents} down events`,
      `Pods: avg ${stats.avgHealthyPodPct}% healthy, peak ${stats.maxProblemPods} problems`,
      `Current: ${latest.healthyPods}/${latest.totalPods} pods healthy`,
    ].join("\n");
    return { ...stats, periodStart: history.periodStart, periodEnd: history.periodEnd, snapshotCount: history.snapshotCount, aiSummary: fallback, stats };
  }

  ctx.log.info("Analyzing trends with Claude");

  const response = await anthropic.fetch!("/v1/messages", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "anthropic-version": "2023-06-01",
    },
    body: JSON.stringify({
      model: "claude-haiku-4-5-20251001",
      max_tokens: 512,
      messages: [{ role: "user", content: prompt }],
    }),
  });

  let aiSummary: string;
  if (!response.ok) {
    const errText = await response.text();
    ctx.log.error(`Anthropic API error: ${response.status} ${errText.substring(0, 200)}`);
    aiSummary = `[AI analysis unavailable — API returned ${response.status}]\n` +
      `Nodes: ${latest.readyNodes}/${latest.totalNodes} ready\n` +
      `Pods: ${latest.healthyPods}/${latest.totalPods} healthy, ${latest.problemPods} problems`;
  } else {
    const result = await response.json();
    aiSummary = ((result as Record<string, unknown[]>).content ?? [])
      .map((c: unknown) => (c as Record<string, string>).text)
      .join("\n");
  }

  ctx.log.info("Analysis complete");

  return {
    periodStart: history.periodStart,
    periodEnd: history.periodEnd,
    snapshotCount: history.snapshotCount,
    aiSummary,
    stats,
  };
}
