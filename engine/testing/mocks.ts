import type { Context, Logger } from "../types.ts";

/** Log entry captured by the mock context */
export interface LogEntry {
  level: string;
  msg: string;
  args: unknown[];
}

/** Extended mock context that exposes captured logs and fetch response registration */
export interface MockContext extends Context {
  /** All log entries captured during test execution */
  _logs: LogEntry[];
  /** Register a mock fetch response for a given service:path key */
  _setFetchResponse(service: string, path: string, response: Response): void;
}

/** Create a mock Context for testing nodes in isolation */
export function createMockContext(overrides?: Partial<Context>): MockContext {
  const logs: LogEntry[] = [];

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
    _logs: logs,
    _setFetchResponse(service: string, path: string, response: Response) {
      fetchResponses.set(`${service}:${path}`, response);
    },
    ...overrides,
  };

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
