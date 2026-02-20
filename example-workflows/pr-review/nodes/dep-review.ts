import type { Context } from "tentacular";
import type { PrContext } from "./fetch-pr.ts";

/** A dependency vulnerability from GitHub Dependency Review API */
export interface Vulnerability {
  package_name: string;
  package_ecosystem: string;
  manifest_path: string;
  change_type: "added" | "removed";
  vulnerabilities: Array<{
    severity: "critical" | "high" | "moderate" | "low";
    advisory_ghsa_id: string;
    advisory_summary: string;
    advisory_url: string;
  }>;
}

/** A license concern from the Dependency Review API */
export interface LicenseConcern {
  package_name: string;
  license: string;
  manifest_path: string;
}

export interface DepReviewOutput {
  vulnerabilities: Vulnerability[];
  license_concerns: LicenseConcern[];
}

/**
 * Query GitHub Dependency Review API for new CVEs introduced by this PR.
 *
 * Free for public repos. Private repos require GitHub Advanced Security.
 * Returns empty results (not an error) when unavailable.
 */
export default async function run(ctx: Context, input: unknown): Promise<DepReviewOutput> {
  const pr = input as PrContext;
  ctx.log.info(
    `Checking dependency vulnerabilities for ${pr.owner}/${pr.repo} PR#${pr.pr_number}`,
  );

  const github = ctx.dependency("github");
  const auth = `Bearer ${github.secret}`;

  const res = await github.fetch!(
    `/repos/${pr.owner}/${pr.repo}/dependency-graph/compare/${pr.base_sha}...${pr.head_sha}`,
    { headers: { Authorization: auth, Accept: "application/vnd.github+json" } },
  );

  // 404 = dependency graph not enabled; 403 = private repo without GHAS
  if (res.status === 404 || res.status === 403) {
    ctx.log.warn(`Dependency Review API not available (${res.status}) — skipping`);
    return { vulnerabilities: [], license_concerns: [] };
  }

  if (!res.ok) {
    throw new Error(`GitHub Dependency Review API error: ${res.status} ${await res.text()}`);
  }

  const data = await res.json() as Record<string, unknown>;
  const packages = (data["packages"] as Record<string, unknown>[] | undefined) ?? [];

  const vulnerabilities: Vulnerability[] = [];
  const licenseConcerns: LicenseConcern[] = [];

  // Define permissive licenses (safe) — anything else is flagged
  const PERMISSIVE = new Set([
    "MIT", "Apache-2.0", "BSD-2-Clause", "BSD-3-Clause", "ISC", "0BSD",
    "CC0-1.0", "Unlicense", "WTFPL",
  ]);

  for (const pkg of packages) {
    const changeType = String(pkg["change_type"] ?? "");
    // Only care about newly added dependencies
    if (changeType !== "added") continue;

    const vulns = pkg["vulnerabilities"] as Record<string, unknown>[] | undefined;
    if (vulns && vulns.length > 0) {
      vulnerabilities.push({
        package_name: String(pkg["name"] ?? ""),
        package_ecosystem: String(pkg["ecosystem"] ?? ""),
        manifest_path: String(pkg["manifest"] ?? ""),
        change_type: "added",
        vulnerabilities: vulns.map((v) => ({
          severity: String(v["severity"] ?? "low") as Vulnerability["vulnerabilities"][0]["severity"],
          advisory_ghsa_id: String(v["advisory_ghsa_id"] ?? ""),
          advisory_summary: String(v["advisory_summary"] ?? ""),
          advisory_url: String(v["advisory_url"] ?? ""),
        })),
      });
    }

    const license = String(pkg["license"] ?? "UNKNOWN");
    if (!PERMISSIVE.has(license)) {
      licenseConcerns.push({
        package_name: String(pkg["name"] ?? ""),
        license,
        manifest_path: String(pkg["manifest"] ?? ""),
      });
    }
  }

  ctx.log.info(
    `Dependency review: ${vulnerabilities.length} vulnerable packages, ` +
    `${licenseConcerns.length} license concerns`,
  );

  return { vulnerabilities, license_concerns: licenseConcerns };
}
