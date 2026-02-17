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

interface SepDelta {
  addedCount: number;
  removedCount: number;
  updatedCount: number;
}

interface StoreResult {
  stored: boolean;
  snapshotId: number;
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
`;

const INSERT_SNAPSHOT = `
INSERT INTO sep_snapshots (collected_at, repo, sep_count, seps_json)
VALUES ($1, $2, $3, $4)
RETURNING id;
`;

function formatBlobTimestamp(iso: string): string {
  const d = new Date(iso);
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${d.getUTCFullYear()}${pad(d.getUTCMonth() + 1)}${pad(d.getUTCDate())}-${pad(d.getUTCHours())}${pad(d.getUTCMinutes())}${pad(d.getUTCSeconds())}`;
}

/** Store SEP snapshot to Postgres, upload HTML report to Azure Blob Storage */
export default async function run(ctx: Context, input: unknown): Promise<StoreResult> {
  // Fan-in: input is keyed by upstream node names
  const merged = input as {
    "fetch-seps": SepSnapshot;
    "render-html": HtmlReport;
    "diff-seps"?: SepDelta;
  };
  const snapshot = merged["fetch-seps"];
  const report = merged["render-html"];
  const delta = merged["diff-seps"];

  const postgres = ctx.dependency("postgres");
  if (!postgres.secret) {
    ctx.log.warn("No postgres.password in secrets -- skipping (no credentials)");
    return { stored: false, snapshotId: 0, reportUrl: "" };
  }

  ctx.log.info(`Connecting to Postgres at ${postgres.host}:${postgres.port}/${postgres.database}`);

  const client = new Client({
    hostname: postgres.host,
    port: postgres.port,
    database: postgres.database,
    user: postgres.user,
    password: postgres.secret,
    tls: { enabled: false },
  });

  let snapshotId = 0;

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
  } finally {
    await client.end();
  }

  // Upload HTML to Azure Blob Storage
  let reportUrl = "";
  const azureBlob = ctx.dependency("azure-blob");
  const blobBaseUrl = ctx.config.azure_blob_base_url as string;
  const totalChanges = delta
    ? delta.addedCount + delta.removedCount + delta.updatedCount
    : undefined;

  if (totalChanges === 0) {
    ctx.log.info("No SEP changes detected, skipping Azure report upload");
    return { stored: true, snapshotId, reportUrl };
  }

  if (azureBlob.secret && blobBaseUrl) {
    const blobName = `sep-report-${formatBlobTimestamp(snapshot.timestamp)}.html`;
    const uploadPath = `/${blobName}`;
    const publicUrl = `${blobBaseUrl}/${blobName}`;

    ctx.log.info(`Uploading report to Azure Blob Storage: ${blobName}`);

    try {
      const uploadRes = await azureBlob.fetch!(uploadPath, {
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

  return { stored: true, snapshotId, reportUrl };
}
