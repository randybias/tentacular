import { assertEquals, assertExists, assertThrows } from "jsr:@std/assert@1.0.11";
import { createMockContext } from "./mocks.ts";

Deno.test("contract threading: contract validates dependency declarations", () => {
  const contract = {
    version: "1",
    dependencies: {
      "test-api": {
        protocol: "https",
        host: "api.example.com",
        auth: {
          type: "bearer-token",
          secret: "api.token",
        },
      },
    },
  };

  const ctx = createMockContext({ contract });

  // Should allow access to declared dependency
  const dep = ctx.dependency("test-api");
  assertExists(dep);
  assertEquals(dep.protocol, "https");

  // Should throw for undeclared dependency
  assertThrows(
    () => ctx.dependency("undeclared-api"),
    Error,
    "not declared in contract",
  );
});

Deno.test("contract threading: mock context without contract allows all deps", () => {
  const ctx = createMockContext(); // No contract

  // Should create default mock for any dependency
  const dep = ctx.dependency("any-service");
  assertExists(dep);
  assertEquals(dep.protocol, "https");
  assertEquals(dep.host, "mock-any-service.example.com");
});

Deno.test("contract threading: multiple dependencies validated", () => {
  const contract = {
    version: "1",
    dependencies: {
      "github-api": {
        protocol: "https",
        host: "api.github.com",
      },
      postgres: {
        protocol: "postgresql",
        host: "postgres.svc",
        database: "appdb",
        user: "postgres",
      },
    },
  };

  const ctx = createMockContext({ contract });

  // Both declared dependencies should work
  const github = ctx.dependency("github-api");
  assertExists(github);
  assertEquals(github.protocol, "https");

  const pg = ctx.dependency("postgres");
  assertExists(pg);
  assertEquals(pg.protocol, "postgresql"); // Resolved from contract

  // Undeclared should fail
  assertThrows(
    () => ctx.dependency("redis"),
    Error,
    "not declared in contract",
  );
});
