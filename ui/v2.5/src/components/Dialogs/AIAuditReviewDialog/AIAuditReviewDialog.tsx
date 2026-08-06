import React, { useState } from "react";
import { Button, Form } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { ImageCard } from "src/components/Images/ImageCard";
import { useToast } from "src/hooks/Toast";
import { FormattedMessage, useIntl } from "react-intl";
import { faSearch } from "@fortawesome/free-solid-svg-icons";

interface IAIAuditReviewDialogProps {
  onClose: () => void;
}

const FIELD_LABELS: Record<string, string> = {
  title: "Title",
  details: "Details",
  performers: "Performers",
};

export const AIAuditReviewDialog: React.FC<IAIAuditReviewDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [status, setStatus] = useState<string>("pending");

  const { data, loading, refetch } = GQL.useAiAuditsQuery({
    variables: { status },
  });
  const [applyAudit] = GQL.useAiAuditApplyMutation();
  const [rejectAudit] = GQL.useAiAuditRejectMutation();
  const [applyAll] = GQL.useAiAuditApplyAllMutation();
  const [rejectAll] = GQL.useAiAuditRejectAllMutation();

  const audits = data?.aiAudits ?? [];

  async function onApply(id: string) {
    try {
      await applyAudit({
        variables: { audit_id: id },
      });
      Toast.success(intl.formatMessage({ id: "toast.updated_entity" }));
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onReject(id: string) {
    try {
      await rejectAudit({
        variables: { audit_id: id },
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
          `: ${result.data?.aiAuditApplyAll ?? 0}`
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

  function entityPreview(a: GQL.AiAuditsQuery["aiAudits"][number]) {
    if (a.entity_type === "image" && a.image) {
      return <ImageCard image={a.image} zoomIndex={1} />;
    }
    if (a.scene) {
      return <SceneCard scene={a.scene} />;
    }
    return null;
  }

  return (
    <ModalComponent
      show
      icon={faSearch}
      header={intl.formatMessage({ id: "config.tasks.ai_audit.review" })}
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

      {!loading && audits.length === 0 && (
        <div className="text-muted">
          <FormattedMessage id="config.tasks.ai_audit.empty" />
        </div>
      )}

      {audits.map((a) => (
        <div key={a.id} className="row mb-3 align-items-center">
          <div className="col-3">{entityPreview(a)}</div>
          <div className="col-6">
            <div>
              <strong>{FIELD_LABELS[a.field] ?? a.field}</strong>
              {a.current_value && (
                <div className="text-muted small">now: {a.current_value}</div>
              )}
              <div className="small">AI: {a.ai_value}</div>
            </div>
          </div>
          <div className="col-3">
            {a.status === "pending" && (
              <div className="d-flex flex-column align-items-start">
                <Button
                  size="sm"
                  variant="primary"
                  className="mb-2"
                  onClick={() => onApply(a.id)}
                >
                  <FormattedMessage id="actions.apply" />
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => onReject(a.id)}
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

export default AIAuditReviewDialog;
