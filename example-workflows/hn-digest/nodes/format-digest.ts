import type { Context } from "tentacular";

interface Story {
  id: number;
  title: string;
  url: string;
  score: number;
  by: string;
  descendants: number;
}

/** Format filtered stories into a readable digest */
export default async function run(ctx: Context, input: unknown): Promise<{ digest: string; count: number }> {
  const data = input as { stories: Story[]; filtered: number };

  ctx.log.info(`Formatting digest with ${data.stories.length} stories`);

  const lines = data.stories.map((s, i) =>
    `${i + 1}. ${s.title} (${s.score} pts, ${s.descendants} comments)\n   ${s.url}\n   by ${s.by}`
  );

  const digest = [
    `== Hacker News Top Stories ==`,
    `${data.stories.length} stories (${data.filtered} filtered out)`,
    ``,
    ...lines,
  ].join("\n");

  ctx.log.info("Digest formatted");
  return { digest, count: data.stories.length };
}
