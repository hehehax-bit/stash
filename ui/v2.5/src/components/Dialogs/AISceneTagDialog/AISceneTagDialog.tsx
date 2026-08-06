import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAISceneTag } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faMagic } from "@fortawesome/free-solid-svg-icons";

interface IAISceneTagDialogProps {
  onClose: () => void;
  selectedIds?: string[];
}

export const AISceneTagDialog: React.FC<IAISceneTagDialogProps> = ({
  onClose,
  selectedIds,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [maxScenes, setMaxScenes] = useState<number>(0);
  const [createMissingPerformers, setCreateMissingPerformers] = useState(true);
  const [createMissingTags, setCreateMissingTags] = useState(true);
  const [timeout, setTimeout] = useState<number>(0);
  const [frames, setFrames] = useState<number>(0);
  const [fillMissingOnly, setFillMissingOnly] = useState(false);

  async function onTag() {
    try {
      const input: GQL.AiSceneTagInput = {
        sceneIds:
          selectedIds && selectedIds.length > 0 ? selectedIds : undefined,
        maxScenes:
          selectedIds && selectedIds.length > 0
            ? undefined
            : maxScenes > 0
              ? maxScenes
              : undefined,
        createMissingPerformers,
        createMissingTags,
        timeout: timeout > 0 ? timeout : undefined,
        frames: frames > 0 ? frames : undefined,
        fillMissingOnly,
      };

      await mutateMetadataAISceneTag(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: "AI Scene Tag" }
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
      header={intl.formatMessage({ id: "config.tasks.ai_scene_tag.heading" })}
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
            <FormattedMessage id="config.tasks.ai_scene_tag.max_scenes" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={maxScenes}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMaxScenes(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_scene_tag.max_scenes_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-scene-create-missing-performers"
          checked={createMissingPerformers}
          label={intl.formatMessage({
            id: "config.tasks.ai_scene_tag.create_missing_performers",
          })}
          onChange={() => setCreateMissingPerformers(!createMissingPerformers)}
        />
        <Form.Check
          id="ai-scene-fill-missing-only"
          checked={fillMissingOnly}
          label={intl.formatMessage({
            id: "config.tasks.ai_scene_tag.fill_missing_only",
          })}
          onChange={() => setFillMissingOnly(!fillMissingOnly)}
        />
        <Form.Check
          id="ai-scene-create-missing-tags"
          checked={createMissingTags}
          label={intl.formatMessage({
            id: "config.tasks.ai_scene_tag.create_missing_tags",
          })}
          onChange={() => setCreateMissingTags(!createMissingTags)}
        />
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_scene_tag.frames" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={frames}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setFrames(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_scene_tag.frames_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_scene_tag.timeout" />
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
            <FormattedMessage id="config.tasks.ai_scene_tag.timeout_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AISceneTagDialog;
