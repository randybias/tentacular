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

interface SepSnapshot {
  timestamp: string;
  repo: string;
  seps: Sep[];
  count: number;
}

interface SepChange {
  changeType: "added" | "removed" | "updated";
  sep: Sep;
  previousState?: string;
  changes?: string[];
}

interface SepDelta {
  timestamp: string;
  repo: string;
  previousTimestamp: string | null;
  currentTimestamp: string;
  changes: SepChange[];
  addedCount: number;
  removedCount: number;
  updatedCount: number;
  totalCount: number;
  isFirstRun: boolean;
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

function statusClass(state: string): string {
  switch (state.toLowerCase()) {
    case "open": return "status-open";
    case "draft": return "status-draft";
    case "closed": case "merged": return "status-closed";
    default: return "status-open";
  }
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" });
}

function renderChangeBadge(changeType: string): string {
  const labels: Record<string, string> = { added: "NEW", removed: "REMOVED", updated: "UPDATED" };
  return `<span class="change-badge change-${changeType}">${labels[changeType] ?? changeType.toUpperCase()}</span>`;
}

function renderSepCard(sep: Sep, change?: SepChange): string {
  const highlightClass = change ? ` highlight-${change.changeType}` : "";
  const badge = change ? renderChangeBadge(change.changeType) : "";
  const changeDetails = change?.changes?.length
    ? `<div class="change-details">${escapeHtml(change.changes.join(" | "))}</div>`
    : "";

  const description = sep.summary && sep.summary.length > 0
    ? `<div class="sep-description">${escapeHtml(sep.summary)}</div>`
    : "";

  return `<div class="sep-card${highlightClass}">
  <div class="sep-header">
    <span class="sep-id"><a href="${escapeHtml(sep.url)}">${escapeHtml(sep.sepId)}</a> #${sep.number}${badge}</span>
    <span class="status-badge ${statusClass(sep.state)}">${escapeHtml(sep.state.toUpperCase())}</span>
  </div>
  <div class="sep-title">${escapeHtml(sep.title)}</div>
  <div class="sep-meta">by ${escapeHtml(sep.author)} | created ${formatDate(sep.createdAt)} | updated ${formatDate(sep.updatedAt)}</div>
  ${description}
  ${changeDetails}
</div>`;
}

/** Generate HTML report from SEP delta and full snapshot */
export default async function run(ctx: Context, input: unknown): Promise<HtmlReport> {
  // Fan-in: receives both diff-seps delta and fetch-seps snapshot
  const merged = input as { "diff-seps": SepDelta; "fetch-seps": SepSnapshot };
  const delta = merged["diff-seps"];
  const snapshot = merged["fetch-seps"];

  const changeMap = new Map(delta.changes.map((c) => [c.sep.sepId, c]));

  const summaryText = delta.isFirstRun
    ? `Initial snapshot: ${delta.totalCount} SEPs tracked`
    : delta.addedCount + delta.removedCount + delta.updatedCount === 0
      ? `No changes across ${delta.totalCount} SEPs`
      : `+${delta.addedCount} added, -${delta.removedCount} removed, ~${delta.updatedCount} updated`;

  // Count SEPs by state
  const stateCounts: Record<string, number> = {};
  for (const sep of snapshot.seps) {
    const st = sep.state.toLowerCase();
    stateCounts[st] = (stateCounts[st] ?? 0) + 1;
  }

  // Render change cards (if any non-first-run changes)
  let changesSection = "";
  if (!delta.isFirstRun && delta.changes.length > 0) {
    const changeCards = delta.changes.map((c) => renderSepCard(c.sep, c)).join("\n");
    changesSection = `<div class="section">
  <h2>Changes Since Last Report</h2>
  <p class="section-meta">Comparing with snapshot from ${delta.previousTimestamp ? formatDate(delta.previousTimestamp) : "N/A"}</p>
  ${changeCards}
</div>`;
  } else if (!delta.isFirstRun) {
    changesSection = `<div class="section">
  <h2>Changes Since Last Report</h2>
  <p class="no-changes">No changes detected since last report.</p>
</div>`;
  }

  // Render all current SEP cards (sorted by SEP ID)
  const sortedSeps = [...snapshot.seps].sort((a, b) => a.sepId.localeCompare(b.sepId));
  const sepCards = sortedSeps
    .map((sep) => renderSepCard(sep, changeMap.get(sep.sepId)))
    .join("\n");

  // State breakdown for metrics bar
  const stateEntries = Object.entries(stateCounts).sort(([, a], [, b]) => b - a);
  const stateMetrics = stateEntries.map(([st, count]) =>
    `<div class="metric"><div class="value">${count}</div><div class="label">${escapeHtml(st)}</div></div>`
  ).join("\n");

  const html = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>MCP SEP Status Report - ${escapeHtml(delta.timestamp)}</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background: #f6f8fa; color: #24292e; }
    .container { max-width: 960px; margin: 0 auto; }
    .header { background: linear-gradient(135deg, #667eea 0%, #764ba2 100%); color: white; padding: 32px; border-radius: 12px 12px 0 0; }
    .header h1 { font-size: 26px; font-weight: 700; margin-bottom: 8px; }
    .header .subtitle { opacity: 0.9; font-size: 14px; }
    .metrics-bar { display: flex; gap: 0; background: white; border-bottom: 1px solid #e1e4e8; }
    .metrics-bar .metric { flex: 1; text-align: center; padding: 20px 12px; border-right: 1px solid #e1e4e8; }
    .metrics-bar .metric:last-child { border-right: none; }
    .metric .value { font-size: 28px; font-weight: 700; color: #0366d6; }
    .metric .label { font-size: 11px; color: #6a737d; text-transform: uppercase; letter-spacing: 0.5px; margin-top: 4px; }
    .summary-bar { display: flex; gap: 0; background: white; border-bottom: 1px solid #e1e4e8; }
    .summary-bar .metric { flex: 1; text-align: center; padding: 16px 12px; border-right: 1px solid #e1e4e8; }
    .summary-bar .metric:last-child { border-right: none; }
    .summary-bar .added .value { color: #22863a; }
    .summary-bar .removed .value { color: #cb2431; }
    .summary-bar .updated .value { color: #e36209; }
    .summary-bar .total .value { color: #0366d6; }
    .section { background: white; padding: 24px 32px; border-bottom: 1px solid #e1e4e8; }
    .section h2 { font-size: 18px; font-weight: 600; margin-bottom: 12px; }
    .section-meta { font-size: 13px; color: #6a737d; margin-bottom: 16px; }
    .sep-card { border: 1px solid #e1e4e8; border-radius: 8px; padding: 16px; margin-bottom: 12px; transition: box-shadow 0.15s; }
    .sep-card:hover { box-shadow: 0 1px 6px rgba(0,0,0,0.1); }
    .sep-card.highlight-added { border-left: 4px solid #22863a; background: #f6fff8; }
    .sep-card.highlight-removed { border-left: 4px solid #cb2431; background: #fff8f8; }
    .sep-card.highlight-updated { border-left: 4px solid #e36209; background: #fffcf5; }
    .sep-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; flex-wrap: wrap; gap: 8px; }
    .sep-id { font-weight: 700; font-size: 15px; }
    .sep-id a { color: #0366d6; text-decoration: none; }
    .sep-id a:hover { text-decoration: underline; }
    .status-badge { display: inline-block; padding: 2px 10px; border-radius: 12px; font-size: 11px; font-weight: 600; text-transform: uppercase; letter-spacing: 0.3px; }
    .status-open { background: #dcffe4; color: #22863a; }
    .status-draft { background: #fff3cd; color: #856d0a; }
    .status-closed { background: #ffdce0; color: #cb2431; }
    .sep-title { font-size: 14px; color: #24292e; margin-bottom: 4px; }
    .sep-meta { font-size: 12px; color: #6a737d; }
    .sep-description { font-size: 13px; color: #586069; margin-top: 8px; line-height: 1.5; border-top: 1px solid #f0f0f0; padding-top: 8px; }
    .change-badge { display: inline-block; padding: 1px 6px; border-radius: 3px; font-size: 10px; font-weight: 700; margin-left: 8px; vertical-align: middle; }
    .change-added { background: #dcffe4; color: #22863a; }
    .change-removed { background: #ffdce0; color: #cb2431; }
    .change-updated { background: #fff3cd; color: #856d0a; }
    .change-details { font-size: 12px; color: #e36209; margin-top: 6px; font-style: italic; }
    .no-changes { color: #6a737d; font-style: italic; padding: 16px 0; }
    .footer { padding: 20px 32px; background: white; border-radius: 0 0 12px 12px; text-align: center; font-size: 12px; color: #6a737d; }
    @media (max-width: 640px) {
      body { padding: 0; }
      .header { border-radius: 0; }
      .footer { border-radius: 0; }
      .metrics-bar, .summary-bar { flex-wrap: wrap; }
      .metrics-bar .metric, .summary-bar .metric { flex-basis: 50%; border-bottom: 1px solid #e1e4e8; }
      .section { padding: 16px; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <h1>MCP SEP Status Report</h1>
      <div class="subtitle">${escapeHtml(snapshot.repo)} | ${formatDate(delta.timestamp)}</div>
    </div>
    <div class="summary-bar">
      <div class="metric total"><div class="value">${delta.totalCount}</div><div class="label">Total SEPs</div></div>
      <div class="metric added"><div class="value">+${delta.addedCount}</div><div class="label">Added</div></div>
      <div class="metric removed"><div class="value">-${delta.removedCount}</div><div class="label">Removed</div></div>
      <div class="metric updated"><div class="value">~${delta.updatedCount}</div><div class="label">Updated</div></div>
    </div>
    <div class="metrics-bar">
      ${stateMetrics}
    </div>
    ${changesSection}
    <div class="section">
      <h2>All Current SEPs (${snapshot.seps.length})</h2>
      <p class="section-meta">${summaryText}</p>
      ${sepCards}
    </div>
    <div class="footer">
      Generated by Tentacular SEP Tracker | ${escapeHtml(delta.timestamp)}
    </div>
  </div>
</body>
</html>`;

  ctx.log.info(`Rendered HTML report: ${summaryText} (${snapshot.seps.length} SEPs)`);

  return {
    html,
    title: `MCP SEP Status Report - ${delta.timestamp}`,
    summary: summaryText,
  };
}
