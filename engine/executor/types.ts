import type { CompiledDAG, Context, ExecutionResult } from "../types.ts";

/**
 * WorkflowExecutor — the pluggable executor interface.
 * SimpleExecutor implements this now; TemporalExecutor can be swapped in later.
 */
export interface WorkflowExecutor {
  execute(graph: CompiledDAG, nodeRunner: NodeRunner, ctx: Context, input?: unknown): Promise<ExecutionResult>;
}

/**
 * NodeRunner — loads and runs individual nodes.
 * Nodes are "activities" in Temporal terms.
 */
export interface NodeRunner {
  run(nodeId: string, ctx: Context, input: unknown): Promise<unknown>;
}
