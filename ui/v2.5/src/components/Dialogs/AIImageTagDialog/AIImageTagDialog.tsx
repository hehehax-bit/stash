import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAIImageTag } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faMagic } from "@fortawesome/free-solid-svg-icons";

interface IAIImageTagDialogProps {
  onClose: () => void;
  selectedIds?: string[];
}

export const AIImageTagDialog: React.FC<IAIImageTagDialogProps> = ({
  onClose,
  selectedIds,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [maxImages, setMaxImages] = useState<number>(0);
  const [createMissingPerformers, setCreateMissingPerformers] = useState(true);
  const [createMissingTags, setCreateMissingTags] = useState(true);
  const [fillMissingOnly, setFillMissingOnly] = useState(false);

  async function onTag() {
    try {
      const input: GQL.AiImageTagInput = {
        imageIds:
          selectedIds && selectedIds.length > 0 ? selectedIds : undefined,
        maxImages:
          selectedIds && selectedIds.length > 0
            ? undefined
            : maxImages > 0
              ? maxImages
              : undefined,
        createMissingPerformers,
        createMissingTags,
        fillMissingOnly,
      };

      await mutateMetadataAIImageTag(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: "AI Image Tag" }
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
      header={intl.formatMessage({ id: "config.tasks.ai_image_tag.heading" })}
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
            <FormattedMessage id="config.tasks.ai_image_tag.max_images" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={maxImages}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMaxImages(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_image_tag.max_images_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-create-missing-performers"
          checked={createMissingPerformers}
          label={intl.formatMessage({
            id: "config.tasks.ai_image_tag.create_missing_performers",
          })}
          onChange={() => setCreateMissingPerformers(!createMissingPerformers)}
        />
        <Form.Check
          id="ai-image-fill-missing-only"
          checked={fillMissingOnly}
          label={intl.formatMessage({
            id: "config.tasks.ai_image_tag.fill_missing_only",
          })}
          onChange={() => setFillMissingOnly(!fillMissingOnly)}
        />
        <Form.Check
          id="ai-create-missing-tags"
          checked={createMissingTags}
          label={intl.formatMessage({
            id: "config.tasks.ai_image_tag.create_missing_tags",
          })}
          onChange={() => setCreateMissingTags(!createMissingTags)}
        />
      </Form>
    </ModalComponent>
  );
};

export default AIImageTagDialog;
