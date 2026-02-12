import { resolve } from "std/path";

export interface TestFixture {
  input: unknown;
  config?: Record<string, unknown>;
  secrets?: Record<string, Record<string, string>>;
  expected?: unknown;
}

/** Load a test fixture from a JSON file */
export async function loadFixture(fixturePath: string): Promise<TestFixture> {
  const content = await Deno.readTextFile(fixturePath);
  return JSON.parse(content) as TestFixture;
}

/** Find fixture files for a given node */
export async function findFixtures(
  testDir: string,
  nodeName: string,
): Promise<string[]> {
  const fixturesDir = resolve(testDir, "fixtures");
  const fixtures: string[] = [];

  try {
    for await (const entry of Deno.readDir(fixturesDir)) {
      if (entry.isFile && entry.name.startsWith(nodeName) && entry.name.endsWith(".json")) {
        fixtures.push(resolve(fixturesDir, entry.name));
      }
    }
  } catch (err) {
    if (!(err instanceof Deno.errors.NotFound)) {
      throw err;
    }
  }

  return fixtures;
}
