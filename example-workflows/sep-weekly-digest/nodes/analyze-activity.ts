import type { Context } from "tentacular";

interface Sep {
  number: number;
  sepId: string;
  title: string;
  state: string;
  author: string;
  createdAt: string;
  updatedAt: string;
  url: string;
  labels: string[];
  summary: string;
}

interface WeeklySnapshot {
  id: number;
  collectedAt: string;
  repo: string;
  sepCount: number;
  seps: Sep[];
}

interface StoreResult {
  stored: boolean;
  snapshotId: number;
  weeklyHistory: WeeklySnapshot[];
}

interface StateTransition {
  sepId: string;
  from: string;
  to: string;
}

interface WeekOverWeekMetric {
  metric: string;
  thisWeek: number;
  lastWeek: number;
  delta: number;
}

interface ActivityMetrics {
  highVelocity: Sep[];
  inactive: Sep[];
  newThisWeek: Sep[];
  closedThisWeek: Sep[];
  stateTransitions: StateTransition[];
  totalActive: number;
  totalInactive: number;
  velocityScore: number;
  weekOverWeek: WeekOverWeekMetric[];
}

const SEVEN_DAYS_MS = 7 * 24 * 60 * 60 * 1000;
const TWENTY_EIGHT_DAYS_MS = 28 * 24 * 60 * 60 * 1000;

/** Analyze SEP activity trends from stored weekly snapshots */
export default async function run(ctx: Context, input: unknown): Promise<ActivityMetrics> {
  const storeResult = input as StoreResult;
  const history = storeResult.weeklyHistory ?? [];
  const now = Date.now();

  // History is ordered by collected_at DESC (most recent first)
  const currentSnapshot = history.length > 0 ? history[0] : null;
  const previousSnapshot = history.length > 1 ? history[1] : null;

  const currentSeps = currentSnapshot?.seps ?? [];
  const previousSeps = previousSnapshot?.seps ?? [];

  const currentMap = new Map(currentSeps.map((s) => [s.sepId, s]));
  const previousMap = new Map(previousSeps.map((s) => [s.sepId, s]));

  // High velocity: updated within last 7 days
  const highVelocity = currentSeps.filter((sep) => {
    const updated = new Date(sep.updatedAt).getTime();
    return now - updated < SEVEN_DAYS_MS;
  });

  // Inactive: not updated in 4+ weeks
  const inactive = currentSeps.filter((sep) => {
    const updated = new Date(sep.updatedAt).getTime();
    return now - updated >= TWENTY_EIGHT_DAYS_MS;
  });

  // New this week: in current but not in previous
  const newThisWeek = currentSeps.filter((sep) => !previousMap.has(sep.sepId));

  // Closed this week: in previous but not in current
  const closedThisWeek = previousSeps.filter((sep) => !currentMap.has(sep.sepId));

  // State transitions: SEPs present in both with different state
  const stateTransitions: StateTransition[] = [];
  for (const sep of currentSeps) {
    const prev = previousMap.get(sep.sepId);
    if (prev && prev.state !== sep.state) {
      stateTransitions.push({
        sepId: sep.sepId,
        from: prev.state,
        to: sep.state,
      });
    }
  }

  const totalActive = highVelocity.length;
  const totalInactive = inactive.length;
  const total = currentSeps.length;

  // Velocity score: ratio of active to total * 100
  const velocityScore = total > 0 ? Math.round((totalActive / total) * 100) : 0;

  // Week-over-week comparisons
  const prevActive = previousSeps.filter((sep) => {
    const updated = new Date(sep.updatedAt).getTime();
    const snapshotTime = previousSnapshot
      ? new Date(previousSnapshot.collectedAt).getTime()
      : now;
    return snapshotTime - updated < SEVEN_DAYS_MS;
  }).length;

  const prevInactive = previousSeps.filter((sep) => {
    const updated = new Date(sep.updatedAt).getTime();
    const snapshotTime = previousSnapshot
      ? new Date(previousSnapshot.collectedAt).getTime()
      : now;
    return snapshotTime - updated >= TWENTY_EIGHT_DAYS_MS;
  }).length;

  const weekOverWeek: WeekOverWeekMetric[] = [
    {
      metric: "total",
      thisWeek: total,
      lastWeek: previousSeps.length,
      delta: total - previousSeps.length,
    },
    {
      metric: "active",
      thisWeek: totalActive,
      lastWeek: prevActive,
      delta: totalActive - prevActive,
    },
    {
      metric: "inactive",
      thisWeek: totalInactive,
      lastWeek: prevInactive,
      delta: totalInactive - prevInactive,
    },
  ];

  ctx.log.info(
    `Activity: ${totalActive} active, ${totalInactive} inactive, ${newThisWeek.length} new, ${closedThisWeek.length} closed, velocity=${velocityScore}`,
  );

  return {
    highVelocity,
    inactive,
    newThisWeek,
    closedThisWeek,
    stateTransitions,
    totalActive,
    totalInactive,
    velocityScore,
    weekOverWeek,
  };
}
