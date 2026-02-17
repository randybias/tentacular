import { assertEquals } from "jsr:@std/assert@1.0.11";
import { createMockContext } from "./mocks.ts";
import { detectDrift, formatDriftReport, getDependencyAccessSummary } from "./drift.ts";
import type { ContractSpec } from "../context/types.ts";

// Helper to build a contract with given dependency names (all HTTPS)
function makeContract(...names: string[]): ContractSpec {
  const dependencies: ContractSpec["dependencies"] = {};
  for (const name of names) {
    dependencies[name] = { protocol: "https", host: `${name}.example.com`, port: 443 };
  }
  return { dependencies };
}

Deno.test("detectDrift returns no violations for contract-less workflow", () => {
  const ctx = createMockContext();

  const report = detectDrift(ctx);

  assertEquals(report.violations.length, 0);
  assertEquals(report.summary.hasViolations, false);
});

Deno.test("detectDrift tracks dependency accesses", () => {
  const ctx = createMockContext();

  // Simulate dependency access
  ctx.dependency("github-api");
  ctx.dependency("postgres");

  const report = detectDrift(ctx, makeContract("github-api", "postgres"));

  assertEquals(report.summary.dependenciesAccessed, 2);
});

Deno.test("getDependencyAccessSummary returns accessed dependencies", async () => {
  const ctx = createMockContext();

  // Access dependencies
  const github = ctx.dependency("github-api");
  const _postgres = ctx.dependency("postgres");

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

  const report = detectDrift(ctx, makeContract("test-dep"));
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

// --- Additional Comprehensive Drift Detection Tests ---

Deno.test("drift: duplicate dependency accesses are merged", () => {
  const ctx = createMockContext();

  ctx.dependency("postgres");
  ctx.dependency("postgres");
  ctx.dependency("postgres");

  const accesses = getDependencyAccessSummary(ctx);
  assertEquals(accesses.length, 1);
  assertEquals(accesses[0]!.name, "postgres");
});

Deno.test("drift: empty context has zero accesses", () => {
  const ctx = createMockContext();

  const report = detectDrift(ctx, makeContract());

  assertEquals(report.summary.dependenciesAccessed, 0);
  assertEquals(report.violations.length, 0);
});

Deno.test("drift: detects undeclared dependency usage", () => {
  const ctx = createMockContext();

  ctx.dependency("undeclared-dep");

  const report = detectDrift(ctx, makeContract("declared-dep"));

  // Should have violations for undeclared dependency AND dead declaration
  assertEquals(report.violations.length > 0, true);
  const hasUndeclared = report.violations.some((v) => v.type === "undeclared-dependency");
  assertEquals(hasUndeclared, true);
});

Deno.test("drift: no violations without contract", () => {
  const ctx = createMockContext();
  ctx.dependency("test");

  const report = detectDrift(ctx);

  assertEquals(report.violations.length, 0);
  assertEquals(report.warnings.length, 0);
});

Deno.test("drift: report structure validation", () => {
  const ctx = createMockContext();

  const report = detectDrift(ctx, makeContract());

  assertEquals(Array.isArray(report.violations), true);
  assertEquals(Array.isArray(report.warnings), true);
  assertEquals(typeof report.summary, "object");
  assertEquals(typeof report.summary.hasViolations, "boolean");
  assertEquals(typeof report.summary.dependenciesAccessed, "number");
  assertEquals(typeof report.summary.directFetchCalls, "number");
  assertEquals(typeof report.summary.directSecretsAccess, "number");
});

Deno.test("drift: summary counts are accurate with matching contract", () => {
  const ctx = createMockContext();

  ctx.dependency("api1");
  ctx.dependency("api2");
  ctx.dependency("api3");

  // Contract declares all three — no violations
  const report = detectDrift(ctx, makeContract("api1", "api2", "api3"));

  assertEquals(report.summary.dependenciesAccessed, 3);
  assertEquals(report.summary.directFetchCalls, 0);
  assertEquals(report.summary.directSecretsAccess, 0);
  assertEquals(report.summary.hasViolations, false);
});

Deno.test("formatDriftReport: multiline output with sections", () => {
  const ctx = createMockContext();
  ctx.dependency("test");

  const report = detectDrift(ctx, makeContract("test"));
  const formatted = formatDriftReport(report);

  const lines = formatted.split("\n");
  assertEquals(lines.length > 5, true);

  // Check for required sections
  assertEquals(formatted.includes("==="), true); // Header
  assertEquals(formatted.includes("SUMMARY:"), true);
  assertEquals(formatted.includes("Dependencies accessed:"), true);
  assertEquals(formatted.includes("Has violations:"), true);
});

Deno.test("formatDriftReport: shows violations for dead dependency", () => {
  const ctx = createMockContext();

  ctx.dependency("used");

  const report = detectDrift(ctx, makeContract("used", "unused"));
  const formatted = formatDriftReport(report);

  // Dead declaration is a violation
  assertEquals(report.violations.some((v) => v.type === "dead-declaration"), true);
  assertEquals(formatted.includes("VIOLATIONS:"), true);
  assertEquals(formatted.includes("unused"), true);
});

Deno.test("getDependencyAccessSummary: preserves access order", () => {
  const ctx = createMockContext();

  ctx.dependency("first");
  ctx.dependency("second");
  ctx.dependency("third");

  const summary = getDependencyAccessSummary(ctx);

  assertEquals(summary.length, 3);
  assertEquals(summary[0]!.name, "first");
  assertEquals(summary[1]!.name, "second");
  assertEquals(summary[2]!.name, "third");
});

Deno.test("drift: multiple fetch calls tracked per dependency", async () => {
  const ctx = createMockContext();

  const dep = ctx.dependency("api");

  if (dep.fetch) {
    await dep.fetch("/endpoint1");
    await dep.fetch("/endpoint2");
    await dep.fetch("/endpoint3");
  }

  const accesses = getDependencyAccessSummary(ctx);
  const apiAccess = accesses.find((a) => a.name === "api");

  assertEquals(apiAccess?.fetches.length, 3);
  assertEquals(apiAccess?.fetches.includes("/endpoint1"), true);
  assertEquals(apiAccess?.fetches.includes("/endpoint2"), true);
  assertEquals(apiAccess?.fetches.includes("/endpoint3"), true);
});

Deno.test("drift: mixed dependencies with and without fetch", async () => {
  const ctx = createMockContext();

  const https = ctx.dependency("github");
  const _postgres = ctx.dependency("postgres");

  if (https.fetch) {
    await https.fetch("/repos");
  }
  // postgres doesn't have fetch

  const accesses = getDependencyAccessSummary(ctx);

  assertEquals(accesses.length, 2);

  const githubAccess = accesses.find((a) => a.name === "github");
  const postgresAccess = accesses.find((a) => a.name === "postgres");

  assertEquals(githubAccess?.fetches.length, 1);
  assertEquals(postgresAccess?.fetches.length, 0);
});

Deno.test("drift: no violations when no contract provided", () => {
  const ctx = createMockContext();

  // Access many things
  ctx.dependency("api1");
  ctx.dependency("api2");

  const report = detectDrift(ctx);

  assertEquals(report.violations.length, 0);
  assertEquals(report.summary.hasViolations, false);
});

Deno.test("formatDriftReport: correct counts in output", () => {
  const ctx = createMockContext();

  ctx.dependency("dep1");
  ctx.dependency("dep2");

  const report = detectDrift(ctx, makeContract("dep1", "dep2"));
  const formatted = formatDriftReport(report);

  assertEquals(formatted.includes("Dependencies accessed: 2"), true);
  assertEquals(formatted.includes("Direct fetch() calls: 0"), true);
  assertEquals(formatted.includes("Direct secrets access: 0"), true);
});

Deno.test("drift: case-sensitive dependency names", () => {
  const ctx = createMockContext();

  ctx.dependency("GitHub");
  ctx.dependency("github");
  ctx.dependency("GITHUB");

  const accesses = getDependencyAccessSummary(ctx);

  // Should be 3 separate entries (case-sensitive)
  assertEquals(accesses.length, 3);
});

// --- Direct bypass detection tests ---

Deno.test("drift: detects direct ctx.fetch() bypass", async () => {
  const ctx = createMockContext();

  // Use direct ctx.fetch() instead of ctx.dependency().fetch()
  await ctx.fetch("github", "/repos/test");

  const report = detectDrift(ctx, makeContract("github-api"));

  assertEquals(report.summary.directFetchCalls, 1);
  const hasFetchViolation = report.violations.some((v) => v.type === "direct-fetch");
  assertEquals(hasFetchViolation, true);
});

Deno.test("drift: detects direct ctx.secrets access bypass", () => {
  const ctx = createMockContext({
    secrets: { github: { token: "test123" } },
  });

  // Access secrets directly instead of ctx.dependency().secret
  const _token = ctx.secrets["github"];

  const report = detectDrift(ctx, makeContract("github-api"));

  assertEquals(report.summary.directSecretsAccess > 0, true);
  const hasSecretsViolation = report.violations.some((v) => v.type === "direct-secrets");
  assertEquals(hasSecretsViolation, true);
});

Deno.test("drift: detects dead declaration", () => {
  const ctx = createMockContext();

  // Only access github-api, but contract also declares unused postgres
  ctx.dependency("github-api");

  const report = detectDrift(ctx, makeContract("github-api", "postgres"));

  assertEquals(report.summary.deadDeclarations, 1);
  const hasDead = report.violations.some((v) => v.type === "dead-declaration");
  assertEquals(hasDead, true);
});

Deno.test("drift: clean report when all deps match contract", async () => {
  const ctx = createMockContext();

  const gh = ctx.dependency("github-api");
  ctx.dependency("postgres");

  if (gh.fetch) {
    await gh.fetch("/repos/test");
  }

  const report = detectDrift(ctx, makeContract("github-api", "postgres"));

  assertEquals(report.summary.hasViolations, false);
  assertEquals(report.violations.length, 0);
  assertEquals(report.summary.deadDeclarations, 0);
  assertEquals(report.summary.undeclaredDependencies, 0);
  assertEquals(report.summary.directFetchCalls, 0);
  assertEquals(report.summary.directSecretsAccess, 0);
});
