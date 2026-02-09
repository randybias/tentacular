import type { SecretsConfig } from "./types.ts";

/**
 * Load secrets from a YAML file (local dev) or a directory of files (K8s volume mount).
 */
export async function loadSecrets(path: string): Promise<SecretsConfig> {
  try {
    const stat = await Deno.stat(path);
    if (stat.isDirectory) {
      return await loadSecretsFromDir(path);
    }
    return await loadSecretsFromFile(path);
  } catch (err) {
    if (err instanceof Deno.errors.NotFound) {
      return {};
    }
    throw err;
  }
}

/** Load secrets from a YAML file (e.g., .secrets.yaml) */
async function loadSecretsFromFile(path: string): Promise<SecretsConfig> {
  const { parse } = await import("std/yaml");
  const content = await Deno.readTextFile(path);
  const parsed = parse(content);
  if (parsed === null || typeof parsed !== "object") return {};
  return parsed as SecretsConfig;
}

/** Load secrets from a directory of files (K8s Secret volume mount) */
async function loadSecretsFromDir(dirPath: string): Promise<SecretsConfig> {
  const secrets: SecretsConfig = {};
  for await (const entry of Deno.readDir(dirPath)) {
    if (!entry.isFile && !entry.isSymlink) continue;
    // Skip hidden files
    if (entry.name.startsWith(".")) continue;
    const content = await Deno.readTextFile(`${dirPath}/${entry.name}`);
    // Try to parse as JSON first, fall back to plain string
    try {
      const parsed = JSON.parse(content);
      if (typeof parsed === "object" && parsed !== null) {
        secrets[entry.name] = parsed;
      } else {
        secrets[entry.name] = { value: content.trim() };
      }
    } catch {
      secrets[entry.name] = { value: content.trim() };
    }
  }
  return secrets;
}
