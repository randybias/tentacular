import { assertEquals } from "std/assert";
import { SimpleExecutor } from "./simple.ts";
import { compile } from "../compiler/mod.ts";
import type { WorkflowSpec, Context } from "../types.ts";
import type { NodeRunner } from "./types.ts";
import { createMockContext } from "../testing/mocks.ts";

function makeRunner(handlers: Record<string, (input: unknown) => unknown>): NodeRunner {
  return {
    async run(nodeId: string, _ctx: Context, input: unknown): Promise<unknown> {
      const handler = handlers[nodeId];
      if (!handler) throw new Error(`No handler for ${nodeId}`);
      return handler(input);
    },
  };
}

function makeSpec(
  nodes: Record<string, { path: string }>,
  edges: { from: string; to: string }[],
): WorkflowSpec {
  return {
    name: "test",
    version: "1.0",
    triggers: [{ type: "manual" }],
    nodes,
    edges,
  };
}

Deno.test("SimpleExecutor: single node", async () => {
  const spec = makeSpec({ a: { path: "./a.ts" } }, []);
  const graph = compile(spec);
  const ctx = createMockContext();

  const runner = makeRunner({
    a: () => ({ result: 42 }),
  });

  const executor = new SimpleExecutor();
  const result = await executor.execute(graph, runner, ctx);

  assertEquals(result.success, true);
  assertEquals(result.outputs["a"], { result: 42 });
});

Deno.test("SimpleExecutor: linear chain passes data", async () => {
  const spec = makeSpec(
    { a: { path: "./a.ts" }, b: { path: "./b.ts" } },
    [{ from: "a", to: "b" }],
  );
  const graph = compile(spec);
  const ctx = createMockContext();

  const runner = makeRunner({
    a: () => ({ value: 10 }),
    b: (input) => ({ doubled: (input as { value: number }).value * 2 }),
  });

  const executor = new SimpleExecutor();
  const result = await executor.execute(graph, runner, ctx);

  assertEquals(result.success, true);
  assertEquals(result.outputs["b"], { doubled: 20 });
});

Deno.test("SimpleExecutor: node failure stops execution", async () => {
  const spec = makeSpec(
    { a: { path: "./a.ts" }, b: { path: "./b.ts" } },
    [{ from: "a", to: "b" }],
  );
  const graph = compile(spec);
  const ctx = createMockContext();

  const runner = makeRunner({
    a: () => {
      throw new Error("boom");
    },
    b: () => ({ never: true }),
  });

  const executor = new SimpleExecutor();
  const result = await executor.execute(graph, runner, ctx);

  assertEquals(result.success, false);
  assertEquals("a" in result.errors, true);
  assertEquals("b" in result.outputs, false);
});

Deno.test("SimpleExecutor: parallel nodes in same stage", async () => {
  const spec = makeSpec(
    {
      a: { path: "./a.ts" },
      b: { path: "./b.ts" },
      c: { path: "./c.ts" },
    },
    [
      { from: "a", to: "b" },
      { from: "a", to: "c" },
    ],
  );
  const graph = compile(spec);
  const ctx = createMockContext();

  const executionOrder: string[] = [];
  const runner: NodeRunner = {
    async run(nodeId: string, _ctx: Context, input: unknown): Promise<unknown> {
      executionOrder.push(nodeId);
      return { nodeId };
    },
  };

  const executor = new SimpleExecutor();
  const result = await executor.execute(graph, runner, ctx);

  assertEquals(result.success, true);
  assertEquals(executionOrder[0], "a");
  // b and c should both be present (executed in parallel)
  assertEquals(executionOrder.includes("b"), true);
  assertEquals(executionOrder.includes("c"), true);
});

Deno.test("SimpleExecutor: retry with exponential backoff", async () => {
  const spec = makeSpec({ a: { path: "./a.ts" } }, []);
  const graph = compile(spec);
  const ctx = createMockContext();

  let attempts = 0;
  const runner: NodeRunner = {
    async run(): Promise<unknown> {
      attempts++;
      if (attempts < 3) throw new Error(`fail attempt ${attempts}`);
      return { ok: true };
    },
  };

  const executor = new SimpleExecutor({ maxRetries: 2 });
  const result = await executor.execute(graph, runner, ctx);

  assertEquals(result.success, true);
  assertEquals(attempts, 3);
  assertEquals(result.outputs["a"], { ok: true });
});

Deno.test("SimpleExecutor: retry exhausted returns failure", async () => {
  const spec = makeSpec({ a: { path: "./a.ts" } }, []);
  const graph = compile(spec);
  const ctx = createMockContext();

  let attempts = 0;
  const runner: NodeRunner = {
    async run(): Promise<unknown> {
      attempts++;
      throw new Error("always fails");
    },
  };

  const executor = new SimpleExecutor({ maxRetries: 1 });
  const result = await executor.execute(graph, runner, ctx);

  assertEquals(result.success, false);
  assertEquals(attempts, 2); // initial + 1 retry
  assertEquals(result.errors["a"]?.includes("always fails"), true);
});

Deno.test({ name: "SimpleExecutor: timeout", sanitizeOps: false, sanitizeResources: false, fn: async () => {
  const spec = makeSpec({ a: { path: "./a.ts" } }, []);
  const graph = compile(spec);
  const ctx = createMockContext();

  const runner: NodeRunner = {
    async run(): Promise<unknown> {
      await new Promise((r) => setTimeout(r, 5000));
      return {};
    },
  };

  const executor = new SimpleExecutor({ timeoutMs: 100 });
  const result = await executor.execute(graph, runner, ctx);

  assertEquals(result.success, false);
  assertEquals(result.errors["a"]?.includes("timed out"), true);
}});
