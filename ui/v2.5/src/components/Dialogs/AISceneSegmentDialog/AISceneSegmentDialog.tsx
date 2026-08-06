import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAISceneSegment } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faCut } from "@fortawesome/free-solid-svg-icons";
import { SceneIDSelect, Scene } from "src/components/Scenes/SceneSelect";

interface IAISceneSegmentDialogProps {
  onClose: () => void;
}

export const AISceneSegmentDialog: React.FC<IAISceneSegmentDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [sceneIds, setSceneIds] = useState<string[]>([]);
  const [maxScenes, setMaxScenes] = useState<number>(0);
  const [overwrite, setOverwrite] = useState<boolean>(false);
  const [timeout, setTimeout] = useState<number>(0);
  const [frames, setFrames] = useState<number>(0);

  async function onSegment() {
    try {
      const input: GQL.AiSceneSegmentInput = {
        scene_ids: sceneIds.length > 0 ? sceneIds.map(Number) : undefined,
        max_scenes: maxScenes > 0 ? maxScenes : undefined,
        overwrite,
        timeout: timeout > 0 ? timeout : undefined,
        frames: frames > 0 ? frames : undefined,
      };

      await mutateMetadataAISceneSegment(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "config.tasks.ai_scene_segment.heading",
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
      icon={faCut}
      header={intl.formatMessage({
        id: "config.tasks.ai_scene_segment.heading",
      })}
      accept={{
        onClick: onSegment,
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
            <FormattedMessage id="config.tasks.ai_scene_segment.scene_ids" />
          </Form.Label>
          <SceneIDSelect
            isMulti
            ids={sceneIds}
            onSelect={(items: Scene[]) => setSceneIds(items.map((i) => i.id))}
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_scene_segment.scene_ids_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_scene_segment.max_scenes" />
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
            <FormattedMessage id="config.tasks.ai_scene_segment.max_scenes_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-scene-segment-overwrite"
          checked={overwrite}
          label={intl.formatMessage({
            id: "config.tasks.ai_scene_segment.overwrite",
          })}
          onChange={() => setOverwrite(!overwrite)}
        />
        <Form.Text className="text-muted">
          <FormattedMessage id="config.tasks.ai_scene_segment.overwrite_desc" />
        </Form.Text>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_scene_segment.frames" />
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
            <FormattedMessage id="config.tasks.ai_scene_segment.frames_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_scene_segment.timeout" />
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
            <FormattedMessage id="config.tasks.ai_scene_segment.timeout_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AISceneSegmentDialog;
