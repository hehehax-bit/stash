import React from "react";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { useStats } from "src/core/StashService";

function loadStreak(): number {
  try {
    const raw = localStorage.getItem("stash.dailyGoon.streak");
    const saved = raw ? JSON.parse(raw) : null;
    return saved?.streak ?? 0;
  } catch {
    return 0;
  }
}

function firstSessionDone(): boolean {
  return localStorage.getItem("stash.achievement.firstSession") === "1";
}

interface IAchievement {
  id: string;
  messageID: string;
  current: number;
  target: number;
}

export const Achievements: React.FC = () => {
  const { data: statsData } = useStats();
  const { data: oData } = GQL.useAiOHistoryLeaderboardQuery({
    variables: { limit: 100 },
  });
  const { data: savedData } = GQL.useAiSavedMomentsQuery();

  const streak = loadStreak();
  const performersFinished =
    (oData?.aiOHistoryLeaderboard ?? []).filter((e) => e.performer).length ?? 0;
  const scenesPlayed = statsData?.stats.scenes_played ?? 0;
  const savedCount = savedData?.aiSavedMoments?.length ?? 0;

  const achievements: IAchievement[] = [
    {
      id: "streak",
      messageID: "achievements.streak",
      current: streak,
      target: 7,
    },
    {
      id: "performers",
      messageID: "achievements.performers",
      current: performersFinished,
      target: 50,
    },
    {
      id: "scenes",
      messageID: "achievements.scenes",
      current: scenesPlayed,
      target: 100,
    },
    {
      id: "saved",
      messageID: "achievements.saved",
      current: savedCount,
      target: 10,
    },
    {
      id: "session",
      messageID: "achievements.session",
      current: firstSessionDone() ? 1 : 0,
      target: 1,
    },
  ];

  const unlocked = achievements.filter((a) => a.current >= a.target).length;

  return (
    <div className="for-you-row achievements">
      <h5>
        <FormattedMessage id="achievements.heading" />{" "}
        <span className="text-muted">
          ({unlocked}/{achievements.length})
        </span>
      </h5>
      <div className="row">
        {achievements.map((a) => {
          const done = a.current >= a.target;
          return (
            <div key={a.id} className="col-6 col-sm-4 col-md-2 achievement">
              <div className={done ? "achievement-done" : ""}>
                {done ? "🏆 " : "🔒 "}
                <FormattedMessage id={a.messageID} />
              </div>
              <div className="progress mt-1">
                <div
                  className="progress-bar"
                  role="progressbar"
                  style={{
                    width: `${Math.min(100, (a.current / a.target) * 100)}%`,
                  }}
                />
              </div>
              <div className="small text-muted">
                {Math.min(a.current, a.target)}/{a.target}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default Achievements;
