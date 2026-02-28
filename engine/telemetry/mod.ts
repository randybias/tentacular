/**
 * Tentacular Engine — Telemetry Module
 *
 * Public entry point for the telemetry package.
 * Exports all types, NoopSink, BasicSink, and the NewTelemetrySink factory.
 *
 * The active sink is created from the TELEMETRY_SINK env var (default "basic").
 */

export type { TelemetryEvent, TelemetrySnapshot, TelemetrySink } from "./types.ts";
export { BasicSink } from "./basic.ts";

import type { TelemetryEvent, TelemetrySnapshot, TelemetrySink } from "./types.ts";
import { BasicSink } from "./basic.ts";

/** NoopSink — zero-cost implementation; all methods are no-ops. */
export class NoopSink implements TelemetrySink {
  record(_event: TelemetryEvent): void {}
  snapshot(): TelemetrySnapshot {
    return {
      totalEvents: 0,
      errorCount: 0,
      errorRate: 0,
      uptimeMs: 0,
      lastError: null,
      lastErrorAt: null,
      recentEvents: [],
      status: "ok",
      lastRunFailed: false,
      inFlight: 0,
    };
  }
}

/**
 * NewTelemetrySink — factory function.
 * Returns NoopSink for "noop", BasicSink for "basic" or any unrecognized kind.
 */
export function NewTelemetrySink(kind?: string): TelemetrySink {
  if (kind === "noop") {
    return new NoopSink();
  }
  return new BasicSink();
}
