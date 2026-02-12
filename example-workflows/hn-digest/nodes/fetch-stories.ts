import type { Context } from "tentacular";

interface Story {
  id: number;
  title: string;
  url: string;
  score: number;
  by: string;
  descendants: number;
}

/** Fetch top stories from Hacker News API (public, no auth required) */
export default async function run(ctx: Context, _input: unknown): Promise<{ stories: Story[] }> {
  ctx.log.info("Fetching top stories from Hacker News");

  const topRes = await ctx.fetch("hn", "https://hacker-news.firebaseio.com/v0/topstories.json");
  const topIds: number[] = await topRes.json();
  if (!Array.isArray(topIds)) return { stories: [] };

  // Fetch details for top 10 stories
  const storyIds = topIds.slice(0, 10);
  const stories: Story[] = [];

  for (const id of storyIds) {
    const res = await ctx.fetch("hn", `https://hacker-news.firebaseio.com/v0/item/${id}.json`);
    const item = await res.json();
    stories.push({
      id: item.id,
      title: item.title || "",
      url: item.url || `https://news.ycombinator.com/item?id=${item.id}`,
      score: item.score || 0,
      by: item.by || "unknown",
      descendants: item.descendants || 0,
    });
  }

  ctx.log.info(`Fetched ${stories.length} stories`);
  return { stories };
}
