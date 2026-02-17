import { assertEquals } from "jsr:@std/assert@1.0.11";
import { createMockContext } from "./mocks.ts";
import { detectDrift, formatDriftReport, getDependencyAccessSummary } from "./drift.ts";

Deno.test("detectDrift returns no violations for contract-less workflow", () => {
  const ctx = createMockContext();

  const report = detectDrift(ctx, false);

  assertEquals(report.violations.length, 0);
  assertEquals(report.summary.hasViolations, false);
});

Deno.test("detectDrift tracks dependency accesses", () => {
  const ctx = createMockContext();

  // Simulate dependency access
  ctx.dependency("github-api");
  ctx.dependency("postgres");

  const report = detectDrift(ctx, true);

  assertEquals(report.summary.dependenciesAccessed, 2);
});

Deno.test("getDependencyAccessSummary returns accessed dependencies", async () => {
  const ctx = createMockContext();

  // Access dependencies
  const github = ctx.dependency("github-api");
  const postgres = ctx.dependency("postgres");

  // Use the dependencies
  if (github.fetch) {
    await github.fetch("/repos/test");
  }

  const accesses = getDependencyAccessSummary(ctx);

  assertEquals(accesses.length, 2);
  assertEquals(accesses[0]?.name, "github-api");
  assertEquals(accesses[1]?.name, "postgres");
  assertEquals(accesses[0]?.fetches.length, 1);
  assertEquals(accesses[0]?.fetches[0], "/repos/test");
});

Deno.test("formatDriftReport produces readable output", () => {
  const ctx = createMockContext();

  ctx.dependency("test-dep");

  const report = detectDrift(ctx, true);
  const formatted = formatDriftReport(report);

  assertEquals(formatted.includes("Contract Drift Report"), true);
  assertEquals(formatted.includes("Dependencies accessed: 1"), true);
});

Deno.test("dependency fetch calls are tracked", async () => {
  const ctx = createMockContext();

  const dep = ctx.dependency("github-api");

  if (dep.fetch) {
    await dep.fetch("/repos/owner/repo");
    await dep.fetch("/users/username");
  }

  const accesses = getDependencyAccessSummary(ctx);
  const githubAccess = accesses.find((a) => a.name === "github-api");

  assertEquals(githubAccess?.fetches.length, 2);
  assertEquals(githubAccess?.fetches[0], "/repos/owner/repo");
  assertEquals(githubAccess?.fetches[1], "/users/username");
});
