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

function compareSeps(prev: Sep, curr: Sep): string[] | null {
  const changes: string[] = [];
  if (prev.state !== curr.state) {
    changes.push(`state: ${prev.state} -> ${curr.state}`);
  }
  if (prev.title !== curr.title) {
    changes.push("title updated");
  }
  if (JSON.stringify(prev.labels) !== JSON.stringify(curr.labels)) {
    changes.push("labels changed");
  }
  if (prev.updatedAt !== curr.updatedAt) {
    changes.push(`last updated: ${curr.updatedAt}`);
  }
  return changes.length > 0 ? changes : null;
}

/** Compare current SEP snapshot with previous from Postgres */
export default async function run(ctx: Context, input: unknown): Promise<SepDelta> {
  const snapshot = input as SepSnapshot;

  const pg = ctx.dependency("postgres");

  let previousSeps: Sep[] | null = null;
  let previousTimestamp: string | null = null;

  if (pg.secret) {
    ctx.log.info(`Connecting to Postgres at ${pg.host}:${pg.port}/${pg.database}`);
    const client = new Client({
      hostname: pg.host,
      port: pg.port,
      database: pg.database,
      user: pg.user,
      password: pg.secret,
      tls: { enabled: false },
    });

    try {
      await client.connect();
      await client.queryArray(`
        CREATE TABLE IF NOT EXISTS sep_snapshots (
          id SERIAL PRIMARY KEY,
          collected_at TIMESTAMPTZ NOT NULL,
          repo TEXT NOT NULL,
          sep_count INT NOT NULL,
          seps_json JSONB NOT NULL
        );
        CREATE INDEX IF NOT EXISTS idx_sep_snapshots_repo_collected
          ON sep_snapshots (repo, collected_at DESC);
      `);
      const result = await client.queryObject<{ seps_json: string; collected_at: string }>(
        "SELECT seps_json, collected_at FROM sep_snapshots WHERE repo=$1 ORDER BY collected_at DESC LIMIT 1",
        [snapshot.repo],
      );
      if (result.rows.length > 0) {
        const row = result.rows[0];
        // @db/postgres auto-parses JSONB columns into JS objects
        previousSeps = typeof row.seps_json === "string"
          ? JSON.parse(row.seps_json)
          : row.seps_json as unknown as Sep[];
        previousTimestamp = row.collected_at;
        ctx.log.info(`Found previous snapshot from ${previousTimestamp}`);
      } else {
        ctx.log.info("No previous snapshot found, this is the first run");
      }
    } finally {
      await client.end();
    }
  } else {
    ctx.log.warn("No postgres credentials, skipping previous snapshot lookup");
  }

  const isFirstRun = previousSeps === null;
  const changes: SepChange[] = [];

  if (previousSeps !== null) {
    const prevMap = new Map(previousSeps.map((s) => [s.sepId, s]));
    const currMap = new Map(snapshot.seps.map((s) => [s.sepId, s]));

    // Added: in current but not in previous
    for (const sep of snapshot.seps) {
      if (!prevMap.has(sep.sepId)) {
        changes.push({ changeType: "added", sep });
      }
    }

    // Removed: in previous but not in current
    for (const sep of previousSeps) {
      if (!currMap.has(sep.sepId)) {
        changes.push({ changeType: "removed", sep });
      }
    }

    // Updated: in both but changed
    for (const sep of snapshot.seps) {
      const prev = prevMap.get(sep.sepId);
      if (prev) {
        const changeDetails = compareSeps(prev, sep);
        if (changeDetails) {
          changes.push({
            changeType: "updated",
            sep,
            previousState: prev.state,
            changes: changeDetails,
          });
        }
      }
    }
  } else {
    // First run: all current SEPs are "added"
    for (const sep of snapshot.seps) {
      changes.push({ changeType: "added", sep });
    }
  }

  const addedCount = changes.filter((c) => c.changeType === "added").length;
  const removedCount = changes.filter((c) => c.changeType === "removed").length;
  const updatedCount = changes.filter((c) => c.changeType === "updated").length;

  ctx.log.info(`Delta: +${addedCount} added, -${removedCount} removed, ~${updatedCount} updated`);

  return {
    timestamp: snapshot.timestamp,
    repo: snapshot.repo,
    previousTimestamp,
    currentTimestamp: snapshot.timestamp,
    changes,
    addedCount,
    removedCount,
    updatedCount,
    totalCount: snapshot.seps.length,
    isFirstRun,
  };
}
