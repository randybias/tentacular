import type { Context } from "tentacular";

/** A file changed in the PR with its patch/diff */
interface ChangedFile {
  filename: string;
  status: "added" | "modified" | "removed" | "renamed";
  additions: number;
  deletions: number;
  patch?: string; // may be absent for binary files
}

/** Structured PR context passed downstream to all parallel scanner nodes */
export interface PrContext {
  owner: string;
  repo: string;
  pr_number: number;
  head_sha: string;
  base_sha: string;
  pr_title: string;
  pr_body: string;
  pr_url: string;
  changed_files: ChangedFile[];
  /** Concatenated patches (all files), capped at 20k chars */
  diff_summary: string;
}

/** Minimal subset of a GitHub webhook pull_request payload */
interface WebhookPayload {
  _webhook?: { event: string; action?: string; delivery_id: string };
  pull_request: {
    number: number;
    title: string;
    body: string | null;
    html_url: string;
    head: { sha: string };
    base: { sha: string };
  };
  repository: {
    name: string;
    full_name: string;
    owner: { login: string };
  };
}

/**
 * Fetch PR metadata, changed files, and per-file diffs from GitHub.
 * This is the root node — receives either:
 *   - A raw GitHub webhook payload (production: webhook trigger)
 *   - An empty/manual input (dev: uses ctx.config fallback for test_owner/test_repo/test_pr_number)
 */
export default async function run(ctx: Context, input: unknown): Promise<PrContext> {
  const payload = input as WebhookPayload;

  // Manual trigger fallback: use config-defined test PR
  const cfg = ctx.config as Record<string, unknown>;
  const owner = payload?.repository?.owner?.login ?? String(cfg["test_owner"] ?? "");
  const repoName = payload?.repository?.name ?? String(cfg["test_repo"] ?? "");
  const prNumber = payload?.pull_request?.number ?? Number(cfg["test_pr_number"] ?? 0);

  if (!owner || !repoName || !prNumber) {
    throw new Error(
      "No webhook payload and no test_owner/test_repo/test_pr_number in config. " +
      "Set config.test_owner, config.test_repo, config.test_pr_number for manual trigger.",
    );
  }

  // Build minimal pr/repo references for downstream use
  const pr = payload?.pull_request ?? {
    number: prNumber,
    title: "(manual trigger)",
    body: null,
    html_url: `https://github.com/${owner}/${repoName}/pull/${prNumber}`,
    head: { sha: "" },
    base: { sha: "" },
  };

  ctx.log.info(`Fetching PR #${prNumber} from ${owner}/${repoName}`);

  const github = ctx.dependency("github");
  const auth = `Bearer ${github.secret}`;

  // For manual trigger, head.sha is empty — fetch PR details from API
  let headSha = pr.head.sha;
  let baseSha = pr.base.sha;
  let prTitle = pr.title;
  let prBody = pr.body ?? "";
  let prUrl = pr.html_url;

  if (!headSha) {
    const prRes = await github.fetch!(
      `/repos/${owner}/${repoName}/pulls/${prNumber}`,
      { headers: { Authorization: auth, Accept: "application/vnd.github+json" } },
    );
    if (!prRes.ok) {
      throw new Error(`GitHub PR API error: ${prRes.status} ${await prRes.text()}`);
    }
    const prData = await prRes.json() as Record<string, unknown>;
    const head = prData["head"] as Record<string, unknown>;
    const base = prData["base"] as Record<string, unknown>;
    headSha = String(head?.["sha"] ?? "");
    baseSha = String(base?.["sha"] ?? "");
    prTitle = String(prData["title"] ?? prTitle);
    prBody = String(prData["body"] ?? "");
    prUrl = String(prData["html_url"] ?? prUrl);
    ctx.log.info(`Fetched PR details: "${prTitle}" ${headSha.slice(0, 7)}`);
  }

  // Fetch changed files (includes per-file patches/diffs)
  const filesRes = await github.fetch!(
    `/repos/${owner}/${repoName}/pulls/${prNumber}/files?per_page=100`,
    { headers: { Authorization: auth, Accept: "application/vnd.github+json" } },
  );

  if (!filesRes.ok) {
    throw new Error(`GitHub files API error: ${filesRes.status} ${await filesRes.text()}`);
  }

  const rawFilesData = await filesRes.json();
  const rawFiles = Array.isArray(rawFilesData) ? rawFilesData as Record<string, unknown>[] : [];

  const changedFiles: ChangedFile[] = rawFiles.map((f) => ({
    filename: String(f["filename"] ?? ""),
    status: String(f["status"] ?? "modified") as ChangedFile["status"],
    additions: Number(f["additions"] ?? 0),
    deletions: Number(f["deletions"] ?? 0),
    patch: typeof f["patch"] === "string" ? f["patch"] : undefined,
  }));

  // Concatenate all patches into a diff summary (cap at 20k chars to stay within Claude context)
  const MAX_DIFF_CHARS = 20_000;
  let diffSummary = "";
  for (const file of changedFiles) {
    if (file.patch) {
      const header = `\n--- ${file.filename} (${file.status}) +${file.additions} -${file.deletions} ---\n`;
      if ((diffSummary + header + file.patch).length > MAX_DIFF_CHARS) {
        diffSummary += `\n[diff truncated — ${changedFiles.length} files total]`;
        break;
      }
      diffSummary += header + file.patch;
    }
  }

  ctx.log.info(
    `PR #${prNumber}: ${changedFiles.length} files changed, diff ${diffSummary.length} chars`,
  );

  return {
    owner,
    repo: repoName,
    pr_number: prNumber,
    head_sha: headSha,
    base_sha: baseSha,
    pr_title: prTitle,
    pr_body: prBody,
    pr_url: prUrl,
    changed_files: changedFiles,
    diff_summary: diffSummary,
  };
}
