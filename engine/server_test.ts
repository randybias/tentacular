import { assertEquals, assertExists } from "std/assert";
import { startServer } from "./server.ts";
import { BasicSink, NoopSink } from "./telemetry/mod.ts";
import { compile } from "./compiler/mod.ts";
import type { WorkflowSpec } from "./types.ts";
import type { NodeRunner } from "./executor/types.ts";
import { createMockContext } from "./testing/mocks.ts";

function makeSpec(): WorkflowSpec {
  return {
    name: "test",
    version: "1.0",
    triggers: [{ type: "manual" }],
    nodes: { a: { path: "./a.ts" } },
    edges: [],
  };
}

function makeRunner(): NodeRunner {
  return {
    async run(): Promise<unknown> {
      return { ok: true };
    },
  };
}

// --- /health endpoint ---

Deno.test("server: GET /health returns status ok", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const sink = new NoopSink();
  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx, sink });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health`);
    const body = await resp.json();
    assertEquals(resp.status, 200);
    assertEquals(body.status, "ok");
    // Plain /health must not include telemetry fields (backwards compat)
    assertEquals(Object.keys(body).length, 1, "plain /health must only return {status}");
  } finally {
    await server.shutdown();
  }
});

Deno.test("server: GET /health content-type is application/json", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health`);
    await resp.body?.cancel();
    const ct = resp.headers.get("content-type") ?? "";
    assertEquals(ct.includes("application/json"), true);
  } finally {
    await server.shutdown();
  }
});

// --- /health?detail=1 endpoint ---

Deno.test("server: GET /health?detail=1 returns TelemetrySnapshot shape", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const sink = new BasicSink();
  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx, sink });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health?detail=1`);
    const body = await resp.json();
    assertEquals(resp.status, 200);
    assertEquals(body.status, "ok");
    assertEquals(typeof body.totalEvents, "number");
    assertEquals(typeof body.errorCount, "number");
    assertEquals(typeof body.errorRate, "number");
    assertEquals(typeof body.uptimeMs, "number");
    assertExists(body.recentEvents);
    assertEquals(Array.isArray(body.recentEvents), true);
    // New G/A/R classification fields
    assertEquals(typeof body.lastRunFailed, "boolean");
    assertEquals(typeof body.inFlight, "number");
  } finally {
    await server.shutdown();
  }
});

Deno.test("server: GET /health?detail=1 reflects recorded events", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const sink = new BasicSink();
  // Record events before the server responds
  sink.record({ type: "node-start", timestamp: Date.now(), metadata: { node: "a" } });
  sink.record({ type: "node-error", timestamp: Date.now(), metadata: { node: "a", error: "boom" } });

  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx, sink });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health?detail=1`);
    const body = await resp.json();
    assertEquals(body.totalEvents, 2);
    assertEquals(body.errorCount, 1);
    assertEquals(body.errorRate, 0.5);
    assertEquals(body.lastError, "boom");
    assertEquals(body.recentEvents.length, 2);
  } finally {
    await server.shutdown();
  }
});

Deno.test("server: GET /health?detail=1 with NoopSink returns zeroed counters", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const sink = new NoopSink();
  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx, sink });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health?detail=1`);
    const body = await resp.json();
    assertEquals(body.totalEvents, 0);
    assertEquals(body.errorCount, 0);
    assertEquals(body.errorRate, 0);
    assertEquals(body.uptimeMs, 0);
    assertEquals(body.recentEvents.length, 0);
    assertEquals(body.status, "ok");
    assertEquals(body.lastRunFailed, false);
    assertEquals(body.inFlight, 0);
  } finally {
    await server.shutdown();
  }
});

Deno.test("server: GET /health?detail=1 uptimeMs is non-negative", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const sink = new BasicSink();
  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx, sink });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health?detail=1`);
    const body = await resp.json();
    assertEquals(body.uptimeMs >= 0, true, "uptimeMs must be non-negative");
  } finally {
    await server.shutdown();
  }
});

// --- G/A/R classification fields ---

Deno.test("server: GET /health?detail=1 lastRunFailed is true after failed run", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const sink = new BasicSink();
  // Simulate a completed run that had a node-error: request-in, node-error, request-out
  sink.record({ type: "request-in", timestamp: Date.now() });
  sink.record({ type: "node-error", timestamp: Date.now(), metadata: { node: "a", error: "boom" } });
  sink.record({ type: "request-out", timestamp: Date.now() });

  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx, sink });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health?detail=1`);
    const body = await resp.json();
    assertEquals(body.lastRunFailed, true, "lastRunFailed must be true after run with node-error");
    assertEquals(body.inFlight, 0, "inFlight must be 0 after request-out");
  } finally {
    await server.shutdown();
  }
});

Deno.test("server: GET /health?detail=1 inFlight is 1 while request in progress", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const sink = new BasicSink();
  // Simulate a request that has started but not completed
  sink.record({ type: "request-in", timestamp: Date.now() });

  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx, sink });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health?detail=1`);
    const body = await resp.json();
    assertEquals(body.inFlight, 1, "inFlight must be 1 while request is in progress");
    assertEquals(body.lastRunFailed, false, "lastRunFailed must be false before any run completes");
  } finally {
    await server.shutdown();
  }
});

Deno.test("server: GET /health?detail=1 lastRunFailed resets after successful run", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const sink = new BasicSink();
  // First run: failed
  sink.record({ type: "request-in", timestamp: Date.now() });
  sink.record({ type: "node-error", timestamp: Date.now(), metadata: { node: "a", error: "err" } });
  sink.record({ type: "request-out", timestamp: Date.now() });
  // Second run: success
  sink.record({ type: "request-in", timestamp: Date.now() });
  sink.record({ type: "request-out", timestamp: Date.now() });

  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx, sink });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/health?detail=1`);
    const body = await resp.json();
    assertEquals(body.lastRunFailed, false, "lastRunFailed must reset to false after successful run");
    assertEquals(body.inFlight, 0);
  } finally {
    await server.shutdown();
  }
});

// --- unknown routes ---

Deno.test("server: unknown path returns 404", async () => {
  const graph = compile(makeSpec());
  const ctx = createMockContext();
  const server = startServer({ port: 0, graph, runner: makeRunner(), ctx });

  try {
    const addr = server.addr as Deno.NetAddr;
    const resp = await fetch(`http://localhost:${addr.port}/not-found`);
    await resp.body?.cancel();
    assertEquals(resp.status, 404);
  } finally {
    await server.shutdown();
  }
});
