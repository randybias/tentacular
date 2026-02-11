import type { Context } from "tentacular";

interface Input {
  text: string;
}

interface Output {
  words: string[];
  counts: Record<string, number>;
}

export default async function run(ctx: Context, input: Input): Promise<Output> {
  ctx.log.info("Tokenizing text");
  const words = input.text.toLowerCase().split(/\s+/).filter((w) => w.length > 0);
  const counts: Record<string, number> = {};
  for (const word of words) {
    counts[word] = (counts[word] || 0) + 1;
  }
  ctx.log.info(`Found ${words.length} words, ${Object.keys(counts).length} unique`);
  return { words, counts };
}
