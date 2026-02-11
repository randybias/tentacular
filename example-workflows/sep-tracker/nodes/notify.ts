import type { Context } from "tentacular";

interface Sep {
  number: number;
  sepId: string;
  title: string;
  state: string;
  author: string;
  url: string;
}

interface SepChange {
  changeType: "added" | "removed" | "updated";
  sep: Sep;
  previousState?: string;
  changes?: string[];
}

interface SepDelta {
  timestamp: string;
  repo: string;
  changes: SepChange[];
  addedCount: number;
  removedCount: number;
  updatedCount: number;
  totalCount: number;
  isFirstRun: boolean;
}

interface StoreResult {
  stored: boolean;
  snapshotId: number;
  reportId: number;
  reportUrl: string;
}

interface NotifyOutput {
  delivered: boolean;
  status: number;
}

/** Send Slack notification with SEP report summary and link to full report */
export default async function run(ctx: Context, input: unknown): Promise<NotifyOutput> {
  // Fan-in: receives both store-report and diff-seps outputs
  const merged = input as { "store-report": StoreResult; "diff-seps": SepDelta };
  const storeResult = merged["store-report"];
  const delta = merged["diff-seps"];

  const webhookUrl = ctx.secrets?.slack?.webhook_url;
  if (!webhookUrl) {
    ctx.log.warn("No slack webhook_url in secrets, skipping notification");
    return { delivered: false, status: 0 };
  }

  // Build header
  const emoji = delta.isFirstRun ? ":new:" : delta.addedCount + delta.removedCount + delta.updatedCount > 0 ? ":arrows_counterclockwise:" : ":white_check_mark:";
  const header = `${emoji} *MCP SEP Tracker Report*`;

  // Build summary line
  let summary: string;
  if (delta.isFirstRun) {
    summary = `Initial snapshot: *${delta.totalCount} SEPs* tracked from \`${delta.repo}\``;
  } else if (delta.addedCount + delta.removedCount + delta.updatedCount === 0) {
    summary = `No changes detected across *${delta.totalCount} SEPs* in \`${delta.repo}\``;
  } else {
    const parts: string[] = [];
    if (delta.addedCount > 0) parts.push(`+${delta.addedCount} added`);
    if (delta.removedCount > 0) parts.push(`-${delta.removedCount} removed`);
    if (delta.updatedCount > 0) parts.push(`~${delta.updatedCount} updated`);
    summary = `Changes: ${parts.join(", ")} (${delta.totalCount} total SEPs in \`${delta.repo}\`)`;
  }

  // Build change details (up to 10)
  const changeLines: string[] = [];
  for (const change of delta.changes.slice(0, 10)) {
    const icon = change.changeType === "added" ? ":heavy_plus_sign:" :
                 change.changeType === "removed" ? ":heavy_minus_sign:" : ":pencil2:";
    changeLines.push(`${icon} <${change.sep.url}|${change.sep.sepId}: ${change.sep.title}> (${change.sep.state}, by ${change.sep.author})`);
  }
  if (delta.changes.length > 10) {
    changeLines.push(`_...and ${delta.changes.length - 10} more_`);
  }

  // Build storage line
  const storageLine = storeResult.stored
    ? `:floppy_disk: Stored as snapshot #${storeResult.snapshotId}, report #${storeResult.reportId}`
    : ":warning: Storage failed";

  const blocks: Record<string, unknown>[] = [
    {
      type: "header",
      text: { type: "plain_text", text: "MCP SEP Tracker Report" },
    },
    {
      type: "section",
      text: { type: "mrkdwn", text: `${header}\n${summary}` },
    },
  ];

  if (changeLines.length > 0) {
    blocks.push({
      type: "section",
      text: { type: "mrkdwn", text: changeLines.join("\n") },
    });
  }

  // Add "View Full Report" button if report URL is available
  if (storeResult.reportUrl) {
    blocks.push({
      type: "actions",
      elements: [
        {
          type: "button",
          text: { type: "plain_text", text: ":bar_chart: View Full Report" },
          url: storeResult.reportUrl,
          style: "primary",
        },
      ],
    });
  }

  blocks.push(
    { type: "divider" },
    {
      type: "context",
      elements: [{ type: "mrkdwn", text: `${storageLine} | ${delta.timestamp}` }],
    },
  );

  ctx.log.info("Sending Slack notification");

  const res = await ctx.fetch("slack", webhookUrl, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ blocks }),
  });

  ctx.log.info(`Slack response: ${res.status}`);

  return { delivered: res.ok, status: res.status };
}
