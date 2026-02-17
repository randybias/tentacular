import type { Context } from "tentacular";

interface TopLink {
  title: string;
  url: string;
  source: string;
  reason: string;
}

interface SummaryInput {
  executiveSummary: string;
  topLinks: TopLink[];
  model: string;
  articleCount: number;
}

/** Post AI news roundup to Slack via webhook */
export default async function run(ctx: Context, input: unknown): Promise<{ delivered: boolean; status: number }> {
  const data = input as SummaryInput;
  const config = ctx.config as Record<string, unknown>;
  const clusterId = config.cluster_id ?? "unknown";

  const slack = ctx.dependency("slack");
  if (!slack.secret) {
    ctx.log.error("No slack.webhook_url — cannot send notification");
    return { delivered: false, status: 0 };
  }

  const today = new Date().toLocaleDateString("en-US", {
    weekday: "long",
    year: "numeric",
    month: "long",
    day: "numeric",
    timeZone: "Europe/Bratislava",
  });

  // Build blocks — stay under Slack's limits (50 blocks, ~3000 char per text field)
  const blocks: Record<string, unknown>[] = [
    {
      type: "header",
      text: { type: "plain_text", text: `🤖 AI News Roundup — ${today}`, emoji: true },
    },
    {
      type: "section",
      text: {
        type: "mrkdwn",
        text: `*Executive Summary*`,
      },
    },
  ];

  // Split executive summary into paragraphs, each as its own block (3000 char limit per block)
  const paragraphs = data.executiveSummary.split(/\n\n+/).filter((p: string) => p.trim());
  for (const para of paragraphs) {
    blocks.push({
      type: "section",
      text: {
        type: "mrkdwn",
        text: para.slice(0, 2900),
      },
    });
  }

  blocks.push({ type: "divider" });

  // Add top links — each as a compact section
  const linkCount = Math.min(data.topLinks.length, 10);
  for (let i = 0; i < linkCount; i++) {
    const link = data.topLinks[i];
    // Keep each block under 3000 chars
    const text = `*${i + 1}.* <${link.url}|${link.title.slice(0, 120)}>\n_${link.source}_ — ${link.reason.slice(0, 200)}`;
    blocks.push({
      type: "section",
      text: { type: "mrkdwn", text },
    });
  }

  // Footer
  blocks.push(
    { type: "divider" },
    {
      type: "context",
      elements: [
        {
          type: "mrkdwn",
          text: `tentacular/ai-news-roundup v1.0 | cluster: ${clusterId} | ${data.articleCount} articles scanned | model: ${data.model}`,
        },
      ],
    },
  );

  // Slack webhook payload — keep under 50 blocks
  if (blocks.length > 50) {
    blocks.splice(48);
    blocks.push({
      type: "context",
      elements: [{ type: "mrkdwn", text: "_Truncated to fit Slack limits_" }],
    });
  }

  const payload = {
    blocks,
  };

  ctx.log.info(`Sending roundup to Slack (${blocks.length} blocks, ${linkCount} links)`);

  const webhookUrl = slack.secret;
  const response = await globalThis.fetch(webhookUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  ctx.log.info(`Slack response: ${response.status}`);
  return { delivered: response.ok, status: response.status };
}
