import type { Context } from "tentacular";

export default async function run(ctx: Context, input: unknown): Promise<unknown> {
  const data = input as { articles?: unknown[] };
  const articleCount = data?.articles?.length ?? 0;

  // Check if we have OpenAI credentials available
  const openaiKey = ctx.secrets?.["openai"]?.["api_key"];
  if (!openaiKey) {
    // Gracefully degrade without credentials (mock context)
    return {
      summary: `[Mock summary] ${articleCount} articles collected. No OpenAI credentials available.`,
      model: "mock",
      articleCount,
    };
  }

  try {
    const resp = await ctx.fetch("https://api.openai.com/v1/chat/completions", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "Authorization": `Bearer ${openaiKey}`,
      },
      body: JSON.stringify({
        model: "gpt-4o",
        max_completion_tokens: 400,
        response_format: { type: "json_object" },
        messages: [
          {
            role: "system",
            content: "Summarize the following news articles into a brief digest. Return JSON with a 'summary' field.",
          },
          {
            role: "user",
            content: JSON.stringify(data?.articles ?? []),
          },
        ],
      }),
    });

    const result = resp.body ?? resp;
    return {
      summary: result,
      model: "gpt-4o",
      articleCount,
    };
  } catch {
    return {
      summary: `[Error] Failed to call OpenAI. ${articleCount} articles pending.`,
      model: "error",
      articleCount,
    };
  }
}
