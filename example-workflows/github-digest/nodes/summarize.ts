import type { Context } from "tentacular";

/** Repository data from upstream fetch-repos node */
interface RepoData {
  name: string;
  description: string;
  stars: number;
}

/** Digest summary produced by this node */
interface DigestSummary {
  title: string;
  summary: string;
  repoCount: number;
}

/**
 * Summarize node - pure transform that creates a digest from repository data.
 *
 * @param ctx - Execution context (only logging used, no fetch)
 * @param input - Repository data from fetch-repos node
 * @returns Digest summary with title, formatted summary text, and repo count
 */
export default async function run(ctx: Context, input: unknown): Promise<DigestSummary> {
  const data = input as { repos: RepoData[] };
  const repos = data.repos || [];

  ctx.log.info(`Summarizing ${repos.length} repositories`);

  const totalStars = repos.reduce((sum, r) => sum + r.stars, 0);
  const lines = repos.map((r) => `- ${r.name} (${r.stars} stars): ${r.description}`);

  return {
    title: "GitHub Repository Digest",
    summary: `Found ${repos.length} repositories with ${totalStars} total stars:\n${lines.join("\n")}`,
    repoCount: repos.length,
  };
}
