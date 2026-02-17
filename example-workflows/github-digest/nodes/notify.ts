import type { Context } from "tentacular";

/** Digest summary from upstream summarize node */
interface DigestSummary {
  title: string;
  summary: string;
  repoCount: number;
}

/**
 * Notify node - sink node that sends the digest to a webhook.
 *
 * @param ctx - Execution context with fetch for webhook delivery
 * @param input - Digest summary from summarize node
 * @returns Confirmation of delivery with status
 */
export default async function run(ctx: Context, input: unknown): Promise<{ delivered: boolean; status: number }> {
  const digest = input as DigestSummary;

  ctx.log.info(`Sending notification: ${digest.title}`);

  const slack = ctx.dependency("slack");
  if (!slack.secret) {
    ctx.log.warn("No slack webhook_url in secrets, skipping notification");
    return { delivered: false, status: 0 };
  }

  // For webhook-url type, the secret IS the full webhook URL
  const webhookUrl = slack.secret;
  const response = await globalThis.fetch(webhookUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      text: `*${digest.title}*\n${digest.summary}`,
    }),
  });

  ctx.log.info(`Notification sent, status: ${response.status}`);

  return { delivered: response.ok, status: response.status };
}
