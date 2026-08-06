import React, { useState, useEffect } from "react";
import { Button, Form, ListGroup } from "react-bootstrap";
import { useQuery, useMutation } from "@apollo/client";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import {
  faFileSignature,
  faCheck,
  faTimes,
} from "@fortawesome/free-solid-svg-icons";
import { Icon } from "src/components/Shared/Icon";

interface IAIFileRenameReviewDialogProps {
  onClose: () => void;
}

export const AIFileRenameReviewDialog: React.FC<
  IAIFileRenameReviewDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();

  const [status, setStatus] = useState<string>("pending");

  const { data, loading, refetch } = useQuery(GQL.AiFileRenamesDocument, {
    variables: { status },
    skip: !status,
  });

  const [applyRename] = useMutation(GQL.AiFileRenameApplyDocument);
  const [rejectRename] = useMutation(GQL.AiFileRenameRejectDocument);

  useEffect(() => {
    refetch({ status });
  }, [status, refetch]);

  function updateRenameStatus(renameId: string, newStatus: string) {
    return (cache: import("@apollo/client").ApolloCache<unknown>) => {
      cache.modify({
        id: cache.identify({ __typename: "AIFileRename", id: renameId }),
        fields: {
          status: () => newStatus,
        },
      });
    };
  }

  async function handleApply(renameId: string) {
    try {
      await applyRename({
        variables: { input: { rename_id: renameId } },
        optimisticResponse: { aiFileRenameApply: true },
        update: updateRenameStatus(renameId, "applied"),
      });
      Toast.success(
        intl.formatMessage({ id: "config.tasks.ai_file_rename.apply" })
      );
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  async function handleReject(renameId: string) {
    try {
      await rejectRename({
        variables: { rename_id: renameId },
        optimisticResponse: { aiFileRenameReject: true },
        update: updateRenameStatus(renameId, "rejected"),
      });
      Toast.success(
        intl.formatMessage({ id: "config.tasks.ai_file_rename.reject" })
      );
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  const renames = data?.aiFileRenames ?? [];

  // refresh while the dialog is open so renames appear during a running job
  useEffect(() => {
    const timer = setInterval(() => refetch(), 10000);
    return () => clearInterval(timer);
  }, [refetch]);

  return (
    <ModalComponent
      show
      onHide={onClose}
      icon={faFileSignature}
      header={intl.formatMessage({ id: "config.tasks.ai_file_rename.heading" })}
      dialogClassName="modal-xl"
    >
      <Form>
        <Form.Group className="mb-3">
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_file_rename.filter_status" />
          </Form.Label>
          <Form.Control
            as="select"
            value={status}
            onChange={(e) => setStatus(e.currentTarget.value)}
          >
            <option value="pending">Pending</option>
            <option value="applied">Applied</option>
            <option value="rejected">Rejected</option>
          </Form.Control>
        </Form.Group>

        {loading ? (
          <div className="text-center py-4">
            <FormattedMessage id="actions.loading" />
          </div>
        ) : renames.length === 0 ? (
          <div className="text-center py-4 text-muted">
            <FormattedMessage id="config.tasks.ai_file_rename.no_renames" />
          </div>
        ) : (
          <ListGroup>
            {renames.map((rename: GQL.AiFileRename) => (
              <ListGroup.Item
                key={rename.id}
                className="d-flex flex-column flex-md-row justify-content-between align-items-md-center"
              >
                <div className="mb-2 mb-md-0">
                  <h6 className="mb-1">
                    {rename.entity_type}: {rename.current_name}
                  </h6>
                  <div className="d-flex align-items-center">
                    <span className="text-muted mr-2">→</span>
                    <strong>{rename.suggested_name}</strong>
                    {rename.entity_type === "scene" && rename.scene && (
                      <span className="ml-2 text-muted small">
                        ({rename.scene.title})
                      </span>
                    )}
                    {rename.entity_type === "image" && rename.image && (
                      <span className="ml-2 text-muted small">
                        ({rename.image.paths?.image})
                      </span>
                    )}
                  </div>
                </div>
                <div className="d-flex gap-2">
                  {rename.status === "pending" && (
                    <>
                      <Button
                        variant="success"
                        size="sm"
                        onClick={() => handleApply(rename.id)}
                      >
                        <Icon icon={faCheck} />{" "}
                        <FormattedMessage id="config.tasks.ai_file_rename.apply" />
                      </Button>
                      <Button
                        variant="danger"
                        size="sm"
                        onClick={() => handleReject(rename.id)}
                      >
                        <Icon icon={faTimes} />{" "}
                        <FormattedMessage id="config.tasks.ai_file_rename.reject" />
                      </Button>
                    </>
                  )}
                  <span className="badge badge-secondary">{rename.status}</span>
                </div>
              </ListGroup.Item>
            ))}
          </ListGroup>
        )}
      </Form>
    </ModalComponent>
  );
};

export default AIFileRenameReviewDialog;
