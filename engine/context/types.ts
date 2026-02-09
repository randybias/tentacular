import type { Context, Logger } from "../types.ts";

export type { Context, Logger };

export interface SecretsConfig {
  [service: string]: Record<string, string>;
}

export interface ContextOptions {
  secrets?: SecretsConfig;
  config?: Record<string, unknown>;
  nodeId?: string;
}
