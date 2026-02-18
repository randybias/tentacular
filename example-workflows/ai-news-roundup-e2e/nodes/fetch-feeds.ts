import type { Context } from "tentacular";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const feedUrls = [
    "https://news.ycombinator.com/rss",
    "https://feeds.arstechnica.com/arstechnica/index",
    "https://www.techmeme.com/feed.xml",
  ];

  try {
    const results = await Promise.all(
      feedUrls.map(async (url) => {
        const resp = await ctx.fetch(url);
        return { url, status: resp.status ?? "mock", body: resp.body ?? resp };
      })
    );
    return { feeds: results, fetchedAt: new Date().toISOString() };
  } catch {
    // Gracefully degrade in mock context
    return {
      feeds: feedUrls.map((url) => ({
        url,
        status: "mock",
        body: { mock: true },
      })),
      fetchedAt: new Date().toISOString(),
    };
  }
}
