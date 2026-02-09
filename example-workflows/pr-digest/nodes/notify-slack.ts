import type { Context } from "pipedreamer";

interface AnalyzedDigest {
  repo: string;
  summary: string;
  prCount: number;
}

/** Send the PR digest to Slack via webhook */
export default async function run(ctx: Context, input: unknown): Promise<{ delivered: boolean; status: number }> {
  const digest = input as AnalyzedDigest;
  ctx.log.info(`Sending PR digest for ${digest.repo} to Slack`);

  const webhookUrl = ctx.secrets?.slack?.webhook_url;
  if (!webhookUrl) {
    ctx.log.warn("No slack webhook_url in secrets, skipping notification");
    return { delivered: false, status: 0 };
  }

  const message = `*PR Digest: ${digest.repo}*\n_${digest.prCount} open PRs_\n\n${digest.summary}`;

  const response = await ctx.fetch("slack", webhookUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text: message }),
  });

  ctx.log.info(`Slack notification sent, status: ${response.status}`);
  return { delivered: response.ok, status: response.status };
}
