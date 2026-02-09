import type { Context } from "pipedreamer";

export default async function run(ctx: Context, _input: unknown): Promise<{ text: string }> {
  ctx.log.info("Producing source text");
  const text = "the quick brown fox jumps over the lazy dog the fox";
  return { text };
}
