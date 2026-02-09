import type { SecretsConfig } from "./types.ts";
import { loadSecrets } from "./secrets.ts";
import { resolve } from "std/path";

export interface ResolveSecretsOptions {
  explicitPath?: string;
  workflowDir: string;
}

/**
 * Resolve secrets using a cascade strategy:
 *   1. If explicitPath is set, use only that source
 *   2. Otherwise: .secrets/ directory → .secrets.yaml → merge
 *   3. Always merge /app/secrets on top (K8s volume mount)
 */
export async function resolveSecrets(opts: ResolveSecretsOptions): Promise<SecretsConfig> {
  let secrets: SecretsConfig = {};

  if (opts.explicitPath) {
    secrets = await loadSecrets(resolve(opts.explicitPath));
  } else {
    // Try .secrets/ directory first
    const secretsDir = await loadSecrets(resolve(opts.workflowDir, ".secrets"));
    if (Object.keys(secretsDir).length > 0) {
      Object.assign(secrets, secretsDir);
    }

    // Then .secrets.yaml (merges on top)
    const secretsYaml = await loadSecrets(resolve(opts.workflowDir, ".secrets.yaml"));
    if (Object.keys(secretsYaml).length > 0) {
      Object.assign(secrets, secretsYaml);
    }
  }

  // Also check /app/secrets for K8s volume mounts (merges on top)
  try {
    const k8sSecrets = await loadSecrets("/app/secrets");
    Object.assign(secrets, k8sSecrets);
  } catch (err) {
    if (!(err instanceof Deno.errors.NotFound)) {
      throw err;
    }
  }

  return secrets;
}
