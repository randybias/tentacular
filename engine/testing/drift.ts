/**
 * Runtime tracing and contract drift detection
 *
 * Compares actual runtime access patterns (captured from MockContext) against
 * declared contract dependencies to detect drift and contract violations.
 */

import type { MockContext, DependencyAccess } from "./mocks.ts";
import type { ContractSpec } from "../context/types.ts";

export interface DriftReport {
  violations: ContractViolation[];
  warnings: string[];
  summary: DriftSummary;
}

export interface ContractViolation {
  type: "direct-fetch" | "direct-secrets" | "undeclared-dependency" | "dead-declaration";
  message: string;
  suggestion: string;
}

export interface DriftSummary {
  hasViolations: boolean;
  dependenciesAccessed: number;
  directFetchCalls: number;
  directSecretsAccess: number;
  deadDeclarations: number;
  undeclaredDependencies: number;
}

/**
 * Analyze MockContext access patterns for contract violations.
 *
 * Detects four types of drift:
 * 1. direct-fetch: Code uses ctx.fetch() directly instead of ctx.dependency().fetch()
 * 2. direct-secrets: Code accesses ctx.secrets directly instead of ctx.dependency().secret
 * 3. undeclared-dependency: Code calls ctx.dependency(name) for a dep not in the contract
 * 4. dead-declaration: Contract declares a dep that code never accesses
 */
export function detectDrift(ctx: MockContext, contract?: ContractSpec): DriftReport {
  const violations: ContractViolation[] = [];
  const warnings: string[] = [];

  const declaredDeps = contract?.dependencies ?? {};
  const declaredNames = new Set(Object.keys(declaredDeps));
  const accessedNames = new Set(ctx._dependencyAccesses.map((a) => a.name));

  // 1. Direct ctx.fetch() bypass detection
  const fetchCalls = ctx._fetchCalls ?? [];
  if (contract && fetchCalls.length > 0) {
    for (const call of fetchCalls) {
      violations.push({
        type: "direct-fetch",
        message: `Direct ctx.fetch("${call.service}", "${call.path}") bypasses contract`,
        suggestion: `Use ctx.dependency("${call.service}").fetch("${call.path}") instead`,
      });
    }
  }

  // 2. Direct ctx.secrets bypass detection
  const secretsAccesses = ctx._secretsAccesses ?? [];
  if (contract && secretsAccesses.length > 0) {
    const uniqueAccesses = [...new Set(secretsAccesses)];
    for (const key of uniqueAccesses) {
      violations.push({
        type: "direct-secrets",
        message: `Direct ctx.secrets["${key}"] access bypasses contract`,
        suggestion: `Use ctx.dependency("<name>").secret instead`,
      });
    }
  }

  // 3. Undeclared dependency detection
  if (contract) {
    for (const name of accessedNames) {
      if (!declaredNames.has(name)) {
        violations.push({
          type: "undeclared-dependency",
          message: `Dependency "${name}" accessed but not declared in contract`,
          suggestion: `Add "${name}" to contract.dependencies in workflow.yaml`,
        });
      }
    }
  }

  // 4. Dead declaration detection
  if (contract) {
    for (const name of declaredNames) {
      if (!accessedNames.has(name)) {
        violations.push({
          type: "dead-declaration",
          message: `Dependency "${name}" declared in contract but never accessed`,
          suggestion: `Remove "${name}" from contract.dependencies or ensure the node uses it`,
        });
      }
    }
  }

  const deadCount = [...declaredNames].filter((n) => !accessedNames.has(n)).length;
  const undeclaredCount = [...accessedNames].filter((n) => !declaredNames.has(n)).length;

  const summary: DriftSummary = {
    hasViolations: violations.length > 0,
    dependenciesAccessed: accessedNames.size,
    directFetchCalls: fetchCalls.length,
    directSecretsAccess: secretsAccesses.length,
    deadDeclarations: deadCount,
    undeclaredDependencies: undeclaredCount,
  };

  return {
    violations,
    warnings,
    summary,
  };
}

/**
 * Format drift report as human-readable text
 */
export function formatDriftReport(report: DriftReport): string {
  const lines: string[] = [];

  lines.push("=== Contract Drift Report ===");
  lines.push("");

  if (report.violations.length > 0) {
    lines.push("VIOLATIONS:");
    for (const v of report.violations) {
      lines.push(`  [${v.type}] ${v.message}`);
      lines.push(`     Suggestion: ${v.suggestion}`);
    }
    lines.push("");
  }

  if (report.warnings.length > 0) {
    lines.push("WARNINGS:");
    for (const w of report.warnings) {
      lines.push(`  ${w}`);
    }
    lines.push("");
  }

  lines.push("SUMMARY:");
  lines.push(`  Dependencies accessed: ${report.summary.dependenciesAccessed}`);
  lines.push(`  Direct fetch() calls: ${report.summary.directFetchCalls}`);
  lines.push(`  Direct secrets access: ${report.summary.directSecretsAccess}`);
  lines.push(`  Dead declarations: ${report.summary.deadDeclarations}`);
  lines.push(`  Undeclared dependencies: ${report.summary.undeclaredDependencies}`);
  lines.push(`  Has violations: ${report.summary.hasViolations ? "YES" : "NO"}`);

  return lines.join("\n");
}

/**
 * Get dependency access summary from mock context
 */
export function getDependencyAccessSummary(ctx: MockContext): DependencyAccess[] {
  return ctx._dependencyAccesses || [];
}
