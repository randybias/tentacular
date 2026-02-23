/**
 * Tentacular Engine — Main Entrypoint
 *
 * This is the self-contained engine that runs in both environments:
 *   - Local dev: spawned by `tentacular dev` via `deno run engine/main.ts`
 *   - Production: container ENTRYPOINT
 *
 * Usage:
 *   deno run --allow-net --allow-read --allow-write=/tmp engine/main.ts \
 *     --workflow ./workflow.yaml --port 8080 [--watch]
 */

import { parse as parseFlags } from "std/flags";
import { parse as parseYaml } from "std/yaml";
import { dirname, resolve } from "std/path";
import type { Context, WorkflowSpec } from "./types.ts";
import { compile } from "./compiler/mod.ts";
import { createContext } from "./context/mod.ts";
import { resolveSecrets } from "./context/cascade.ts";
import { loadAllNodes, clearModuleCache } from "./loader.ts";
import { startServer } from "./server.ts";
import { watchFiles } from "./watcher.ts";
import type { NodeRunner } from "./executor/types.ts";

const flags = parseFlags(Deno.args, {
  string: ["workflow", "port", "secrets"],
  boolean: ["watch"],
  default: {
    port: "8080",
    watch: false,
  },
});

const workflowPath: string = flags.workflow ?? "";
if (!workflowPath) {
  console.error("Usage: deno run engine/main.ts --workflow <path> [--port <port>] [--watch]");
  Deno.exit(1);
}

const port = parseInt(flags.port, 10);
const workflowDir = dirname(resolve(workflowPath));

// Load workflow spec
async function loadWorkflow(): Promise<WorkflowSpec> {
  const content = await Deno.readTextFile(resolve(workflowPath));
  const spec = parseYaml(content) as WorkflowSpec;
  return spec;
}

// Main startup
async function main() {
  console.log("Tentacular Engine starting...");

  const spec = await loadWorkflow();
  console.log(`Workflow: ${spec.name} v${spec.version}`);

  // Compile DAG
  const graph = compile(spec);
  console.log(
    `DAG compiled: ${graph.stages.length} stage(s), ${graph.nodeOrder.length} node(s)`,
  );
  for (let i = 0; i < graph.stages.length; i++) {
    const stage = graph.stages[i]!;
    console.log(`  Stage ${i + 1}: [${stage.nodes.join(", ")}]`);
  }

  // Load secrets with cascade: explicit → .secrets/ → .secrets.yaml → /app/secrets
  const secrets = await resolveSecrets({
    explicitPath: flags.secrets,
    workflowDir,
  });

  // Load all node modules — wrapped in a ref for atomic hot-reload swaps
  const nodeRef = { current: await loadAllNodes(spec.nodes, workflowDir) };

  // Create context
  const ctx = createContext({
    secrets,
    config: spec.config as Record<string, unknown> ?? {},
    contract: spec.contract,
  });

  // Create node runner
  const runner: NodeRunner = {
    async run(nodeId: string, _ctx: Context, input: unknown): Promise<unknown> {
      const fn = nodeRef.current.get(nodeId);
      if (!fn) throw new Error(`Node "${nodeId}" not loaded`);

      // Create a node-specific context
      const nodeCtx = createContext({
        secrets,
        config: spec.config as Record<string, unknown> ?? {},
        nodeId,
        contract: spec.contract,
      });

      return fn(nodeCtx, input);
    },
  };

  // Parse timeout from config
  let timeoutMs = 30_000;
  if (spec.config?.timeout) {
    const match = spec.config.timeout.match(/^(\d+)(s|m)$/);
    if (match) {
      const value = parseInt(match[1]!, 10);
      timeoutMs = match[2] === "m" ? value * 60_000 : value * 1_000;
    } else {
      console.warn(`Invalid timeout format "${spec.config.timeout}" — expected "<number>s" or "<number>m". Using default 30s.`);
    }
  }

  // Resolve webhook secret — only read WEBHOOK_SECRET env var when a webhook
  // trigger is actually declared. Reading Deno.env unconditionally crashes pods
  // that run without --allow-env (e.g. cron/queue workflows under gVisor).
  const hasWebhookTrigger = spec.triggers.some((t) => t.type === "webhook");
  const secretWebhookSecret =
    (secrets["github"] as Record<string, string> | undefined)?.["webhook_secret"];
  const webhookSecret = secretWebhookSecret ??
    (hasWebhookTrigger ? Deno.env.get("WEBHOOK_SECRET") : undefined);

  if (webhookSecret) {
    console.log("  Webhook secret: configured");
  } else if (hasWebhookTrigger) {
    console.warn(
      "  WARNING: Webhook trigger configured but no webhook secret found. " +
        "Set secrets.github.webhook_secret or WEBHOOK_SECRET env var. " +
        "Signature validation will be SKIPPED — do not use in production.",
    );
  }

  // Start HTTP server
  const server = startServer({
    port,
    graph,
    runner,
    ctx,
    timeoutMs,
    maxRetries: spec.config?.retries ?? 0,
    webhookSecret,
  });

  // Start NATS triggers for queue-type triggers
  let natsHandle: { close(): Promise<void> } | null = null;
  const queueTriggers = spec.triggers.filter((t) => t.type === "queue");
  if (queueTriggers.length > 0) {
    const config = spec.config as Record<string, unknown> ?? {};
    const natsUrl = config.nats_url as string | undefined;
    const natsToken = secrets.nats?.token;

    if (!natsUrl) {
      console.warn("Queue triggers defined but config.nats_url is not set — skipping NATS setup");
    } else if (!natsToken) {
      console.warn("Queue triggers defined but secrets.nats.token is not set — skipping NATS setup");
    } else {
      try {
        const { startNATSTriggers } = await import("./triggers/nats.ts");
        natsHandle = await startNATSTriggers({
          url: natsUrl,
          token: natsToken,
          triggers: queueTriggers,
          graph,
          runner,
          ctx,
          timeoutMs,
          maxRetries: spec.config?.retries ?? 0,
        });
      } catch (err) {
        console.error("Failed to start NATS triggers:", err);
      }
    }
  }

  // Graceful shutdown on SIGTERM/SIGINT
  const shutdown = async () => {
    console.log("Shutting down...");
    if (natsHandle) {
      await natsHandle.close();
    }
    server.shutdown();
    Deno.exit(0);
  };
  Deno.addSignalListener("SIGTERM", shutdown);
  Deno.addSignalListener("SIGINT", shutdown);

  // File watcher for hot-reload
  if (flags.watch) {
    watchFiles({
      dir: workflowDir,
      onChange: async () => {
        try {
          clearModuleCache();
          const newFunctions = await loadAllNodes(spec.nodes, workflowDir, true);
          nodeRef.current = newFunctions;
          console.log("Nodes reloaded successfully.");
        } catch (err) {
          console.error("Failed to reload nodes:", err);
        }
      },
    });
  }
}

main().catch((err) => {
  console.error("Engine failed to start:", err);
  Deno.exit(1);
});
