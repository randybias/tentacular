import type { Context } from "tentacular";
import type { PrContext } from "./fetch-pr.ts";

/** A single CI check run result */
export interface CheckRun {
  name: string;
  status: "queued" | "in_progress" | "completed";
  conclusion: "success" | "failure" | "neutral" | "cancelled" | "skipped" | "timed_out" | "action_required" | null;
  url: string;
  started_at: string | null;
  completed_at: string | null;
}

export interface CheckRunsOutput {
  checks: CheckRun[];
  overall_status: "pass" | "fail" | "pending" | "none";
  failed_checks: string[];
}

/**
 * Query GitHub Check Runs API for CI status on the PR head commit.
 *
 * Always free — requires no special permissions beyond repo read access.
 * Returns overall_status "pass" when all checks succeed, "fail" if any failed,
 * "pending" if any are still running, "none" if no checks exist.
 */
export default async function run(ctx: Context, input: unknown): Promise<CheckRunsOutput> {
  const pr = input as PrContext;
  ctx.log.info(`Fetching check runs for ${pr.owner}/${pr.repo}@${pr.head_sha.slice(0, 7)}`);

  const github = ctx.dependency("github");
  const auth = `Bearer ${github.secret}`;

  const res = await github.fetch!(
    `/repos/${pr.owner}/${pr.repo}/commits/${pr.head_sha}/check-runs?per_page=100`,
    { headers: { Authorization: auth, Accept: "application/vnd.github+json" } },
  );

  if (!res.ok) {
    throw new Error(`GitHub Check Runs API error: ${res.status} ${await res.text()}`);
  }

  const data = await res.json() as Record<string, unknown>;
  const raw = (data["check_runs"] as Record<string, unknown>[]) ?? [];

  const checks: CheckRun[] = raw.map((c) => ({
    name: String(c["name"] ?? ""),
    status: String(c["status"] ?? "completed") as CheckRun["status"],
    conclusion: (c["conclusion"] as CheckRun["conclusion"]) ?? null,
    url: String(c["html_url"] ?? ""),
    started_at: (c["started_at"] as string | null) ?? null,
    completed_at: (c["completed_at"] as string | null) ?? null,
  }));

  const failedChecks = checks
    .filter((c) => c.conclusion === "failure" || c.conclusion === "timed_out" || c.conclusion === "action_required")
    .map((c) => c.name);

  const hasPending = checks.some((c) => c.status !== "completed");
  const hasFailed = failedChecks.length > 0;

  const overallStatus: CheckRunsOutput["overall_status"] =
    checks.length === 0
      ? "none"
      : hasPending
      ? "pending"
      : hasFailed
      ? "fail"
      : "pass";

  ctx.log.info(
    `Check runs: ${checks.length} total, ${failedChecks.length} failed, status=${overallStatus}`,
  );

  return { checks, overall_status: overallStatus, failed_checks: failedChecks };
}
