import React from "react";
import { useHistory } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage } from "react-intl";

interface IPerformerAudioStatsPanelProps {
  performerId: string;
}

export const PerformerAudioStatsPanel: React.FC<
  IPerformerAudioStatsPanelProps
> = ({ performerId }) => {
  const history = useHistory();

  const { data, loading, error } = GQL.useAiPerformerAudioStatsQuery({
    variables: { performer_id: performerId },
    skip: !performerId,
  });

  if (loading || error || !data?.aiPerformerAudioStats) {
    return null;
  }

  const stats = data.aiPerformerAudioStats;
  if (stats.scenes_with_audio === 0) {
    return null;
  }

  function openMoaningScenes() {
    // show this performer's scenes that have audio analysis with moans
    history.push(`/scenes?performerids=${performerId}`);
  }

  return (
    <div className="performer-audio-stats">
      <h6>
        <FormattedMessage id="performers.audio_stats.heading" />
      </h6>
      <ul>
        <li>
          <FormattedMessage id="performers.audio_stats.scenes_with_audio" />:{" "}
          <strong>{stats.scenes_with_audio}</strong>
        </li>
        <li>
          <FormattedMessage id="performers.audio_stats.moan_scenes" />:{" "}
          <strong>{stats.moan_scenes}</strong> (
          {Math.round((stats.moan_rate ?? 0) * 100)}%)
        </li>
        <li>
          <FormattedMessage id="performers.audio_stats.avg_silence" />:{" "}
          <strong>{Math.round(stats.avg_silence ?? 0)}%</strong>
        </li>
      </ul>
      {stats.moan_scenes > 0 && (
        <button
          className="btn btn-sm btn-outline-secondary"
          onClick={() => openMoaningScenes()}
        >
          <FormattedMessage id="performers.audio_stats.open_moaning" />
        </button>
      )}
    </div>
  );
};

export default PerformerAudioStatsPanel;
