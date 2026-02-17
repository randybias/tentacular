import type { Context } from "tentacular";

interface Source {
  name: string;
  url: string;
  type: "rss" | "json";
}

interface Article {
  title: string;
  url: string;
  source: string;
  publishedAt: string;
  snippet: string;
}

/** Fetch articles from RSS feeds and Hacker News API */
export default async function run(ctx: Context, _input: unknown): Promise<{ articles: Article[] }> {
  const config = ctx.config as Record<string, unknown>;
  const sources = config.sources as Source[];
  const articles: Article[] = [];

  for (const source of sources) {
    try {
      ctx.log.info(`Fetching ${source.name} (${source.type})...`);
      const resp = await globalThis.fetch(source.url, {
        headers: { "User-Agent": "tentacular/ai-news-roundup 1.0" },
      });

      if (!resp.ok) {
        ctx.log.warn(`${source.name} returned ${resp.status}, skipping`);
        continue;
      }

      const body = await resp.text();

      if (source.type === "json") {
        // Hacker News Algolia API
        const data = JSON.parse(body);
        const hits = (data.hits || []) as Array<Record<string, unknown>>;
        for (const hit of hits.slice(0, 30)) {
          articles.push({
            title: (hit.title as string) || "",
            url: (hit.url as string) || `https://news.ycombinator.com/item?id=${hit.objectID}`,
            source: source.name,
            publishedAt: (hit.created_at as string) || new Date().toISOString(),
            snippet: (hit.title as string) || "",
          });
        }
      } else {
        // RSS parsing — extract items with regex (no XML parser needed)
        const items = body.match(/<item[\s\S]*?<\/item>/gi) || [];
        for (const item of items.slice(0, 30)) {
          const titleMatch = item.match(/<title><!\[CDATA\[([\s\S]*?)\]\]><\/title>/) ||
                             item.match(/<title>([\s\S]*?)<\/title>/);
          const linkMatch = item.match(/<link><!\[CDATA\[([\s\S]*?)\]\]><\/link>/) ||
                            item.match(/<link>([\s\S]*?)<\/link>/);
          const pubDateMatch = item.match(/<pubDate>([\s\S]*?)<\/pubDate>/);
          const descMatch = item.match(/<description><!\[CDATA\[([\s\S]*?)\]\]><\/description>/) ||
                            item.match(/<description>([\s\S]*?)<\/description>/);

          const title = titleMatch?.[1]?.trim() || "";
          const url = linkMatch?.[1]?.trim() || "";
          const publishedAt = pubDateMatch?.[1]?.trim() || new Date().toISOString();
          const rawDesc = descMatch?.[1]?.trim() || "";
          // Strip HTML tags from description
          const snippet = rawDesc.replace(/<[^>]*>/g, "").slice(0, 300);

          if (title && url) {
            articles.push({ title, url, source: source.name, publishedAt, snippet });
          }
        }
      }

      ctx.log.info(`${source.name}: ${articles.filter(a => a.source === source.name).length} articles`);
    } catch (err) {
      ctx.log.error(`Failed to fetch ${source.name}: ${err}`);
    }
  }

  ctx.log.info(`Total articles fetched: ${articles.length}`);
  return { articles };
}
