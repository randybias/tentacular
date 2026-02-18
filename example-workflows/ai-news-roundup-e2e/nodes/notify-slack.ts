import type { Context } from "tentacular";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const data = input as { summary?: string; model?: string; articleCount?: number };
  const summary = data?.summary ?? "No summary available";

  // Check if we have Slack credentials available
  const webhookUrl = ctx.secrets?.["slack"]?.["webhook_url"];
  if (!webhookUrl) {
    // Gracefully degrade without credentials (mock context)
    console.log("[mock] Slack notification skipped - no webhook URL");
    return {
      notified: false,
      reason: "no credentials",
      summary,
    };
  }

  try {
    // Extract path from full webhook URL for scoped fetch
    const path = new URL(webhookUrl).pathname;
    await ctx.fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        text: `*AI News Roundup*\n\n${summary}\n\n_${data?.articleCount ?? 0} articles summarized by ${data?.model ?? "unknown"}_`,
      }),
    });

    return { notified: true, channel: "slack" };
  } catch {
    console.error("[error] Slack notification failed");
    return { notified: false, reason: "send failed", summary };
  }
}
