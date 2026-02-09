import type { CompiledDAG, Edge, Stage, WorkflowSpec } from "../types.ts";

/**
 * Compile a workflow spec into an executable DAG with topologically sorted stages.
 * Nodes in the same stage have no dependencies on each other and can run in parallel.
 */
export function compile(spec: WorkflowSpec): CompiledDAG {
  validateSpec(spec);
  validateEdges(spec);
  const sorted = topologicalSort(spec);
  const stages = buildStages(spec, sorted);

  return {
    workflow: spec,
    stages,
    nodeOrder: sorted,
  };
}

/** Validate required fields are present in the workflow spec */
function validateSpec(spec: WorkflowSpec): void {
  if (!spec || typeof spec !== "object") {
    throw new Error("Invalid workflow spec: expected an object");
  }
  if (!spec.name || typeof spec.name !== "string") {
    throw new Error('Workflow spec missing required field: "name"');
  }
  if (!spec.nodes || typeof spec.nodes !== "object" || Object.keys(spec.nodes).length === 0) {
    throw new Error('Workflow spec missing required field: "nodes" (must define at least one node)');
  }
  if (!Array.isArray(spec.edges)) {
    throw new Error('Workflow spec missing required field: "edges" (must be an array)');
  }
}

/** Validate all edge references point to defined nodes */
function validateEdges(spec: WorkflowSpec): void {
  for (const edge of spec.edges) {
    if (!(edge.from in spec.nodes)) {
      throw new Error(`Edge references undefined node: "${edge.from}"`);
    }
    if (!(edge.to in spec.nodes)) {
      throw new Error(`Edge references undefined node: "${edge.to}"`);
    }
    if (edge.from === edge.to) {
      throw new Error(`Self-loop on node: "${edge.from}"`);
    }
  }
}

/**
 * Kahn's algorithm for topological sort.
 * Detects cycles and returns nodes in dependency order.
 */
function topologicalSort(spec: WorkflowSpec): string[] {
  const nodeNames = Object.keys(spec.nodes);
  const inDegree = new Map<string, number>();
  const adj = new Map<string, string[]>();

  for (const name of nodeNames) {
    inDegree.set(name, 0);
    adj.set(name, []);
  }

  for (const edge of spec.edges) {
    adj.get(edge.from)!.push(edge.to);
    inDegree.set(edge.to, (inDegree.get(edge.to) ?? 0) + 1);
  }

  const queue: string[] = [];
  for (const [name, degree] of inDegree) {
    if (degree === 0) queue.push(name);
  }

  // Sort queue for deterministic output
  queue.sort();

  const sorted: string[] = [];
  while (queue.length > 0) {
    const node = queue.shift()!;
    sorted.push(node);

    for (const neighbor of adj.get(node) ?? []) {
      const newDegree = (inDegree.get(neighbor) ?? 1) - 1;
      inDegree.set(neighbor, newDegree);
      if (newDegree === 0) {
        queue.push(neighbor);
        queue.sort();
      }
    }
  }

  if (sorted.length !== nodeNames.length) {
    throw new Error("Cycle detected in workflow DAG");
  }

  return sorted;
}

/**
 * Group topologically sorted nodes into stages.
 * Nodes in the same stage have all their dependencies in earlier stages.
 */
function buildStages(spec: WorkflowSpec, sorted: string[]): Stage[] {
  const deps = buildDependencyMap(spec.edges);
  const nodeToStage = new Map<string, number>();
  const stages: Stage[] = [];

  for (const node of sorted) {
    const nodeDeps = deps.get(node) ?? [];
    let stageIdx = 0;

    for (const dep of nodeDeps) {
      const depStage = nodeToStage.get(dep) ?? 0;
      stageIdx = Math.max(stageIdx, depStage + 1);
    }

    nodeToStage.set(node, stageIdx);

    while (stages.length <= stageIdx) {
      stages.push({ nodes: [] });
    }
    stages[stageIdx]!.nodes.push(node);
  }

  // If no edges, put all nodes in one stage
  if (stages.length === 0 && sorted.length > 0) {
    stages.push({ nodes: [...sorted] });
  }

  return stages;
}

function buildDependencyMap(edges: Edge[]): Map<string, string[]> {
  const deps = new Map<string, string[]>();
  for (const edge of edges) {
    if (!deps.has(edge.to)) deps.set(edge.to, []);
    deps.get(edge.to)!.push(edge.from);
  }
  return deps;
}
