import type { Context } from "tentacular";
import { Client } from "jsr:@db/postgres@0.19.5";

export interface HealthRecord {
  id: number;
  collectedAt: string;
  totalNodes: number;
  readyNodes: number;
  totalPods: number;
  healthyPods: number;
  problemPods: number;
  nodesJson: unknown;
  problemPodsJson: unknown;
  namespacesJson: unknown;
}

export interface HealthHistory {
  records: HealthRecord[];
  periodStart: string;
  periodEnd: string;
  snapshotCount: number;
}

const QUERY = `
SELECT id, collected_at, total_nodes, ready_nodes, total_pods, healthy_pods,
       problem_pods, nodes_json, problem_pods_json, namespaces_json
FROM cluster_health_snapshots
WHERE collected_at >= NOW() - INTERVAL '24 hours'
ORDER BY collected_at ASC;
`;

/** Query the last 24 hours of cluster health snapshots from Postgres */
export default async function run(ctx: Context, _input: unknown): Promise<HealthHistory> {
  const postgres = ctx.dependency("postgres");
  if (!postgres.secret) {
    ctx.log.warn("No postgres.password in secrets -- skipping (no credentials)");
    return { records: [], periodStart: "", periodEnd: "", snapshotCount: 0 };
  }

  ctx.log.info(`Querying health history from ${postgres.host}:${postgres.port}/${postgres.database}`);

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
    const result = await client.queryObject<{
      id: number;
      collected_at: Date;
      total_nodes: number;
      ready_nodes: number;
      total_pods: number;
      healthy_pods: number;
      problem_pods: number;
      nodes_json: unknown;
      problem_pods_json: unknown;
      namespaces_json: unknown;
    }>(QUERY);

    const records: HealthRecord[] = result.rows.map((r) => ({
      id: r.id,
      collectedAt: r.collected_at.toISOString(),
      totalNodes: r.total_nodes,
      readyNodes: r.ready_nodes,
      totalPods: r.total_pods,
      healthyPods: r.healthy_pods,
      problemPods: r.problem_pods,
      nodesJson: r.nodes_json,
      problemPodsJson: r.problem_pods_json,
      namespacesJson: r.namespaces_json,
    }));

    const now = new Date().toISOString();
    const periodStart = records.length > 0
      ? records[0].collectedAt
      : new Date(Date.now() - 86400000).toISOString();
    const periodEnd = records.length > 0
      ? records[records.length - 1].collectedAt
      : now;

    ctx.log.info(`Retrieved ${records.length} snapshots from last 24h`);

    return {
      records,
      periodStart,
      periodEnd,
      snapshotCount: records.length,
    };
  } finally {
    await client.end();
  }
}
