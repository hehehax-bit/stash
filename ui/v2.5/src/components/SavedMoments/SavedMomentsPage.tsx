import React from "react";
import { Button } from "react-bootstrap";
import { useHistory } from "react-router-dom";
import { FormattedMessage, useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import {
  mutateMetadataGenerateGoonReel,
  mutateAiUnsaveMoment,
} from "src/core/StashService";
import { useToast } from "src/hooks/Toast";

export const SavedMomentsPage: React.FC = () => {
  const intl = useIntl();
  const Toast = useToast();
  const history = useHistory();

  const { data, loading, refetch } = GQL.useAiSavedMomentsQuery();

  async function onUnsave(markerId: string) {
    try {
      await mutateAiUnsaveMoment(markerId);
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onCreateReel() {
    const moments = data?.aiSavedMoments ?? [];
    const sceneIds = moments
      .map((m) => m.scene?.id)
      .filter((id): id is string => !!id);
    if (sceneIds.length === 0) return;

    try {
      const result = await mutateMetadataGenerateGoonReel(sceneIds);
      Toast.success(
        intl.formatMessage({ id: "toast.reel_generated" }) +
          `: ${result.data?.metadataGenerateGoonReel ?? ""}`
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  function openScene(sceneId: string | undefined, seconds: number) {
    if (!sceneId) return;
    history.push(`/scenes/${sceneId}?t=${seconds}`);
  }

  if (loading) return <LoadingIndicator />;

  const moments = data?.aiSavedMoments ?? [];

  return (
    <div className="saved-moments-page container">
      <div className="d-flex align-items-center my-3">
        <h3 className="mb-0">
          <FormattedMessage id="saved_moments.heading" />
        </h3>
        {moments.length > 0 && (
          <Button
            variant="outline-danger"
            className="ml-3"
            onClick={() => onCreateReel()}
          >
            <FormattedMessage id="saved_moments.create_reel" />
          </Button>
        )}
      </div>

      {moments.length === 0 && (
        <div className="text-muted">
          <FormattedMessage id="saved_moments.empty" />
        </div>
      )}

      <div className="row">
        {moments.map((m) => (
          <div
            key={m.marker_id}
            className="col-6 col-sm-4 col-md-3 col-xl-2 mb-3"
          >
            <div
              role="button"
              className="saved-moment"
              onClick={() => openScene(m.scene?.id, m.seconds)}
            >
              <img src={m.screenshot} alt="" />
              <div className="saved-moment-title">{m.title}</div>
              <div className="small text-muted">
                {m.scene?.title ?? ""} · {Math.round(m.seconds)}s
              </div>
            </div>
            <Button
              size="sm"
              variant="outline-danger"
              className="w-100 mt-1"
              onClick={() => onUnsave(m.marker_id)}
            >
              <FormattedMessage id="saved_moments.unsave" />
            </Button>
          </div>
        ))}
      </div>
    </div>
  );
};

export default SavedMomentsPage;
