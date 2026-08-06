import React from "react";
import { useHistory } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { FormattedMessage, useIntl } from "react-intl";

interface IPerformerAppearancesPanelProps {
  performer: GQL.PerformerDataFragment;
}

function formatTime(seconds: number): string {
  const s = Math.floor(seconds);
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return `${String(m).padStart(2, "0")}:${String(rem).padStart(2, "0")}`;
}

export const PerformerAppearancesPanel: React.FC<
  IPerformerAppearancesPanelProps
> = ({ performer }) => {
  const intl = useIntl();
  const history = useHistory();

  const { data, loading, error } = GQL.useAiPerformerAppearancesQuery({
    variables: { performer_id: performer.id, limit: 200 },
    skip: !performer.id,
  });

  if (loading) return <LoadingIndicator />;
  if (error) return <span className="text-muted">{error.message}</span>;

  const appearances = data?.aiPerformerAppearances ?? [];

  if (appearances.length === 0) {
    return (
      <div className="text-muted">
        <FormattedMessage id="performers.appearances.empty" />
      </div>
    );
  }

  function onOpenAppearance(
    a: GQL.AiPerformerAppearancesQuery["aiPerformerAppearances"][number]
  ) {
    const sceneId = a.scene?.id;
    if (!sceneId) return;
    // seek into the scene at the marker start time
    history.push(`/scenes/${sceneId}?t=${a.seconds}`);
  }

  return (
    <div className="row">
      {appearances.map((a) => (
        <div
          key={a.marker_id}
          className="col-6 col-sm-4 col-md-3 col-xl-2 mb-3"
        >
          <div
            role="button"
            className="performer-appearance"
            onClick={() => onOpenAppearance(a)}
          >
            <img src={a.screenshot} alt="" />
            <div className="title">{a.title}</div>
            <div className="small text-muted">
              {a.scene?.title ?? intl.formatMessage({ id: "scene" })} ·{" "}
              {formatTime(a.seconds)}
            </div>
          </div>
        </div>
      ))}
    </div>
  );
};

export default PerformerAppearancesPanel;
