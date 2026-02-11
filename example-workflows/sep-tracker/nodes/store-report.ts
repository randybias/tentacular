import type { Context } from "tentacular";
import { Client } from "jsr:@db/postgres@0.19.5";

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

interface HtmlReport {
  html: string;
  title: string;
  summary: string;
}

interface StoreResult {
  stored: boolean;
  snapshotId: number;
  reportId: number;
  reportUrl: string;
}

const CREATE_TABLES = `
CREATE TABLE IF NOT EXISTS sep_snapshots (
  id SERIAL PRIMARY KEY,
  collected_at TIMESTAMPTZ NOT NULL,
  repo TEXT NOT NULL,
  sep_count INT NOT NULL,
  seps_json JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sep_snapshots_repo_collected
  ON sep_snapshots (repo, collected_at DESC);

CREATE TABLE IF NOT EXISTS sep_reports (
  id SERIAL PRIMARY KEY,
  created_at TIMESTAMPTZ NOT NULL,
  title TEXT NOT NULL,
  summary TEXT NOT NULL,
  html TEXT NOT NULL,
  snapshot_id INT REFERENCES sep_snapshots(id)
);
`;

const INSERT_SNAPSHOT = `
INSERT INTO sep_snapshots (collected_at, repo, sep_count, seps_json)
VALUES ($1, $2, $3, $4)
RETURNING id;
`;

const INSERT_REPORT = `
INSERT INTO sep_reports (created_at, title, summary, html, snapshot_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;
`;

function formatBlobTimestamp(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getUTCFullYear()}${pad(d.getUTCMonth() + 1)}${pad(d.getUTCDate())}-${pad(d.getUTCHours())}${pad(d.getUTCMinutes())}${pad(d.getUTCSeconds())}`;
}

/** Store SEP snapshot and HTML report to Postgres, upload HTML to Azure Blob Storage */
export default async function run(ctx: Context, input: unknown): Promise<StoreResult> {
  // Fan-in: input is keyed by upstream node names
  const merged = input as { "fetch-seps": SepSnapshot; "render-html": HtmlReport };
  const snapshot = merged["fetch-seps"];
  const report = merged["render-html"];

  const pgHost = ctx.config.pg_host as string;
  const pgPort = ctx.config.pg_port as number;
  const pgDatabase = ctx.config.pg_database as string;
  const pgUser = ctx.config.pg_user as string;
  const pgPassword = ctx.secrets?.postgres?.password;

  if (!pgPassword) {
    ctx.log.error("No postgres.password in secrets");
    throw new Error("Missing postgres.password secret");
  }

  ctx.log.info(`Connecting to Postgres at ${pgHost}:${pgPort}/${pgDatabase}`);

  const client = new Client({
    hostname: pgHost,
    port: pgPort,
    database: pgDatabase,
    user: pgUser,
    password: pgPassword,
    tls: { enabled: false },
  });

  let snapshotId = 0;
  let reportId = 0;

  try {
    await client.connect();

    // Ensure tables exist
    await client.queryArray(CREATE_TABLES);

    // Insert snapshot
    const snapResult = await client.queryArray(INSERT_SNAPSHOT, [
      snapshot.timestamp,
      snapshot.repo,
      snapshot.count,
      JSON.stringify(snapshot.seps),
    ]);
    snapshotId = Number(snapResult.rows[0]?.[0] ?? 0);
    ctx.log.info(`Stored snapshot as row ${snapshotId}`);

    // Insert report
    const reportResult = await client.queryArray(INSERT_REPORT, [
      snapshot.timestamp,
      report.title,
      report.summary,
      report.html,
      snapshotId,
    ]);
    reportId = Number(reportResult.rows[0]?.[0] ?? 0);
    ctx.log.info(`Stored report as row ${reportId}`);
  } finally {
    await client.end();
  }

  // Upload HTML to Azure Blob Storage
  let reportUrl = "";
  const sasToken = ctx.secrets?.azure?.sas_token;
  const blobBaseUrl = ctx.config.azure_blob_base_url as string;

  if (sasToken && blobBaseUrl) {
    const blobName = `sep-report-${formatBlobTimestamp(snapshot.timestamp)}.html`;
    const uploadUrl = `${blobBaseUrl}/${blobName}?${sasToken}`;
    const publicUrl = `${blobBaseUrl}/${blobName}`;

    ctx.log.info(`Uploading report to Azure Blob Storage: ${blobName}`);

    try {
      const uploadRes = await ctx.fetch("azure-blob", uploadUrl, {
        method: "PUT",
        headers: {
          "Content-Type": "text/html; charset=utf-8",
          "x-ms-blob-type": "BlockBlob",
        },
        body: report.html,
      });

      if (uploadRes.ok) {
        reportUrl = publicUrl;
        ctx.log.info(`Uploaded report to ${reportUrl}`);
      } else {
        const body = await uploadRes.text();
        ctx.log.warn(`Azure upload failed: ${uploadRes.status} - ${body}`);
      }
    } catch (err) {
      ctx.log.warn(`Azure upload error: ${err}`);
    }
  } else {
    ctx.log.warn("No azure.sas_token in secrets or azure_blob_base_url in config, skipping blob upload");
  }

  return { stored: true, snapshotId, reportId, reportUrl };
}
