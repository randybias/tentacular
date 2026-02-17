import type { Context } from "tentacular";

interface Sep {
  number: number;
  sepId: string;
  title: string;
  state: string;
  author: string;
  createdAt: string;
  updatedAt: string;
  url: string;
  labels: string[];
  summary: string;
}

interface SepSnapshot {
  timestamp: string;
  repo: string;
  seps: Sep[];
  count: number;
}

interface StateTransition {
  sepId: string;
  from: string;
  to: string;
}

interface WeekOverWeekMetric {
  metric: string;
  thisWeek: number;
  lastWeek: number;
  delta: number;
}

interface ActivityMetrics {
  highVelocity: Sep[];
  inactive: Sep[];
  newThisWeek: Sep[];
  closedThisWeek: Sep[];
  stateTransitions: StateTransition[];
  totalActive: number;
  totalInactive: number;
  velocityScore: number;
  weekOverWeek: WeekOverWeekMetric[];
}

interface ReportSection {
  title: string;
  content: string;
}

interface LLMReport {
  analysis: string;
  recommendations: string[];
  healthScore: number;
  sections: ReportSection[];
}

interface FanInInput {
  "fetch-seps": SepSnapshot;
  "analyze-activity": ActivityMetrics;
}

const SYSTEM_PROMPT = `You are a technical analyst reviewing SEP (Specification Enhancement Proposal) activity for the Model Context Protocol specification repository.

Analyze the provided SEP data and activity metrics. Produce a structured JSON response with:
- "analysis": A 2-3 paragraph summary of the overall SEP ecosystem health, velocity trends, and notable activity.
- "recommendations": An array of 3-5 actionable recommendations for specification maintainers.
- "healthScore": An integer 0-100 representing overall ecosystem health based on:
  - Velocity score (are proposals actively moving?)
  - Inactivity ratio (are many proposals stalled?)
  - New proposal rate (is the community engaged?)
  - State transitions (are proposals progressing through stages?)
- "sections": An array of report sections, each with "title" and "content" fields. Include sections for:
  - "Velocity Assessment" - which SEPs are moving quickly and why
  - "Inactivity Flags" - which SEPs appear stalled and may need attention
  - "New Proposals" - overview of newly submitted SEPs
  - "State Changes" - notable transitions between states
  - "Week-over-Week Trends" - how metrics changed compared to last week

Respond with valid JSON only, no markdown fences.`;

function buildAnalysisPrompt(seps: SepSnapshot, metrics: ActivityMetrics): string {
  return JSON.stringify({
    snapshot: {
      repo: seps.repo,
      timestamp: seps.timestamp,
      totalSeps: seps.count,
      seps: seps.seps.map((s) => ({
        sepId: s.sepId,
        title: s.title,
        state: s.state,
        author: s.author,
        updatedAt: s.updatedAt,
        labels: s.labels,
      })),
    },
    metrics: {
      totalActive: metrics.totalActive,
      totalInactive: metrics.totalInactive,
      velocityScore: metrics.velocityScore,
      newThisWeek: metrics.newThisWeek.map((s) => s.sepId),
      closedThisWeek: metrics.closedThisWeek.map((s) => s.sepId),
      stateTransitions: metrics.stateTransitions,
      highVelocity: metrics.highVelocity.map((s) => ({
        sepId: s.sepId,
        title: s.title,
        updatedAt: s.updatedAt,
      })),
      inactive: metrics.inactive.map((s) => ({
        sepId: s.sepId,
        title: s.title,
        updatedAt: s.updatedAt,
      })),
      weekOverWeek: metrics.weekOverWeek,
    },
  });
}

function fallbackReport(metrics: ActivityMetrics): LLMReport {
  const sections: ReportSection[] = [
    {
      title: "Velocity Assessment",
      content: metrics.highVelocity.length > 0
        ? `${metrics.highVelocity.length} SEPs updated in the last 7 days: ${metrics.highVelocity.map((s) => s.sepId).join(", ")}.`
        : "No SEPs were updated in the last 7 days.",
    },
    {
      title: "Inactivity Flags",
      content: metrics.inactive.length > 0
        ? `${metrics.inactive.length} SEPs have not been updated in 4+ weeks: ${metrics.inactive.map((s) => s.sepId).join(", ")}.`
        : "No SEPs are flagged as inactive.",
    },
    {
      title: "New Proposals",
      content: metrics.newThisWeek.length > 0
        ? `${metrics.newThisWeek.length} new SEPs this week: ${metrics.newThisWeek.map((s) => `${s.sepId} (${s.title})`).join("; ")}.`
        : "No new SEPs this week.",
    },
    {
      title: "State Changes",
      content: metrics.stateTransitions.length > 0
        ? metrics.stateTransitions.map((t) => `${t.sepId}: ${t.from} -> ${t.to}`).join("; ")
        : "No state transitions this week.",
    },
    {
      title: "Week-over-Week Trends",
      content: metrics.weekOverWeek
        .map((w) => `${w.metric}: ${w.lastWeek} -> ${w.thisWeek} (${w.delta >= 0 ? "+" : ""}${w.delta})`)
        .join("; "),
    },
  ];

  const total = metrics.totalActive + metrics.totalInactive;
  const healthScore = total > 0
    ? Math.round(
        (metrics.velocityScore * 0.4) +
        ((1 - metrics.totalInactive / total) * 100 * 0.3) +
        (Math.min(metrics.newThisWeek.length * 10, 100) * 0.2) +
        (Math.min(metrics.stateTransitions.length * 20, 100) * 0.1),
      )
    : 0;

  return {
    analysis: `Stats-only report (no LLM available). Velocity score: ${metrics.velocityScore}. ` +
      `${metrics.totalActive} active, ${metrics.totalInactive} inactive SEPs. ` +
      `${metrics.newThisWeek.length} new and ${metrics.closedThisWeek.length} closed this week.`,
    recommendations: [
      metrics.inactive.length > 0
        ? `Review ${metrics.inactive.length} inactive SEPs for possible closure or revival.`
        : "All SEPs are actively maintained.",
      metrics.velocityScore < 50
        ? "Consider triaging SEPs to improve overall velocity."
        : "Velocity is healthy, maintain current review cadence.",
    ],
    healthScore,
    sections,
  };
}

/** Generate an LLM-powered weekly digest report for SEP activity */
export default async function run(ctx: Context, input: unknown): Promise<LLMReport> {
  const fanIn = input as FanInInput;
  const seps = fanIn["fetch-seps"];
  const metrics = fanIn["analyze-activity"];

  const openai = ctx.dependency("openai");

  if (!openai.secret) {
    ctx.log.warn("No openai.api_key, falling back to stats-only report");
    return fallbackReport(metrics);
  }

  ctx.log.info("Generating LLM report via OpenAI GPT-5.2");

  const response = await openai.fetch!("/v1/chat/completions", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${openai.secret}`,
    },
    body: JSON.stringify({
      model: "gpt-5.2",
      response_format: { type: "json_object" },
      messages: [
        { role: "system", content: SYSTEM_PROMPT },
        { role: "user", content: buildAnalysisPrompt(seps, metrics) },
      ],
      max_completion_tokens: 4096,
    }),
  });

  if (!response.ok) {
    ctx.log.warn(`OpenAI API returned ${response.status}, falling back to stats-only report`);
    return fallbackReport(metrics);
  }

  const data = await response.json();
  const content = data?.choices?.[0]?.message?.content;

  if (!content) {
    ctx.log.warn("Empty response from OpenAI, falling back to stats-only report");
    return fallbackReport(metrics);
  }

  try {
    const parsed = JSON.parse(content) as LLMReport;

    // Validate required fields
    if (
      typeof parsed.analysis !== "string" ||
      !Array.isArray(parsed.recommendations) ||
      typeof parsed.healthScore !== "number" ||
      !Array.isArray(parsed.sections)
    ) {
      ctx.log.warn("LLM response missing required fields, falling back to stats-only report");
      return fallbackReport(metrics);
    }

    ctx.log.info(`LLM report generated: healthScore=${parsed.healthScore}, ${parsed.sections.length} sections`);
    return parsed;
  } catch {
    ctx.log.warn("Failed to parse LLM response as JSON, falling back to stats-only report");
    return fallbackReport(metrics);
  }
}
