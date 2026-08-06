import React, { useEffect, useState } from "react";
import { Button, Form } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { ImageCard } from "src/components/Images/ImageCard";
import { PerformerSelect } from "src/components/Performers/PerformerSelect";
import { useToast } from "src/hooks/Toast";
import { FormattedMessage, useIntl } from "react-intl";
import { faUserPlus } from "@fortawesome/free-solid-svg-icons";

interface IAIPerformerDiscoveryReviewDialogProps {
  onClose: () => void;
}

export const AIPerformerDiscoveryReviewDialog: React.FC<
  IAIPerformerDiscoveryReviewDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();

  const [status, setStatus] = useState<string>("pending");

  const { data, loading, refetch } = GQL.useAiPerformerCandidatesQuery({
    variables: { status },
  });
  const [applyCandidate] = GQL.useAiPerformerCandidateApplyMutation();
  const [rejectCandidate] = GQL.useAiPerformerCandidateRejectMutation();
  const [applyAll] = GQL.useAiPerformerCandidateApplyAllMutation();
  const [rejectAll] = GQL.useAiPerformerCandidateRejectAllMutation();

  const candidates = data?.aiPerformerCandidates ?? [];

  // refresh while the dialog is open so candidates appear during a running job
  useEffect(() => {
    const timer = setInterval(() => refetch(), 10000);
    return () => clearInterval(timer);
  }, [refetch]);

  async function onCreate(id: string) {
    try {
      await applyCandidate({
        variables: { candidate_id: id },
      });
      Toast.success(intl.formatMessage({ id: "toast.created_entity" }));
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onMergeInto(id: string, targetId: string | undefined) {
    try {
      await applyCandidate({
        variables: { candidate_id: id, target_performer_id: targetId },
      });
      Toast.success(intl.formatMessage({ id: "toast.merged_entities" }));
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onReject(id: string) {
    try {
      await rejectCandidate({
        variables: { candidate_id: id },
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
          `: ${result.data?.aiPerformerCandidateApplyAll ?? 0}`
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

  function entityPreview(
    c: GQL.AiPerformerCandidatesQuery["aiPerformerCandidates"][number]
  ) {
    if (c.entity_type === "image" && c.image) {
      return <ImageCard image={c.image} zoomIndex={1} />;
    }
    if (c.scene) {
      return <SceneCard scene={c.scene} />;
    }
    return null;
  }

  return (
    <ModalComponent
      show
      onHide={onClose}
      icon={faUserPlus}
      header={intl.formatMessage({
        id: "config.tasks.ai_performer_discovery.review",
      })}
      dialogClassName="modal-xl"
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

      {!loading && candidates.length === 0 && (
        <div className="text-muted">
          <FormattedMessage id="config.tasks.ai_performer_discovery.empty" />
        </div>
      )}

      {candidates.map((c) => (
        <div key={c.id} className="row mb-3 align-items-center">
          <div className="col-4">{entityPreview(c)}</div>
          <div className="col-4">
            <div>
              <strong>{c.name}</strong>{" "}
              <span className="text-muted">({c.member_count} appearances)</span>
            </div>
            {c.description && (
              <div className="small text-muted">{c.description}</div>
            )}
          </div>
          <div className="col-4">
            {c.status === "pending" && (
              <div className="d-flex flex-column align-items-start">
                <Button
                  size="sm"
                  variant="primary"
                  className="mb-2"
                  onClick={() => onCreate(c.id)}
                >
                  <FormattedMessage
                    id="config.tasks.ai_performer_discovery.create"
                    defaultMessage="Create performer"
                  />
                </Button>
                <Form.Group className="w-100">
                  <PerformerSelect
                    onSelect={(items) => {
                      if (items && items.length > 0)
                        onMergeInto(c.id, items[0].id);
                    }}
                  />
                </Form.Group>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => onReject(c.id)}
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

export default AIPerformerDiscoveryReviewDialog;
