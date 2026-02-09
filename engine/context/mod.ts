import type { Context, Logger } from "../types.ts";
import type { ContextOptions, SecretsConfig } from "./types.ts";

export type { Context, ContextOptions, SecretsConfig };

/** Create a Context object for a node execution */
export function createContext(opts: ContextOptions = {}): Context {
  const nodeId = opts.nodeId ?? "unknown";
  const secrets = opts.secrets ?? {};
  const config = opts.config ?? {};

  const logger = createLogger(nodeId);

  return {
    fetch: createFetch(secrets),
    log: logger,
    config,
    secrets,
  };
}

function createLogger(nodeId: string): Logger {
  const prefix = `[${nodeId}]`;
  return {
    info(msg: string, ...args: unknown[]) {
      console.log(`${prefix} INFO`, msg, ...args);
    },
    warn(msg: string, ...args: unknown[]) {
      console.warn(`${prefix} WARN`, msg, ...args);
    },
    error(msg: string, ...args: unknown[]) {
      console.error(`${prefix} ERROR`, msg, ...args);
    },
    debug(msg: string, ...args: unknown[]) {
      console.debug(`${prefix} DEBUG`, msg, ...args);
    },
  };
}

/**
 * Create a fetch function that resolves service names to URLs and injects auth.
 * Currently does direct HTTP; future: route through Gateway proxy.
 */
function createFetch(
  secrets: SecretsConfig,
): (service: string, path: string, init?: RequestInit) => Promise<Response> {
  return async (service: string, path: string, init?: RequestInit): Promise<Response> => {
    const serviceSecrets = secrets[service];
    const headers = new Headers(init?.headers);

    // Inject auth token if available in secrets
    if (serviceSecrets) {
      if (serviceSecrets["token"]) {
        headers.set("Authorization", `Bearer ${serviceSecrets["token"]}`);
      }
      if (serviceSecrets["api_key"]) {
        headers.set("X-API-Key", serviceSecrets["api_key"]!);
      }
    }

    // Build full URL — if path starts with http, use as-is
    const url = path.startsWith("http") ? path : `https://api.${service}.com${path}`;

    return globalThis.fetch(url, {
      ...init,
      headers,
    });
  };
}
