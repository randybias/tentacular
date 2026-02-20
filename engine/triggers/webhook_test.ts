/**
 * Tests for the GitHub webhook trigger handler.
 */

import { assertEquals } from "jsr:@std/assert";
import { handleGitHubWebhook } from "./webhook.ts";
import type { WebhookTriggerOptions } from "./webhook.ts";
import type { CompiledDAG, Context } from "../types.ts";
import type { NodeRunner } from "../executor/types.ts";
import { SimpleExecutor } from "../executor/simple.ts";

// --- Test helpers ---

const SECRET = "test-webhook-secret";

async function makeSignature(body: string, secret: string): Promise<string> {
  const key = await crypto.subtle.importKey(
    "raw",
    new TextEncoder().encode(secret),
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );
  const sigBytes = await crypto.subtle.sign("HMAC", key, new TextEncoder().encode(body));
  const hex = Array.from(new Uint8Array(sigBytes))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");
  return `sha256=${hex}`;
}

function makeRequest(
  body: string,
  event: string,
  signature?: string,
): Request {
  const headers: Record<string, string> = {
    "content-type": "application/json",
    "x-github-event": event,
    "x-github-delivery": "test-delivery-id",
  };
  if (signature) {
    headers["x-hub-signature-256"] = signature;
  }
  return new Request("http://localhost:8080/webhook/github", {
    method: "POST",
    headers,
    body,
  });
}

function makeOpts(overrides: Partial<WebhookTriggerOptions> = {}): WebhookTriggerOptions {
  const mockGraph: CompiledDAG = {
    workflow: {
      name: "test",
      version: "1.0",
      triggers: [
        {
          type: "webhook",
          provider: "github",
          event: "pull_request",
          actions: ["opened", "synchronize", "reopened"],
        },
      ],
      nodes: {},
      edges: [],
    },
    stages: [],
    nodeOrder: [],
  };

  const mockCtx: Context = {
    fetch: async () => new Response("{}"),
    log: {
      info: () => {},
      warn: () => {},
      error: () => {},
      debug: () => {},
    },
    config: {},
    secrets: {},
    dependency: () => ({ protocol: "https", host: "example.com", port: 443 }),
  };

  const mockRunner: NodeRunner = {
    run: async (_nodeId: string, _ctx: Context, input: unknown) => input,
  };

  return {
    triggers: mockGraph.workflow.triggers,
    graph: mockGraph,
    runner: mockRunner,
    ctx: mockCtx,
    executor: new SimpleExecutor(),
    secret: SECRET,
    ...overrides,
  };
}

// --- Tests ---

Deno.test("webhook: valid signature is accepted", async () => {
  const body = JSON.stringify({ action: "opened", pull_request: { number: 1 } });
  const sig = await makeSignature(body, SECRET);
  const req = makeRequest(body, "pull_request", sig);
  const res = await handleGitHubWebhook(req, makeOpts());
  assertEquals(res.status, 200);
  const json = await res.json();
  assertEquals(json.ok, true);
  assertEquals(json.matched, true);
});

Deno.test("webhook: invalid signature returns 401", async () => {
  const body = JSON.stringify({ action: "opened", pull_request: { number: 1 } });
  const req = makeRequest(body, "pull_request", "sha256=badhash");
  const res = await handleGitHubWebhook(req, makeOpts());
  assertEquals(res.status, 401);
});

Deno.test("webhook: missing signature returns 401 when secret configured", async () => {
  const body = JSON.stringify({ action: "opened" });
  const req = makeRequest(body, "pull_request"); // no signature header
  const res = await handleGitHubWebhook(req, makeOpts());
  assertEquals(res.status, 401);
});

Deno.test("webhook: no secret configured skips validation", async () => {
  const body = JSON.stringify({ action: "opened", pull_request: { number: 1 } });
  const req = makeRequest(body, "pull_request"); // no signature
  const opts = makeOpts({ secret: undefined });
  const res = await handleGitHubWebhook(req, opts);
  assertEquals(res.status, 200);
});

Deno.test("webhook: unmatched event returns 200 with matched=false", async () => {
  const body = JSON.stringify({ action: "created" });
  const sig = await makeSignature(body, SECRET);
  const req = makeRequest(body, "issue_comment", sig); // event not in triggers
  const res = await handleGitHubWebhook(req, makeOpts());
  assertEquals(res.status, 200);
  const json = await res.json();
  assertEquals(json.matched, false);
});

Deno.test("webhook: unmatched action returns 200 with matched=false", async () => {
  const body = JSON.stringify({ action: "closed" }); // not in actions list
  const sig = await makeSignature(body, SECRET);
  const req = makeRequest(body, "pull_request", sig);
  const res = await handleGitHubWebhook(req, makeOpts());
  assertEquals(res.status, 200);
  const json = await res.json();
  assertEquals(json.matched, false);
});

Deno.test("webhook: invalid JSON body returns 400", async () => {
  const body = "not valid json";
  const sig = await makeSignature(body, SECRET);
  const req = makeRequest(body, "pull_request", sig);
  const res = await handleGitHubWebhook(req, makeOpts());
  assertEquals(res.status, 400);
});
