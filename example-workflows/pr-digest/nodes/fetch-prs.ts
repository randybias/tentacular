import type { Context } from "tentacular";

interface PullRequest {
  number: number;
  title: string;
  author: string;
  url: string;
  labels: string[];
  draft: boolean;
  createdAt: string;
  body: string;
}

/** Fetch open pull requests from a GitHub repository */
export default async function run(ctx: Context, _input: unknown): Promise<{ prs: PullRequest[]; repo: string }> {
  const repo = "denoland/deno";
  ctx.log.info(`Fetching open PRs from ${repo}`);

  const github = ctx.dependency("github");
  const response = await github.fetch!(`/repos/${repo}/pulls?state=open&per_page=10&sort=updated`, {
    headers: { "Authorization": `Bearer ${github.secret}` },
  });
  const data = await response.json();

  const prs: PullRequest[] = Array.isArray(data)
    ? data.map((pr: Record<string, unknown>) => ({
        number: Number(pr.number),
        title: String(pr.title || ""),
        author: String((pr.user as Record<string, unknown>)?.login || "unknown"),
        url: String(pr.html_url || ""),
        labels: Array.isArray(pr.labels)
          ? (pr.labels as Record<string, unknown>[]).map((l) => String(l.name || ""))
          : [],
        draft: Boolean(pr.draft),
        createdAt: String(pr.created_at || ""),
        body: String(pr.body || "").substring(0, 500),
      }))
    : [];

  ctx.log.info(`Fetched ${prs.length} open PRs`);
  return { prs, repo };
}
