/**
 * Tentacular Engine — Public API
 *
 * This is the module that workflow nodes import from:
 *   import type { Context } from "tentacular";
 */

// Core types for node authors
export type { Context, Logger, NodeFunction, NodeModule } from "./types.ts";

// Execution types (for advanced users)
export type { CompiledDAG, Stage, ExecutionResult, WorkflowSpec, NodeSpec, Edge, Trigger } from "./types.ts";

// Executor interface (for extending)
export type { WorkflowExecutor, NodeRunner } from "./executor/types.ts";
