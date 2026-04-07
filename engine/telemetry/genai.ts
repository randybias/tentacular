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

import { type Span, trace } from "@opentelemetry/api";

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
 * Parse SSE text to extract usage data from Anthropic streaming responses.
 * Looks for message_start (input_tokens) and message_delta (output_tokens) events.
 */
function parseSSEUsage(
  text: string,
): { respBody: Record<string, unknown>; usage: Record<string, unknown> } | null {
  const lines = text.split("\n");
  let inputTokens: number | undefined;
  let outputTokens: number | undefined;
  let model: string | undefined;
  let stopReason: string | undefined;
  let thinkingTokens: number | undefined;
  let cacheCreationInputTokens: number | undefined;
  let cacheReadInputTokens: number | undefined;
  const contentBlocks: unknown[] = [];

  for (const line of lines) {
    if (!line.startsWith("data: ")) continue;
    const dataStr = line.slice(6).trim();
    if (!dataStr || dataStr === "[DONE]") continue;

    try {
      const data = JSON.parse(dataStr) as Record<string, unknown>;
      const eventType = data["type"];

      if (eventType === "message_start") {
        const message = data["message"] as Record<string, unknown> | undefined;
        if (message) {
          if (typeof message["model"] === "string") {
            model = message["model"];
          }
          const u = message["usage"] as Record<string, unknown> | undefined;
          if (u && typeof u["input_tokens"] === "number") {
            inputTokens = u["input_tokens"] as number;
          }
          if (u && typeof u["cache_creation_input_tokens"] === "number") {
            cacheCreationInputTokens = u["cache_creation_input_tokens"] as number;
          }
          if (u && typeof u["cache_read_input_tokens"] === "number") {
            cacheReadInputTokens = u["cache_read_input_tokens"] as number;
          }
        }
      } else if (eventType === "message_delta") {
        const u = data["usage"] as Record<string, unknown> | undefined;
        if (u && typeof u["output_tokens"] === "number") {
          outputTokens = u["output_tokens"] as number;
        }
        const delta = data["delta"] as Record<string, unknown> | undefined;
        if (delta && typeof delta["stop_reason"] === "string") {
          stopReason = delta["stop_reason"];
        }
        if (u && typeof u["thinking_tokens"] === "number") {
          thinkingTokens = u["thinking_tokens"] as number;
        }
      } else if (eventType === "content_block_start") {
        const contentBlock = data["content_block"] as Record<string, unknown> | undefined;
        if (contentBlock) {
          contentBlocks.push(contentBlock);
        }
      }
    } catch {
      // Skip unparseable SSE data lines
    }
  }

  // Only return if we found at least some usage data
  if (inputTokens === undefined && outputTokens === undefined) {
    return null;
  }

  const usage: Record<string, unknown> = {};
  if (inputTokens !== undefined) usage["input_tokens"] = inputTokens;
  if (outputTokens !== undefined) usage["output_tokens"] = outputTokens;
  if (thinkingTokens !== undefined) usage["thinking_tokens"] = thinkingTokens;
  if (cacheCreationInputTokens !== undefined) {
    usage["cache_creation_input_tokens"] = cacheCreationInputTokens;
  }
  if (cacheReadInputTokens !== undefined) {
    usage["cache_read_input_tokens"] = cacheReadInputTokens;
  }

  const respBody: Record<string, unknown> = { usage };
  if (model !== undefined) respBody["model"] = model;
  if (stopReason !== undefined) respBody["stop_reason"] = stopReason;
  if (contentBlocks.length > 0) respBody["content"] = contentBlocks;

  return { respBody, usage };
}

/**
 * Enrich the provided OTel span with GenAI attributes from an Anthropic API response.
 * All attribute-setting is wrapped in try/catch so enrichment failures are silent.
 */
function enrichAnthropicSpan(
  span: Span,
  reqBody: Record<string, unknown> | null,
  respBody: Record<string, unknown>,
): void {
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
      // Extended thinking tokens
      if (typeof u["thinking_tokens"] === "number") {
        span.setAttribute("gen_ai.usage.thinking_tokens", u["thinking_tokens"]);
      }
    }

    // Tool use tracking
    const content = respBody["content"];
    if (Array.isArray(content)) {
      const toolUseBlocks = content.filter(
        (block: unknown) =>
          block && typeof block === "object" &&
          (block as Record<string, unknown>)["type"] === "tool_use",
      );
      if (toolUseBlocks.length > 0) {
        span.setAttribute("gen_ai.tool_use.count", toolUseBlocks.length);
        const toolNames = toolUseBlocks
          .map((block: unknown) => (block as Record<string, unknown>)["name"])
          .filter((n): n is string => typeof n === "string");
        if (toolNames.length > 0) {
          span.setAttribute("gen_ai.tool_use.names", toolNames);
        }
      }
    }
  } catch {
    // Span enrichment must never throw
  }
}

/**
 * Enrich the provided OTel span with GenAI attributes from an OpenAI API response.
 */
function enrichOpenAISpan(
  span: Span,
  reqBody: Record<string, unknown> | null,
  respBody: Record<string, unknown>,
): void {
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

    // Detect streaming mode
    const isStreaming = reqBody && reqBody["stream"] === true;

    // Capture the active span BEFORE the fetch call. After the await, Deno's
    // auto-instrumented fetch span may have ended, so getActiveSpan() could
    // return the wrong span or null. By capturing here, we get the
    // execute_node span — the right place for GenAI attributes.
    const span = trace.getActiveSpan();

    // Execute the original fetch
    const response = await originalFetch(input, init);

    // Enrich span from response body — must clone to avoid consuming the stream
    try {
      if (span) {
        // Request ID header
        const requestId = response.headers.get("request-id");
        if (requestId) {
          span.setAttribute("gen_ai.request.id", requestId);
        }

        // Rate limit headers (Anthropic-specific)
        try {
          const rlRequestsRemaining = response.headers.get(
            "anthropic-ratelimit-requests-remaining",
          );
          const rlTokensRemaining = response.headers.get(
            "anthropic-ratelimit-tokens-remaining",
          );
          if (rlRequestsRemaining) {
            span.setAttribute(
              "anthropic.ratelimit.requests.remaining",
              parseInt(rlRequestsRemaining, 10),
            );
          }
          if (rlTokensRemaining) {
            span.setAttribute(
              "anthropic.ratelimit.tokens.remaining",
              parseInt(rlTokensRemaining, 10),
            );
          }
        } catch {
          // ignore header read failures
        }
      }

      const cloned = response.clone();
      const text = await cloned.text();

      if (isStreaming) {
        // Streaming response: parse SSE events for usage data
        if (span && system === "anthropic") {
          const sseResult = parseSSEUsage(text);
          if (sseResult) {
            enrichAnthropicSpan(span, reqBody, sseResult.respBody);
          }
        }
        // OpenAI streaming is not yet supported — would need a similar parser
      } else {
        // Non-streaming response: parse as JSON
        const parsed = JSON.parse(text);
        if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
          const respBody = parsed as Record<string, unknown>;

          // Error response parsing
          if (!response.ok && span) {
            const errorObj = (parsed as Record<string, unknown>)["error"];
            if (errorObj && typeof errorObj === "object") {
              const err = errorObj as Record<string, unknown>;
              if (typeof err["type"] === "string") {
                span.setAttribute("gen_ai.error.type", err["type"]);
              }
              if (typeof err["message"] === "string") {
                span.setAttribute("gen_ai.error.message", err["message"]);
              }
            }
          }

          if (span) {
            if (system === "anthropic") {
              enrichAnthropicSpan(span, reqBody, respBody);
            } else if (system === "openai") {
              enrichOpenAISpan(span, reqBody, respBody);
            }
          }
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
