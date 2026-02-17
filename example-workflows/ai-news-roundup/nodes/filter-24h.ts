import type { Context } from "tentacular";

interface Article {
  title: string;
  url: string;
  source: string;
  publishedAt: string;
  snippet: string;
}

/** Filter articles to last 24 hours and deduplicate */
export default async function run(ctx: Context, input: unknown): Promise<{ articles: Article[]; totalBefore: number; totalAfter: number }> {
  const data = input as { articles: Article[] };
  const cutoff = Date.now() - 24 * 60 * 60 * 1000;

  // Deduplicate by URL
  const seen = new Set<string>();
  const unique: Article[] = [];
  for (const article of data.articles) {
    const normalizedUrl = article.url.replace(/\/$/, "").toLowerCase();
    if (!seen.has(normalizedUrl)) {
      seen.add(normalizedUrl);
      unique.push(article);
    }
  }

  // Filter to last 24 hours
  const recent = unique.filter((a) => {
    const pubTime = new Date(a.publishedAt).getTime();
    // If date parsing fails, include it (better to over-include)
    if (isNaN(pubTime)) return true;
    return pubTime >= cutoff;
  });

  // Sort by freshness (newest first)
  recent.sort((a, b) => {
    const ta = new Date(a.publishedAt).getTime() || 0;
    const tb = new Date(b.publishedAt).getTime() || 0;
    return tb - ta;
  });

  ctx.log.info(`Filtered: ${data.articles.length} → ${unique.length} unique → ${recent.length} in last 24h`);
  return { articles: recent, totalBefore: data.articles.length, totalAfter: recent.length };
}
