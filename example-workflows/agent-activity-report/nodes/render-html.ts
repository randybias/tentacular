import type { Context } from "tentacular";

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

interface LLMAnalysis {
  narrative: string;
  highlights: string[];
  prediction: string;
  healthScores: Record<string, number>;
}

interface HtmlReport {
  html: string;
  title: string;
  summary: string;
}

function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
}

function healthColor(score: number): string {
  if (score > 70) return "#22863a";
  if (score >= 40) return "#e36209";
  return "#cb2431";
}

function healthLabel(score: number): string {
  if (score > 70) return "Healthy";
  if (score >= 40) return "Needs Attention";
  return "At Risk";
}

function rankBadge(rank: number): string {
  const colors = ["#ffd700", "#c0c0c0", "#cd7f32"];
  const labels = ["1st", "2nd", "3rd"];
  const color = colors[rank] ?? "#6a737d";
  const label = labels[rank] ?? `${rank + 1}th`;
  return `<span style="background:${color};color:#24292e;padding:2px 8px;border-radius:10px;font-size:11px;font-weight:700">${label}</span>`;
}

function sparkline(values: number[]): string {
  if (values.length === 0) return "";
  const max = Math.max(...values, 1);
  const bars = values.map((v) => {
    const height = Math.max(Math.round((v / max) * 20), 1);
    return `<span style="display:inline-block;width:6px;height:${height}px;background:#0366d6;margin:0 1px;vertical-align:bottom;border-radius:1px"></span>`;
  });
  return `<span style="display:inline-flex;align-items:flex-end;height:22px">${bars.join("")}</span>`;
}

function renderRepoCard(repo: RepoMetrics, healthScore: number): string {
  const color = healthColor(healthScore);
  const label = healthLabel(healthScore);

  return `<div class="repo-card">
  <div class="repo-header">
    <h3>${escapeHtml(repo.repo)}</h3>
    <span class="health-badge" style="background:${color}">${healthScore} - ${label}</span>
  </div>
  <div class="repo-stats">
    <div class="stat"><span class="stat-value">${repo.stars.toLocaleString()}</span><span class="stat-label">Stars</span></div>
    <div class="stat"><span class="stat-value">${repo.forks.toLocaleString()}</span><span class="stat-label">Forks</span></div>
    <div class="stat"><span class="stat-value">${repo.openIssues}</span><span class="stat-label">Issues</span></div>
    <div class="stat"><span class="stat-value">${repo.openPRs}</span><span class="stat-label">PRs</span></div>
    <div class="stat"><span class="stat-value">${repo.recentCommits}</span><span class="stat-label">Commits</span></div>
    <div class="stat"><span class="stat-value">${repo.velocity}</span><span class="stat-label">Velocity</span></div>
  </div>
  <div class="repo-meta">
    <span>Language: <strong>${escapeHtml(repo.topLanguage)}</strong></span>
    <span>Contributors: <strong>${repo.contributorCount}</strong></span>
    <span>Releases: <strong>${repo.releaseCount}</strong></span>
    <span>Latest: <strong>${escapeHtml(repo.latestRelease || "N/A")}</strong></span>
  </div>
  <div class="repo-trend">Weekly commits: ${sparkline(repo.weeklyCommitTrend)}</div>
</div>`;
}

/** Render styled HTML report from LLM analysis and composite metrics */
export default async function run(ctx: Context, input: unknown): Promise<HtmlReport> {
  const merged = input as { "generate-analysis": LLMAnalysis; "compute-metrics": CompositeMetrics };
  const analysis = merged["generate-analysis"];
  const metrics = merged["compute-metrics"];

  const now = new Date().toISOString();
  const title = `AI Agent Activity Report - ${formatDate(now)}`;

  // Health scores section
  const healthBadges = Object.entries(analysis.healthScores)
    .map(([repo, score]) => {
      const color = healthColor(score);
      return `<div class="health-item">
    <span class="health-name">${escapeHtml(repo)}</span>
    <span class="health-score" style="color:${color}">${score}</span>
    <span class="health-label" style="color:${color}">${healthLabel(score)}</span>
  </div>`;
    })
    .join("\n");

  // Rankings table
  const maxRankLen = Math.max(
    metrics.rankings.byStars.length,
    metrics.rankings.byVelocity.length,
    metrics.rankings.byCommits.length,
    metrics.rankings.byIssues.length,
  );
  const rankingRows = Array.from({ length: maxRankLen }, (_, i) => {
    const star = metrics.rankings.byStars[i] ?? "-";
    const vel = metrics.rankings.byVelocity[i] ?? "-";
    const com = metrics.rankings.byCommits[i] ?? "-";
    const iss = metrics.rankings.byIssues[i] ?? "-";
    return `<tr>
  <td>${rankBadge(i)}</td>
  <td>${escapeHtml(star)}</td>
  <td>${escapeHtml(vel)}</td>
  <td>${escapeHtml(com)}</td>
  <td>${escapeHtml(iss)}</td>
</tr>`;
  }).join("\n");

  // Per-repo detail cards
  const repoCards = metrics.repos
    .map((repo) => renderRepoCard(repo, analysis.healthScores[repo.repo] ?? 0))
    .join("\n");

  // Highlights list
  const highlightItems = analysis.highlights
    .map((h) => `<li>${escapeHtml(h)}</li>`)
    .join("\n");

  // Narrative paragraphs
  const narrativeParagraphs = analysis.narrative
    .split("\n")
    .filter((p) => p.trim())
    .map((p) => `<p>${escapeHtml(p.trim())}</p>`)
    .join("\n");

  const summaryText = Object.entries(analysis.healthScores)
    .map(([repo, score]) => `${repo}: ${score}/100`)
    .join(" | ");

  const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>${escapeHtml(title)}</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background: #f6f8fa; color: #24292e; }
    .container { max-width: 960px; margin: 0 auto; }
    .header { background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%); color: white; padding: 32px; border-radius: 12px 12px 0 0; }
    .header h1 { font-size: 26px; font-weight: 700; margin-bottom: 8px; }
    .header .subtitle { opacity: 0.9; font-size: 14px; }
    .totals-bar { display: flex; gap: 0; background: white; border-bottom: 1px solid #e1e4e8; }
    .totals-bar .metric { flex: 1; text-align: center; padding: 20px 12px; border-right: 1px solid #e1e4e8; }
    .totals-bar .metric:last-child { border-right: none; }
    .metric .value { font-size: 28px; font-weight: 700; color: #0366d6; }
    .metric .label { font-size: 11px; color: #6a737d; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px; }
    .section { background: white; padding: 24px 32px; border-bottom: 1px solid #e1e4e8; }
    .section h2 { font-size: 18px; font-weight: 600; margin-bottom: 12px; }
    .health-grid { display: flex; gap: 24px; justify-content: center; flex-wrap: wrap; }
    .health-item { text-align: center; min-width: 140px; }
    .health-name { display: block; font-size: 13px; font-weight: 600; margin-bottom: 4px; }
    .health-score { display: block; font-size: 48px; font-weight: 800; line-height: 1; }
    .health-label { display: block; font-size: 12px; font-weight: 600; margin-top: 4px; }
    table { width: 100%; border-collapse: collapse; margin-top: 8px; }
    th { text-align: left; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: #6a737d; padding: 8px 12px; border-bottom: 2px solid #e1e4e8; }
    td { padding: 10px 12px; border-bottom: 1px solid #e1e4e8; font-size: 13px; }
    .repo-card { background: #f6f8fa; border: 1px solid #e1e4e8; border-radius: 8px; padding: 20px; margin-bottom: 16px; }
    .repo-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px; }
    .repo-header h3 { font-size: 16px; font-weight: 700; }
    .health-badge { color: white; padding: 4px 12px; border-radius: 12px; font-size: 12px; font-weight: 600; }
    .repo-stats { display: flex; gap: 16px; flex-wrap: wrap; margin-bottom: 12px; }
    .stat { text-align: center; min-width: 64px; }
    .stat-value { display: block; font-size: 20px; font-weight: 700; color: #0366d6; }
    .stat-label { display: block; font-size: 10px; color: #6a737d; text-transform: uppercase; letter-spacing: 0.3px; }
    .repo-meta { display: flex; gap: 16px; flex-wrap: wrap; font-size: 12px; color: #6a737d; margin-bottom: 8px; }
    .repo-trend { font-size: 12px; color: #6a737d; }
    .narrative p { font-size: 14px; line-height: 1.7; color: #444d56; margin-bottom: 12px; }
    .highlights { padding-left: 20px; }
    .highlights li { font-size: 14px; line-height: 1.8; color: #444d56; }
    .prediction { background: #f1f8ff; border-left: 4px solid #0366d6; padding: 16px 20px; font-size: 14px; color: #24292e; margin-top: 12px; border-radius: 0 4px 4px 0; }
    .footer { padding: 20px 32px; background: white; border-radius: 0 0 12px 12px; text-align: center; font-size: 12px; color: #6a737d; }
    @media (max-width: 640px) {
      body { padding: 0; }
      .header { border-radius: 0; }
      .footer { border-radius: 0; }
      .totals-bar { flex-wrap: wrap; }
      .totals-bar .metric { flex-basis: 50%; border-bottom: 1px solid #e1e4e8; }
      .section { padding: 16px; }
      .health-score { font-size: 36px; }
      .repo-stats { gap: 8px; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>AI Agent Activity Report</h1>
      <div class="subtitle">${formatDate(now)}</div>
    </div>
    <div class="totals-bar">
      <div class="metric"><div class="value">${metrics.totals.totalStars.toLocaleString()}</div><div class="label">Total Stars</div></div>
      <div class="metric"><div class="value">${metrics.totals.totalForks.toLocaleString()}</div><div class="label">Total Forks</div></div>
      <div class="metric"><div class="value">${metrics.totals.totalOpenIssues}</div><div class="label">Open Issues</div></div>
      <div class="metric"><div class="value">${metrics.totals.totalRecentCommits}</div><div class="label">Recent Commits</div></div>
    </div>
    <div class="section">
      <h2>Health Scores</h2>
      <div class="health-grid">
        ${healthBadges}
      </div>
    </div>
    <div class="section">
      <h2>Rankings</h2>
      <table>
        <thead><tr><th>Rank</th><th>Stars</th><th>Velocity</th><th>Commits</th><th>Issues</th></tr></thead>
        <tbody>${rankingRows}</tbody>
      </table>
    </div>
    <div class="section">
      <h2>Repository Details</h2>
      ${repoCards}
    </div>
    <div class="section">
      <h2>Analysis</h2>
      <div class="narrative">${narrativeParagraphs}</div>
    </div>
    <div class="section">
      <h2>Key Highlights</h2>
      <ul class="highlights">${highlightItems}</ul>
    </div>
    <div class="section">
      <h2>Momentum Prediction</h2>
      <div class="prediction">${escapeHtml(analysis.prediction)}</div>
    </div>
    <div class="footer">
      Generated by Tentacular Agent Activity Report | ${escapeHtml(now)}
    </div>
  </div>
</body>
</html>`;

  ctx.log.info(`Rendered report: ${summaryText}`);

  return { html, title, summary: summaryText };
}
