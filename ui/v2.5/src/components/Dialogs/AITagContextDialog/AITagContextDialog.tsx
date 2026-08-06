import React, { useState } from "react";
import { Form } from "react-bootstrap";
import {
  mutateMetadataAIImageTag,
  mutateMetadataAISceneTag,
} from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import { FormattedMessage, useIntl } from "react-intl";
import { faMagic } from "@fortawesome/free-solid-svg-icons";

interface IAITagContextDialogProps {
  entityType: "scene" | "image";
  entityId: string;
  onClose: () => void;
}

export const AITagContextDialog: React.FC<IAITagContextDialogProps> = ({
  entityType,
  entityId,
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [context, setContext] = useState("");

  const isScene = entityType === "scene";
  const operationName = isScene ? "AI Scene Tag" : "AI Image Tag";

  async function onTag() {
    try {
      if (isScene) {
        await mutateMetadataAISceneTag({
          sceneIds: [entityId],
          createMissingPerformers: true,
          createMissingTags: true,
          context: context || undefined,
        });
      } else {
        await mutateMetadataAIImageTag({
          imageIds: [entityId],
          createMissingPerformers: true,
          createMissingTags: true,
          context: context || undefined,
        });
      }

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: operationName }
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
      icon={faMagic}
      header={intl.formatMessage({
        id: isScene
          ? "config.tasks.ai_scene_tag.heading"
          : "config.tasks.ai_image_tag.heading",
      })}
      accept={{
        onClick: onTag,
        text: intl.formatMessage({ id: "actions.tag" }),
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
            <FormattedMessage
              id={
                isScene
                  ? "config.tasks.ai_scene_tag.context"
                  : "config.tasks.ai_image_tag.context"
              }
            />
          </Form.Label>
          <Form.Control
            as="textarea"
            rows={4}
            value={context}
            placeholder={intl.formatMessage({
              id: isScene
                ? "config.tasks.ai_scene_tag.context_placeholder"
                : "config.tasks.ai_image_tag.context_placeholder",
            })}
            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
              setContext(e.currentTarget.value)
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage
              id={
                isScene
                  ? "config.tasks.ai_scene_tag.context_desc"
                  : "config.tasks.ai_image_tag.context_desc"
              }
            />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AITagContextDialog;
