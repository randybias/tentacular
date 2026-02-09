import { assertEquals } from "std/assert";
import { resolveSecrets } from "./cascade.ts";

Deno.test("resolveSecrets: explicit --secrets path takes precedence", async () => {
  const tmpFile = await Deno.makeTempFile({ suffix: ".yaml" });
  await Deno.writeTextFile(
    tmpFile,
    `explicit:
  key: explicit-value
`,
  );

  // Create a workflowDir with .secrets.yaml that should be ignored
  const tmpDir = await Deno.makeTempDir();
  await Deno.writeTextFile(
    `${tmpDir}/.secrets.yaml`,
    `ignored:
  key: should-not-appear
`,
  );

  const secrets = await resolveSecrets({
    explicitPath: tmpFile,
    workflowDir: tmpDir,
  });

  assertEquals(secrets["explicit"]?.["key"], "explicit-value");
  assertEquals(secrets["ignored"], undefined);

  await Deno.remove(tmpFile);
  await Deno.remove(tmpDir, { recursive: true });
});

Deno.test("resolveSecrets: .secrets/ directory loaded as base layer", async () => {
  const tmpDir = await Deno.makeTempDir();
  const secretsDir = `${tmpDir}/.secrets`;
  await Deno.mkdir(secretsDir);
  await Deno.writeTextFile(`${secretsDir}/github`, JSON.stringify({ token: "ghp_from_dir" }));

  const secrets = await resolveSecrets({ workflowDir: tmpDir });

  assertEquals(secrets["github"]?.["token"], "ghp_from_dir");

  await Deno.remove(tmpDir, { recursive: true });
});

Deno.test("resolveSecrets: .secrets.yaml merges on top of .secrets/", async () => {
  const tmpDir = await Deno.makeTempDir();

  // Create .secrets/ directory with a key
  const secretsDir = `${tmpDir}/.secrets`;
  await Deno.mkdir(secretsDir);
  await Deno.writeTextFile(`${secretsDir}/github`, JSON.stringify({ token: "dir-token" }));

  // Create .secrets.yaml with overlapping key
  await Deno.writeTextFile(
    `${tmpDir}/.secrets.yaml`,
    `github:
  token: yaml-token
`,
  );

  const secrets = await resolveSecrets({ workflowDir: tmpDir });

  // YAML should win for overlapping keys
  assertEquals(secrets["github"]?.["token"], "yaml-token");

  await Deno.remove(tmpDir, { recursive: true });
});

Deno.test("resolveSecrets: empty when no sources exist", async () => {
  const tmpDir = await Deno.makeTempDir();

  const secrets = await resolveSecrets({ workflowDir: tmpDir });

  assertEquals(Object.keys(secrets).length, 0);

  await Deno.remove(tmpDir, { recursive: true });
});

Deno.test("resolveSecrets: loads from .secrets.yaml when no .secrets/ dir", async () => {
  const tmpDir = await Deno.makeTempDir();
  await Deno.writeTextFile(
    `${tmpDir}/.secrets.yaml`,
    `stripe:
  api_key: sk_yaml_only
`,
  );

  const secrets = await resolveSecrets({ workflowDir: tmpDir });

  assertEquals(secrets["stripe"]?.["api_key"], "sk_yaml_only");

  await Deno.remove(tmpDir, { recursive: true });
});

Deno.test("resolveSecrets: non-overlapping keys from both sources preserved", async () => {
  const tmpDir = await Deno.makeTempDir();

  // .secrets/ has github
  const secretsDir = `${tmpDir}/.secrets`;
  await Deno.mkdir(secretsDir);
  await Deno.writeTextFile(`${secretsDir}/github`, JSON.stringify({ token: "ghp_dir" }));

  // .secrets.yaml has stripe
  await Deno.writeTextFile(
    `${tmpDir}/.secrets.yaml`,
    `stripe:
  api_key: sk_yaml
`,
  );

  const secrets = await resolveSecrets({ workflowDir: tmpDir });

  assertEquals(secrets["github"]?.["token"], "ghp_dir");
  assertEquals(secrets["stripe"]?.["api_key"], "sk_yaml");

  await Deno.remove(tmpDir, { recursive: true });
});

Deno.test("resolveSecrets: explicit path skips .secrets/ and .secrets.yaml", async () => {
  const tmpDir = await Deno.makeTempDir();

  // Create both .secrets/ and .secrets.yaml
  const secretsDir = `${tmpDir}/.secrets`;
  await Deno.mkdir(secretsDir);
  await Deno.writeTextFile(`${secretsDir}/from-dir`, JSON.stringify({ token: "dir-val" }));
  await Deno.writeTextFile(
    `${tmpDir}/.secrets.yaml`,
    `from_yaml:
  key: yaml-val
`,
  );

  // Explicit path with different content
  const explicitFile = await Deno.makeTempFile({ suffix: ".yaml" });
  await Deno.writeTextFile(
    explicitFile,
    `explicit:
  key: explicit-only
`,
  );

  const secrets = await resolveSecrets({
    explicitPath: explicitFile,
    workflowDir: tmpDir,
  });

  assertEquals(secrets["explicit"]?.["key"], "explicit-only");
  assertEquals(secrets["from-dir"], undefined);
  assertEquals(secrets["from_yaml"], undefined);

  await Deno.remove(explicitFile);
  await Deno.remove(tmpDir, { recursive: true });
});
