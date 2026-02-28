/**
 * Tentacular Engine — Telemetry Types
 *
 * TelemetrySink interface, TelemetryEvent, and TelemetrySnapshot.
 * Imported by BasicSink, NoopSink, and all callers that use the sink.
 */

/** A single telemetry event */
export interface TelemetryEvent {
  type:
    | "engine-start"
    | "node-start"
    | "node-complete"
    | "node-error"
    | "request-in"
    | "request-out"
    | "nats-message";
  timestamp: number; // epoch ms
  metadata?: Record<string, unknown>;
}

/** Snapshot of in-memory telemetry state */
export interface TelemetrySnapshot {
  totalEvents: number;
  errorCount: number;
  errorRate: number; // 0..1
  uptimeMs: number;
  lastError: string | null;
  lastErrorAt: number | null; // epoch ms
  recentEvents: TelemetryEvent[];
  status: "ok";
}

/** TelemetrySink — interface for recording events and reading snapshots. */
export interface TelemetrySink {
  record(event: TelemetryEvent): void;
  snapshot(): TelemetrySnapshot;
}
