import { assertEquals } from "jsr:@std/assert@1.0.11";
import { createMockContext } from "./mocks.ts";

Deno.test("mock context: supports custom auth types", () => {
  const ctx = createMockContext();

  // Register a mock dependency with custom auth type
  ctx._setDependency("hmac-api", {
    protocol: "https",
    host: "api.example.com",
    port: 443,
    authType: "hmac-sha256",
    secret: "test-key",
    fetch: async (path: string) => {
      return new Response(JSON.stringify({ mock: true, path }), {
        headers: { "content-type": "application/json" },
      });
    },
  });

  const dep = ctx.dependency("hmac-api");

  assertEquals(dep.authType, "hmac-sha256");
  assertEquals(dep.secret, "test-key");
});

Deno.test("mock context: supports OAuth2 auth type", () => {
  const ctx = createMockContext();

  ctx._setDependency("oauth-api", {
    protocol: "https",
    host: "oauth.example.com",
    port: 443,
    authType: "oauth2-client-credentials",
    secret: "client-secret-abc",
    fetch: async (path: string) => {
      return new Response(JSON.stringify({ mock: true, path }), {
        headers: { "content-type": "application/json" },
      });
    },
  });

  const dep = ctx.dependency("oauth-api");

  assertEquals(dep.authType, "oauth2-client-credentials");
  assertEquals(dep.secret, "client-secret-abc");
});

Deno.test("mock context: fetch does not auto-inject auth headers", async () => {
  const ctx = createMockContext();

  ctx._setDependency("api", {
    protocol: "https",
    host: "api.example.com",
    port: 443,
    authType: "bearer-token",
    secret: "test-token",
    fetch: async (path: string, init?: RequestInit) => {
      // Verify no Authorization header was added
      const headers = new Headers(init?.headers);
      assertEquals(headers.get("Authorization"), null);

      return new Response(JSON.stringify({ mock: true, path }), {
        headers: { "content-type": "application/json" },
      });
    },
  });

  const dep = ctx.dependency("api");
  await dep.fetch!("/test");
});

Deno.test("mock context: default mock works without auth type", () => {
  const ctx = createMockContext();

  // Access a dependency that hasn't been registered
  const dep = ctx.dependency("unregistered-api");

  assertEquals(dep.protocol, "https");
  assertEquals(dep.host, "mock-unregistered-api.example.com");
  assertEquals(dep.port, 443);
  assertEquals(dep.authType, "test-auth"); // Default mock auth type
  assertEquals(dep.secret, undefined); // No secret in mock
  assertEquals(typeof dep.fetch, "function");
});

Deno.test("mock context: records dependency accesses for drift detection", async () => {
  const ctx = createMockContext();

  const dep = ctx.dependency("test-api");
  await dep.fetch!("/endpoint1");
  await dep.fetch!("/endpoint2");

  // Verify accesses were recorded
  assertEquals(ctx._dependencyAccesses.length, 1);
  const access = ctx._dependencyAccesses[0];
  assertEquals(access?.name, "test-api");
  assertEquals(access?.fetches, ["/endpoint1", "/endpoint2"]);
});
