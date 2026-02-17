import type { Context } from "tentacular";

const OWNER = "block";
const NAME = "goose";

interface RepoSnapshot {
  repo: string;
  fetchedAt: string;
  stars: number;
  forks: number;
  openIssues: number;
  openPRs: number;
  recentCommits: number;
  recentReleases: { tag: string; name: string; publishedAt: string }[];
  topContributors: { login: string; contributions: number }[];
  commitActivity: { week: string; count: number }[];
  languages: Record<string, number>;
}

async function ghFetch(
  github: { fetch: ((path: string, init?: RequestInit) => Promise<Response>) | null; secret: string },
  path: string,
): Promise<Response> {
  const headers = {
    "Authorization": `Bearer ${github.secret}`,
    "Accept": "application/vnd.github+json",
  };
  const resp = await github.fetch!(path, { headers });
  if (!resp.ok) {
    throw new Error(`GitHub API error ${resp.status} on ${path}: ${await resp.text()}`);
  }
  return resp;
}

export default async function run(ctx: Context, _input: unknown): Promise<RepoSnapshot> {
  const repo = `${OWNER}/${NAME}`;
  ctx.log.info(`Fetching activity snapshot for ${repo}`);

  const github = ctx.dependency("github");

  // 1. Repo metadata — stars, forks, open issues
  const repoData = await (await ghFetch(github, `/repos/${repo}`)).json() as Record<string, unknown>;
  const stars = Number(repoData.stargazers_count ?? 0);
  const forks = Number(repoData.forks_count ?? 0);
  const openIssues = Number(repoData.open_issues_count ?? 0);

  // 2. Open PR count via search API
  const searchData = await (await ghFetch(
    github,
    `/search/issues?q=repo:${repo}+type:pr+state:open`,
  )).json() as Record<string, unknown>;
  const openPRs = Number(searchData.total_count ?? 0);

  // 3. Recent commits (last 7 days)
  const since = new Date(Date.now() - 7 * 24 * 60 * 60 * 1000).toISOString();
  const commitsData = await (await ghFetch(
    github,
    `/repos/${repo}/commits?since=${since}&per_page=100`,
  )).json() as unknown[];
  const recentCommits = Array.isArray(commitsData) ? commitsData.length : 0;

  // 4. Recent releases (last 5)
  const releasesData = await (await ghFetch(
    github,
    `/repos/${repo}/releases?per_page=5`,
  )).json() as Record<string, unknown>[];
  const recentReleases = Array.isArray(releasesData)
    ? releasesData.map((r) => ({
        tag: String(r.tag_name ?? ""),
        name: String(r.name ?? ""),
        publishedAt: String(r.published_at ?? ""),
      }))
    : [];

  // 5. Top contributors (top 10)
  const contribData = await (await ghFetch(
    github,
    `/repos/${repo}/contributors?per_page=10`,
  )).json() as Record<string, unknown>[];
  const topContributors = Array.isArray(contribData)
    ? contribData.map((c) => ({
        login: String(c.login ?? ""),
        contributions: Number(c.contributions ?? 0),
      }))
    : [];

  // 6. Commit activity (last 4 weeks from stats API)
  const activityData = await (await ghFetch(
    github,
    `/repos/${repo}/stats/commit_activity`,
  )).json() as Record<string, unknown>[];
  const commitActivity = Array.isArray(activityData)
    ? activityData.slice(-4).map((w) => ({
        week: new Date(Number(w.week) * 1000).toISOString(),
        count: Number(w.total ?? 0),
      }))
    : [];

  // 7. Language breakdown
  const languages = await (await ghFetch(
    github,
    `/repos/${repo}/languages`,
  )).json() as Record<string, number>;

  const snapshot: RepoSnapshot = {
    repo,
    fetchedAt: new Date().toISOString(),
    stars,
    forks,
    openIssues,
    openPRs,
    recentCommits,
    recentReleases,
    topContributors,
    commitActivity,
    languages: languages ?? {},
  };

  ctx.log.info(`Snapshot complete for ${repo}: ${stars} stars, ${openPRs} open PRs, ${recentCommits} recent commits`);
  return snapshot;
}
