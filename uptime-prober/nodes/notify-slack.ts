import type { Context } from "pipedreamer";

interface ReportInput {
  alert: boolean;
  probedAt: string;
  summary: string;
  sections: { down: string; healthy: string };
  totalEndpoints: number;
  unhealthyCount: number;
  healthyCount: number;
}

/** Send a structured uptime report to Slack via webhook */
export default async function run(ctx: Context, input: unknown): Promise<{ delivered: boolean; status: number }> {
  const data = input as ReportInput;

  const webhookUrl = ctx.secrets?.slack?.webhook_url;
  if (!webhookUrl) {
    ctx.log.error("No slack.webhook_url in secrets — cannot send notification");
    return { delivered: false, status: 0 };
  }

  const ts = new Date(data.probedAt).toLocaleString("en-US", {
    timeZone: "America/Los_Angeles",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  });

  const color = data.alert ? "#e74c3c" : "#2ecc71";
  const icon = data.alert ? ":red_circle:" : ":large_green_circle:";
  const title = data.alert ? "Uptime Alert" : "Uptime OK";

  const blocks: Record<string, unknown>[] = [
    {
      type: "header",
      text: { type: "plain_text", text: `${title}`, emoji: true },
    },
    {
      type: "section",
      text: {
        type: "mrkdwn",
        text: `${icon}  ${data.summary}`,
      },
    },
  ];

  if (data.alert && data.sections.down) {
    blocks.push(
      { type: "divider" },
      {
        type: "section",
        text: {
          type: "mrkdwn",
          text: `:warning: *Unreachable*\n${data.sections.down}`,
        },
      },
    );
  }

  if (data.sections.healthy) {
    blocks.push(
      { type: "divider" },
      {
        type: "section",
        text: {
          type: "mrkdwn",
          text: `:white_check_mark: *Healthy*\n${data.sections.healthy}`,
        },
      },
    );
  }

  blocks.push({
    type: "context",
    elements: [
      { type: "mrkdwn", text: `pipedreamer/uptime-prober v1.0 | ${ts} PT` },
    ],
  });

  const payload = {
    attachments: [{ color, blocks }],
  };

  ctx.log.info(`Sending ${data.alert ? "alert" : "all-clear"} to Slack`);

  const response = await ctx.fetch("slack", webhookUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });

  ctx.log.info(`Slack response: ${response.status}`);
  return { delivered: response.ok, status: response.status };
}
