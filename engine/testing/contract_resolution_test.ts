import { assertEquals } from "https://deno.land/std@0.208.0/assert/mod.ts";
import { createMockContext } from "./mocks.ts";

Deno.test("Mock context resolves dependency from contract spec", () => {
  const contract = {
    version: "1",
    dependencies: {
      "test-api": {
        protocol: "https",
        host: "api.example.com",
        port: 443,
        auth: {
          type: "bearer-token",
          secret: "secrets.api_key",
        },
      },
      "test-db": {
        protocol: "postgresql",
        host: "db.internal.local",
        database: "mydb",
        user: "app",
        auth: {
          type: "password",
          secret: "db.password",
        },
      },
    },
  };

  const secrets = {
    secrets: { api_key: "test-key-123" },
    db: { password: "test-db-pass" },
  };

  const ctx = createMockContext({ contract, secrets });

  // Resolve test-api dependency
  const apiDep = ctx.dependency("test-api");
  assertEquals(apiDep.protocol, "https");
  assertEquals(apiDep.host, "api.example.com");
  assertEquals(apiDep.port, 443);
  assertEquals(apiDep.authType, "bearer-token");
  assertEquals(apiDep.secret, "test-key-123"); // Resolved from secrets

  // Resolve test-db dependency
  const dbDep = ctx.dependency("test-db");
  assertEquals(dbDep.protocol, "postgresql");
  assertEquals(dbDep.host, "db.internal.local");
  assertEquals(dbDep.port, 5432); // Default PostgreSQL port
  assertEquals(dbDep.database, "mydb");
  assertEquals(dbDep.user, "app");
  assertEquals(dbDep.authType, "password");
  assertEquals(dbDep.secret, "test-db-pass"); // Resolved from secrets
});

Deno.test("Mock context applies default ports based on protocol", () => {
  const contract = {
    version: "1",
    dependencies: {
      "https-api": {
        protocol: "https",
        host: "api.example.com",
      },
      "pg-db": {
        protocol: "postgresql",
        host: "db.example.com",
        database: "test",
        user: "app",
      },
      "nats-queue": {
        protocol: "nats",
        host: "nats.example.com",
        subject: "events",
      },
    },
  };

  const ctx = createMockContext({ contract });

  assertEquals(ctx.dependency("https-api").port, 443);
  assertEquals(ctx.dependency("pg-db").port, 5432);
  assertEquals(ctx.dependency("nats-queue").port, 4222);
});

Deno.test("Mock context throws on undeclared dependency when contract present", () => {
  const contract = {
    version: "1",
    dependencies: {
      "declared-api": {
        protocol: "https",
        host: "api.example.com",
      },
    },
  };

  const ctx = createMockContext({ contract });

  // Should work for declared dependency
  ctx.dependency("declared-api");

  // Should throw for undeclared dependency
  try {
    ctx.dependency("undeclared-api");
    throw new Error("Expected error for undeclared dependency");
  } catch (err) {
    assertEquals(
      (err as Error).message,
      'Dependency "undeclared-api" not declared in contract. Add it to workflow.yaml contract.dependencies.'
    );
  }
});

Deno.test("Mock context falls back to default mock when no contract", () => {
  const ctx = createMockContext();

  const dep = ctx.dependency("any-service");
  assertEquals(dep.protocol, "https");
  assertEquals(dep.host, "mock-any-service.example.com");
  assertEquals(dep.port, 443);
  assertEquals(dep.authType, "test-auth");
});

Deno.test("Mock context provides fetch convenience for HTTPS dependencies", async () => {
  const contract = {
    version: "1",
    dependencies: {
      "api": {
        protocol: "https",
        host: "api.example.com",
        auth: {
          type: "api-key",
          secret: "secrets.key",
        },
      },
    },
  };

  const ctx = createMockContext({ contract });
  const dep = ctx.dependency("api");

  // HTTPS dependencies should have fetch method
  assertEquals(typeof dep.fetch, "function");

  // Fetch should return mock response
  const response = await dep.fetch!("/test");
  const data = await response.json();
  assertEquals(data, { mock: true, dependency: "api", path: "/test" });

  // Fetch should be tracked for drift detection
  assertEquals(ctx._dependencyAccesses.length, 1);
  assertEquals(ctx._dependencyAccesses[0]!.name, "api");
  assertEquals(ctx._dependencyAccesses[0]!.fetches, ["/test"]);
});
