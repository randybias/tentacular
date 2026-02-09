import { assertEquals, assertExists } from "std/assert";
import { createContext } from "./mod.ts";

// --- createContext returns all four members ---

Deno.test("createContext: returns object with fetch, log, config, secrets", () => {
  const ctx = createContext({ nodeId: "n1" });
  assertEquals(typeof ctx.fetch, "function");
  assertExists(ctx.log);
  assertExists(ctx.config);
  assertExists(ctx.secrets);
});

// --- Logger prefix ---

Deno.test("createContext: logger prefixes output with node ID", () => {
  const logged: string[] = [];
  const origLog = console.log;
  console.log = (...args: unknown[]) => {
    logged.push(args.map(String).join(" "));
  };

  const ctx = createContext({ nodeId: "my-node" });
  ctx.log.info("hello");

  console.log = origLog;

  assertEquals(logged.length, 1);
  assertEquals(logged[0]!.includes("[my-node]"), true);
  assertEquals(logged[0]!.includes("INFO"), true);
});

Deno.test("createContext: logger debug uses console.debug", () => {
  const logged: string[] = [];
  const origDebug = console.debug;
  console.debug = (...args: unknown[]) => {
    logged.push(args.map(String).join(" "));
  };

  const ctx = createContext({ nodeId: "dbg" });
  ctx.log.debug("test debug");

  console.debug = origDebug;

  assertEquals(logged.length, 1);
  assertEquals(logged[0]!.includes("[dbg]"), true);
  assertEquals(logged[0]!.includes("DEBUG"), true);
});

// --- Fetch URL resolution ---

Deno.test("createContext: fetch resolves convention-based URL", async () => {
  let capturedUrl = "";
  const origFetch = globalThis.fetch;
  globalThis.fetch = (input: string | URL | Request, _init?: RequestInit): Promise<Response> => {
    capturedUrl = typeof input === "string" ? input : input.toString();
    return Promise.resolve(new Response("ok"));
  };

  const ctx = createContext({});
  await ctx.fetch("github", "/repos/foo/bar");

  globalThis.fetch = origFetch;

  assertEquals(capturedUrl, "https://api.github.com/repos/foo/bar");
});

Deno.test("createContext: fetch passes through full URL", async () => {
  let capturedUrl = "";
  const origFetch = globalThis.fetch;
  globalThis.fetch = (input: string | URL | Request, _init?: RequestInit): Promise<Response> => {
    capturedUrl = typeof input === "string" ? input : input.toString();
    return Promise.resolve(new Response("ok"));
  };

  const ctx = createContext({});
  await ctx.fetch("slack", "https://hooks.slack.com/services/T00/B00/xxx");

  globalThis.fetch = origFetch;

  assertEquals(capturedUrl, "https://hooks.slack.com/services/T00/B00/xxx");
});

// --- Fetch auth injection ---

Deno.test("createContext: fetch injects Bearer token from secrets", async () => {
  let capturedHeaders: Headers | undefined;
  const origFetch = globalThis.fetch;
  globalThis.fetch = (_input: string | URL | Request, init?: RequestInit): Promise<Response> => {
    capturedHeaders = new Headers(init?.headers);
    return Promise.resolve(new Response("ok"));
  };

  const ctx = createContext({
    secrets: { github: { token: "ghp_abc123" } },
  });
  await ctx.fetch("github", "/user");

  globalThis.fetch = origFetch;

  assertEquals(capturedHeaders!.get("Authorization"), "Bearer ghp_abc123");
});

Deno.test("createContext: fetch injects X-API-Key from secrets", async () => {
  let capturedHeaders: Headers | undefined;
  const origFetch = globalThis.fetch;
  globalThis.fetch = (_input: string | URL | Request, init?: RequestInit): Promise<Response> => {
    capturedHeaders = new Headers(init?.headers);
    return Promise.resolve(new Response("ok"));
  };

  const ctx = createContext({
    secrets: { stripe: { api_key: "sk_test_xyz" } },
  });
  await ctx.fetch("stripe", "/v1/charges");

  globalThis.fetch = origFetch;

  assertEquals(capturedHeaders!.get("X-API-Key"), "sk_test_xyz");
});

Deno.test("createContext: fetch sends no auth when no secrets for service", async () => {
  let capturedHeaders: Headers | undefined;
  const origFetch = globalThis.fetch;
  globalThis.fetch = (_input: string | URL | Request, init?: RequestInit): Promise<Response> => {
    capturedHeaders = new Headers(init?.headers);
    return Promise.resolve(new Response("ok"));
  };

  const ctx = createContext({ secrets: {} });
  await ctx.fetch("github", "/repos");

  globalThis.fetch = origFetch;

  assertEquals(capturedHeaders!.get("Authorization"), null);
  assertEquals(capturedHeaders!.get("X-API-Key"), null);
});

// --- Custom headers preserved alongside injected auth ---

Deno.test("createContext: fetch preserves custom headers alongside injected auth", async () => {
  let capturedHeaders: Headers | undefined;
  const origFetch = globalThis.fetch;
  globalThis.fetch = (_input: string | URL | Request, init?: RequestInit): Promise<Response> => {
    capturedHeaders = new Headers(init?.headers);
    return Promise.resolve(new Response("ok"));
  };

  const ctx = createContext({
    secrets: { github: { token: "ghp_test" } },
  });
  await ctx.fetch("github", "/repos", {
    headers: { "Content-Type": "application/json", "Accept": "application/vnd.github.v3+json" },
  });

  globalThis.fetch = origFetch;

  assertEquals(capturedHeaders!.get("Authorization"), "Bearer ghp_test");
  assertEquals(capturedHeaders!.get("Content-Type"), "application/json");
  assertEquals(capturedHeaders!.get("Accept"), "application/vnd.github.v3+json");
});

// --- Config and secrets ---

Deno.test("createContext: provides config", () => {
  const ctx = createContext({ config: { key: "value" } });
  assertEquals(ctx.config["key"], "value");
});

Deno.test("createContext: provides secrets", () => {
  const ctx = createContext({
    secrets: {
      github: { token: "ghp_test" },
    },
  });
  assertEquals(ctx.secrets["github"]?.["token"], "ghp_test");
});

Deno.test("createContext: defaults to empty config and secrets", () => {
  const ctx = createContext();
  assertEquals(Object.keys(ctx.config).length, 0);
  assertEquals(Object.keys(ctx.secrets).length, 0);
});
