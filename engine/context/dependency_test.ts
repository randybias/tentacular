import { assertEquals, assertThrows } from "jsr:@std/assert@1.0.11";
import { createContext } from "./mod.ts";
import type { ContractSpec } from "./types.ts";

Deno.test("dependency() returns connection metadata for HTTPS dependency", () => {
  const contract: ContractSpec = {
    dependencies: {
      "github-api": {
        protocol: "https",
        host: "api.github.com",
        port: 443,
        auth: {
          secret: "github.token",
        },
      },
    },
  };

  const secrets = {
    github: {
      token: "ghp_test123",
    },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("github-api");

  assertEquals(dep.protocol, "https");
  assertEquals(dep.host, "api.github.com");
  assertEquals(dep.port, 443);
  assertEquals(dep.authType, "bearer-token");
  assertEquals(dep.secret, "ghp_test123");
  assertEquals(typeof dep.fetch, "function");
});

Deno.test("dependency() applies default port for HTTPS", () => {
  const contract: ContractSpec = {
    dependencies: {
      api: {
        protocol: "https",
        host: "api.example.com",
      },
    },
  };

  const ctx = createContext({ contract });
  const dep = ctx.dependency("api");

  assertEquals(dep.port, 443);
});

Deno.test("dependency() returns PostgreSQL connection metadata", () => {
  const contract: ContractSpec = {
    dependencies: {
      postgres: {
        protocol: "postgresql",
        host: "postgres.svc.cluster.local",
        port: 5432,
        database: "appdb",
        user: "postgres",
        auth: {
          secret: "postgres.password",
        },
      },
    },
  };

  const secrets = {
    postgres: {
      password: "secret123",
    },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("postgres");

  assertEquals(dep.protocol, "postgresql");
  assertEquals(dep.host, "postgres.svc.cluster.local");
  assertEquals(dep.port, 5432);
  assertEquals(dep.database, "appdb");
  assertEquals(dep.user, "postgres");
  assertEquals(dep.authType, "password");
  assertEquals(dep.secret, "secret123");
  assertEquals(dep.fetch, undefined); // No fetch for non-HTTPS
});

Deno.test("dependency() throws for undeclared dependency", () => {
  const contract: ContractSpec = {
    dependencies: {},
  };

  const ctx = createContext({ contract });

  assertThrows(
    () => ctx.dependency("unknown-service"),
    Error,
    'Dependency "unknown-service" not declared in contract',
  );
});

Deno.test("dependency() throws when no contract", () => {
  const ctx = createContext({});

  assertThrows(
    () => ctx.dependency("github-api"),
    Error,
    'Dependency "github-api" not declared in contract',
  );
});

Deno.test("dependency().fetch() auto-injects bearer token", async () => {
  const contract: ContractSpec = {
    dependencies: {
      "github-api": {
        protocol: "https",
        host: "api.github.com",
        port: 443,
        auth: {
          secret: "github.token",
        },
      },
    },
  };

  const secrets = {
    github: {
      token: "ghp_test123",
    },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("github-api");

  // Mock globalThis.fetch to verify headers
  const originalFetch = globalThis.fetch;
  let capturedHeaders: Headers | undefined;
  let capturedUrl: string | undefined;

  globalThis.fetch = async (input: string | URL | Request, init?: RequestInit) => {
    capturedUrl = typeof input === "string" ? input : input.toString();
    capturedHeaders = new Headers(init?.headers);
    return new Response(JSON.stringify({ ok: true }), { status: 200 });
  };

  try {
    await dep.fetch!("/repos/test/repo");

    assertEquals(capturedUrl, "https://api.github.com:443/repos/test/repo");
    assertEquals(capturedHeaders?.get("Authorization"), "Bearer ghp_test123");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

Deno.test("dependency().fetch() auto-injects API key", async () => {
  const contract: ContractSpec = {
    dependencies: {
      "external-api": {
        protocol: "https",
        host: "api.example.com",
        auth: {
          secret: "external.api_key",
        },
      },
    },
  };

  const secrets = {
    external: {
      api_key: "key_12345",
    },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("external-api");

  const originalFetch = globalThis.fetch;
  let capturedHeaders: Headers | undefined;

  globalThis.fetch = async (_input: string | URL | Request, init?: RequestInit) => {
    capturedHeaders = new Headers(init?.headers);
    return new Response(JSON.stringify({ ok: true }), { status: 200 });
  };

  try {
    await dep.fetch!("/endpoint");
    assertEquals(capturedHeaders?.get("X-API-Key"), "key_12345");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

Deno.test("dependency().fetch() appends SAS token as query param", async () => {
  const contract: ContractSpec = {
    dependencies: {
      "azure-blob": {
        protocol: "https",
        host: "storage.blob.core.windows.net",
        auth: {
          secret: "azure.sas_token",
        },
      },
    },
  };

  const secrets = {
    azure: {
      sas_token: "sv=2023&sig=abc123",
    },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("azure-blob");

  const originalFetch = globalThis.fetch;
  let capturedUrl: string | undefined;

  globalThis.fetch = async (input: string | URL | Request) => {
    capturedUrl = typeof input === "string" ? input : input.toString();
    return new Response(JSON.stringify({ ok: true }), { status: 200 });
  };

  try {
    await dep.fetch!("/container/blob");
    assertEquals(capturedUrl, "https://storage.blob.core.windows.net:443/container/blob?sv=2023&sig=abc123");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

Deno.test("dependency() handles missing secrets gracefully", () => {
  const contract: ContractSpec = {
    dependencies: {
      "github-api": {
        protocol: "https",
        host: "api.github.com",
        auth: {
          secret: "github.token",
        },
      },
    },
  };

  const ctx = createContext({ contract, secrets: {} });
  const dep = ctx.dependency("github-api");

  assertEquals(dep.secret, undefined);
});
