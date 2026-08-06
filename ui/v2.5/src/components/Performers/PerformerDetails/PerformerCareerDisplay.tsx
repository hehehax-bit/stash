import React from "react";
import { FormattedMessage, useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { PatchComponent } from "src/patch";

interface IPerformerCareerDisplayProps {
  performerID: string;
}

const PerformerCareerDisplay: React.FC<IPerformerCareerDisplayProps> =
  PatchComponent("PerformerCareerDisplay", (props) => {
    const intl = useIntl();
    const { data, loading, error } = GQL.useAiPerformerCareerQuery({
      variables: {
        performer_id: props.performerID,
      },
      skip: !props.performerID,
    });

    if (loading || !data?.aiPerformerCareer) {
      return null;
    }

    if (error) {
      return null;
    }

    const career = data.aiPerformerCareer;

    return (
      <div className="ai-performer-career-display mb-3 p-3 border rounded">
        <h6 className="mb-3">
          <FormattedMessage
            id="ai.performer_career.title"
            defaultMessage="AI Career Analytics"
          />
        </h6>

        {(career.career_start || career.career_end) && (
          <div className="row mb-3">
            <div className="col-md-6">
              <strong>
                <FormattedMessage
                  id="ai.performer_career.career_start"
                  defaultMessage="Career Start"
                />
                :
              </strong>{" "}
              <span className="ml-2">
                {career.career_start
                  ? career.career_start
                  : intl.formatMessage({
                      id: "unknown",
                      defaultMessage: "Unknown",
                    })}
              </span>
            </div>
            <div className="col-md-6">
              <strong>
                <FormattedMessage
                  id="ai.performer_career.career_end"
                  defaultMessage="Career End"
                />
                :
              </strong>{" "}
              <span className="ml-2">
                {career.career_end
                  ? career.career_end
                  : intl.formatMessage({
                      id: "present",
                      defaultMessage: "Present",
                    })}
              </span>
            </div>
          </div>
        )}

        {career.active_years.length > 0 && (
          <div className="mb-3">
            <strong>
              <FormattedMessage
                id="ai.performer_career.active_years"
                defaultMessage="Active Years"
              />
              :
            </strong>
            <div className="mt-1">
              {career.active_years.map((year) => (
                <span key={year} className="badge badge-info mr-1 mb-1">
                  {year}
                </span>
              ))}
            </div>
          </div>
        )}

        {career.primary_niches.length > 0 && (
          <div className="mb-3">
            <strong>
              <FormattedMessage
                id="ai.performer_career.primary_niches"
                defaultMessage="Primary Niches"
              />
              :
            </strong>
            <div className="mt-1">
              {career.primary_niches.map((niche) => (
                <span key={niche} className="badge badge-secondary mr-1 mb-1">
                  {niche}
                </span>
              ))}
            </div>
          </div>
        )}

        {career.notable_studios.length > 0 && (
          <div className="mb-3">
            <strong>
              <FormattedMessage
                id="ai.performer_career.notable_studios"
                defaultMessage="Notable Studios"
              />
              :
            </strong>
            <div className="mt-1">
              {career.notable_studios.map((studio) => (
                <span key={studio} className="badge badge-primary mr-1 mb-1">
                  {studio}
                </span>
              ))}
            </div>
          </div>
        )}

        {career.career_highlights && (
          <div className="mb-3">
            <strong>
              <FormattedMessage
                id="ai.performer_career.career_highlights"
                defaultMessage="Career Highlights"
              />
              :
            </strong>
            <p className="mt-1 mb-0">{career.career_highlights}</p>
          </div>
        )}

        {career.summary && (
          <div className="mb-3">
            <strong>
              <FormattedMessage
                id="ai.performer_career.summary"
                defaultMessage="Summary"
              />
              :
            </strong>
            <p className="mt-1 mb-0">{career.summary}</p>
          </div>
        )}
      </div>
    );
  });

export default PerformerCareerDisplay;
