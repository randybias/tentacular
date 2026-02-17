import type { Context } from "tentacular";

interface PullRequest {
  number: number;
  title: string;
  author: string;
  url: string;
  labels: string[];
  draft: boolean;
  createdAt: string;
  body: string;
}

interface AnalyzedDigest {
  repo: string;
  summary: string;
  prCount: number;
}

/** Analyze PRs using Anthropic Claude API and produce a summary */
export default async function run(ctx: Context, input: unknown): Promise<AnalyzedDigest> {
  const data = input as { prs: PullRequest[]; repo: string };
  ctx.log.info(`Analyzing ${data.prs.length} PRs with Claude`);

  const prList = data.prs.map((pr) => {
    const labels = pr.labels.length > 0 ? ` [${pr.labels.join(", ")}]` : "";
    const draft = pr.draft ? " (DRAFT)" : "";
    return `#${pr.number}: ${pr.title}${draft}${labels}\n  by ${pr.author} | ${pr.body.substring(0, 200)}`;
  }).join("\n\n");

  const prompt = `You are a senior developer reviewing open pull requests for ${data.repo}. Summarize the following ${data.prs.length} open PRs in a concise digest suitable for a Slack message. Group them by theme if possible. Use bullet points. Keep it under 500 words.

PRs:
${prList}`;

  const anthropic = ctx.dependency("anthropic");
  const response = await anthropic.fetch!("/v1/messages", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "anthropic-version": "2023-06-01",
    },
    body: JSON.stringify({
      model: "claude-haiku-4-5-20251001",
      max_tokens: 1024,
      messages: [{ role: "user", content: prompt }],
    }),
  });

  if (!response.ok) {
    const errText = await response.text();
    ctx.log.error(`Anthropic API error: ${response.status} ${errText}`);
    // Fallback to simple summary
    const lines = data.prs.map((pr) => `- #${pr.number}: ${pr.title} (by ${pr.author})`);
    return {
      repo: data.repo,
      summary: `Open PRs for ${data.repo}:\n${lines.join("\n")}`,
      prCount: data.prs.length,
    };
  }

  const result = await response.json();
  const summary = (result as Record<string, unknown[]>).content
    ?.map((c: unknown) => (c as Record<string, string>).text)
    .join("\n") || "No summary generated";

  ctx.log.info("Analysis complete");
  return { repo: data.repo, summary, prCount: data.prs.length };
}
