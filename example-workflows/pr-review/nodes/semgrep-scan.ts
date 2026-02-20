import type { Context } from "tentacular";
import type { PrContext } from "./fetch-pr.ts";

/** A Semgrep finding from GitHub Code Scanning */
export interface SemgrepAlert {
  id: number;
  rule_id: string;
  severity: "critical" | "high" | "medium" | "low" | "note" | "warning" | "error";
  message: string;
  path: string;
  line: number;
  end_line: number;
  url: string;
}

export interface SemgrepScanOutput {
  alerts: SemgrepAlert[];
  tool: "Semgrep";
}

/**
 * Query GitHub Code Scanning API for Semgrep alerts on the PR head commit.
 *
 * Works when Semgrep is configured as a GitHub Actions workflow (free for public repos).
 * For private repos, requires GitHub Advanced Security.
 *
 * Returns an empty alerts list (not an error) if Code Scanning is not configured —
 * this allows the workflow to proceed without failing.
 */
export default async function run(ctx: Context, input: unknown): Promise<SemgrepScanOutput> {
  const pr = input as PrContext;
  ctx.log.info(`Querying Semgrep alerts for ${pr.owner}/${pr.repo}@${pr.head_sha.slice(0, 7)}`);

  const github = ctx.dependency("github");
  const auth = `Bearer ${github.secret}`;

  const res = await github.fetch!(
    `/repos/${pr.owner}/${pr.repo}/code-scanning/alerts` +
    `?ref=${pr.head_sha}&tool_name=Semgrep&state=open&per_page=50`,
    { headers: { Authorization: auth, Accept: "application/vnd.github+json" } },
  );

  // 404 = code scanning not enabled; 403 = GHAS required — treat both as "no results"
  if (res.status === 404 || res.status === 403) {
    ctx.log.warn(`Semgrep code scanning not available (${res.status}) — skipping`);
    return { alerts: [], tool: "Semgrep" };
  }

  if (!res.ok) {
    throw new Error(`GitHub Code Scanning API error: ${res.status} ${await res.text()}`);
  }

  const raw = await res.json() as Record<string, unknown>[];

  const alerts: SemgrepAlert[] = raw.map((a) => {
    const location = (a["most_recent_instance"] as Record<string, unknown> | undefined)
      ?.["location"] as Record<string, unknown> | undefined;
    const rule = (a["rule"] as Record<string, unknown> | undefined);
    return {
      id: Number(a["number"] ?? 0),
      rule_id: String(rule?.["id"] ?? "unknown"),
      severity: String(a["rule"]
        ? (rule?.["severity"] ?? "warning")
        : "warning") as SemgrepAlert["severity"],
      message: String((a["most_recent_instance"] as Record<string, unknown> | undefined)
        ?.["message"]?.valueOf() ?? a["rule"]?.valueOf() ?? ""),
      path: String(location?.["path"] ?? ""),
      line: Number(location?.["start_line"] ?? 0),
      end_line: Number(location?.["end_line"] ?? 0),
      url: String(a["html_url"] ?? ""),
    };
  });

  // Filter to only alerts touching changed files
  const changedPaths = new Set(pr.changed_files.map((f) => f.filename));
  const relevant = alerts.filter((a) => changedPaths.has(a.path));

  ctx.log.info(`Semgrep: ${relevant.length}/${alerts.length} alerts on changed files`);
  return { alerts: relevant, tool: "Semgrep" };
}
