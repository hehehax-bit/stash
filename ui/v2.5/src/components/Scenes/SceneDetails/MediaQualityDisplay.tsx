import React from "react";
import { FormattedMessage, useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { PatchComponent } from "src/patch";

interface IMediaQualityDisplayProps {
  entityType: "scene" | "image";
  entityID: string;
}

const MediaQualityDisplay: React.FC<IMediaQualityDisplayProps> = PatchComponent(
  "MediaQualityDisplay",
  (props) => {
    const intl = useIntl();
    const { data, loading, error } = GQL.useAiMediaQualityQuery({
      variables: {
        entity_type: props.entityType,
        entity_id: props.entityID,
      },
      skip: !props.entityID,
    });

    if (loading || !data?.aiMediaQuality) {
      return null;
    }

    if (error) {
      return null;
    }

    const quality = data.aiMediaQuality;

    const scoreColor = (score: number) => {
      if (score >= 80) return "text-success";
      if (score >= 60) return "text-warning";
      return "text-danger";
    };

    const renderScore = (label: string, score: number, fullWidth = false) => (
      <div className={fullWidth ? "col-12" : "col-6 col-md-3"} key={label}>
        <div className="ai-quality-score">
          <div className={`ai-quality-score-value ${scoreColor(score)}`}>
            {score}
          </div>
          <div className="ai-quality-score-label">{label}</div>
        </div>
      </div>
    );

    return (
      <div className="ai-media-quality-display mb-3 p-3 border rounded">
        <h6 className="mb-3">
          <FormattedMessage
            id="ai.media_quality.title"
            defaultMessage="AI Media Quality Assessment"
          />
        </h6>
        <div className="row">
          <div className="col-12 mb-3">
            <div
              className={`ai-quality-overall ${scoreColor(quality.quality_score)}`}
            >
              <div className="ai-quality-overall-value">
                {quality.quality_score}
              </div>
              <div className="ai-quality-overall-label">
                <FormattedMessage
                  id="ai.media_quality.overall"
                  defaultMessage="Overall Quality Score"
                />
              </div>
            </div>
          </div>
          {renderScore(
            intl.formatMessage({
              id: "ai.media_quality.visual_clarity",
              defaultMessage: "Visual Clarity",
            }),
            quality.visual_clarity
          )}
          {renderScore(
            intl.formatMessage({
              id: "ai.media_quality.lighting",
              defaultMessage: "Lighting",
            }),
            quality.lighting
          )}
          {renderScore(
            intl.formatMessage({
              id: "ai.media_quality.composition",
              defaultMessage: "Composition",
            }),
            quality.composition
          )}
          {renderScore(
            intl.formatMessage({
              id: "ai.media_quality.camera_work",
              defaultMessage: "Camera Work",
            }),
            quality.camera_work
          )}
        </div>
        {quality.notes && (
          <div className="ai-quality-notes mt-3">
            <small className="text-muted">{quality.notes}</small>
          </div>
        )}
      </div>
    );
  }
);

export default MediaQualityDisplay;
