import type { Context } from "tentacular";

interface Article {
  title: string;
  url: string;
  source: string;
  publishedAt: string;
  snippet: string;
}

interface TopLink {
  title: string;
  url: string;
  source: string;
  reason: string;
}

interface SummaryOutput {
  executiveSummary: string;
  topLinks: TopLink[];
  model: string;
  articleCount: number;
}

/** Summarize articles via OpenAI gpt-5.2 */
export default async function run(ctx: Context, input: unknown): Promise<SummaryOutput> {
  const data = input as { articles: Article[]; totalBefore: number; totalAfter: number };
  const config = ctx.config as Record<string, unknown>;
  const model = (config.openai_model as string) || "gpt-5.2";

  const openai = ctx.dependency("openai-api");
  if (!openai.secret) {
    ctx.log.error("No openai.api_key — cannot summarize");
    return {
      executiveSummary: "Unable to generate summary: missing API key",
      topLinks: [],
      model,
      articleCount: data.articles.length,
    };
  }

  // Build article list for the prompt
  const articleList = data.articles
    .map((a, i) => `${i + 1}. [${a.source}] "${a.title}" — ${a.snippet.slice(0, 150)}... (${a.url})`)
    .join("\n");

  const systemPrompt = `You are an AI news analyst specializing in AI Agents, Agentic AI, AIOps, and Agents for Operations. You produce daily briefings for a VP of Open Source at an infrastructure company. You write clearly and directly — no filler, no hype, just signal.`;

  const userPrompt = `Here are ${data.articles.length} AI-related articles from the last 24 hours:

${articleList}

Produce a JSON response with exactly this structure:
{
  "executiveSummary": "A 2-3 paragraph executive summary. First paragraph: the biggest story or theme of the day. Second paragraph: agentic AI and AIOps-specific developments — new agent platforms, partnerships, frameworks, operational tooling. Third paragraph: infrastructure and market moves — funding, compute buildouts, model releases, regulatory shifts. Write in a direct, analytical tone. No bullet points — flowing prose.",
  "topLinks": [
    {
      "title": "Article title",
      "url": "https://...",
      "source": "Source name",
      "reason": "One sentence on why this is relevant to AI agents/AIOps"
    }
  ]
}

Select the top 10 links ranked by:
1. Relevance to AI Agents, particularly Agents for Operations and AIOps
2. Freshness (newer is better)
3. Actionable insights for infrastructure/operations teams

Return ONLY valid JSON, no markdown fencing.`;

  ctx.log.info(`Sending ${data.articles.length} articles to ${model} for summarization`);

  const resp = await globalThis.fetch("https://api.openai.com/v1/chat/completions", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${openai.secret}`,
    },
    body: JSON.stringify({
      model,
      messages: [
        { role: "system", content: systemPrompt },
        { role: "user", content: userPrompt },
      ],
      temperature: 0.3,
      max_completion_tokens: 3000,
    }),
  });

  if (!resp.ok) {
    const errText = await resp.text();
    ctx.log.error(`OpenAI API error ${resp.status}: ${errText.slice(0, 1000)}`);
    return {
      executiveSummary: `OpenAI API error: ${resp.status}`,
      topLinks: [],
      model,
      articleCount: data.articles.length,
    };
  }

  const result = await resp.json();
  const content = result.choices?.[0]?.message?.content || "";

  try {
    const parsed = JSON.parse(content);
    ctx.log.info(`Summary generated: ${parsed.topLinks?.length || 0} top links selected`);
    return {
      executiveSummary: parsed.executiveSummary || "No summary generated",
      topLinks: (parsed.topLinks || []).slice(0, 10),
      model,
      articleCount: data.articles.length,
    };
  } catch (err) {
    ctx.log.error(`Failed to parse LLM response: ${err}`);
    ctx.log.error(`Raw content: ${content.slice(0, 500)}`);
    return {
      executiveSummary: content.slice(0, 500),
      topLinks: [],
      model,
      articleCount: data.articles.length,
    };
  }
}
