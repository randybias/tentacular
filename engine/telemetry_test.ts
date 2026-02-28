import { assertEquals, assertExists, assertGreater } from "std/assert";
import { NoopSink, BasicSink, NewTelemetrySink } from "./telemetry/mod.ts";
import { SimpleExecutor } from "./executor/simple.ts";
import { compile } from "./compiler/mod.ts";
import type { WorkflowSpec, Context } from "./types.ts";
import type { NodeRunner } from "./executor/types.ts";
import { createMockContext } from "./testing/mocks.ts";

// --- NoopSink ---

Deno.test("NoopSink: record does nothing", () => {
  const sink = new NoopSink();
  sink.record({ type: "engine-start", timestamp: Date.now() });
  sink.record({ type: "node-start", timestamp: Date.now(), metadata: { node: "a" } });
  // must not throw
});

Deno.test("NoopSink: snapshot returns zeroed state", () => {
  const sink = new NoopSink();
  const snap = sink.snapshot();
  assertEquals(snap.totalEvents, 0);
  assertEquals(snap.errorCount, 0);
  assertEquals(snap.errorRate, 0);
  assertEquals(snap.uptimeMs, 0);
  assertEquals(snap.lastError, null);
  assertEquals(snap.lastErrorAt, null);
  assertEquals(snap.recentEvents, []);
  assertEquals(snap.status, "ok");
});

// --- BasicSink ---

Deno.test("BasicSink: snapshot includes status ok", () => {
  const sink = new BasicSink();
  assertEquals(sink.snapshot().status, "ok");
});

Deno.test("BasicSink: uptimeMs is non-negative", () => {
  const sink = new BasicSink();
  assertEquals(sink.snapshot().uptimeMs >= 0, true);
});

Deno.test("BasicSink: records and counts 5 events with 2 errors", () => {
  const sink = new BasicSink();
  sink.record({ type: "node-start", timestamp: Date.now(), metadata: { node: "a" } });
  sink.record({ type: "node-complete", timestamp: Date.now(), metadata: { node: "a" } });
  sink.record({ type: "node-error", timestamp: Date.now(), metadata: { node: "a", error: "e1" } });
  sink.record({ type: "request-in", timestamp: Date.now() });
  sink.record({ type: "node-error", timestamp: Date.now(), metadata: { node: "b", error: "e2" } });
  const snap = sink.snapshot();
  assertEquals(snap.totalEvents, 5);
  assertEquals(snap.errorCount, 2);
});

Deno.test("BasicSink: errorRate is errorCount/totalEvents", () => {
  const sink = new BasicSink();
  for (let i = 0; i < 90; i++) {
    sink.record({ type: "node-complete", timestamp: Date.now(), metadata: { node: "a" } });
  }
  for (let i = 0; i < 10; i++) {
    sink.record({ type: "node-error", timestamp: Date.now(), metadata: { node: "a", error: "e" } });
  }
  const snap = sink.snapshot();
  assertEquals(snap.totalEvents, 100);
  assertEquals(snap.errorCount, 10);
  assertEquals(snap.errorRate, 0.1);
});

Deno.test("BasicSink: errorRate is 0 when no events", () => {
  const sink = new BasicSink();
  assertEquals(sink.snapshot().errorRate, 0);
});

Deno.test("BasicSink: lastError and lastErrorAt set on node-error", () => {
  const sink = new BasicSink();
  const ts = Date.now();
  sink.record({ type: "node-error", timestamp: ts, metadata: { node: "a", error: "boom" } });
  const snap = sink.snapshot();
  assertEquals(snap.lastError, "boom");
  assertEquals(snap.lastErrorAt, ts);
});

Deno.test("BasicSink: lastError updates to most recent error", () => {
  const sink = new BasicSink();
  sink.record({ type: "node-error", timestamp: 1000, metadata: { node: "a", error: "first" } });
  sink.record({ type: "node-error", timestamp: 2000, metadata: { node: "b", error: "second" } });
  const snap = sink.snapshot();
  assertEquals(snap.lastError, "second");
  assertEquals(snap.lastErrorAt, 2000);
});

Deno.test("BasicSink: recentEvents has all events in insertion order", () => {
  const sink = new BasicSink();
  sink.record({ type: "engine-start", timestamp: 1 });
  sink.record({ type: "node-start", timestamp: 2, metadata: { node: "a" } });
  sink.record({ type: "node-complete", timestamp: 3, metadata: { node: "a" } });
  const snap = sink.snapshot();
  assertEquals(snap.recentEvents.length, 3);
  assertEquals(snap.recentEvents[0]?.type, "engine-start");
  assertEquals(snap.recentEvents[2]?.type, "node-complete");
});

Deno.test("BasicSink: ring buffer wraps at capacity of 1000", () => {
  const sink = new BasicSink();
  for (let i = 0; i < 1100; i++) {
    sink.record({ type: "node-start", timestamp: i, metadata: { node: "a" } });
  }
  const snap = sink.snapshot();
  assertEquals(snap.totalEvents, 1100);
  assertEquals(snap.recentEvents.length, 1000);
  // Most recent 1000: timestamps 100..1099
  assertEquals(snap.recentEvents[0]?.timestamp, 100);
  assertEquals(snap.recentEvents[999]?.timestamp, 1099);
});

Deno.test("BasicSink: totalEvents reflects true total past ring capacity", () => {
  const sink = new BasicSink();
  for (let i = 0; i < 2000; i++) {
    sink.record({ type: "node-complete", timestamp: i });
  }
  assertEquals(sink.snapshot().totalEvents, 2000);
  assertEquals(sink.snapshot().recentEvents.length, 1000);
});

// --- NewTelemetrySink factory ---

Deno.test("NewTelemetrySink: 'noop' returns NoopSink", () => {
  const sink = NewTelemetrySink("noop");
  sink.record({ type: "node-start", timestamp: Date.now() });
  assertEquals(sink.snapshot().totalEvents, 0);
});

Deno.test("NewTelemetrySink: 'basic' returns BasicSink", () => {
  const sink = NewTelemetrySink("basic");
  sink.record({ type: "node-start", timestamp: Date.now() });
  assertEquals(sink.snapshot().totalEvents, 1);
});

Deno.test("NewTelemetrySink: undefined defaults to BasicSink", () => {
  const sink = NewTelemetrySink();
  sink.record({ type: "node-start", timestamp: Date.now() });
  assertEquals(sink.snapshot().totalEvents, 1);
});

Deno.test("NewTelemetrySink: unknown kind defaults to BasicSink", () => {
  const sink = NewTelemetrySink("prometheus");
  sink.record({ type: "node-start", timestamp: Date.now() });
  assertEquals(sink.snapshot().totalEvents, 1);
});

// --- SimpleExecutor telemetry integration ---

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

Deno.test("SimpleExecutor: records node-start and node-complete on success", async () => {
  const spec = makeSpec({ a: { path: "./a.ts" } }, []);
  const graph = compile(spec);
  const ctx = createMockContext();
  const sink = new BasicSink();

  const runner: NodeRunner = {
    async run(_nodeId: string, _ctx: Context, _input: unknown): Promise<unknown> {
      return { ok: true };
    },
  };

  const executor = new SimpleExecutor({ sink });
  await executor.execute(graph, runner, ctx);

  const snap = sink.snapshot();
  assertEquals(snap.totalEvents, 2); // node-start + node-complete
  assertEquals(snap.errorCount, 0);
  const types = snap.recentEvents.map((e) => e.type);
  assertEquals(types.includes("node-start"), true);
  assertEquals(types.includes("node-complete"), true);
});

Deno.test("SimpleExecutor: records node-start and node-error on failure", async () => {
  const spec = makeSpec({ a: { path: "./a.ts" } }, []);
  const graph = compile(spec);
  const ctx = createMockContext();
  const sink = new BasicSink();

  const runner: NodeRunner = {
    async run(): Promise<unknown> {
      throw new Error("node failed");
    },
  };

  const executor = new SimpleExecutor({ sink });
  await executor.execute(graph, runner, ctx);

  const snap = sink.snapshot();
  assertEquals(snap.errorCount, 1);
  assertEquals(snap.lastError, "node failed");
  assertExists(snap.lastErrorAt);
  const types = snap.recentEvents.map((e) => e.type);
  assertEquals(types.includes("node-start"), true);
  assertEquals(types.includes("node-error"), true);
});

Deno.test("SimpleExecutor: node events include node name in metadata", async () => {
  const spec = makeSpec({ mynode: { path: "./mynode.ts" } }, []);
  const graph = compile(spec);
  const ctx = createMockContext();
  const sink = new BasicSink();

  const runner: NodeRunner = {
    async run(): Promise<unknown> { return {}; },
  };

  const executor = new SimpleExecutor({ sink });
  await executor.execute(graph, runner, ctx);

  const snap = sink.snapshot();
  for (const event of snap.recentEvents) {
    assertEquals(event.metadata?.["node"], "mynode");
  }
});

Deno.test("SimpleExecutor: records events for multiple nodes across stages", async () => {
  const spec = makeSpec(
    { a: { path: "./a.ts" }, b: { path: "./b.ts" } },
    [{ from: "a", to: "b" }],
  );
  const graph = compile(spec);
  const ctx = createMockContext();
  const sink = new BasicSink();

  const runner: NodeRunner = {
    async run(nodeId: string): Promise<unknown> { return { nodeId }; },
  };

  const executor = new SimpleExecutor({ sink });
  await executor.execute(graph, runner, ctx);

  const snap = sink.snapshot();
  assertEquals(snap.totalEvents, 4); // 2 nodes × (start + complete)
  assertEquals(snap.errorCount, 0);
});

Deno.test("SimpleExecutor: uptimeMs is non-negative after execution", async () => {
  const spec = makeSpec({ a: { path: "./a.ts" } }, []);
  const graph = compile(spec);
  const ctx = createMockContext();
  const sink = new BasicSink();

  const runner: NodeRunner = {
    async run(): Promise<unknown> { return {}; },
  };

  const executor = new SimpleExecutor({ sink });
  await executor.execute(graph, runner, ctx);

  assertGreater(sink.snapshot().uptimeMs, -1);
});
