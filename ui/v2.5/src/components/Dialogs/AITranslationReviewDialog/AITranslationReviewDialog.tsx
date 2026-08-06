import React, { useState } from "react";
import { Button, Form } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { ImageCard } from "src/components/Images/ImageCard";
import { PerformerCard } from "src/components/Performers/PerformerCard";
import { useToast } from "src/hooks/Toast";
import { FormattedMessage, useIntl } from "react-intl";
import { faLanguage } from "@fortawesome/free-solid-svg-icons";

interface IAITranslationReviewDialogProps {
  onClose: () => void;
}

export const AITranslationReviewDialog: React.FC<
  IAITranslationReviewDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();

  const [status, setStatus] = useState<string>("pending");

  const { data, loading, refetch } = GQL.useAiTranslationsQuery({
    variables: { status },
  });
  const [applyTranslation] = GQL.useAiTranslationApplyMutation();
  const [rejectTranslation] = GQL.useAiTranslationRejectMutation();
  const [applyAll] = GQL.useAiTranslationApplyAllMutation();
  const [rejectAll] = GQL.useAiTranslationRejectAllMutation();

  const translations = data?.aiTranslations ?? [];

  async function onApply(id: string) {
    try {
      await applyTranslation({
        variables: { translation_id: id },
      });
      Toast.success(intl.formatMessage({ id: "toast.updated_entity" }));
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onReject(id: string) {
    try {
      await rejectTranslation({
        variables: { translation_id: id },
      });
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onApplyAll() {
    if (
      !window.confirm(intl.formatMessage({ id: "actions.apply_all_confirm" }))
    ) {
      return;
    }
    try {
      const result = await applyAll();
      Toast.success(
        intl.formatMessage({ id: "toast.applied_all" }) +
          `: ${result.data?.aiTranslationApplyAll ?? 0}`
      );
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onRejectAll() {
    try {
      await rejectAll();
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  function entityPreview(t: GQL.AiTranslationsQuery["aiTranslations"][number]) {
    if (t.entity_type === "image" && t.image) {
      return <ImageCard image={t.image} zoomIndex={1} />;
    }
    if (t.entity_type === "performer" && t.performer) {
      return <PerformerCard performer={t.performer} />;
    }
    if (t.scene) {
      return <SceneCard scene={t.scene} />;
    }
    return null;
  }

  return (
    <ModalComponent
      show
      icon={faLanguage}
      header={intl.formatMessage({ id: "config.tasks.ai_translate.review" })}
      cancel={{
        onClick: () => onClose(),
        text: intl.formatMessage({ id: "actions.close" }),
        variant: "secondary",
      }}
    >
      <Form.Group>
        <Form.Control
          as="select"
          value={status}
          onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
            setStatus(e.currentTarget.value)
          }
        >
          <option value="pending">
            {intl.formatMessage({ id: "suggestion_status_pending" })}
          </option>
          <option value="accepted">
            {intl.formatMessage({ id: "suggestion_status_accepted" })}
          </option>
          <option value="rejected">
            {intl.formatMessage({ id: "suggestion_status_rejected" })}
          </option>
        </Form.Control>
      </Form.Group>

      <div className="d-flex mb-2">
        <Button
          size="sm"
          variant="primary"
          className="mr-2"
          onClick={() => onApplyAll()}
        >
          <FormattedMessage id="actions.apply_all" />
        </Button>
        <Button size="sm" variant="secondary" onClick={() => onRejectAll()}>
          <FormattedMessage id="actions.reject_all" />
        </Button>
      </div>

      {loading && <LoadingIndicator inline />}

      {!loading && translations.length === 0 && (
        <div className="text-muted">
          <FormattedMessage id="config.tasks.ai_translate.empty" />
        </div>
      )}

      {translations.map((t) => (
        <div key={t.id} className="row mb-3 align-items-center">
          <div className="col-3">{entityPreview(t)}</div>
          <div className="col-6">
            <div className="small text-muted">
              {t.field} ({t.language})
            </div>
            <div>{t.translated_text}</div>
          </div>
          <div className="col-3">
            {t.status === "pending" && (
              <div className="d-flex flex-column align-items-start">
                <Button
                  size="sm"
                  variant="primary"
                  className="mb-2"
                  onClick={() => onApply(t.id)}
                >
                  <FormattedMessage id="actions.apply" />
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => onReject(t.id)}
                >
                  <FormattedMessage id="actions.reject" />
                </Button>
              </div>
            )}
          </div>
        </div>
      ))}
    </ModalComponent>
  );
};

export default AITranslationReviewDialog;
