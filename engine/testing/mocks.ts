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

/** Create a mock Context for testing nodes in isolation */
export function createMockContext(overrides?: Partial<Context>): MockContext {
  const logs: LogEntry[] = [];
  const dependencyAccesses: DependencyAccess[] = [];
  const fetchCalls: { service: string; path: string }[] = [];
  const secretsAccesses: string[] = [];
  const mockDependencies = new Map<string, DependencyConnection>();

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

      // Return registered mock or create default mock
      const mockDep = mockDependencies.get(name);
      if (mockDep) {
        return mockDep;
      }

      // Default mock dependency
      const defaultMock: DependencyConnection = {
        protocol: "https",
        host: `mock-${name}.example.com`,
        port: 443,
        authType: "bearer-token",
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

  // Apply overrides without clobbering dependency tracking wrapper or internal fields
  if (overrides) {
    if (overrides.config) ctx.config = overrides.config;
    if (overrides.secrets) ctx.secrets = overrides.secrets;
    if (overrides.log) ctx.log = overrides.log;
    // fetch override wraps to preserve tracking
    if (overrides.fetch) {
      const originalOverrideFetch = overrides.fetch;
      ctx.fetch = async (service: string, path: string, init?: RequestInit) => {
        fetchCalls.push({ service, path });
        return originalOverrideFetch(service, path, init);
      };
    }
    // dependency() is never overridden — drift detection tracking must be preserved
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
