import type { Context } from "tentacular";
import { Client } from "jsr:@db/postgres@0.19.5";

interface ClusterHealthSnapshot {
  collectedAt: string;
  nodes: unknown[];
  problemPods: unknown[];
  namespaces: unknown[];
  summary: {
    totalNodes: number;
    readyNodes: number;
    totalPods: number;
    healthyPods: number;
    problemPods: number;
  };
}

const CREATE_TABLE = `
CREATE TABLE IF NOT EXISTS cluster_health_snapshots (
  id SERIAL PRIMARY KEY,
  collected_at TIMESTAMPTZ NOT NULL,
  total_nodes INT NOT NULL,
  ready_nodes INT NOT NULL,
  total_pods INT NOT NULL,
  healthy_pods INT NOT NULL,
  problem_pods INT NOT NULL,
  nodes_json JSONB NOT NULL,
  problem_pods_json JSONB NOT NULL,
  namespaces_json JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_health_collected_at
  ON cluster_health_snapshots (collected_at DESC);
`;

const INSERT = `
INSERT INTO cluster_health_snapshots (
  collected_at, total_nodes, ready_nodes, total_pods, healthy_pods,
  problem_pods, nodes_json, problem_pods_json, namespaces_json
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id;
`;

/** Store cluster health snapshot in Postgres */
export default async function run(ctx: Context, input: unknown): Promise<{ stored: boolean; rowId: number }> {
  const snapshot = input as ClusterHealthSnapshot;

  const postgres = ctx.dependency("postgres");
  if (!postgres.secret) {
    ctx.log.warn("No postgres.password in secrets -- skipping (no credentials)");
    return { stored: false, rowId: 0 };
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

  try {
    await client.connect();

    // Ensure table exists
    await client.queryArray(CREATE_TABLE);

    // Insert snapshot
    const result = await client.queryArray(INSERT, [
      snapshot.collectedAt,
      snapshot.summary.totalNodes,
      snapshot.summary.readyNodes,
      snapshot.summary.totalPods,
      snapshot.summary.healthyPods,
      snapshot.summary.problemPods,
      JSON.stringify(snapshot.nodes),
      JSON.stringify(snapshot.problemPods),
      JSON.stringify(snapshot.namespaces),
    ]);

    const rowId = Number(result.rows[0]?.[0] ?? 0);
    ctx.log.info(`Stored snapshot as row ${rowId}`);
    return { stored: true, rowId };
  } finally {
    await client.end();
  }
}
