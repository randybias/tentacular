import type { Context } from "tentacular";

interface Story {
  id: number;
  title: string;
  url: string;
  score: number;
  by: string;
  descendants: number;
}

/** Filter stories by minimum score threshold */
export default async function run(ctx: Context, input: unknown): Promise<{ stories: Story[]; filtered: number }> {
  const data = input as { stories: Story[] };
  const minScore = (ctx.config as Record<string, unknown>).min_score as number ?? 50;

  ctx.log.info(`Filtering ${data.stories.length} stories with min score ${minScore}`);

  const filtered = data.stories.filter((s) => s.score >= minScore);
  filtered.sort((a, b) => b.score - a.score);

  ctx.log.info(`${filtered.length} stories passed filter`);
  return { stories: filtered, filtered: data.stories.length - filtered.length };
}
