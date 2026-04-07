/**
 * Tentacular Engine — GenAI Fetch Wrapper
 *
 * Wraps globalThis.fetch to detect calls to known LLM API endpoints and enrich
 * the active OTel span with GenAI semantic convention attributes from the
 * response body.
 *
 * Usage: call installGenAIWrapper() once at engine startup (before any fetch
 * calls). The wrapper is error-resilient: if span enrichment fails for any
 * reason, the original response is returned unmodified.
 *
 * What is NOT captured by default:
 *   - gen_ai.input.messages (prompt content)
 *   - gen_ai.output.messages (completion content)
 */

import { trace } from "@opentelemetry/api";

/** Known LLM API endpoint patterns and their system names. */
const LLM_ENDPOINTS: { pattern: RegExp; system: string }[] = [
  { pattern: /^https:\/\/api\.anthropic\.com\/v1\/messages/, system: "anthropic" },
  { pattern: /^https:\/\/api\.openai\.com\/v1\/chat\/completions/, system: "openai" },
];

/**
 * Detect whether a URL matches a known LLM API endpoint.
 * Returns the system name ("anthropic" | "openai") or null.
 */
function detectLLMSystem(url: string): string | null {
  for (const endpoint of LLM_ENDPOINTS) {
    if (endpoint.pattern.test(url)) {
      return endpoint.system;
    }
  }
  return null;
}

/**
 * Parse request body JSON safely.
 * Returns null if the body is not valid JSON or cannot be read.
 */
function parseRequestBody(body: BodyInit | null | undefined): Record<string, unknown> | null {
  if (!body || typeof body !== "string") return null;
  try {
    const parsed = JSON.parse(body);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
  } catch {
    // non-JSON body — ignore
  }
  return null;
}

/**
 * Enrich the active OTel span with GenAI attributes from an Anthropic API response.
 * All attribute-setting is wrapped in try/catch so enrichment failures are silent.
 */
function enrichAnthropicSpan(
  reqBody: Record<string, unknown> | null,
  respBody: Record<string, unknown>,
): void {
  const span = trace.getActiveSpan();
  if (!span) return;

  try {
    span.setAttribute("gen_ai.system", "anthropic");
    span.setAttribute("gen_ai.operation.name", "chat");

    // Request model
    if (reqBody && typeof reqBody["model"] === "string") {
      span.setAttribute("gen_ai.request.model", reqBody["model"]);
    }

    // Response model
    if (typeof respBody["model"] === "string") {
      span.setAttribute("gen_ai.response.model", respBody["model"]);
    }

    // Finish reasons (stop_reason in Anthropic API)
    if (typeof respBody["stop_reason"] === "string") {
      span.setAttribute("gen_ai.response.finish_reasons", [respBody["stop_reason"]]);
    }

    // Token usage
    const usage = respBody["usage"];
    if (usage && typeof usage === "object" && !Array.isArray(usage)) {
      const u = usage as Record<string, unknown>;

      if (typeof u["input_tokens"] === "number") {
        span.setAttribute("gen_ai.usage.input_tokens", u["input_tokens"]);
      }
      if (typeof u["output_tokens"] === "number") {
        span.setAttribute("gen_ai.usage.output_tokens", u["output_tokens"]);
      }
      // Anthropic cache token attributes
      if (typeof u["cache_creation_input_tokens"] === "number") {
        span.setAttribute(
          "gen_ai.usage.cache_creation.input_tokens",
          u["cache_creation_input_tokens"],
        );
      }
      if (typeof u["cache_read_input_tokens"] === "number") {
        span.setAttribute(
          "gen_ai.usage.cache_read.input_tokens",
          u["cache_read_input_tokens"],
        );
      }
    }
  } catch {
    // Span enrichment must never throw
  }
}

/**
 * Enrich the active OTel span with GenAI attributes from an OpenAI API response.
 */
function enrichOpenAISpan(
  reqBody: Record<string, unknown> | null,
  respBody: Record<string, unknown>,
): void {
  const span = trace.getActiveSpan();
  if (!span) return;

  try {
    span.setAttribute("gen_ai.system", "openai");
    span.setAttribute("gen_ai.operation.name", "chat");

    // Request model
    if (reqBody && typeof reqBody["model"] === "string") {
      span.setAttribute("gen_ai.request.model", reqBody["model"]);
    }

    // Response model
    if (typeof respBody["model"] === "string") {
      span.setAttribute("gen_ai.response.model", respBody["model"]);
    }

    // Finish reasons (from choices[].finish_reason)
    const choices = respBody["choices"];
    if (Array.isArray(choices) && choices.length > 0) {
      const finishReasons = choices
        .map((
          c,
        ) => (c && typeof c === "object" ? (c as Record<string, unknown>)["finish_reason"] : null))
        .filter((r): r is string => typeof r === "string");
      if (finishReasons.length > 0) {
        span.setAttribute("gen_ai.response.finish_reasons", finishReasons);
      }
    }

    // Token usage
    const usage = respBody["usage"];
    if (usage && typeof usage === "object" && !Array.isArray(usage)) {
      const u = usage as Record<string, unknown>;
      if (typeof u["prompt_tokens"] === "number") {
        span.setAttribute("gen_ai.usage.input_tokens", u["prompt_tokens"]);
      }
      if (typeof u["completion_tokens"] === "number") {
        span.setAttribute("gen_ai.usage.output_tokens", u["completion_tokens"]);
      }
    }
  } catch {
    // Span enrichment must never throw
  }
}

/**
 * Install the GenAI fetch wrapper.
 *
 * Replaces globalThis.fetch with a wrapper that intercepts LLM API calls,
 * reads the response body, enriches the active OTel span with usage attributes,
 * and returns a reconstructed response to the caller.
 *
 * Safe to call multiple times — subsequent calls are no-ops if already installed.
 */
export function installGenAIWrapper(): void {
  // Guard: do not double-install
  if ((globalThis.fetch as { __genAIWrapped?: boolean }).__genAIWrapped) {
    return;
  }

  const originalFetch = globalThis.fetch.bind(globalThis);

  const wrappedFetch: typeof fetch = async (
    input: string | URL | Request,
    init?: RequestInit,
  ): Promise<Response> => {
    const url = typeof input === "string"
      ? input
      : input instanceof URL
      ? input.toString()
      : input.url;

    const system = detectLLMSystem(url);

    // Non-LLM URL: pass through unchanged
    if (!system) {
      return originalFetch(input, init);
    }

    // Parse request body for model extraction (best effort)
    let reqBody: Record<string, unknown> | null = null;
    try {
      const bodySource = init?.body ??
        (input instanceof Request ? await input.clone().text() : null);
      reqBody = parseRequestBody(bodySource as BodyInit | null);
    } catch {
      // Ignore body parse failures
    }

    // Execute the original fetch
    const response = await originalFetch(input, init);

    // Enrich span from response body — must clone to avoid consuming the stream
    try {
      const cloned = response.clone();
      const text = await cloned.text();
      const parsed = JSON.parse(text);
      if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
        const respBody = parsed as Record<string, unknown>;
        if (system === "anthropic") {
          enrichAnthropicSpan(reqBody, respBody);
        } else if (system === "openai") {
          enrichOpenAISpan(reqBody, respBody);
        }
      }
    } catch {
      // Never fail the caller due to enrichment errors
    }

    return response;
  };

  // Mark as installed
  (wrappedFetch as { __genAIWrapped?: boolean }).__genAIWrapped = true;
  globalThis.fetch = wrappedFetch;
}
