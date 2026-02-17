import type { Context } from "tentacular";

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

interface RepoMetrics {
  repo: string;
  stars: number;
  forks: number;
  openIssues: number;
  openPRs: number;
  recentCommits: number;
  releaseCount: number;
  latestRelease: string;
  topLanguage: string;
  contributorCount: number;
  weeklyCommitTrend: number[];
  velocity: number;
}

interface CompositeMetrics {
  repos: RepoMetrics[];
  rankings: {
    byStars: string[];
    byVelocity: string[];
    byCommits: string[];
    byIssues: string[];
  };
  totals: {
    totalStars: number;
    totalForks: number;
    totalOpenIssues: number;
    totalRecentCommits: number;
  };
  generatedAt: string;
}

/** Compute aggregate metrics from fan-in repo snapshots */
function computeRepoMetrics(snapshot: RepoSnapshot): RepoMetrics {
  const releaseCount = snapshot.recentReleases.length;
  const latestRelease = releaseCount > 0
    ? snapshot.recentReleases[0].tag
    : "none";

  // Top language by bytes
  const langEntries = Object.entries(snapshot.languages);
  const topLanguage = langEntries.length > 0
    ? langEntries.sort((a, b) => b[1] - a[1])[0][0]
    : "none";

  // Last 4 weeks of commit activity
  const weeklyCommitTrend = snapshot.commitActivity
    .slice(-4)
    .map((w) => w.count);

  const velocity = snapshot.recentCommits * 2 + snapshot.openPRs + releaseCount * 3;

  return {
    repo: snapshot.repo,
    stars: snapshot.stars,
    forks: snapshot.forks,
    openIssues: snapshot.openIssues,
    openPRs: snapshot.openPRs,
    recentCommits: snapshot.recentCommits,
    releaseCount,
    latestRelease,
    topLanguage,
    contributorCount: snapshot.topContributors.length,
    weeklyCommitTrend,
    velocity,
  };
}

/** Rank repo names by a numeric field in descending order */
function rankBy(repos: RepoMetrics[], field: keyof RepoMetrics): string[] {
  return [...repos]
    .sort((a, b) => (b[field] as number) - (a[field] as number))
    .map((r) => r.repo);
}

/** Fan-in node: compute composite metrics from upstream repo snapshots */
export default async function run(ctx: Context, input: unknown): Promise<CompositeMetrics> {
  const merged = input as {
    "fetch-codex": RepoSnapshot;
    "fetch-gemini": RepoSnapshot;
    "fetch-goose": RepoSnapshot;
  };

  const snapshots = [merged["fetch-codex"], merged["fetch-gemini"], merged["fetch-goose"]];
  const repos = snapshots.map(computeRepoMetrics);

  const rankings = {
    byStars: rankBy(repos, "stars"),
    byVelocity: rankBy(repos, "velocity"),
    byCommits: rankBy(repos, "recentCommits"),
    byIssues: rankBy(repos, "openIssues"),
  };

  const totals = {
    totalStars: repos.reduce((sum, r) => sum + r.stars, 0),
    totalForks: repos.reduce((sum, r) => sum + r.forks, 0),
    totalOpenIssues: repos.reduce((sum, r) => sum + r.openIssues, 0),
    totalRecentCommits: repos.reduce((sum, r) => sum + r.recentCommits, 0),
  };

  const result: CompositeMetrics = {
    repos,
    rankings,
    totals,
    generatedAt: new Date().toISOString(),
  };

  ctx.log.info(
    `Metrics: ${repos.length} repos, ${totals.totalStars} stars, ${totals.totalRecentCommits} commits, top velocity=${rankings.byVelocity[0]}`,
  );

  return result;
}
