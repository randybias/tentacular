import { assertEquals } from "std/assert";
import { loadSecrets } from "./secrets.ts";

// --- loadSecrets from YAML file ---

Deno.test("loadSecrets: loads secrets from YAML file", async () => {
  const tmpFile = await Deno.makeTempFile({ suffix: ".yaml" });
  await Deno.writeTextFile(
    tmpFile,
    `github:
  token: ghp_abc123
stripe:
  api_key: sk_test_xyz
`,
  );

  const secrets = await loadSecrets(tmpFile);

  assertEquals(secrets["github"]?.["token"], "ghp_abc123");
  assertEquals(secrets["stripe"]?.["api_key"], "sk_test_xyz");

  await Deno.remove(tmpFile);
});

// --- loadSecrets from directory ---

Deno.test("loadSecrets: loads secrets from directory", async () => {
  const tmpDir = await Deno.makeTempDir();
  await Deno.writeTextFile(`${tmpDir}/github`, JSON.stringify({ token: "ghp_dir_token" }));
  await Deno.writeTextFile(`${tmpDir}/stripe`, JSON.stringify({ api_key: "sk_dir_key" }));

  const secrets = await loadSecrets(tmpDir);

  assertEquals(secrets["github"]?.["token"], "ghp_dir_token");
  assertEquals(secrets["stripe"]?.["api_key"], "sk_dir_key");

  await Deno.remove(tmpDir, { recursive: true });
});

// --- Missing file returns empty ---

Deno.test("loadSecrets: missing file returns empty object", async () => {
  const secrets = await loadSecrets("/tmp/nonexistent-secrets-file-12345.yaml");
  assertEquals(Object.keys(secrets).length, 0);
});

// --- Hidden files skipped ---

Deno.test("loadSecrets: hidden files in directory are skipped", async () => {
  const tmpDir = await Deno.makeTempDir();
  await Deno.writeTextFile(`${tmpDir}/.hidden`, JSON.stringify({ secret: "nope" }));
  await Deno.writeTextFile(`${tmpDir}/visible`, JSON.stringify({ token: "yes" }));

  const secrets = await loadSecrets(tmpDir);

  assertEquals(secrets[".hidden"], undefined);
  assertEquals(secrets["visible"]?.["token"], "yes");

  await Deno.remove(tmpDir, { recursive: true });
});

// --- Invalid YAML returns empty ---

Deno.test("loadSecrets: invalid YAML file returns empty object", async () => {
  const tmpFile = await Deno.makeTempFile({ suffix: ".yaml" });
  await Deno.writeTextFile(tmpFile, ":::not valid yaml:::[[[");

  const secrets = await loadSecrets(tmpFile);
  assertEquals(Object.keys(secrets).length, 0);

  await Deno.remove(tmpFile);
});

// --- Plain text file handling ---

Deno.test("loadSecrets: plain text file in directory falls back to { value: content }", async () => {
  const tmpDir = await Deno.makeTempDir();
  await Deno.writeTextFile(`${tmpDir}/api-token`, "my-plain-token-value");

  const secrets = await loadSecrets(tmpDir);

  assertEquals(secrets["api-token"]?.["value"], "my-plain-token-value");

  await Deno.remove(tmpDir, { recursive: true });
});
