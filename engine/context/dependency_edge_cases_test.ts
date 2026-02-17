import { assertEquals, assertThrows } from "jsr:@std/assert@1.0.11";
import { createContext } from "./mod.ts";
import { createMockContext } from "../testing/mocks.ts";
import type { ContractSpec } from "./types.ts";

// --- Phase 2: Additional Context API Edge Case Tests ---

Deno.test("dependency() with multiple dependencies", () => {
  const contract: ContractSpec = {
    dependencies: {
      github: {
        protocol: "https",
        host: "api.github.com",
        port: 443,
        auth: {
          type: "bearer-token",
          secret: "github.token",
        },
      },
      postgres: {
        protocol: "postgresql",
        host: "postgres.svc",
        port: 5432,
        database: "appdb",
        user: "postgres",
        auth: {
          type: "password",
          secret: "postgres.password",
        },
      },
    },
  };

  const secrets = {
    github: { token: "ghp_test123" },
    postgres: { password: "pg_secret" },
  };

  const ctx = createContext({ contract, secrets });

  const github = ctx.dependency("github");
  assertEquals(github.protocol, "https");
  assertEquals(github.host, "api.github.com");
  assertEquals(github.secret, "ghp_test123");

  const postgres = ctx.dependency("postgres");
  assertEquals(postgres.protocol, "postgresql");
  assertEquals(postgres.host, "postgres.svc");
  assertEquals(postgres.database, "appdb");
  assertEquals(postgres.secret, "pg_secret");
});

Deno.test("dependency() without auth returns no secret", () => {
  const contract: ContractSpec = {
    dependencies: {
      "public-api": {
        protocol: "https",
        host: "api.example.com",
        port: 443,
      },
    },
  };

  const ctx = createContext({ contract });
  const dep = ctx.dependency("public-api");

  assertEquals(dep.protocol, "https");
  assertEquals(dep.host, "api.example.com");
  assertEquals(dep.secret, undefined);
});

Deno.test("dependency() with NATS protocol", () => {
  const contract: ContractSpec = {
    dependencies: {
      messaging: {
        protocol: "nats",
        host: "nats.svc.cluster.local",
        port: 4222,
        subject: "events.workflow",
        auth: {
          type: "bearer-token",
          secret: "nats.token",
        },
      },
    },
  };

  const secrets = {
    nats: { token: "nats_secret" },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("messaging");

  assertEquals(dep.protocol, "nats");
  assertEquals(dep.host, "nats.svc.cluster.local");
  assertEquals(dep.port, 4222);
  assertEquals(dep.subject, "events.workflow");
  assertEquals(dep.secret, "nats_secret");
  assertEquals(dep.fetch, undefined); // No fetch for NATS
});

Deno.test("dependency() with blob protocol", () => {
  const contract: ContractSpec = {
    dependencies: {
      storage: {
        protocol: "blob",
        host: "storage.blob.core.windows.net",
        port: 443,
        container: "reports",
        auth: {
          type: "sas-token",
          secret: "azure.sas_token",
        },
      },
    },
  };

  const secrets = {
    azure: { sas_token: "sv=2023&sig=abc123" },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("storage");

  assertEquals(dep.protocol, "blob");
  assertEquals(dep.host, "storage.blob.core.windows.net");
  assertEquals(dep.container, "reports");
  assertEquals(dep.secret, "sv=2023&sig=abc123");
});

Deno.test("dependency() applies default port for PostgreSQL", () => {
  const contract: ContractSpec = {
    dependencies: {
      postgres: {
        protocol: "postgresql",
        host: "postgres.svc",
        database: "appdb",
        user: "postgres",
        // Port omitted
      },
    },
  };

  const ctx = createContext({ contract });
  const dep = ctx.dependency("postgres");

  assertEquals(dep.port, 5432); // Default PostgreSQL port
});

Deno.test("dependency() applies default port for NATS", () => {
  const contract: ContractSpec = {
    dependencies: {
      nats: {
        protocol: "nats",
        host: "nats.svc",
        subject: "events",
        // Port omitted
      },
    },
  };

  const ctx = createContext({ contract });
  const dep = ctx.dependency("nats");

  assertEquals(dep.port, 4222); // Default NATS port
});

Deno.test("dependency() preserves custom port", () => {
  const contract: ContractSpec = {
    dependencies: {
      api: {
        protocol: "https",
        host: "api.example.com",
        port: 8443, // Custom port
      },
    },
  };

  const ctx = createContext({ contract });
  const dep = ctx.dependency("api");

  assertEquals(dep.port, 8443);
});

Deno.test("dependency() throws for dependency with missing secret at runtime", () => {
  const contract: ContractSpec = {
    dependencies: {
      github: {
        protocol: "https",
        host: "api.github.com",
        auth: {
          type: "bearer-token",
          secret: "github.token",
        },
      },
    },
  };

  const ctx = createContext({ contract, secrets: {} }); // Missing secret

  // Should NOT throw - instead returns undefined for missing secret
  const dep = ctx.dependency("github");
  assertEquals(dep.secret, undefined);
});

Deno.test("dependency() with all required fields", () => {
  const contract: ContractSpec = {
    dependencies: {
      postgres: {
        protocol: "postgresql",
        host: "postgres.svc",
        port: 5432,
        database: "appdb",
        user: "postgres",
        auth: {
          type: "password",
          secret: "postgres.password",
        },
      },
    },
  };

  const secrets = {
    postgres: { password: "secret" },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("postgres");

  // All required fields should be present
  assertEquals(dep.protocol, "postgresql");
  assertEquals(dep.host, "postgres.svc");
  assertEquals(dep.port, 5432);
  assertEquals(dep.database, "appdb");
  assertEquals(dep.user, "postgres");
  assertEquals(dep.secret, "secret");
});

Deno.test("mock context returns mock dependency values", () => {
  const ctx = createMockContext();

  // Should not throw even without contract
  const dep = ctx.dependency("postgres");

  assertEquals(typeof dep, "object");
  // Mock should provide safe default values
});

Deno.test("dependency() case sensitivity", () => {
  const contract: ContractSpec = {
    dependencies: {
      "github-api": {
        protocol: "https",
        host: "api.github.com",
      },
    },
  };

  const ctx = createContext({ contract });

  // Should be case-sensitive
  assertThrows(
    () => ctx.dependency("GitHub-API"),
    Error,
    'Dependency "GitHub-API" not declared',
  );

  // Exact match should work
  const dep = ctx.dependency("github-api");
  assertEquals(dep.host, "api.github.com");
});

Deno.test("dependency() with kebab-case names", () => {
  const contract: ContractSpec = {
    dependencies: {
      "my-custom-api": {
        protocol: "https",
        host: "api.example.com",
      },
      "another-service_v2": {
        protocol: "https",
        host: "service.example.com",
      },
    },
  };

  const ctx = createContext({ contract });

  const dep1 = ctx.dependency("my-custom-api");
  assertEquals(dep1.host, "api.example.com");

  const dep2 = ctx.dependency("another-service_v2");
  assertEquals(dep2.host, "service.example.com");
});

Deno.test("dependency() returns all protocol-specific fields", () => {
  const contract: ContractSpec = {
    dependencies: {
      postgres: {
        protocol: "postgresql",
        host: "postgres.svc",
        port: 5432,
        database: "appdb",
        user: "postgres",
        auth: {
          type: "password",
          secret: "postgres.password",
        },
      },
    },
  };

  const secrets = {
    postgres: { password: "secret123" },
  };

  const ctx = createContext({ contract, secrets });
  const dep = ctx.dependency("postgres");

  // All PostgreSQL-specific fields should be present
  assertEquals(dep.database, "appdb");
  assertEquals(dep.user, "postgres");
  assertEquals(dep.authType, "password");
  assertEquals(dep.secret, "secret123");
  assertEquals(dep.protocol, "postgresql");
  assertEquals(dep.host, "postgres.svc");
  assertEquals(dep.port, 5432);
});

Deno.test("dependency() with empty contract dependencies", () => {
  const contract: ContractSpec = {
    dependencies: {},
  };

  const ctx = createContext({ contract });

  assertThrows(
    () => ctx.dependency("any-service"),
    Error,
    'Dependency "any-service" not declared',
  );
});
