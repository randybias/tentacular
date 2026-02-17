import type { Context } from "tentacular";

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

interface ReportSection {
  title: string;
  content: string;
}

interface LLMReport {
  analysis: string;
  recommendations: string[];
  healthScore: number;
  sections: ReportSection[];
}

interface ActivityMetrics {
  highVelocity: Sep[];
  inactive: Sep[];
  newThisWeek: Sep[];
  closedThisWeek: Sep[];
  stateTransitions: { sepId: string; from: string; to: string }[];
  totalActive: number;
  totalInactive: number;
  velocityScore: number;
  weekOverWeek: { metric: string; thisWeek: number; lastWeek: number; delta: number }[];
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

function deltaIndicator(delta: number): string {
  if (delta > 0) return `<span style="color:#22863a">+${delta}</span>`;
  if (delta < 0) return `<span style="color:#cb2431">${delta}</span>`;
  return `<span style="color:#6a737d">0</span>`;
}

function renderSepRow(sep: Sep): string {
  return `<tr>
  <td><a href="${escapeHtml(sep.url)}">${escapeHtml(sep.sepId)}</a></td>
  <td>${escapeHtml(sep.title)}</td>
  <td><span class="status-badge status-${sep.state.toLowerCase()}">${escapeHtml(sep.state.toUpperCase())}</span></td>
  <td>${escapeHtml(sep.author)}</td>
  <td>${formatDate(sep.updatedAt)}</td>
</tr>`;
}

/** Generate styled HTML digest from LLM report and activity metrics */
export default async function run(ctx: Context, input: unknown): Promise<HtmlReport> {
  const merged = input as { "generate-report": LLMReport; "analyze-activity": ActivityMetrics };
  const report = merged["generate-report"];
  const metrics = merged["analyze-activity"];

  const now = new Date().toISOString();
  const title = `SEP Weekly Digest - ${formatDate(now)}`;
  const score = report.healthScore;
  const scoreColor = healthColor(score);
  const scoreLabel = healthLabel(score);

  // LLM analysis sections
  const llmSections = report.sections.map((s) => `
    <div class="section">
      <h2>${escapeHtml(s.title)}</h2>
      <div class="section-content">${escapeHtml(s.content)}</div>
    </div>`).join("\n");

  // High-velocity SEPs table
  const highVelocityRows = metrics.highVelocity.length > 0
    ? metrics.highVelocity.map(renderSepRow).join("\n")
    : `<tr><td colspan="5" class="empty-row">No high-velocity SEPs this week</td></tr>`;

  // Inactive SEPs table
  const inactiveRows = metrics.inactive.length > 0
    ? metrics.inactive.map(renderSepRow).join("\n")
    : `<tr><td colspan="5" class="empty-row">No inactive SEPs</td></tr>`;

  // Week-over-week trends
  const trendRows = metrics.weekOverWeek.length > 0
    ? metrics.weekOverWeek.map((w) => `<tr>
  <td>${escapeHtml(w.metric)}</td>
  <td>${w.lastWeek}</td>
  <td>${w.thisWeek}</td>
  <td>${deltaIndicator(w.delta)}</td>
</tr>`).join("\n")
    : `<tr><td colspan="4" class="empty-row">No trend data available</td></tr>`;

  // Recommendations
  const recommendationItems = report.recommendations.length > 0
    ? report.recommendations.map((r) => `<li>${escapeHtml(r)}</li>`).join("\n")
    : `<li class="empty-row">No recommendations at this time</li>`;

  const summaryText = `Health: ${score}/100 (${scoreLabel}) | Active: ${metrics.totalActive} | Inactive: ${metrics.totalInactive} | Velocity: ${metrics.velocityScore}`;

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
    .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 32px; border-radius: 12px 12px 0 0; }
    .header h1 { font-size: 26px; font-weight: 700; margin-bottom: 8px; }
    .header .subtitle { opacity: 0.9; font-size: 14px; }
    .health-score { background: white; padding: 32px; text-align: center; border-bottom: 1px solid #e1e4e8; }
    .health-score .score { font-size: 64px; font-weight: 800; line-height: 1; }
    .health-score .score-label { font-size: 16px; font-weight: 600; margin-top: 8px; }
    .health-score .score-sublabel { font-size: 12px; color: #6a737d; margin-top: 4px; }
    .metrics-bar { display: flex; gap: 0; background: white; border-bottom: 1px solid #e1e4e8; }
    .metrics-bar .metric { flex: 1; text-align: center; padding: 20px 12px; border-right: 1px solid #e1e4e8; }
    .metrics-bar .metric:last-child { border-right: none; }
    .metric .value { font-size: 28px; font-weight: 700; color: #0366d6; }
    .metric .label { font-size: 11px; color: #6a737d; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px; }
    .section { background: white; padding: 24px 32px; border-bottom: 1px solid #e1e4e8; }
    .section h2 { font-size: 18px; font-weight: 600; margin-bottom: 12px; }
    .section-content { font-size: 14px; line-height: 1.6; color: #444d56; white-space: pre-wrap; }
    table { width: 100%; border-collapse: collapse; margin-top: 8px; }
    th { text-align: left; font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; color: #6a737d; padding: 8px 12px; border-bottom: 2px solid #e1e4e8; }
    td { padding: 10px 12px; border-bottom: 1px solid #e1e4e8; font-size: 13px; }
    td a { color: #0366d6; text-decoration: none; font-weight: 600; }
    td a:hover { text-decoration: underline; }
    .empty-row { color: #6a737d; font-style: italic; text-align: center; }
    .status-badge { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.3px; }
    .status-open { background: #dcffe4; color: #22863a; }
    .status-draft { background: #fff3cd; color: #856d0a; }
    .status-closed, .status-merged { background: #ffdce0; color: #cb2431; }
    .recommendations { padding-left: 20px; }
    .recommendations li { font-size: 14px; line-height: 1.8; color: #444d56; }
    .footer { padding: 20px 32px; background: white; border-radius: 0 0 12px 12px; text-align: center; font-size: 12px; color: #6a737d; }
    @media (max-width: 640px) {
      body { padding: 0; }
      .header { border-radius: 0; }
      .footer { border-radius: 0; }
      .metrics-bar { flex-wrap: wrap; }
      .metrics-bar .metric { flex-basis: 50%; border-bottom: 1px solid #e1e4e8; }
      .section { padding: 16px; }
      .health-score .score { font-size: 48px; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>SEP Weekly Digest</h1>
      <div class="subtitle">${formatDate(now)}</div>
    </div>
    <div class="health-score">
      <div class="score" style="color: ${scoreColor}">${score}</div>
      <div class="score-label" style="color: ${scoreColor}">${escapeHtml(scoreLabel)}</div>
      <div class="score-sublabel">Health Score (0-100)</div>
    </div>
    <div class="metrics-bar">
      <div class="metric"><div class="value">${metrics.totalActive}</div><div class="label">Active</div></div>
      <div class="metric"><div class="value">${metrics.totalInactive}</div><div class="label">Inactive</div></div>
      <div class="metric"><div class="value" style="color:#22863a">${metrics.newThisWeek.length}</div><div class="label">New This Week</div></div>
      <div class="metric"><div class="value" style="color:#cb2431">${metrics.closedThisWeek.length}</div><div class="label">Closed This Week</div></div>
      <div class="metric"><div class="value">${metrics.velocityScore}</div><div class="label">Velocity</div></div>
    </div>
    ${llmSections}
    <div class="section">
      <h2>High-Velocity SEPs (${metrics.highVelocity.length})</h2>
      <table>
        <thead><tr><th>ID</th><th>Title</th><th>State</th><th>Author</th><th>Updated</th></tr></thead>
        <tbody>${highVelocityRows}</tbody>
      </table>
    </div>
    <div class="section">
      <h2>Inactive SEPs (${metrics.inactive.length})</h2>
      <table>
        <thead><tr><th>ID</th><th>Title</th><th>State</th><th>Author</th><th>Updated</th></tr></thead>
        <tbody>${inactiveRows}</tbody>
      </table>
    </div>
    <div class="section">
      <h2>Week-over-Week Trends</h2>
      <table>
        <thead><tr><th>Metric</th><th>Last Week</th><th>This Week</th><th>Delta</th></tr></thead>
        <tbody>${trendRows}</tbody>
      </table>
    </div>
    <div class="section">
      <h2>Recommendations</h2>
      <ul class="recommendations">${recommendationItems}</ul>
    </div>
    <div class="footer">
      Generated by Tentacular SEP Weekly Digest | ${escapeHtml(now)}
    </div>
  </div>
</body>
</html>`;

  ctx.log.info(`Rendered digest: ${summaryText}`);

  return { html, title, summary: summaryText };
}
