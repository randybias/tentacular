/**
 * Tentacular Engine — BasicSink
 *
 * In-memory ring buffer (capacity 1000) with aggregate counters.
 * record() is O(1). snapshot() is O(n) where n = min(recorded, capacity).
 */

import type { TelemetryEvent, TelemetrySnapshot, TelemetrySink } from "./types.ts";

const RING_BUFFER_CAPACITY = 1000;

export class BasicSink implements TelemetrySink {
  private readonly startMs: number;
  private totalEvents = 0;
  private errorCount = 0;
  private lastError: string | null = null;
  private lastErrorAt: number | null = null;

  // Ring buffer: fixed-capacity circular array
  private readonly buf: TelemetryEvent[];
  private bufHead = 0; // next write position
  private bufSize = 0; // number of valid entries (up to capacity)

  constructor() {
    this.startMs = Date.now();
    this.buf = new Array(RING_BUFFER_CAPACITY);
  }

  record(event: TelemetryEvent): void {
    this.totalEvents++;

    if (event.type === "node-error") {
      this.errorCount++;
      this.lastErrorAt = event.timestamp;
      const errMsg = event.metadata?.["error"];
      this.lastError = typeof errMsg === "string" ? errMsg : String(errMsg ?? "");
    }

    // Write into ring buffer at head position
    this.buf[this.bufHead] = event;
    this.bufHead = (this.bufHead + 1) % RING_BUFFER_CAPACITY;
    if (this.bufSize < RING_BUFFER_CAPACITY) {
      this.bufSize++;
    }
  }

  snapshot(): TelemetrySnapshot {
    const uptimeMs = Date.now() - this.startMs;
    const errorRate = this.totalEvents > 0 ? this.errorCount / this.totalEvents : 0;

    // Read ring buffer in insertion order: oldest → newest
    const recentEvents: TelemetryEvent[] = [];
    if (this.bufSize < RING_BUFFER_CAPACITY) {
      // Buffer not yet full: entries are at indices 0..bufSize-1
      for (let i = 0; i < this.bufSize; i++) {
        recentEvents.push(this.buf[i]!);
      }
    } else {
      // Buffer full: oldest entry is at current bufHead (wraps around)
      for (let i = 0; i < RING_BUFFER_CAPACITY; i++) {
        recentEvents.push(this.buf[(this.bufHead + i) % RING_BUFFER_CAPACITY]!);
      }
    }

    return {
      totalEvents: this.totalEvents,
      errorCount: this.errorCount,
      errorRate,
      uptimeMs,
      lastError: this.lastError,
      lastErrorAt: this.lastErrorAt,
      recentEvents,
      status: "ok",
    };
  }
}
