import React, { useEffect, useState } from "react";
import { Button, Form } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { PerformerCard } from "src/components/Performers/PerformerCard";
import { useToast } from "src/hooks/Toast";
import { FormattedMessage, useIntl } from "react-intl";
import { faPeopleArrows } from "@fortawesome/free-solid-svg-icons";

interface IAIPerformerMergeSuggestDialogProps {
  onClose: () => void;
}

export const AIPerformerMergeSuggestDialog: React.FC<
  IAIPerformerMergeSuggestDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();

  const [status, setStatus] = useState<string>("pending");

  const { data, loading, refetch } = GQL.useAiPerformerMergeSuggestionsQuery({
    variables: { status },
  });
  const [applySuggestion] = GQL.useAiPerformerSuggestionApplyMutation();
  const [rejectSuggestion] = GQL.useAiPerformerSuggestionRejectMutation();
  const [applyAll] = GQL.useAiPerformerSuggestionApplyAllMutation();
  const [rejectAll] = GQL.useAiPerformerSuggestionRejectAllMutation();

  const suggestions = data?.aiPerformerMergeSuggestions ?? [];

  // refresh while the dialog is open so suggestions appear during a running job
  useEffect(() => {
    const timer = setInterval(() => refetch(), 10000);
    return () => clearInterval(timer);
  }, [refetch]);

  async function onApply(id: string) {
    try {
      await applySuggestion({
        variables: { suggestion_id: id },
      });
      Toast.success(intl.formatMessage({ id: "toast.merged_entities" }));
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onReject(id: string) {
    try {
      await rejectSuggestion({
        variables: { suggestion_id: id },
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
          `: ${result.data?.aiPerformerSuggestionApplyAll ?? 0}`
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

  return (
    <ModalComponent
      show
      onHide={onClose}
      icon={faPeopleArrows}
      header={intl.formatMessage({
        id: "config.tasks.ai_performer_merge_suggest.heading",
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

      {!loading && suggestions.length === 0 && (
        <div className="text-muted">
          <FormattedMessage id="config.tasks.ai_performer_merge_suggest.empty" />
        </div>
      )}

      {suggestions.map((s) => (
        <div key={s.id} className="row mb-4 align-items-center">
          <div className="col-4">
            {s.source && <PerformerCard performer={s.source} />}
          </div>
          <div className="col-4 text-center">
            <div className="mb-1">
              <strong>{Math.round((s.confidence ?? 0) * 100)}%</strong>
            </div>
            {s.status === "pending" && (
              <div className="d-flex flex-column align-items-center">
                <Button
                  size="sm"
                  variant="primary"
                  className="mb-2"
                  onClick={() => onApply(s.id)}
                >
                  <FormattedMessage id="actions.merge" />
                </Button>
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => onReject(s.id)}
                >
                  <FormattedMessage id="actions.reject" />
                </Button>
              </div>
            )}
          </div>
          <div className="col-4">
            {s.target && <PerformerCard performer={s.target} />}
          </div>
        </div>
      ))}
    </ModalComponent>
  );
};

export default AIPerformerMergeSuggestDialog;
