import type { Context } from "tentacular";

export default async function run(_ctx: Context, input: unknown): Promise<unknown> {
  const data = input as { feeds?: Array<{ url: string; body: unknown }> };
  if (!data?.feeds) {
    return { articles: [], filtered: true };
  }

  // In a real implementation, this would parse RSS XML and filter by date.
  // For mock/test purposes, pass through the feed data as "articles".
  return {
    articles: data.feeds.map((feed) => ({
      source: feed.url,
      content: feed.body,
    })),
    filtered: true,
    cutoff: new Date(Date.now() - 24 * 60 * 60 * 1000).toISOString(),
  };
}
