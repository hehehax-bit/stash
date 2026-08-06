import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAIPerformerCluster } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faUsers } from "@fortawesome/free-solid-svg-icons";
import {
  PerformerIDSelect,
  Performer,
} from "src/components/Performers/PerformerSelect";

interface IAIPerformerClusterDialogProps {
  onClose: () => void;
}

export const AIPerformerClusterDialog: React.FC<
  IAIPerformerClusterDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();

  const [performerIds, setPerformerIds] = useState<string[]>([]);
  const [entityTypes, setEntityTypes] = useState<string[]>(["scene", "image"]);
  const [maxMediaPerPerformer, setMaxMediaPerPerformer] = useState<number>(0);
  const [minConfidence, setMinConfidence] = useState<number>(0.7);
  const [timeout, setTimeout] = useState<number>(0);

  async function onCluster() {
    try {
      const input: GQL.AiPerformerClusterInput = {
        performer_ids:
          performerIds.length > 0 ? performerIds.map(Number) : undefined,
        entity_types: entityTypes.length > 0 ? entityTypes : undefined,
        max_media_per_performer:
          maxMediaPerPerformer > 0 ? maxMediaPerPerformer : undefined,
        min_confidence: minConfidence > 0 ? minConfidence : undefined,
        timeout: timeout > 0 ? timeout : undefined,
      };

      await mutateMetadataAIPerformerCluster(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "config.tasks.ai_performer_cluster.heading",
            }),
          }
        )
      );
    } catch (e) {
      Toast.error(e);
    } finally {
      onClose();
    }
  }

  return (
    <ModalComponent
      show
      icon={faUsers}
      header={intl.formatMessage({
        id: "config.tasks.ai_performer_cluster.heading",
      })}
      accept={{
        onClick: onCluster,
        text: intl.formatMessage({ id: "actions.cluster_performers" }),
      }}
      cancel={{
        onClick: () => onClose(),
        text: intl.formatMessage({ id: "actions.cancel" }),
        variant: "secondary",
      }}
    >
      <Form>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_performer_cluster.performer_ids" />
          </Form.Label>
          <PerformerIDSelect
            isMulti
            ids={performerIds.map((id) => id.toString())}
            onSelect={(items: Performer[]) =>
              setPerformerIds(items.map((i) => i.id))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_performer_cluster.performer_ids_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_performer_cluster.entity_types" />
          </Form.Label>
          <Form.Check
            type="checkbox"
            checked={entityTypes.includes("scene")}
            label="Scenes"
            onChange={(e) => {
              if (e.currentTarget.checked) {
                setEntityTypes([...entityTypes, "scene"]);
              } else {
                setEntityTypes(entityTypes.filter((t) => t !== "scene"));
              }
            }}
          />
          <Form.Check
            type="checkbox"
            checked={entityTypes.includes("image")}
            label="Images"
            onChange={(e) => {
              if (e.currentTarget.checked) {
                setEntityTypes([...entityTypes, "image"]);
              } else {
                setEntityTypes(entityTypes.filter((t) => t !== "image"));
              }
            }}
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_performer_cluster.entity_types_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_performer_cluster.max_media_per_performer" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={maxMediaPerPerformer}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMaxMediaPerPerformer(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_performer_cluster.max_media_per_performer_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_performer_cluster.min_confidence" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            max={1}
            step={0.05}
            value={minConfidence}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMinConfidence(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_performer_cluster.min_confidence_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_performer_cluster.timeout" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={timeout}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setTimeout(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_performer_cluster.timeout_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AIPerformerClusterDialog;
