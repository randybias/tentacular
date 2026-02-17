import type { Context } from "tentacular";

interface RepoMetrics {
  repo: string;
  stars: number;
  forks: number;
  openIssues: number;
  openPRs: number;
  recentCommits: number;
  releaseCount: number;
  latestRelease: string;
  topLanguage: string;
  contributorCount: number;
  weeklyCommitTrend: number[];
  velocity: number;
}

interface CompositeMetrics {
  repos: RepoMetrics[];
  rankings: {
    byStars: string[];
    byVelocity: string[];
    byCommits: string[];
    byIssues: string[];
  };
  totals: {
    totalStars: number;
    totalForks: number;
    totalOpenIssues: number;
    totalRecentCommits: number;
  };
  generatedAt: string;
}

interface LLMAnalysis {
  narrative: string;
  highlights: string[];
  prediction: string;
  healthScores: Record<string, number>;
}

const SYSTEM_PROMPT = `You are a technical analyst comparing open-source AI agent projects.

Analyze the provided metrics for 3 AI agent repositories (Codex, Gemini CLI, Goose). Produce a structured JSON response with:
- "narrative": A 2-3 paragraph comparative analysis covering community engagement, development velocity, and ecosystem maturity.
- "highlights": An array of 3-5 key findings (strings).
- "prediction": A single sentence predicting which project has the most momentum going forward.
- "healthScores": An object mapping each repo name to a 0-100 health score based on stars, velocity, contributor count, issue activity, and release cadence.

Respond with valid JSON only, no markdown fences.`;

function buildUserPrompt(metrics: CompositeMetrics): string {
  return JSON.stringify({
    repos: metrics.repos.map((r) => ({
      repo: r.repo,
      stars: r.stars,
      forks: r.forks,
      openIssues: r.openIssues,
      openPRs: r.openPRs,
      recentCommits: r.recentCommits,
      releaseCount: r.releaseCount,
      latestRelease: r.latestRelease,
      topLanguage: r.topLanguage,
      contributorCount: r.contributorCount,
      velocity: r.velocity,
      weeklyCommitTrend: r.weeklyCommitTrend,
    })),
    rankings: metrics.rankings,
    totals: metrics.totals,
    generatedAt: metrics.generatedAt,
  });
}

function fallbackAnalysis(metrics: CompositeMetrics): LLMAnalysis {
  const sorted = [...metrics.repos].sort((a, b) => b.velocity - a.velocity);
  const top = sorted[0];

  const healthScores: Record<string, number> = {};
  for (const repo of metrics.repos) {
    const starScore = Math.min(repo.stars / 500, 1) * 25;
    const velocityScore = Math.min(repo.velocity / 100, 1) * 30;
    const contributorScore = Math.min(repo.contributorCount / 50, 1) * 20;
    const releaseScore = Math.min(repo.releaseCount / 10, 1) * 15;
    const issueScore = Math.min(repo.openPRs / 20, 1) * 10;
    healthScores[repo.repo] = Math.round(
      starScore + velocityScore + contributorScore + releaseScore + issueScore,
    );
  }

  const narrative =
    `Stats-only analysis (no LLM available). Across ${metrics.repos.length} repositories, ` +
    `there are ${metrics.totals.totalStars} total stars, ${metrics.totals.totalForks} forks, ` +
    `and ${metrics.totals.totalRecentCommits} recent commits.\n\n` +
    `${top.repo} leads in velocity with a score of ${top.velocity}. ` +
    `Rankings by stars: ${metrics.rankings.byStars.join(", ")}. ` +
    `Rankings by velocity: ${metrics.rankings.byVelocity.join(", ")}.`;

  return {
    narrative,
    highlights: [
      `Total stars across all repos: ${metrics.totals.totalStars}`,
      `Most active by velocity: ${metrics.rankings.byVelocity[0]}`,
      `Most starred: ${metrics.rankings.byStars[0]}`,
      `Total recent commits: ${metrics.totals.totalRecentCommits}`,
    ],
    prediction: `${top.repo} shows the most momentum based on velocity and recent commit activity.`,
    healthScores,
  };
}

/** Generate LLM-powered comparative analysis of AI agent project metrics */
export default async function run(ctx: Context, input: unknown): Promise<LLMAnalysis> {
  const metrics = input as CompositeMetrics;

  const openai = ctx.dependency("openai");

  if (!openai.secret) {
    ctx.log.warn("No openai.api_key, falling back to stats-only analysis");
    return fallbackAnalysis(metrics);
  }

  ctx.log.info("Generating comparative analysis via OpenAI gpt-5.2");

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
        { role: "user", content: buildUserPrompt(metrics) },
      ],
      max_completion_tokens: 4096,
    }),
  });

  if (!response.ok) {
    ctx.log.warn(`OpenAI API returned ${response.status}, falling back to stats-only analysis`);
    return fallbackAnalysis(metrics);
  }

  const data = await response.json();
  const content = data?.choices?.[0]?.message?.content;

  if (!content) {
    ctx.log.warn("Empty response from OpenAI, falling back to stats-only analysis");
    return fallbackAnalysis(metrics);
  }

  try {
    const parsed = JSON.parse(content) as LLMAnalysis;

    if (
      typeof parsed.narrative !== "string" ||
      !Array.isArray(parsed.highlights) ||
      typeof parsed.prediction !== "string" ||
      typeof parsed.healthScores !== "object"
    ) {
      ctx.log.warn("LLM response missing required fields, falling back to stats-only analysis");
      return fallbackAnalysis(metrics);
    }

    ctx.log.info(`LLM analysis generated: ${parsed.highlights.length} highlights, ${Object.keys(parsed.healthScores).length} health scores`);
    return parsed;
  } catch {
    ctx.log.warn("Failed to parse LLM response as JSON, falling back to stats-only analysis");
    return fallbackAnalysis(metrics);
  }
}
