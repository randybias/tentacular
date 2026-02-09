import { assertEquals, assertThrows } from "std/assert";
import { compile } from "./mod.ts";
import type { WorkflowSpec } from "../types.ts";

function makeSpec(overrides: Partial<WorkflowSpec> = {}): WorkflowSpec {
  return {
    name: "test",
    version: "1.0",
    triggers: [{ type: "manual" }],
    nodes: { a: { path: "./a.ts" } },
    edges: [],
    ...overrides,
  };
}

Deno.test("compile: single node, no edges", () => {
  const graph = compile(makeSpec());
  assertEquals(graph.stages.length, 1);
  assertEquals(graph.stages[0]!.nodes, ["a"]);
  assertEquals(graph.nodeOrder, ["a"]);
});

Deno.test("compile: linear chain a→b→c", () => {
  const graph = compile(
    makeSpec({
      nodes: {
        a: { path: "./a.ts" },
        b: { path: "./b.ts" },
        c: { path: "./c.ts" },
      },
      edges: [
        { from: "a", to: "b" },
        { from: "b", to: "c" },
      ],
    }),
  );

  assertEquals(graph.stages.length, 3);
  assertEquals(graph.stages[0]!.nodes, ["a"]);
  assertEquals(graph.stages[1]!.nodes, ["b"]);
  assertEquals(graph.stages[2]!.nodes, ["c"]);
});

Deno.test("compile: fan-out a→b, a→c", () => {
  const graph = compile(
    makeSpec({
      nodes: {
        a: { path: "./a.ts" },
        b: { path: "./b.ts" },
        c: { path: "./c.ts" },
      },
      edges: [
        { from: "a", to: "b" },
        { from: "a", to: "c" },
      ],
    }),
  );

  assertEquals(graph.stages.length, 2);
  assertEquals(graph.stages[0]!.nodes, ["a"]);
  // b and c should be in the same stage (parallel)
  assertEquals(graph.stages[1]!.nodes.sort(), ["b", "c"]);
});

Deno.test("compile: fan-in b→d, c→d", () => {
  const graph = compile(
    makeSpec({
      nodes: {
        a: { path: "./a.ts" },
        b: { path: "./b.ts" },
        c: { path: "./c.ts" },
        d: { path: "./d.ts" },
      },
      edges: [
        { from: "a", to: "b" },
        { from: "a", to: "c" },
        { from: "b", to: "d" },
        { from: "c", to: "d" },
      ],
    }),
  );

  assertEquals(graph.stages.length, 3);
  assertEquals(graph.stages[0]!.nodes, ["a"]);
  assertEquals(graph.stages[1]!.nodes.sort(), ["b", "c"]);
  assertEquals(graph.stages[2]!.nodes, ["d"]);
});

Deno.test("compile: diamond pattern", () => {
  const graph = compile(
    makeSpec({
      nodes: {
        a: { path: "./a.ts" },
        b: { path: "./b.ts" },
        c: { path: "./c.ts" },
        d: { path: "./d.ts" },
      },
      edges: [
        { from: "a", to: "b" },
        { from: "a", to: "c" },
        { from: "b", to: "d" },
        { from: "c", to: "d" },
      ],
    }),
  );

  // a in stage 0, b+c in stage 1, d in stage 2
  assertEquals(graph.stages.length, 3);
  assertEquals(graph.stages[2]!.nodes, ["d"]);
});

Deno.test("compile: cycle throws error", () => {
  assertThrows(
    () =>
      compile(
        makeSpec({
          nodes: {
            a: { path: "./a.ts" },
            b: { path: "./b.ts" },
          },
          edges: [
            { from: "a", to: "b" },
            { from: "b", to: "a" },
          ],
        }),
      ),
    Error,
    "Cycle detected",
  );
});

Deno.test("compile: indirect cycle throws error", () => {
  assertThrows(
    () =>
      compile(
        makeSpec({
          nodes: {
            a: { path: "./a.ts" },
            b: { path: "./b.ts" },
            c: { path: "./c.ts" },
          },
          edges: [
            { from: "a", to: "b" },
            { from: "b", to: "c" },
            { from: "c", to: "a" },
          ],
        }),
      ),
    Error,
    "Cycle detected",
  );
});

Deno.test("compile: undefined node in edge throws error", () => {
  assertThrows(
    () =>
      compile(
        makeSpec({
          nodes: { a: { path: "./a.ts" } },
          edges: [{ from: "a", to: "nonexistent" }],
        }),
      ),
    Error,
    "undefined node",
  );
});

Deno.test("compile: self-loop throws error", () => {
  assertThrows(
    () =>
      compile(
        makeSpec({
          nodes: { a: { path: "./a.ts" } },
          edges: [{ from: "a", to: "a" }],
        }),
      ),
    Error,
    "Self-loop",
  );
});
