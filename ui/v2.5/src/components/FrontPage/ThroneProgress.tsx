import React from "react";
import { FormattedMessage } from "react-intl";
import { useStats } from "src/core/StashService";

export const RANKS = [
  { nameID: "throne.rank_novice", minXP: 0 },
  { nameID: "throne.rank_edge_lord", minXP: 250 },
  { nameID: "throne.rank_ascendant", minXP: 750 },
  { nameID: "throne.rank_heavenly", minXP: 1500 },
  { nameID: "throne.rank_celestial", minXP: 2500 },
  { nameID: "throne.rank_demigod", minXP: 4000 },
  { nameID: "throne.rank_deity", minXP: 6000 },
  { nameID: "throne.rank_apotheosis", minXP: 8500 },
  { nameID: "throne.rank_transcendent", minXP: 11000 },
  { nameID: "throne.rank_mystery", minXP: 15000 },
];

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

export function computeXP(
  scenesPlayed: number,
  totalO: number,
  streak: number
): number {
  const sessionStats = loadSessionStats();
  return (
    scenesPlayed +
    totalO * 5 +
    streak * 10 +
    sessionStats.sessions * 25 +
    Math.floor(sessionStats.totalMinutes / 10)
  );
}

export function rankForXP(xp: number) {
  let rank = RANKS[0];
  for (const r of RANKS) {
    if (xp >= r.minXP) rank = r;
  }
  return rank;
}

export const ThroneProgress: React.FC<{ compact?: boolean }> = ({
  compact,
}) => {
  const { data } = useStats();

  const scenesPlayed = data?.stats.scenes_played ?? 0;
  const totalO = data?.stats.total_o_count ?? 0;
  const streak = loadStreak();
  const xp = computeXP(scenesPlayed, totalO, streak);
  const rank = rankForXP(xp);
  const next = RANKS.find((r) => r.minXP > xp);
  const level = Math.floor(xp / 250) + 1;

  return (
    <div className={compact ? "throne-compact" : "throne"}>
      <h5>
        <FormattedMessage id="throne.heading" /> —{" "}
        <FormattedMessage id={rank.nameID} />{" "}
        <span className="text-muted">· LVL {level}</span>
      </h5>
      <div className="progress">
        <div
          className="progress-bar progress-bar-striped"
          role="progressbar"
          style={{
            width: next
              ? `${Math.min(
                  100,
                  ((xp - rank.minXP) / (next.minXP - rank.minXP)) * 100
                )}%`
              : "100%",
          }}
        />
      </div>
      <div className="small text-muted">
        {xp} XP ·{" "}
        {next ? (
          <>
            <FormattedMessage id="throne.next_rank" />
            {": "}
            <FormattedMessage id={next.nameID} /> ({next.minXP - xp} XP)
          </>
        ) : (
          <FormattedMessage id="throne.summit" />
        )}
      </div>
    </div>
  );
};

export default ThroneProgress;
