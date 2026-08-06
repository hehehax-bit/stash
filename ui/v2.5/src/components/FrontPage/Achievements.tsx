import React from "react";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { useStats } from "src/core/StashService";

const TIER_PIPS = ["🥉", "🥈", "🥇", "🏆", "👑"];

function loadStreak(): number {
  try {
    const raw = localStorage.getItem("stash.dailyGoon.streak");
    const saved = raw ? JSON.parse(raw) : null;
    return saved?.streak ?? 0;
  } catch {
    return 0;
  }
}

function loadSessionStats(): { totalMinutes: number; sessions: number } {
  try {
    const raw = localStorage.getItem("stash.goonSessionStats");
    const stats = raw ? JSON.parse(raw) : {};
    return {
      totalMinutes: stats.totalMinutes ?? 0,
      sessions: stats.sessions ?? 0,
    };
  } catch {
    return { totalMinutes: 0, sessions: 0 };
  }
}

function loadFlag(key: string): boolean {
  return localStorage.getItem(key) === "1";
}

function loadEdgePauses(): number {
  try {
    return Number(localStorage.getItem("stash.edgePauses") ?? "0");
  } catch {
    return 0;
  }
}

interface IAchievementFamily {
  id: string;
  messageID: string;
  tiers: number[];
}

const FAMILIES: IAchievementFamily[] = [
  {
    id: "scenes",
    messageID: "achievements.scenes",
    tiers: [100, 250, 500, 1000, 2500],
  },
  { id: "o", messageID: "achievements.o", tiers: [50, 100, 250, 500] },
  { id: "streak", messageID: "achievements.streak", tiers: [7, 14, 30] },
  {
    id: "sessions",
    messageID: "achievements.sessions",
    tiers: [10, 25, 50, 100],
  },
  { id: "time", messageID: "achievements.time", tiers: [300, 600, 3000] },
  {
    id: "performers",
    messageID: "achievements.performers",
    tiers: [10, 25, 50, 100],
  },
  { id: "saved", messageID: "achievements.saved", tiers: [5, 10, 25, 50] },
  { id: "plans", messageID: "achievements.plans", tiers: [5] },
  { id: "edge", messageID: "achievements.edge", tiers: [10, 50] },
  { id: "transcend", messageID: "achievements.transcend", tiers: [1] },
  { id: "chapel", messageID: "achievements.chapel", tiers: [1] },
];

export const Achievements: React.FC = () => {
  const { data: statsData } = useStats();
  const { data: oData } = GQL.useAiOHistoryLeaderboardQuery({
    variables: { limit: 100 },
  });
  const { data: savedData } = GQL.useAiSavedMomentsQuery();
  const { data: plansData } = GQL.useAiSavedPlansQuery();

  const streak = loadStreak();
  const sessionStats = loadSessionStats();
  const performersFinished =
    (oData?.aiOHistoryLeaderboard ?? []).filter((e) => e.performer).length ?? 0;
  const scenesPlayed = statsData?.stats.scenes_played ?? 0;
  const totalO = statsData?.stats.total_o_count ?? 0;
  const savedCount = savedData?.aiSavedMoments?.length ?? 0;
  const plansCount = plansData?.aiSavedPlans?.length ?? 0;
  const edgePauses = loadEdgePauses();

  const values: Record<string, number> = {
    scenes: scenesPlayed,
    o: totalO,
    streak,
    sessions: sessionStats.sessions,
    time: sessionStats.totalMinutes,
    performers: performersFinished,
    saved: savedCount,
    plans: plansCount,
    edge: edgePauses,
    transcend: loadFlag("stash.achievement.transcend") ? 1 : 0,
    chapel: loadFlag("stash.achievement.nightChapel") ? 1 : 0,
  };

  const unlocked = FAMILIES.reduce(
    (sum, f) => sum + f.tiers.filter((t) => values[f.id] >= t).length,
    0
  );
  const total = FAMILIES.reduce((sum, f) => sum + f.tiers.length, 0);

  return (
    <div className="for-you-row achievements">
      <h5>
        <FormattedMessage id="achievements.heading" />{" "}
        <span className="text-muted">
          ({unlocked}/{total})
        </span>
      </h5>
      <div className="row">
        {FAMILIES.map((f) => {
          const value = values[f.id];
          const reached = f.tiers.filter((t) => value >= t).length;
          const nextTier = f.tiers[reached];
          const prevTier = reached > 0 ? f.tiers[reached - 1] : 0;
          const pct = nextTier
            ? Math.min(100, ((value - prevTier) / (nextTier - prevTier)) * 100)
            : 100;
          return (
            <div
              key={f.id}
              className="col-6 col-sm-4 col-md-3 col-lg-2 achievement"
            >
              <div className="achievement-name">
                {reached > 0
                  ? TIER_PIPS[Math.min(reached - 1, TIER_PIPS.length - 1)]
                  : "🔒"}{" "}
                <FormattedMessage id={f.messageID} />
              </div>
              <div className="progress mt-1">
                <div
                  className="progress-bar"
                  role="progressbar"
                  style={{ width: `${pct}%` }}
                />
              </div>
              <div className="small text-muted">
                {nextTier ? `${value}/${nextTier}` : `${value} · max`}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default Achievements;
