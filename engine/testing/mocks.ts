import type { Context, Logger, DependencyConnection } from "../types.ts";

/** Log entry captured by the mock context */
export interface LogEntry {
  level: string;
  msg: string;
  args: unknown[];
}

/** Dependency access record for runtime tracing */
export interface DependencyAccess {
  name: string;
  fetches: string[]; // paths fetched via dep.fetch()
}

/** Extended mock context that exposes captured logs and fetch response registration */
export interface MockContext extends Context {
  /** All log entries captured during test execution */
  _logs: LogEntry[];
  /** All dependency accesses for drift detection */
  _dependencyAccesses: DependencyAccess[];
  /** Direct ctx.fetch() calls (bypass tracking) */
  _fetchCalls: { service: string; path: string }[];
  /** Direct ctx.secrets property accesses (bypass tracking) */
  _secretsAccesses: string[];
  /** Register a mock fetch response for a given service:path key */
  _setFetchResponse(service: string, path: string, response: Response): void;
  /** Register a mock dependency */
  _setDependency(name: string, dep: DependencyConnection): void;
}

interface CreateMockContextOptions {
  config?: Record<string, unknown>;
  secrets?: Record<string, Record<string, string>>;
  contract?: { version: string; dependencies: Record<string, unknown> };
}

/** Create a mock Context for testing nodes in isolation */
export function createMockContext(options?: CreateMockContextOptions): MockContext {
  const logs: LogEntry[] = [];
  const dependencyAccesses: DependencyAccess[] = [];
  const fetchCalls: { service: string; path: string }[] = [];
  const secretsAccesses: string[] = [];
  const mockDependencies = new Map<string, DependencyConnection>();
  const contractSpec = options?.contract;

  const logger: Logger = {
    info(msg: string, ...args: unknown[]) {
      logs.push({ level: "info", msg, args });
    },
    warn(msg: string, ...args: unknown[]) {
      logs.push({ level: "warn", msg, args });
    },
    error(msg: string, ...args: unknown[]) {
      logs.push({ level: "error", msg, args });
    },
    debug(msg: string, ...args: unknown[]) {
      logs.push({ level: "debug", msg, args });
    },
  };

  const fetchResponses = new Map<string, Response>();

  const ctx: MockContext = {
    fetch: async (service: string, path: string, _init?: RequestInit): Promise<Response> => {
      // Record direct fetch call for bypass detection
      fetchCalls.push({ service, path });

      const key = `${service}:${path}`;
      const mockResponse = fetchResponses.get(key);
      if (mockResponse) return mockResponse;

      return new Response(JSON.stringify({ mock: true, service, path }), {
        headers: { "content-type": "application/json" },
      });
    },
    log: logger,
    config: {},
    secrets: {},
    dependency: (name: string): DependencyConnection => {
      // Record access for drift detection
      let access = dependencyAccesses.find((a) => a.name === name);
      if (!access) {
        access = { name, fetches: [] };
        dependencyAccesses.push(access);
      }

      // Check if contract declares this dependency (for strict enforcement)
      if (contractSpec && !contractSpec.dependencies[name]) {
        throw new Error(
          `Dependency "${name}" not declared in contract. Add it to workflow.yaml contract.dependencies.`
        );
      }

      // Return registered mock or resolve from contract
      const mockDep = mockDependencies.get(name);
      if (mockDep) {
        return mockDep;
      }

      // Resolve from contract if available
      if (contractSpec && contractSpec.dependencies[name]) {
        const depSpec = contractSpec.dependencies[name] as {
          protocol: string;
          host: string;
          port?: number;
          auth?: { type: string; secret: string };
          database?: string;
          user?: string;
          subject?: string;
          container?: string;
        };

        // Apply default ports based on protocol
        const defaultPorts: Record<string, number> = {
          https: 443,
          http: 80,
          postgresql: 5432,
          nats: 4222,
          blob: 443,
        };
        const port = depSpec.port ?? defaultPorts[depSpec.protocol] ?? 443;

        // Resolve secret value from dot-notation key path
        // e.g. "github.token" → options.secrets["github"]["token"]
        let resolvedSecret: string | undefined;
        if (depSpec.auth?.secret && options?.secrets) {
          const parts = depSpec.auth.secret.split(".");
          if (parts.length === 2 && parts[0] && parts[1]) {
            resolvedSecret = options.secrets[parts[0]]?.[parts[1]];
          }
        }

        // Build dependency connection from contract spec
        const isHttpLike = depSpec.protocol === "https" || depSpec.protocol === "http";
        const contractDep: DependencyConnection = {
          protocol: depSpec.protocol,
          host: depSpec.host,
          port,
          authType: depSpec.auth?.type,
          secret: resolvedSecret,
          database: depSpec.database,
          user: depSpec.user,
          subject: depSpec.subject,
          container: depSpec.container,
        };

        // Add fetch convenience method for HTTP-like dependencies
        if (isHttpLike) {
          contractDep.fetch = async (path: string, _init?: RequestInit): Promise<Response> => {
            access!.fetches.push(path);
            return new Response(JSON.stringify({ mock: true, dependency: name, path }), {
              headers: { "content-type": "application/json" },
            });
          };
        }

        return contractDep;
      }

      // Fallback to default mock if no contract
      const defaultMock: DependencyConnection = {
        protocol: "https",
        host: `mock-${name}.example.com`,
        port: 443,
        authType: "test-auth",
        secret: undefined,
        fetch: async (path: string, _init?: RequestInit): Promise<Response> => {
          access!.fetches.push(path);
          return new Response(JSON.stringify({ mock: true, dependency: name, path }), {
            headers: { "content-type": "application/json" },
          });
        },
      };

      return defaultMock;
    },
    _logs: logs,
    _dependencyAccesses: dependencyAccesses,
    _fetchCalls: fetchCalls,
    _secretsAccesses: secretsAccesses,
    _setFetchResponse(service: string, path: string, response: Response) {
      fetchResponses.set(`${service}:${path}`, response);
    },
    _setDependency(name: string, dep: DependencyConnection) {
      mockDependencies.set(name, dep);
    },
  };

  // Apply options
  if (options) {
    if (options.config) ctx.config = options.config;
    if (options.secrets) ctx.secrets = options.secrets;
  }

  // Wrap secrets in a recording Proxy for bypass detection
  const rawSecrets = ctx.secrets;
  ctx.secrets = new Proxy(rawSecrets, {
    get(target, prop: string) {
      if (typeof prop === "string") {
        secretsAccesses.push(prop);
      }
      return target[prop];
    },
  });

  return ctx;
}

/** Helper to get captured logs from a mock context */
export function getLogs(ctx: Context): LogEntry[] {
  if ("_logs" in ctx) {
    return (ctx as MockContext)._logs;
  }
  return [];
}

/** Create a mock fetch response for testing */
export function mockFetchResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}
