import React from "react";
import { Link } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { FormattedMessage } from "react-intl";

export const MoanLeaderboard: React.FC = () => {
  const { data, loading, error } = GQL.useAiMoanLeaderboardQuery({
    variables: { limit: 10 },
  });

  if (loading) return <LoadingIndicator inline />;
  if (error || !data?.aiMoanLeaderboard) return null;

  const entries = data.aiMoanLeaderboard.filter((e) => e.performer);
  if (entries.length === 0) {
    return (
      <div className="col col-sm-8 m-sm-auto text-muted mt-4">
        <FormattedMessage id="stats.moan_leaderboard.empty" />
      </div>
    );
  }

  return (
    <div className="col col-sm-8 m-sm-auto mt-4">
      <h4 className="text-center">
        <FormattedMessage id="stats.moan_leaderboard.heading" />
      </h4>
      <table className="table table-striped">
        <thead>
          <tr>
            <th>#</th>
            <th>
              <FormattedMessage id="performers" />
            </th>
            <th className="text-center">
              <FormattedMessage id="stats.moan_leaderboard.moan_scenes" />
            </th>
            <th className="text-center">
              <FormattedMessage id="stats.moan_leaderboard.moan_rate" />
            </th>
            <th className="text-center">
              <FormattedMessage id="stats.moan_leaderboard.avg_silence" />
            </th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e, i) => (
            <tr key={e.performer?.id}>
              <td>{i + 1}</td>
              <td>
                <Link to={`/performers/${e.performer?.id}`}>
                  {e.performer?.name}
                </Link>
              </td>
              <td className="text-center">{e.moan_scenes}</td>
              <td className="text-center">
                {Math.round((e.moan_rate ?? 0) * 100)}%
              </td>
              <td className="text-center">{Math.round(e.avg_silence ?? 0)}%</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

export default MoanLeaderboard;

export const OHistoryBoard: React.FC = () => {
  const { data, loading, error } = GQL.useAiOHistoryLeaderboardQuery({
    variables: { limit: 10 },
  });

  if (loading) return <LoadingIndicator inline />;
  if (error || !data?.aiOHistoryLeaderboard) return null;

  const entries = data.aiOHistoryLeaderboard.filter((e) => e.performer);
  if (entries.length === 0) {
    return (
      <div className="col col-sm-8 m-sm-auto text-muted mt-4">
        <FormattedMessage id="stats.o_board.empty" />
      </div>
    );
  }

  return (
    <div className="col col-sm-8 m-sm-auto mt-4">
      <h4 className="text-center">
        <FormattedMessage id="stats.o_board.heading" />
      </h4>
      <table className="table table-striped">
        <thead>
          <tr>
            <th>#</th>
            <th>
              <FormattedMessage id="performers" />
            </th>
            <th className="text-center">
              <FormattedMessage id="stats.o_board.o_scenes" />
            </th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e, i) => (
            <tr key={e.performer?.id}>
              <td>{i + 1}</td>
              <td>
                <Link to={`/performers/${e.performer?.id}`}>
                  {e.performer?.name}
                </Link>
              </td>
              <td className="text-center">{e.o_scenes}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};
