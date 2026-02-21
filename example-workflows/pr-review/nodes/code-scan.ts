import type { Context } from "tentacular";
import type { PrContext } from "./fetch-pr.ts";

/** A CodeQL alert from GitHub Code Scanning */
export interface CodeScanAlert {
  id: number;
  rule_id: string;
  rule_description: string;
  severity: "critical" | "high" | "medium" | "low" | "note" | "warning" | "error";
  message: string;
  path: string;
  line: number;
  end_line: number;
  url: string;
  tags: string[];
}

export interface CodeScanOutput {
  alerts: CodeScanAlert[];
  tool: "CodeQL";
}

/**
 * Query GitHub Code Scanning API for CodeQL alerts on the PR head commit.
 *
 * Works when CodeQL is configured as a GitHub Actions workflow (free for public repos).
 * For private repos, requires GitHub Advanced Security.
 * Returns empty results (not an error) when unavailable.
 */
export default async function run(ctx: Context, input: unknown): Promise<CodeScanOutput> {
  const pr = input as PrContext;
  ctx.log.info(`Querying CodeQL alerts for ${pr.owner}/${pr.repo}@${pr.head_sha.slice(0, 7)}`);

  const github = ctx.dependency("github");
  const auth = `Bearer ${github.secret}`;

  const res = await github.fetch!(
    `/repos/${pr.owner}/${pr.repo}/code-scanning/alerts` +
    `?ref=${pr.head_sha}&tool_name=CodeQL&state=open&per_page=50`,
    { headers: { Authorization: auth, Accept: "application/vnd.github+json" } },
  );

  // 404 = code scanning not enabled; 403 = GHAS required
  if (res.status === 404 || res.status === 403) {
    ctx.log.warn(`CodeQL code scanning not available (${res.status}) — skipping`);
    return { alerts: [], tool: "CodeQL" };
  }

  if (!res.ok) {
    throw new Error(`GitHub Code Scanning API error: ${res.status} ${await res.text()}`);
  }

  const rawData = await res.json();
  const raw = Array.isArray(rawData) ? rawData as Record<string, unknown>[] : [];

  const alerts: CodeScanAlert[] = raw.map((a) => {
    const instance = (a["most_recent_instance"] as Record<string, unknown> | undefined) ?? {};
    const location = (instance["location"] as Record<string, unknown> | undefined) ?? {};
    const rule = (a["rule"] as Record<string, unknown> | undefined) ?? {};
    return {
      id: Number(a["number"] ?? 0),
      rule_id: String(rule["id"] ?? "unknown"),
      rule_description: String(rule["description"] ?? rule["name"] ?? ""),
      severity: String(rule["severity"] ?? "warning") as CodeScanAlert["severity"],
      message: String((instance["message"] as Record<string, unknown> | undefined)?.["text"] ?? ""),
      path: String(location["path"] ?? ""),
      line: Number(location["start_line"] ?? 0),
      end_line: Number(location["end_line"] ?? 0),
      url: String(a["html_url"] ?? ""),
      tags: Array.isArray(rule["tags"]) ? rule["tags"].map(String) : [],
    };
  });

  // Filter to only alerts touching changed files
  const changedPaths = new Set(pr.changed_files.map((f) => f.filename));
  const relevant = alerts.filter((a) => changedPaths.has(a.path));

  ctx.log.info(`CodeQL: ${relevant.length}/${alerts.length} alerts on changed files`);
  return { alerts: relevant, tool: "CodeQL" };
}
