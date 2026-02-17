import type { Context } from "tentacular";

interface HealthAnalysis {
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

/** Send the daily health report to Slack */
export default async function run(ctx: Context, input: unknown): Promise<{ delivered: boolean; status: number }> {
  const analysis = input as HealthAnalysis;

  const slack = ctx.dependency("slack");
  if (!slack.secret) {
    ctx.log.error("No slack.webhook_url in secrets — cannot send report");
    return { delivered: false, status: 0 };
  }

  const startDate = new Date(analysis.periodStart).toLocaleDateString("en-US", {
    timeZone: "America/Los_Angeles",
    month: "short",
    day: "numeric",
  });
  const endDate = new Date(analysis.periodEnd).toLocaleString("en-US", {
    timeZone: "America/Los_Angeles",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });

  const s = analysis.stats;
  const healthy = s.avgHealthyPodPct >= 99;
  const color = healthy ? "#2ecc71" : s.avgHealthyPodPct >= 95 ? "#f39c12" : "#e74c3c";
  const icon = healthy ? ":large_green_circle:" : s.avgHealthyPodPct >= 95 ? ":large_yellow_circle:" : ":red_circle:";

  const statsLine = [
    `*Avg healthy:* ${s.avgHealthyPodPct}%`,
    `*Min healthy:* ${s.minHealthyPodPct}%`,
    `*Peak problems:* ${s.maxProblemPods}`,
    `*Node down events:* ${s.nodeDownEvents}`,
    `*Pod range:* ${s.totalPodRange.min}–${s.totalPodRange.max}`,
  ].join("  |  ");

  const blocks: Record<string, unknown>[] = [
    {
      type: "header",
      text: { type: "plain_text", text: "Daily Cluster Health Report", emoji: true },
    },
    {
      type: "section",
      text: {
        type: "mrkdwn",
        text: `${icon} *${startDate} – ${endDate} PT* (${analysis.snapshotCount} snapshots)`,
      },
    },
    { type: "divider" },
    {
      type: "section",
      text: { type: "mrkdwn", text: analysis.aiSummary },
    },
    { type: "divider" },
    {
      type: "section",
      text: { type: "mrkdwn", text: statsLine },
    },
    {
      type: "context",
      elements: [
        { type: "mrkdwn", text: `tentacular/cluster-health-reporter v1.0` },
      ],
    },
  ];

  const payload = { attachments: [{ color, blocks }] };

  ctx.log.info("Sending daily health report to Slack");

  // For webhook-url type, the secret IS the full webhook URL
  const webhookUrl = slack.secret;
  const response = await globalThis.fetch(webhookUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  ctx.log.info(`Slack response: ${response.status}`);
  return { delivered: response.ok, status: response.status };
}
