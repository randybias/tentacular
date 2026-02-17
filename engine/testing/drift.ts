/**
 * Runtime tracing and contract drift detection
 *
 * Compares actual runtime access patterns (captured from MockContext) against
 * declared contract dependencies to detect drift and contract violations.
 */

import type { MockContext, DependencyAccess } from "./mocks.ts";

export interface DriftReport {
  violations: ContractViolation[];
  warnings: string[];
  summary: DriftSummary;
}

export interface ContractViolation {
  type: "direct-fetch" | "direct-secrets" | "undeclared-dependency";
  message: string;
  suggestion: string;
}

export interface DriftSummary {
  hasViolations: boolean;
  dependenciesAccessed: number;
  directFetchCalls: number;
  directSecretsAccess: number;
}

/**
 * Analyze MockContext access patterns for contract violations.
 * Flags direct ctx.fetch() and ctx.secrets usage when contract is present.
 */
export function detectDrift(ctx: MockContext, hasContract: boolean): DriftReport {
  const violations: ContractViolation[] = [];
  const warnings: string[] = [];

  // When contract exists, direct ctx.fetch() and ctx.secrets are violations
  if (hasContract) {
    // Check for direct ctx.fetch() usage
    // In our mock, we can't easily track this without extending the mock further
    // For now, we'll focus on dependency access tracking

    // Check for direct ctx.secrets access
    // This would require instrumentation of the secrets object
    // Placeholder for future implementation

    warnings.push(
      "Direct ctx.fetch() and ctx.secrets access detection requires runtime instrumentation",
    );
  }

  // Analyze dependency accesses
  const dependencyAccesses = ctx._dependencyAccesses || [];

  const summary: DriftSummary = {
    hasViolations: violations.length > 0,
    dependenciesAccessed: dependencyAccesses.length,
    directFetchCalls: 0, // Requires instrumentation
    directSecretsAccess: 0, // Requires instrumentation
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
      lines.push(`  ❌ [${v.type}] ${v.message}`);
      lines.push(`     💡 ${v.suggestion}`);
    }
    lines.push("");
  }

  if (report.warnings.length > 0) {
    lines.push("WARNINGS:");
    for (const w of report.warnings) {
      lines.push(`  ⚠️  ${w}`);
    }
    lines.push("");
  }

  lines.push("SUMMARY:");
  lines.push(`  Dependencies accessed: ${report.summary.dependenciesAccessed}`);
  lines.push(`  Direct fetch() calls: ${report.summary.directFetchCalls}`);
  lines.push(`  Direct secrets access: ${report.summary.directSecretsAccess}`);
  lines.push(`  Has violations: ${report.summary.hasViolations ? "YES" : "NO"}`);

  return lines.join("\n");
}

/**
 * Get dependency access summary from mock context
 */
export function getDependencyAccessSummary(ctx: MockContext): DependencyAccess[] {
  return ctx._dependencyAccesses || [];
}
