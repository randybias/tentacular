import type { Context, Logger, DependencyConnection } from "../types.ts";
import type { ContextOptions, SecretsConfig, ContractSpec } from "./types.ts";

export type { Context, ContextOptions, SecretsConfig, ContractSpec };

/** Create a Context object for a node execution */
export function createContext(opts: ContextOptions = {}): Context {
  const nodeId = opts.nodeId ?? "unknown";
  const secrets = opts.secrets ?? {};
  const config = opts.config ?? {};
  const contract = opts.contract;

  const logger = createLogger(nodeId);

  return {
    fetch: createFetch(secrets),
    log: logger,
    config,
    secrets,
    dependency: createDependencyAccessor(contract, secrets),
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

/**
 * Create a dependency accessor that resolves contract dependencies with connection metadata.
 */
function createDependencyAccessor(
  contract: ContractSpec | undefined,
  secrets: SecretsConfig,
): (name: string) => DependencyConnection {
  return (name: string): DependencyConnection => {
    if (!contract || !contract.dependencies[name]) {
      throw new Error(
        `Dependency "${name}" not declared in contract. Add it to workflow.yaml contract.dependencies.`,
      );
    }

    const dep = contract.dependencies[name];

    // Apply default ports
    const defaultPorts: Record<string, number> = {
      https: 443,
      postgresql: 5432,
      nats: 4222,
    };
    const port = dep.port ?? defaultPorts[dep.protocol] ?? 443;

    // Resolve secret if auth is declared
    let secret: string | undefined;
    let authType: "bearer-token" | "api-key" | "sas-token" | "password" | undefined;

    if (dep.auth) {
      const parts = dep.auth.secret.split(".");
      const serviceName = parts[0];
      const keyName = parts[1];
      if (serviceName && keyName) {
        secret = secrets[serviceName]?.[keyName];
      }

      // Infer auth type from key name or protocol
      if (dep.protocol === "postgresql") {
        authType = "password";
      } else if (keyName === "token") {
        authType = "bearer-token";
      } else if (keyName === "api_key") {
        authType = "api-key";
      } else if (keyName === "sas_token") {
        authType = "sas-token";
      } else {
        authType = "bearer-token"; // default for HTTPS
      }
    }

    const conn: DependencyConnection = {
      protocol: dep.protocol,
      host: dep.host,
      port,
      authType,
      secret,
      database: dep.database,
      user: dep.user,
      subject: dep.subject,
      container: dep.container,
    };

    // Add convenience fetch method for HTTPS dependencies
    if (dep.protocol === "https") {
      conn.fetch = async (path: string, init?: RequestInit): Promise<Response> => {
        const headers = new Headers(init?.headers);

        // Auto-inject auth based on authType
        if (secret) {
          if (authType === "bearer-token") {
            headers.set("Authorization", `Bearer ${secret}`);
          } else if (authType === "api-key") {
            headers.set("X-API-Key", secret);
          } else if (authType === "sas-token") {
            // SAS token appended as query param
            const separator = path.includes("?") ? "&" : "?";
            path = `${path}${separator}${secret}`;
          }
        }

        const url = `https://${dep.host}:${port}${path}`;
        return globalThis.fetch(url, {
          ...init,
          headers,
        });
      };
    }

    return conn;
  };
}
