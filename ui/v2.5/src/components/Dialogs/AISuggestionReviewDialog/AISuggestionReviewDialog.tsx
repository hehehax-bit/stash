import React, { useState, useEffect } from "react";
import { Button, Form, ListGroup } from "react-bootstrap";
import { useQuery, useMutation } from "@apollo/client";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import {
  faWandMagicSparkles,
  faCheck,
  faTimes,
} from "@fortawesome/free-solid-svg-icons";
import { Icon } from "src/components/Shared/Icon";

interface IAISuggestionReviewDialogProps {
  onClose: () => void;
}

export const AISuggestionReviewDialog: React.FC<
  IAISuggestionReviewDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();

  const [status, setStatus] = useState<string>("pending");
  const [entityType, setEntityType] = useState<string>("");

  const { data, loading, refetch } = useQuery(GQL.AiSuggestionsDocument, {
    variables: { status, entity_type: entityType || undefined },
    skip: !status,
  });

  const [applySuggestion] = useMutation(GQL.AiSuggestionApplyDocument);
  const [rejectSuggestion] = useMutation(GQL.AiSuggestionRejectDocument);

  useEffect(() => {
    refetch({ status, entity_type: entityType || undefined });
  }, [status, entityType, refetch]);

  function updateSuggestionStatus(suggestionId: string, newStatus: string) {
    return (cache: import("@apollo/client").ApolloCache<unknown>) => {
      cache.modify({
        id: cache.identify({ __typename: "AISuggestion", id: suggestionId }),
        fields: {
          status: () => newStatus,
        },
      });
    };
  }

  async function handleApply(suggestionId: string) {
    try {
      await applySuggestion({
        variables: { input: { suggestion_id: suggestionId } },
        optimisticResponse: { aiSuggestionApply: true },
        update: updateSuggestionStatus(suggestionId, "accepted"),
      });
      Toast.success(intl.formatMessage({ id: "actions.apply" }));
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function handleReject(suggestionId: string) {
    try {
      await rejectSuggestion({
        variables: { suggestion_id: suggestionId },
        optimisticResponse: { aiSuggestionReject: true },
        update: updateSuggestionStatus(suggestionId, "rejected"),
      });
      Toast.success(intl.formatMessage({ id: "actions.reject" }));
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  const suggestions = data?.aiSuggestions ?? [];

  // refresh while the dialog is open so suggestions appear during a running job
  useEffect(() => {
    const timer = setInterval(() => refetch(), 10000);
    return () => clearInterval(timer);
  }, [refetch]);

  return (
    <ModalComponent
      show
      onHide={onClose}
      icon={faWandMagicSparkles}
      header={intl.formatMessage({ id: "config.tasks.ai_suggestion.heading" })}
      dialogClassName="modal-xl"
    >
      <Form>
        <Form.Row className="mb-3">
          <Form.Group controlId="suggestion-status" className="col-md-6">
            <Form.Label>
              <FormattedMessage id="config.tasks.ai_suggestion.filter_status" />
            </Form.Label>
            <Form.Control
              as="select"
              value={status}
              onChange={(e) => setStatus(e.currentTarget.value)}
            >
              <option value="pending">Pending</option>
              <option value="accepted">Accepted</option>
              <option value="rejected">Rejected</option>
            </Form.Control>
          </Form.Group>
          <Form.Group controlId="suggestion-entity-type" className="col-md-6">
            <Form.Label>
              <FormattedMessage id="config.tasks.ai_suggestion.filter_entity_type" />
            </Form.Label>
            <Form.Control
              as="select"
              value={entityType}
              onChange={(e) => setEntityType(e.currentTarget.value)}
            >
              <option value="">All Types</option>
              <option value="scene">Scene</option>
              <option value="image">Image</option>
            </Form.Control>
          </Form.Group>
        </Form.Row>

        {loading ? (
          <div className="text-center py-4">
            <FormattedMessage id="actions.loading" />
          </div>
        ) : suggestions.length === 0 ? (
          <div className="text-center py-4 text-muted">
            <FormattedMessage id="config.tasks.ai_suggestion.no_suggestions" />
          </div>
        ) : (
          <ListGroup className="ai-suggestion-review-list">
            {suggestions.map((suggestion: GQL.AiSuggestion) => (
              <ListGroup.Item
                key={suggestion.id}
                className="d-flex flex-column flex-md-row justify-content-between align-items-md-center"
              >
                <div className="mb-2 mb-md-0">
                  <h6 className="mb-1">
                    {suggestion.entity_type}: {suggestion.title}
                  </h6>
                  <small className="text-muted">{suggestion.details}</small>
                  {suggestion.performers.length > 0 && (
                    <div className="mt-1">
                      <small>
                        <strong>Performers: </strong>
                        {suggestion.performers.join(", ")}
                      </small>
                    </div>
                  )}
                  {suggestion.tags.length > 0 && (
                    <div>
                      <small>
                        <strong>Tags: </strong>
                        {suggestion.tags.join(", ")}
                      </small>
                    </div>
                  )}
                </div>
                <div className="d-flex gap-2">
                  {suggestion.status === "pending" && (
                    <>
                      <Button
                        variant="success"
                        size="sm"
                        onClick={() => handleApply(suggestion.id)}
                      >
                        <Icon icon={faCheck} />{" "}
                        <FormattedMessage id="config.tasks.ai_suggestion.apply" />
                      </Button>
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => handleReject(suggestion.id)}
                      >
                        <Icon icon={faTimes} />{" "}
                        <FormattedMessage id="config.tasks.ai_suggestion.reject" />
                      </Button>
                    </>
                  )}
                  <span className="badge badge-secondary">
                    {suggestion.status}
                  </span>
                </div>
              </ListGroup.Item>
            ))}
          </ListGroup>
        )}
      </Form>
    </ModalComponent>
  );
};

export default AISuggestionReviewDialog;
