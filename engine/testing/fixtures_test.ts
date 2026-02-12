import { assertEquals, assertExists } from "std/assert";
import { loadFixture } from "./fixtures.ts";
import type { TestFixture } from "./fixtures.ts";

Deno.test("loadFixture: includes config and secrets", async () => {
  // Write a temp fixture JSON with config and secrets fields
  const fixture: TestFixture = {
    input: { key: "value" },
    config: { base_url: "https://api.example.com" },
    secrets: {
      github: { token: "ghp_test123" },
    },
    expected: { result: "ok" },
  };

  const tmpFile = await Deno.makeTempFile({ suffix: ".json" });
  await Deno.writeTextFile(tmpFile, JSON.stringify(fixture));

  const loaded = await loadFixture(tmpFile);

  // Assert all fields are present
  assertExists(loaded.input);
  assertExists(loaded.config);
  assertExists(loaded.secrets);
  assertExists(loaded.expected);

  // Assert config values
  assertEquals(loaded.config!["base_url"], "https://api.example.com");

  // Assert secrets values
  assertEquals(loaded.secrets!["github"]!["token"], "ghp_test123");

  // Assert input
  assertEquals((loaded.input as Record<string, string>)["key"], "value");

  await Deno.remove(tmpFile);
});

Deno.test("loadFixture: works with minimal fixture (input only)", async () => {
  const fixture = {
    input: { items: [1, 2, 3] },
  };

  const tmpFile = await Deno.makeTempFile({ suffix: ".json" });
  await Deno.writeTextFile(tmpFile, JSON.stringify(fixture));

  const loaded = await loadFixture(tmpFile);

  assertExists(loaded.input);
  assertEquals(loaded.config, undefined);
  assertEquals(loaded.secrets, undefined);
  assertEquals(loaded.expected, undefined);

  await Deno.remove(tmpFile);
});

Deno.test("loadFixture: preserves nested secret structure", async () => {
  const fixture: TestFixture = {
    input: {},
    secrets: {
      azure: {
        account_name: "myaccount",
        sas_token: "sv=2023",
      },
      slack: {
        webhook_url: "https://hooks.slack.com/test",
      },
    },
  };

  const tmpFile = await Deno.makeTempFile({ suffix: ".json" });
  await Deno.writeTextFile(tmpFile, JSON.stringify(fixture));

  const loaded = await loadFixture(tmpFile);

  assertEquals(loaded.secrets!["azure"]!["account_name"], "myaccount");
  assertEquals(loaded.secrets!["azure"]!["sas_token"], "sv=2023");
  assertEquals(loaded.secrets!["slack"]!["webhook_url"], "https://hooks.slack.com/test");

  await Deno.remove(tmpFile);
});
