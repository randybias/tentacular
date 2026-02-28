import type { CompiledDAG, Context, ExecutionResult, ExecutionTiming, NodeTiming } from "../types.ts";
import type { NodeRunner, WorkflowExecutor } from "./types.ts";
import type { TelemetrySink } from "../telemetry/mod.ts";
import { NoopSink } from "../telemetry/mod.ts";

/**
 * SimpleExecutor — lightweight in-memory DAG executor.
 * Executes stages in order, with nodes within a stage running in parallel via Promise.all.
 */
export class SimpleExecutor implements WorkflowExecutor {
  private timeoutMs: number;
  private maxRetries: number;
  private sink: TelemetrySink;

  constructor(opts?: { timeoutMs?: number; maxRetries?: number; sink?: TelemetrySink }) {
    this.timeoutMs = opts?.timeoutMs ?? 30_000;
    this.maxRetries = opts?.maxRetries ?? 0;
    this.sink = opts?.sink ?? new NoopSink();
  }

  async execute(graph: CompiledDAG, runner: NodeRunner, ctx: Context, input?: unknown): Promise<ExecutionResult> {
    const startedAt = Date.now();
    const outputs: Record<string, unknown> = {};
    const errors: Record<string, string> = {};
    const nodeTimings: Record<string, NodeTiming> = {};

    // Build input mapping: which nodes feed into which
    const inputMap = this.buildInputMap(graph);

    for (const stage of graph.stages) {
      const stageResults = await Promise.all(
        stage.nodes.map(async (nodeId) => {
          const nodeStart = Date.now();
          this.sink.record({ type: "node-start", timestamp: nodeStart, metadata: { node: nodeId } });
          try {
            // Build input for this node from its dependencies' outputs
            const nodeInput = this.resolveInput(nodeId, inputMap, outputs, input);

            // Execute with timeout and retries
            const output = await this.executeWithRetry(
              () => this.executeWithTimeout(runner, nodeId, ctx, nodeInput),
              this.maxRetries,
            );

            outputs[nodeId] = output;
            const nodeEnd = Date.now();
            const durationMs = nodeEnd - nodeStart;
            nodeTimings[nodeId] = {
              startedAt: nodeStart,
              completedAt: nodeEnd,
              durationMs,
            };
            this.sink.record({ type: "node-complete", timestamp: nodeEnd, metadata: { node: nodeId, durationMs } });
            return { nodeId, success: true };
          } catch (err) {
            const nodeEnd = Date.now();
            const errMsg = err instanceof Error ? err.message : String(err);
            errors[nodeId] = errMsg;
            nodeTimings[nodeId] = {
              startedAt: nodeStart,
              completedAt: nodeEnd,
              durationMs: nodeEnd - nodeStart,
            };
            this.sink.record({ type: "node-error", timestamp: nodeEnd, metadata: { node: nodeId, error: errMsg } });
            return { nodeId, success: false, error: errMsg };
          }
        }),
      );

      // Fail-fast: if any node in a stage fails, stop execution
      const failed = stageResults.filter((r) => !r.success);
      if (failed.length > 0) {
        break;
      }
    }

    const completedAt = Date.now();
    const timing: ExecutionTiming = {
      startedAt,
      completedAt,
      durationMs: completedAt - startedAt,
      nodeTimings,
    };

    return {
      success: Object.keys(errors).length === 0,
      outputs,
      errors,
      timing,
    };
  }

  private buildInputMap(graph: CompiledDAG): Map<string, string[]> {
    const inputs = new Map<string, string[]>();
    for (const edge of graph.workflow.edges) {
      if (!inputs.has(edge.to)) inputs.set(edge.to, []);
      inputs.get(edge.to)!.push(edge.from);
    }
    return inputs;
  }

  private resolveInput(
    nodeId: string,
    inputMap: Map<string, string[]>,
    outputs: Record<string, unknown>,
    initialInput?: unknown,
  ): unknown {
    const deps = inputMap.get(nodeId);
    if (!deps || deps.length === 0) return initialInput ?? {};
    if (deps.length === 1) return outputs[deps[0]!];
    // Multiple inputs: merge into a keyed object
    const merged: Record<string, unknown> = {};
    for (const dep of deps) {
      merged[dep] = outputs[dep];
    }
    return merged;
  }

  private executeWithTimeout(
    runner: NodeRunner,
    nodeId: string,
    ctx: Context,
    input: unknown,
  ): Promise<unknown> {
    return new Promise<unknown>((resolve, reject) => {
      let settled = false;
      const timer = setTimeout(() => {
        if (!settled) {
          settled = true;
          reject(new Error(`Node "${nodeId}" timed out after ${this.timeoutMs}ms`));
        }
      }, this.timeoutMs);

      runner.run(nodeId, ctx, input).then(
        (result) => {
          if (!settled) {
            settled = true;
            clearTimeout(timer);
            resolve(result);
          }
        },
        (err) => {
          if (!settled) {
            settled = true;
            clearTimeout(timer);
            reject(err);
          }
        },
      );
    });
  }

  private async executeWithRetry(
    fn: () => Promise<unknown>,
    maxRetries: number,
  ): Promise<unknown> {
    let lastError: Error | undefined;
    for (let attempt = 0; attempt <= maxRetries; attempt++) {
      try {
        return await fn();
      } catch (err) {
        lastError = err instanceof Error ? err : new Error(String(err));
        if (attempt < maxRetries) {
          // Exponential backoff: 100ms, 200ms, 400ms, ...
          const delay = 100 * Math.pow(2, attempt);
          await new Promise((resolve) => setTimeout(resolve, delay));
        }
      }
    }
    throw lastError;
  }
}
