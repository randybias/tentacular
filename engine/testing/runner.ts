/**
 * Tentacular Test Runner
 *
 * Runs node-level and pipeline-level tests for a workflow.
 *
 * Usage:
 *   deno run --allow-read --allow-net engine/testing/runner.ts \
 *     --workflow ./workflow.yaml [--node <name>] [--pipeline]
 */

import { parse as parseFlags } from "std/flags";
import { parse as parseYaml } from "std/yaml";
import { dirname, resolve } from "std/path";
import type { WorkflowSpec, Context } from "../types.ts";
import { compile } from "../compiler/mod.ts";
import { SimpleExecutor } from "../executor/simple.ts";
import { loadAllNodes, loadNode } from "../loader.ts";
import { createMockContext, type MockContext } from "./mocks.ts";
import { loadFixture, findFixtures } from "./fixtures.ts";
import type { NodeRunner } from "../executor/types.ts";
import { createContext } from "../context/mod.ts";
import { detectDrift, formatDriftReport } from "./drift.ts";

const flags = parseFlags(Deno.args, {
  string: ["workflow", "node", "o"],
  boolean: ["pipeline", "warn"],
  alias: { o: "output" },
});

const workflowPath = flags.workflow;
if (!workflowPath) {
  console.error("Usage: deno run engine/testing/runner.ts --workflow <path> [--node <name>] [--pipeline]");
  Deno.exit(1);
}

const absWorkflowPath = resolve(workflowPath);
const workflowDir = dirname(absWorkflowPath);

const content = await Deno.readTextFile(absWorkflowPath);
const spec = parseYaml(content) as WorkflowSpec;

interface TestResult {
  name: string;
  passed: boolean;
  durationMs: number;
  error?: string;
}

const results: TestResult[] = [];
// Track all contexts for aggregated drift detection
const testContexts: MockContext[] = [];

if (flags.node) {
  // Run tests for a specific node
  await runNodeTests(flags.node);
} else if (flags.pipeline) {
  // Run full pipeline test
  await runPipelineTest();
} else {
  // Run all node tests
  for (const nodeName of Object.keys(spec.nodes)) {
    await runNodeTests(nodeName);
  }
}

// Analyze drift
const driftReport = aggregateContextsAndDetectDrift();

// Output results
const jsonOutput = flags.output === "json" || flags.o === "json";
if (jsonOutput) {
  console.log(JSON.stringify({
    testResults: results,
    drift: driftReport,
  }, null, 2));
} else {
  // Print test results
  console.log("\n─── Test Results ───");
  let allPassed = true;
  for (const r of results) {
    const icon = r.passed ? "✓" : "✗";
    const time = `(${r.durationMs}ms)`;
    console.log(`  ${icon} ${r.name} ${time}`);
    if (!r.passed) {
      allPassed = false;
      if (r.error) console.log(`    ${r.error}`);
    }
  }

  const passCount = results.filter((r) => r.passed).length;
  console.log(`\n${passCount}/${results.length} tests passed`);

  // Print drift report
  if (spec.contract) {
    console.log("\n" + formatDriftReport(driftReport));
    if (driftReport.summary.hasViolations) {
      const severity = flags.warn ? "⚠️  AUDIT MODE" : "❌ STRICT MODE";
      console.log(`\n${severity}: Contract violations detected`);
      if (flags.warn) {
        console.log("Violations logged as warnings (use strict mode in CI)");
      }
    }
  }

  // Exit with failure only in strict mode
  const shouldFail = !allPassed || (driftReport.summary.hasViolations && !flags.warn);
  if (shouldFail) {
    Deno.exit(1);
  }
}

// ---

function aggregateContextsAndDetectDrift() {
  // Merge all test contexts into a single aggregate for drift detection
  const aggregateCtx = createMockContext({ contract: spec.contract });

  for (const ctx of testContexts) {
    // Merge dependency accesses
    aggregateCtx._dependencyAccesses.push(...ctx._dependencyAccesses);
    // Merge direct fetch calls
    aggregateCtx._fetchCalls.push(...ctx._fetchCalls);
    // Merge secrets accesses
    aggregateCtx._secretsAccesses.push(...ctx._secretsAccesses);
  }

  // Use the proper drift detection module
  return detectDrift(aggregateCtx, spec.contract);
}

async function runNodeTests(nodeName: string) {
  const nodeSpec = spec.nodes[nodeName];
  if (!nodeSpec) {
    results.push({
      name: `${nodeName}: node not found`,
      passed: false,
      durationMs: 0,
      error: `Node "${nodeName}" is not defined in workflow.yaml`,
    });
    return;
  }

  const testDir = resolve(workflowDir, "tests");
  const fixtures = await findFixtures(testDir, nodeName);

  if (fixtures.length === 0) {
    results.push({
      name: `${nodeName}: no fixtures`,
      passed: true,
      durationMs: 0,
    });
    return;
  }

  for (const fixturePath of fixtures) {
    const start = Date.now();
    try {
      const fixture = await loadFixture(fixturePath);
      const fn = await loadNode(nodeSpec.path, workflowDir);
      const ctx = createMockContext({
        config: fixture.config ?? {},
        secrets: fixture.secrets ?? {},
        contract: spec.contract,
      });
      const output = await fn(ctx, fixture.input);

      // Collect context for drift detection
      testContexts.push(ctx);

      if (fixture.expected !== undefined) {
        const outputStr = JSON.stringify(output);
        const expectedStr = JSON.stringify(fixture.expected);
        if (outputStr !== expectedStr) {
          results.push({
            name: `${nodeName}: ${fixturePath.split("/").pop()}`,
            passed: false,
            durationMs: Date.now() - start,
            error: `Expected: ${expectedStr}\n    Got: ${outputStr}`,
          });
          continue;
        }
      }

      results.push({
        name: `${nodeName}: ${fixturePath.split("/").pop()}`,
        passed: true,
        durationMs: Date.now() - start,
      });
    } catch (err) {
      results.push({
        name: `${nodeName}: ${fixturePath.split("/").pop()}`,
        passed: false,
        durationMs: Date.now() - start,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  }
}

async function runPipelineTest() {
  const start = Date.now();
  try {
    const graph = compile(spec);
    const nodeFunctions = await loadAllNodes(spec.nodes, workflowDir);

    const ctx = createContext({
      secrets: {},
      config: spec.config as Record<string, unknown> ?? {},
    });

    const runner: NodeRunner = {
      async run(nodeId: string, _ctx: Context, input: unknown): Promise<unknown> {
        const fn = nodeFunctions.get(nodeId);
        if (!fn) throw new Error(`Node "${nodeId}" not loaded`);
        const nodeCtx = createMockContext();
        return fn(nodeCtx, input);
      },
    };

    const executor = new SimpleExecutor({ timeoutMs: 30_000 });
    const graphWithCtx = { ...graph, ctx };
    const result = await executor.execute(graphWithCtx, runner, ctx);

    results.push({
      name: "pipeline",
      passed: result.success,
      durationMs: Date.now() - start,
      error: result.success ? undefined : JSON.stringify(result.errors),
    });
  } catch (err) {
    results.push({
      name: "pipeline",
      passed: false,
      durationMs: Date.now() - start,
      error: err instanceof Error ? err.message : String(err),
    });
  }
}
