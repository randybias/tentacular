import type { Context, Logger } from "../types.ts";

export type { Context, Logger };

export interface SecretsConfig {
  [service: string]: Record<string, string>;
}

export interface ContractSpec {
  dependencies: Record<string, DependencySpec>;
}

export interface DependencySpec {
  protocol: "https" | "postgresql" | "nats" | "blob";
  host: string;
  port?: number;
  auth?: {
    secret: string; // "service.key" format
  };
  // Protocol-specific fields
  database?: string;
  user?: string;
  subject?: string;
  container?: string;
}

export interface ContextOptions {
  secrets?: SecretsConfig;
  config?: Record<string, unknown>;
  nodeId?: string;
  contract?: ContractSpec;
}
