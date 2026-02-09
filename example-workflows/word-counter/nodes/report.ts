import type { Context } from "pipedreamer";

interface Input {
  words: string[];
  counts: Record<string, number>;
}

interface Output {
  totalWords: number;
  uniqueWords: number;
  topWords: Array<{ word: string; count: number }>;
}

export default async function run(ctx: Context, input: Input): Promise<Output> {
  ctx.log.info("Generating word count report");
  const sorted = Object.entries(input.counts)
    .map(([word, count]) => ({ word, count }))
    .sort((a, b) => b.count - a.count);

  const report = {
    totalWords: input.words.length,
    uniqueWords: sorted.length,
    topWords: sorted.slice(0, 5),
  };

  ctx.log.info(`Report: ${report.totalWords} total, ${report.uniqueWords} unique`);
  return report;
}
