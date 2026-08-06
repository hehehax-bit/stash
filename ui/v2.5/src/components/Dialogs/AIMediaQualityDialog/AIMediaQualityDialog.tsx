import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAIMediaQuality } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faStar } from "@fortawesome/free-solid-svg-icons";

interface IAIMediaQualityDialogProps {
  onClose: () => void;
}

export const AIMediaQualityDialog: React.FC<IAIMediaQualityDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [entityTypes, setEntityTypes] = useState<string[]>(["scene", "image"]);
  const [maxItems, setMaxItems] = useState<number>(0);
  const [overwrite, setOverwrite] = useState<boolean>(false);
  const [timeout, setTimeout] = useState<number>(0);

  async function onAssess() {
    try {
      const input: GQL.AiMediaQualityInput = {
        entity_types: entityTypes.length > 0 ? entityTypes : undefined,
        max_items: maxItems > 0 ? maxItems : undefined,
        overwrite,
        timeout: timeout > 0 ? timeout : undefined,
      };

      await mutateMetadataAIMediaQuality(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "config.tasks.ai_media_quality.heading",
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
      icon={faStar}
      header={intl.formatMessage({
        id: "config.tasks.ai_media_quality.heading",
      })}
      accept={{
        onClick: onAssess,
        text: intl.formatMessage({ id: "actions.generate" }),
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
            <FormattedMessage id="config.tasks.ai_media_quality.entity_types" />
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
            <FormattedMessage id="config.tasks.ai_media_quality.entity_types_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_media_quality.max_items" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={maxItems}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMaxItems(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_media_quality.max_items_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-media-quality-overwrite"
          checked={overwrite}
          label={intl.formatMessage({
            id: "config.tasks.ai_media_quality.overwrite",
          })}
          onChange={() => setOverwrite(!overwrite)}
        />
        <Form.Text className="text-muted">
          <FormattedMessage id="config.tasks.ai_media_quality.overwrite_desc" />
        </Form.Text>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_media_quality.timeout" />
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
            <FormattedMessage id="config.tasks.ai_media_quality.timeout_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AIMediaQualityDialog;
