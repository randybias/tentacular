import type { Context } from "tentacular";

/** Repository data returned by this node */
interface RepoData {
  name: string;
  description: string;
  stars: number;
}

/**
 * Fetch repos node - source node that retrieves GitHub repository data.
 *
 * @param ctx - Execution context with fetch, log, config, secrets
 * @param input - Unused (source node, receives undefined)
 * @returns Array of repository records with name, description, and stars
 */
export default async function run(ctx: Context, _input: unknown): Promise<{ repos: RepoData[] }> {
  ctx.log.info("Fetching GitHub repositories");

  const github = ctx.dependency("github");
  const response = await github.fetch!("/users/denoland/repos?sort=stars&per_page=5");
  const data = await response.json();

  const repos: RepoData[] = Array.isArray(data)
    ? data.map((repo: Record<string, unknown>) => ({
        name: String(repo.name || ""),
        description: String(repo.description || "No description"),
        stars: Number(repo.stargazers_count || 0),
      }))
    : [];

  ctx.log.info(`Fetched ${repos.length} repositories`);
  return { repos };
}
