/** Core engine type definitions for Tentacular */

/** Workflow specification parsed from workflow.yaml */
export interface WorkflowSpec {
  name: string;
  version: string;
  description?: string;
  triggers: Trigger[];
  nodes: Record<string, NodeSpec>;
  edges: Edge[];
  config?: WorkflowConfig;
  contract?: ContractSpec;
}

/** Contract specification for external dependencies */
export interface ContractSpec {
  version: string;
  dependencies: Record<string, DependencySpec>;
}

export interface DependencySpec {
  protocol: string;
  host: string;
  port?: number;
  auth?: {
    type: string;
    secret: string;
  };
  // Protocol-specific fields
  database?: string;
  user?: string;
  subject?: string;
  container?: string;
}

export interface Trigger {
  type: "manual" | "cron" | "webhook" | "queue";
  name?: string;
  schedule?: string;
  path?: string;
  subject?: string;
  // webhook-specific fields
  provider?: string;   // e.g. "github"
  event?: string;      // e.g. "pull_request"
  actions?: string[];  // e.g. ["opened", "synchronize", "reopened"]
}

export interface NodeSpec {
  path: string;
  capabilities?: Record<string, string>;
}

export interface Edge {
  from: string;
  to: string;
}

export interface WorkflowConfig {
  timeout?: string;
  retries?: number;
  [key: string]: unknown;
}

/** Compiled DAG — topologically sorted stages of nodes */
export interface CompiledDAG {
  workflow: WorkflowSpec;
  stages: Stage[];
  nodeOrder: string[];
}

/** A stage is a set of nodes that can execute in parallel */
export interface Stage {
  nodes: string[];
}

/** Result of a complete workflow execution */
export interface ExecutionResult {
  success: boolean;
  outputs: Record<string, unknown>;
  errors: Record<string, string>;
  timing: ExecutionTiming;
}

export interface ExecutionTiming {
  startedAt: number;
  completedAt: number;
  durationMs: number;
  nodeTimings: Record<string, NodeTiming>;
}

export interface NodeTiming {
  startedAt: number;
  completedAt: number;
  durationMs: number;
}

/** A loaded node module */
export interface NodeModule {
  default: NodeFunction;
}

/** The node function contract */
export type NodeFunction = (ctx: Context, input: unknown) => Promise<unknown>;

/** Context provided to every node at execution time */
export interface Context {
  /** Make an HTTP request through the gateway/proxy */
  fetch(service: string, path: string, init?: RequestInit): Promise<Response>;
  /** Structured logging */
  log: Logger;
  /** Node-specific configuration */
  config: Record<string, unknown>;
  /** Workflow-level secrets (loaded from file or K8s volume) */
  secrets: Record<string, Record<string, string>>;
  /** Access external service dependency with connection metadata and auth */
  dependency(name: string): DependencyConnection;
}

export interface Logger {
  info(msg: string, ...args: unknown[]): void;
  warn(msg: string, ...args: unknown[]): void;
  error(msg: string, ...args: unknown[]): void;
  debug(msg: string, ...args: unknown[]): void;
}

/** Dependency connection metadata resolved from contract */
export interface DependencyConnection {
  protocol: string;
  host: string;
  port: number;
  authType?: string;
  secret?: string;
  // Protocol-specific fields
  database?: string; // postgresql
  user?: string;     // postgresql
  subject?: string;  // nats
  container?: string; // blob
  // Convenience method for HTTPS dependencies
  fetch?(path: string, init?: RequestInit): Promise<Response>;
}
