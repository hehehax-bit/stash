import React from "react";
import { Link } from "react-router-dom";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";

function loadStreak(): number {
  try {
    const raw = localStorage.getItem("stash.dailyGoon.streak");
    const saved = raw ? JSON.parse(raw) : null;
    return saved?.streak ?? 0;
  } catch {
    return 0;
  }
}

const LeaderboardTable: React.FC<{
  heading: React.ReactNode;
  rows: { id: string; name: string; stat: string; value: string }[];
}> = ({ heading, rows }) => (
  <div className="scoreboard-section">
    <h5>{heading}</h5>
    {rows.length === 0 ? (
      <div className="text-muted">
        <FormattedMessage id="scoreboard.empty" />
      </div>
    ) : (
      <table className="table table-striped">
        <tbody>
          {rows.map((r) => (
            <tr key={r.id}>
              <td>
                <Link to={`/performers/${r.id}`}>{r.name}</Link>
              </td>
              <td className="text-muted">{r.stat}</td>
              <td className="text-right">{r.value}</td>
            </tr>
          ))}
        </tbody>
      </table>
    )}
  </div>
);

export const ScoreboardPage: React.FC = () => {
  const { data: moanData } = GQL.useAiMoanLeaderboardQuery({
    variables: { limit: 10 },
  });
  const { data: oData } = GQL.useAiOHistoryLeaderboardQuery({
    variables: { limit: 10 },
  });
  const { data: timelineData } = GQL.useAiOHistoryTimelineQuery({
    variables: { days: 30 },
  });
  const { data: savedData } = GQL.useAiSavedMomentsQuery();

  const moaners = (moanData?.aiMoanLeaderboard ?? []).filter(
    (e) => e.performer
  );
  const oBoard = (oData?.aiOHistoryLeaderboard ?? []).filter(
    (e) => e.performer
  );
  const timeline = timelineData?.aiOHistoryTimeline ?? [];
  const savedCount = savedData?.aiSavedMoments?.length ?? 0;
  const streak = loadStreak();

  if (!moanData && !oData && !timelineData && !savedData) {
    return <LoadingIndicator />;
  }

  return (
    <div className="scoreboard-page container">
      <h3 className="my-3">
        <FormattedMessage id="scoreboard.heading" />
      </h3>

      <div className="row">
        <div className="col-12 col-sm-6">
          <LeaderboardTable
            heading={
              <>
                🔥👑 <FormattedMessage id="scoreboard.moaners" />
              </>
            }
            rows={moaners.map((e) => ({
              id: e.performer?.id ?? "",
              name: e.performer?.name ?? "",
              stat: `${e.moan_scenes} moan scenes`,
              value: `${Math.round((e.moan_rate ?? 0) * 100)}%`,
            }))}
          />
        </div>
        <div className="col-12 col-sm-6">
          <LeaderboardTable
            heading={<FormattedMessage id="scoreboard.o_board" />}
            rows={oBoard.map((e) => ({
              id: e.performer?.id ?? "",
              name: e.performer?.name ?? "",
              stat: "",
              value: `${e.o_scenes} O`,
            }))}
          />
        </div>
      </div>

      <div className="row">
        <div className="col-12 col-sm-4">
          <div className="scoreboard-section">
            <h5>
              <FormattedMessage id="scoreboard.streak" />
            </h5>
            <p className="display-4">{streak} 🔥</p>
          </div>
        </div>
        <div className="col-12 col-sm-4">
          <div className="scoreboard-section">
            <h5>
              <FormattedMessage id="scoreboard.saved" />
            </h5>
            <p className="display-4">{savedCount} ❤️</p>
            <Link to="/saved">
              <FormattedMessage id="saved_moments.heading" /> →
            </Link>
          </div>
        </div>
        <div className="col-12 col-sm-4">
          <div className="scoreboard-section">
            <h5>
              <FormattedMessage id="scoreboard.finish_history" />
            </h5>
            {timeline.length === 0 ? (
              <div className="text-muted">
                <FormattedMessage id="scoreboard.empty" />
              </div>
            ) : (
              <div className="finish-timeline">
                {timeline.slice(0, 14).map((e) => (
                  <div key={e.date} className="finish-bar-row">
                    <span className="finish-date">{e.date.slice(5)}</span>
                    <div className="finish-bar-track">
                      <div
                        className="finish-bar"
                        style={{
                          width: `${Math.min(100, (e.count / 6) * 100)}%`,
                        }}
                      />
                    </div>
                    <span className="finish-count">{e.count}</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
};

export default ScoreboardPage;
