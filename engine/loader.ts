import type { NodeModule, NodeFunction } from "./types.ts";
import { resolve } from "std/path";

/** Cache of loaded node modules, keyed by absolute path */
const moduleCache = new Map<string, NodeFunction>();

/**
 * Load a node module from a file path.
 * Uses dynamic import() with cache-busting for hot-reload support.
 */
export async function loadNode(
  nodePath: string,
  workflowDir: string,
  bustCache = false,
): Promise<NodeFunction> {
  const absPath = resolve(workflowDir, nodePath);
  const normalizedDir = resolve(workflowDir);
  if (!absPath.startsWith(normalizedDir + "/") && absPath !== normalizedDir) {
    throw new Error(
      `Node path "${nodePath}" resolves outside workflow directory`,
    );
  }

  if (!bustCache) {
    const cached = moduleCache.get(absPath);
    if (cached) return cached;
  }

  const importPath = bustCache
    ? `file://${absPath}?t=${Date.now()}`
    : `file://${absPath}`;

  const mod = (await import(importPath)) as NodeModule;

  if (typeof mod.default !== "function") {
    throw new Error(
      `Node at "${nodePath}" must export a default async function. Got: ${typeof mod.default}`,
    );
  }

  moduleCache.set(absPath, mod.default);
  return mod.default;
}

/** Clear the module cache (used during hot-reload) */
export function clearModuleCache(): void {
  moduleCache.clear();
}

/**
 * Load all nodes defined in a workflow spec.
 * Returns a map of nodeId → NodeFunction.
 */
export async function loadAllNodes(
  nodes: Record<string, { path: string }>,
  workflowDir: string,
  bustCache = false,
): Promise<Map<string, NodeFunction>> {
  const loaded = new Map<string, NodeFunction>();

  for (const [nodeId, spec] of Object.entries(nodes)) {
    const fn = await loadNode(spec.path, workflowDir, bustCache);
    loaded.set(nodeId, fn);
  }

  return loaded;
}
