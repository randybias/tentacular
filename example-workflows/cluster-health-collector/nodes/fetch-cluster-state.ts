import type { Context } from "tentacular";

interface NodeStatus {
  name: string;
  ready: boolean;
  roles: string[];
  kubeletVersion: string;
  conditions: { type: string; status: string; message: string }[];
  capacity: { cpu: string; memory: string; pods: string };
  allocatable: { cpu: string; memory: string; pods: string };
}

interface PodSummary {
  namespace: string;
  name: string;
  phase: string;
  ready: boolean;
  restarts: number;
  nodeName: string;
}

interface NamespaceSummary {
  name: string;
  podCount: number;
  runningCount: number;
  failedCount: number;
  pendingCount: number;
}

export interface ClusterHealthSnapshot {
  collectedAt: string;
  nodes: NodeStatus[];
  problemPods: PodSummary[];
  namespaces: NamespaceSummary[];
  summary: {
    totalNodes: number;
    readyNodes: number;
    totalPods: number;
    healthyPods: number;
    problemPods: number;
  };
}

/** Read the in-cluster service account token and CA for K8s API access */
async function getInClusterAuth(): Promise<{
  token: string;
  apiServer: string;
  caCert: string;
} | null> {
  try {
    const token = (
      await Deno.readTextFile(
        "/var/run/secrets/kubernetes.io/serviceaccount/token",
      )
    ).trim();
    const caCert = await Deno.readTextFile(
      "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
    );
    return { token, apiServer: "https://kubernetes.default.svc", caCert };
  } catch {
    return null;
  }
}

/** Fetch a K8s API endpoint using the in-cluster CA for TLS verification */
async function k8sGet(
  ctx: Context,
  apiServer: string,
  token: string,
  caCert: string,
  path: string,
): Promise<unknown> {
  const client = Deno.createHttpClient({ caCerts: [caCert] });
  try {
    const resp = await fetch(`${apiServer}${path}`, {
      headers: { Authorization: `Bearer ${token}` },
      client,
    });
    if (!resp.ok) {
      const body = await resp.text();
      throw new Error(
        `K8s API ${path} returned ${resp.status}: ${body.substring(0, 200)}`,
      );
    }
    return resp.json();
  } finally {
    client.close();
  }
}

/** Collect cluster health data from the Kubernetes API */
export default async function run(
  ctx: Context,
  _input: unknown,
): Promise<ClusterHealthSnapshot> {
  ctx.log.info("Fetching cluster health state");

  const auth = await getInClusterAuth();
  if (!auth) {
    ctx.log.error("Not running in-cluster — no service account token found");
    throw new Error(
      "In-cluster service account token not available at /var/run/secrets/kubernetes.io/serviceaccount/token",
    );
  }

  const { token, apiServer, caCert } = auth;

  // Fetch nodes
  ctx.log.info("Fetching node status");
  const nodesResp = (await k8sGet(
    ctx,
    apiServer,
    token,
    caCert,
    "/api/v1/nodes",
  )) as {
    items: Record<string, unknown>[];
  };

  const nodes: NodeStatus[] = nodesResp.items.map((node) => {
    const meta = node.metadata as Record<string, unknown>;
    const status = node.status as Record<string, unknown>;
    const spec = node.spec as Record<string, unknown> | undefined;
    const labels = (meta.labels ?? {}) as Record<string, string>;
    const conditions = (status.conditions ?? []) as Record<string, string>[];
    const capacity = (status.capacity ?? {}) as Record<string, string>;
    const allocatable = (status.allocatable ?? {}) as Record<string, string>;
    const nodeInfo = (status.nodeInfo ?? {}) as Record<string, string>;

    const roles = Object.keys(labels)
      .filter((k) => k.startsWith("node-role.kubernetes.io/"))
      .map((k) => k.replace("node-role.kubernetes.io/", ""));

    const readyCondition = conditions.find((c) => c.type === "Ready");

    return {
      name: meta.name as string,
      ready: readyCondition?.status === "True",
      roles,
      kubeletVersion: nodeInfo.kubeletVersion ?? "unknown",
      conditions: conditions.map((c) => ({
        type: c.type,
        status: c.status,
        message: c.message ?? "",
      })),
      capacity: {
        cpu: capacity.cpu ?? "0",
        memory: capacity.memory ?? "0",
        pods: capacity.pods ?? "0",
      },
      allocatable: {
        cpu: allocatable.cpu ?? "0",
        memory: allocatable.memory ?? "0",
        pods: allocatable.pods ?? "0",
      },
    };
  });

  // Fetch all pods
  ctx.log.info("Fetching pod status");
  const podsResp = (await k8sGet(
    ctx,
    apiServer,
    token,
    caCert,
    "/api/v1/pods",
  )) as {
    items: Record<string, unknown>[];
  };

  const allPods: PodSummary[] = podsResp.items.map((pod) => {
    const meta = pod.metadata as Record<string, unknown>;
    const status = pod.status as Record<string, unknown>;
    const spec = pod.spec as Record<string, unknown>;
    const containerStatuses = (status.containerStatuses ?? []) as Record<
      string,
      unknown
    >[];

    const totalRestarts = containerStatuses.reduce(
      (sum, cs) => sum + (Number(cs.restartCount) || 0),
      0,
    );

    const allReady =
      containerStatuses.length > 0 &&
      containerStatuses.every((cs) => cs.ready === true);

    return {
      namespace: meta.namespace as string,
      name: meta.name as string,
      phase: (status.phase as string) ?? "Unknown",
      ready: allReady,
      restarts: totalRestarts,
      nodeName: (spec.nodeName as string) ?? "",
    };
  });

  // Only include problem pods (not Running/Succeeded, or high restarts)
  const problemPods = allPods.filter(
    (p) =>
      (p.phase !== "Running" && p.phase !== "Succeeded") ||
      (!p.ready && p.phase === "Running") ||
      p.restarts > 10,
  );

  // Namespace summaries
  const nsByName = new Map<string, PodSummary[]>();
  for (const pod of allPods) {
    if (!nsByName.has(pod.namespace)) nsByName.set(pod.namespace, []);
    nsByName.get(pod.namespace)!.push(pod);
  }

  const namespaces: NamespaceSummary[] = Array.from(nsByName.entries()).map(
    ([name, pods]) => ({
      name,
      podCount: pods.length,
      runningCount: pods.filter((p) => p.phase === "Running").length,
      failedCount: pods.filter((p) => p.phase === "Failed").length,
      pendingCount: pods.filter((p) => p.phase === "Pending").length,
    }),
  );

  const healthyPods = allPods.filter(
    (p) => (p.phase === "Running" && p.ready) || p.phase === "Succeeded",
  ).length;

  const snapshot: ClusterHealthSnapshot = {
    collectedAt: new Date().toISOString(),
    nodes,
    problemPods,
    namespaces,
    summary: {
      totalNodes: nodes.length,
      readyNodes: nodes.filter((n) => n.ready).length,
      totalPods: allPods.length,
      healthyPods,
      problemPods: problemPods.length,
    },
  };

  ctx.log.info(
    `Cluster state: ${snapshot.summary.readyNodes}/${snapshot.summary.totalNodes} nodes ready, ` +
      `${snapshot.summary.healthyPods}/${snapshot.summary.totalPods} pods healthy, ` +
      `${snapshot.summary.problemPods} problem pods`,
  );

  return snapshot;
}
