import type { Context } from "tentacular";

interface GitHubPR {
  number: number;
  title: string;
  state: string;
  html_url: string;
  created_at: string;
  updated_at: string;
  user: { login: string };
  labels: Array<{ name: string }>;
  body: string | null;
  draft?: boolean;
}

interface Sep {
  number: number;
  sepId: string;
  title: string;
  state: string;
  author: string;
  createdAt: string;
  updatedAt: string;
  url: string;
  labels: string[];
  summary: string;
}

interface SepSnapshot {
  timestamp: string;
  repo: string;
  seps: Sep[];
  count: number;
}

function extractSepId(title: string): string {
  const match = title.toUpperCase().match(/SEP-\d+/);
  return match ? match[0] : "SEP-???";
}

function extractSummary(body: string | null, title: string): string {
  if (body) {
    const sanitized = body.replace(/[\r\n]+/g, " ").trim();
    if (sanitized.length > 200) {
      return sanitized.slice(0, 197) + "...";
    }
    if (sanitized.length > 0) {
      return sanitized;
    }
  }
  return title.replace(/[\r\n]+/g, " ").trim();
}

/** Fetch open SEP PRs from GitHub */
export default async function run(ctx: Context, input: unknown): Promise<SepSnapshot> {
  const repo = (ctx.config.target_repo as string) ?? "modelcontextprotocol/specification";
  const sepLabel = (ctx.config.sep_label as string) ?? "sep";

  ctx.log.info(`Fetching PRs from ${repo}`);

  const github = ctx.dependency("github");
  const res = await github.fetch!(`/repos/${repo}/pulls?state=open&per_page=100`);
  if (!res.ok) {
    throw new Error(`GitHub API returned ${res.status} ${res.statusText}`);
  }

  const data = await res.json();
  const prs: GitHubPR[] = Array.isArray(data) ? data : [];

  const labelLower = sepLabel.toLowerCase();
  const seps: Sep[] = prs
    .filter((pr) => pr.labels.some((l) => l.name.toLowerCase() === labelLower))
    .map((pr) => ({
      number: pr.number,
      sepId: extractSepId(pr.title),
      title: pr.title,
      state: pr.draft ? "draft" : pr.state,
      author: pr.user.login,
      createdAt: pr.created_at,
      updatedAt: pr.updated_at,
      url: pr.html_url,
      labels: pr.labels.map((l) => l.name),
      summary: extractSummary(pr.body, pr.title),
    }));

  ctx.log.info(`Found ${seps.length} SEPs with label "${sepLabel}"`);

  return {
    timestamp: new Date().toISOString(),
    repo,
    seps,
    count: seps.length,
  };
}
