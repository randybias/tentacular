import { assertEquals } from "std/assert";
import { validateOptions } from "./nats.ts";
import type { NATSTriggerOptions } from "./nats.ts";

Deno.test("validateOptions: missing URL returns error", () => {
  const result = validateOptions({
    url: "",
    token: "test-token",
    triggers: [{ type: "queue", subject: "test.subject" }],
  } as Partial<NATSTriggerOptions>);
  assertEquals(result, "NATS URL is required (set config.nats_url in workflow.yaml)");
});

Deno.test("validateOptions: missing token returns error", () => {
  const result = validateOptions({
    url: "nats://localhost:4222",
    token: "",
    triggers: [{ type: "queue", subject: "test.subject" }],
  } as Partial<NATSTriggerOptions>);
  assertEquals(result, "NATS token is required (set secrets.nats.token)");
});

Deno.test("validateOptions: no triggers returns error", () => {
  const result = validateOptions({
    url: "nats://localhost:4222",
    token: "test-token",
    triggers: [],
  } as Partial<NATSTriggerOptions>);
  assertEquals(result, "At least one queue trigger is required");
});

Deno.test("validateOptions: trigger missing subject returns error", () => {
  const result = validateOptions({
    url: "nats://localhost:4222",
    token: "test-token",
    triggers: [{ type: "queue" }],
  } as Partial<NATSTriggerOptions>);
  assertEquals(result, "All queue triggers must have a subject");
});

Deno.test("validateOptions: valid options returns null", () => {
  const result = validateOptions({
    url: "nats://localhost:4222",
    token: "test-token",
    triggers: [{ type: "queue", subject: "events.github.push" }],
  } as Partial<NATSTriggerOptions>);
  assertEquals(result, null);
});

Deno.test("validateOptions: multiple triggers all valid returns null", () => {
  const result = validateOptions({
    url: "nats://localhost:4222",
    token: "test-token",
    triggers: [
      { type: "queue", subject: "events.github.push" },
      { type: "queue", subject: "events.ci.complete", name: "ci-done" },
    ],
  } as Partial<NATSTriggerOptions>);
  assertEquals(result, null);
});

// Integration tests — only run when NATS_TEST=true
const natsTestEnabled = Deno.env.get("NATS_TEST") === "true";

if (natsTestEnabled) {
  Deno.test("integration: connect to live NATS server", async () => {
    const { connect } = await import("@nats-io/transport-deno");
    const token = Deno.env.get("NATS_TOKEN");
    if (!token) {
      console.warn("Skipping NATS integration: NATS_TOKEN not set");
      return;
    }

    const nc = await connect({
      servers: "nats.ospo-dev.miralabs.dev:18453",
      token,
      tls: {},
    });

    // Verify connection
    const info = nc.info;
    assertEquals(typeof info?.server_id, "string");

    await nc.drain();
  });
}
