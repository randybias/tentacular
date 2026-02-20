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
 * This is the root node — receives the raw webhook payload as input.
 */
export default async function run(ctx: Context, input: unknown): Promise<PrContext> {
  const payload = input as WebhookPayload;
  const { pull_request: pr, repository: repo } = payload;

  const owner = repo.owner.login;
  const repoName = repo.name;
  const prNumber = pr.number;

  ctx.log.info(`Fetching PR #${prNumber} from ${owner}/${repoName}`);

  const github = ctx.dependency("github");
  const auth = `Bearer ${github.secret}`;

  // Fetch changed files (includes per-file patches/diffs)
  const filesRes = await github.fetch!(
    `/repos/${owner}/${repoName}/pulls/${prNumber}/files?per_page=100`,
    { headers: { Authorization: auth, Accept: "application/vnd.github+json" } },
  );

  if (!filesRes.ok) {
    throw new Error(`GitHub files API error: ${filesRes.status} ${await filesRes.text()}`);
  }

  const rawFiles = await filesRes.json() as Record<string, unknown>[];

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
    head_sha: pr.head.sha,
    base_sha: pr.base.sha,
    pr_title: pr.title,
    pr_body: pr.body ?? "",
    pr_url: pr.html_url,
    changed_files: changedFiles,
    diff_summary: diffSummary,
  };
}

// Note: GitHub truncates patches for large files (>1000 lines changed).
// In that case, `patch` will be undefined and the file is excluded from the diff summary.
// For very large PRs, consider fetching the raw diff via the compare endpoint instead.

