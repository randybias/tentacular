/**
 * Webhook trigger handler for Tentacular.
 *
 * Supports GitHub webhooks with HMAC-SHA256 signature validation.
 * Executes the workflow asynchronously and returns 200 immediately
 * so GitHub does not time out waiting for the workflow to complete.
 */

import type { CompiledDAG, Context, Trigger } from "../types.ts";
import type { NodeRunner } from "../executor/types.ts";
import { SimpleExecutor } from "../executor/simple.ts";

export interface WebhookTriggerOptions {
  /** Port to listen on for webhook requests */
  port: number;
  /** Webhook triggers from the workflow spec */
  triggers: Trigger[];
  /** Compiled workflow DAG */
  graph: CompiledDAG;
  /** Node runner for executing workflows */
  runner: NodeRunner;
  /** Workflow context */
  ctx: Context;
  /** Execution timeout in ms */
  timeoutMs?: number;
  /** Max retries per node */
  maxRetries?: number;
  /** GitHub webhook secret for HMAC-SHA256 signature validation */
  secret?: string;
}

export interface WebhookTriggerHandle {
  /** Shut down the webhook HTTP server */
  close(): Promise<void>;
}

/**
 * Validate webhook trigger options.
 * Returns an error message if invalid, null if valid.
 */
export function validateOptions(opts: Partial<WebhookTriggerOptions>): string | null {
  if (!opts.triggers || opts.triggers.length === 0) {
    return "At least one webhook trigger is required";
  }
  const missing = opts.triggers.filter((t) => !t.provider);
  if (missing.length > 0) {
    return "All webhook triggers must have a provider (e.g. 'github')";
  }
  return null;
}

/**
 * Validate a GitHub webhook HMAC-SHA256 signature.
 *
 * GitHub sends: X-Hub-Signature-256: sha256=<hex>
 * We compute:   HMAC-SHA256(secret, body) and compare.
 */
async function validateGitHubSignature(
  body: string,
  signatureHeader: string | null,
  secret: string,
): Promise<boolean> {
  if (!signatureHeader || !signatureHeader.startsWith("sha256=")) {
    return false;
  }
  const hex = signatureHeader.slice("sha256=".length);
  if (hex.length % 2 !== 0) return false;

  // Decode hex signature to bytes
  const sigBytes = new Uint8Array(hex.length / 2);
  for (let i = 0; i < hex.length; i += 2) {
    sigBytes[i / 2] = parseInt(hex.substring(i, i + 2), 16);
  }

  // Use crypto.subtle.verify() for guaranteed constant-time HMAC comparison
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["verify"],
  );

  return await crypto.subtle.verify(
    "HMAC",
    key,
    sigBytes,
    new TextEncoder().encode(body),
  );
}

/**
 * Find a matching webhook trigger for the given provider, event, and action.
 */
function findMatchingTrigger(
  triggers: Trigger[],
  provider: string,
  event: string,
  action?: string,
): Trigger | undefined {
  return triggers.find((t) => {
    if (t.type !== "webhook") return false;
    if (t.provider && t.provider !== provider) return false;
    if (t.event && t.event !== event) return false;
    if (t.actions && t.actions.length > 0 && action) {
      if (!t.actions.includes(action)) return false;
    }
    return true;
  });
}

/**
 * Start an HTTP server that handles webhook requests.
 * Integrates with the existing Tentacular engine HTTP server via the handler.
 *
 * Returns a handler function for use in server.ts, plus a close() noop
 * (lifecycle is managed by the main server).
 */
export async function handleGitHubWebhook(
  req: Request,
  opts: WebhookTriggerOptions,
): Promise<Response> {
  const executor = new SimpleExecutor({
    timeoutMs: opts.timeoutMs,
    maxRetries: opts.maxRetries,
  });

  // Read body once
  const body = await req.text();

  // Validate signature if secret is configured
  if (opts.secret) {
    const signature = req.headers.get("x-hub-signature-256");
    const valid = await validateGitHubSignature(body, signature, opts.secret);
    if (!valid) {
      console.warn("[webhook] GitHub signature validation failed — rejecting request");
      return new Response(JSON.stringify({ error: "Invalid signature" }), {
        status: 401,
        headers: { "content-type": "application/json" },
      });
    }
  }

  const event = req.headers.get("x-github-event") ?? "";
  const deliveryId = req.headers.get("x-github-delivery") ?? "unknown";

  let payload: Record<string, unknown> = {};
  try {
    payload = JSON.parse(body);
  } catch {
    return new Response(JSON.stringify({ error: "Invalid JSON payload" }), {
      status: 400,
      headers: { "content-type": "application/json" },
    });
  }

  const action = typeof payload.action === "string" ? payload.action : undefined;

  // Find a matching trigger
  const trigger = findMatchingTrigger(opts.triggers, "github", event, action);
  if (!trigger) {
    console.log(
      `[webhook] No matching trigger for github event="${event}" action="${action ?? "none"}" — dropping`,
    );
    return new Response(JSON.stringify({ ok: true, matched: false }), {
      status: 200,
      headers: { "content-type": "application/json" },
    });
  }

  console.log(
    `[webhook] Matched trigger for github event="${event}" action="${action ?? "none"}" delivery=${deliveryId}`,
  );

  // Enrich input with trigger metadata
  const input: Record<string, unknown> = {
    ...payload,
    _webhook: {
      provider: "github",
      event,
      action,
      delivery_id: deliveryId,
      trigger: trigger.name ?? trigger.event ?? "webhook",
    },
  };

  // Execute workflow asynchronously — do NOT await, return 200 immediately
  (async () => {
    try {
      const result = await executor.execute(opts.graph, opts.runner, opts.ctx, input);
      if (!result.success) {
        console.error(
          `[webhook] Workflow execution failed for delivery=${deliveryId}:`,
          result.errors,
        );
      } else {
        console.log(`[webhook] Workflow completed successfully for delivery=${deliveryId}`);
      }
    } catch (err) {
      console.error(`[webhook] Unexpected error for delivery=${deliveryId}:`, err);
    }
  })();

  return new Response(JSON.stringify({ ok: true, matched: true, delivery_id: deliveryId }), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}
