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

interface WeeklySnapshot {
  id: number;
  collectedAt: string;
  repo: string;
  sepCount: number;
  seps: Sep[];
}

interface StoreResult {
  stored: boolean;
  snapshotId: number;
  weeklyHistory: WeeklySnapshot[];
}

const CREATE_TABLE = `
CREATE TABLE IF NOT EXISTS sep_weekly_snapshots (
  id SERIAL PRIMARY KEY,
  collected_at TIMESTAMPTZ NOT NULL,
  repo TEXT NOT NULL,
  sep_count INT NOT NULL,
  seps_json JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sep_weekly_snapshots_repo_collected
  ON sep_weekly_snapshots (repo, collected_at DESC);
`;

const INSERT_SNAPSHOT = `
INSERT INTO sep_weekly_snapshots (collected_at, repo, sep_count, seps_json)
VALUES ($1, $2, $3, $4)
RETURNING id;
`;

const SELECT_HISTORY = `
SELECT id, collected_at, repo, sep_count, seps_json
FROM sep_weekly_snapshots
WHERE repo = $1
ORDER BY collected_at DESC
LIMIT 8;
`;

/** Store SEP snapshot to Postgres and return weekly history for trend analysis */
export default async function run(ctx: Context, input: unknown): Promise<StoreResult> {
  const snapshot = input as SepSnapshot;

  const pg = ctx.dependency("postgres");
  if (!pg.secret) {
    ctx.log.warn("No postgres credentials, skipping DB operations");
    return { stored: false, snapshotId: 0, weeklyHistory: [] };
  }

  const client = new Client({
    hostname: pg.host,
    port: pg.port,
    database: pg.metadata?.database as string ?? "sep_analytics",
    user: pg.metadata?.user as string ?? "postgres",
    password: pg.secret,
    tls: { enabled: false },
  });

  let snapshotId = 0;
  let weeklyHistory: WeeklySnapshot[] = [];

  try {
    await client.connect();

    await client.queryArray(CREATE_TABLE);

    const insertResult = await client.queryArray(INSERT_SNAPSHOT, [
      snapshot.timestamp,
      snapshot.repo,
      snapshot.count,
      JSON.stringify(snapshot.seps),
    ]);
    snapshotId = Number(insertResult.rows[0]?.[0] ?? 0);
    ctx.log.info(`Stored weekly snapshot as row ${snapshotId}`);

    const historyResult = await client.queryObject<{
      id: number;
      collected_at: Date;
      repo: string;
      sep_count: number;
      seps_json: Sep[];
    }>(SELECT_HISTORY, [snapshot.repo]);

    weeklyHistory = historyResult.rows.map((row) => ({
      id: row.id,
      collectedAt: row.collected_at.toISOString(),
      repo: row.repo,
      sepCount: row.sep_count,
      seps: row.seps_json,
    }));

    ctx.log.info(`Retrieved ${weeklyHistory.length} weeks of history for trend analysis`);
  } finally {
    await client.end();
  }

  return { stored: true, snapshotId, weeklyHistory };
}
