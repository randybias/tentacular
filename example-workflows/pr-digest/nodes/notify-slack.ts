import type { Context } from "tentacular";

interface AnalyzedDigest {
  repo: string;
  summary: string;
  prCount: number;
}

/** Send the PR digest to Slack via webhook */
export default async function run(ctx: Context, input: unknown): Promise<{ delivered: boolean; status: number }> {
  const digest = input as AnalyzedDigest;
  ctx.log.info(`Sending PR digest for ${digest.repo} to Slack`);

  const slack = ctx.dependency("slack");
  if (!slack.secret) {
    ctx.log.warn("No slack webhook_url in secrets, skipping notification");
    return { delivered: false, status: 0 };
  }

  const message = `*PR Digest: ${digest.repo}*\n_${digest.prCount} open PRs_\n\n${digest.summary}`;

  // For webhook-url type, the secret IS the full webhook URL
  const webhookUrl = slack.secret;
  const response = await globalThis.fetch(webhookUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text: message }),
  });

  ctx.log.info(`Slack notification sent, status: ${response.status}`);
  return { delivered: response.ok, status: response.status };
}
