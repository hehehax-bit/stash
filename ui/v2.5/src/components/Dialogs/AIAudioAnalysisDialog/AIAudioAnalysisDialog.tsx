import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAIAudioAnalyze } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faWaveSquare } from "@fortawesome/free-solid-svg-icons";
import { SceneIDSelect, Scene } from "src/components/Scenes/SceneSelect";

interface IAIAudioAnalysisDialogProps {
  onClose: () => void;
  selectedIds?: string[];
}

export const AIAudioAnalysisDialog: React.FC<IAIAudioAnalysisDialogProps> = ({
  onClose,
  selectedIds,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [sceneIds, setSceneIds] = useState<string[]>(selectedIds ?? []);
  const [maxScenes, setMaxScenes] = useState<number>(0);
  const [overwrite, setOverwrite] = useState<boolean>(false);
  const [timeout, setTimeout] = useState<number>(0);

  async function onAnalyze() {
    try {
      const input: GQL.AiAudioAnalyzeInput = {
        scene_ids: sceneIds.length > 0 ? sceneIds.map(Number) : undefined,
        max_scenes: maxScenes > 0 ? maxScenes : undefined,
        overwrite,
        timeout: timeout > 0 ? timeout : undefined,
      };

      await mutateMetadataAIAudioAnalyze(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "config.tasks.ai_audio_analysis.heading",
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
      icon={faWaveSquare}
      header={intl.formatMessage({
        id: "config.tasks.ai_audio_analysis.heading",
      })}
      accept={{
        onClick: onAnalyze,
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
            <FormattedMessage id="config.tasks.ai_audio_analysis.scene_ids" />
          </Form.Label>
          <SceneIDSelect
            isMulti
            ids={sceneIds}
            onSelect={(items: Scene[]) => setSceneIds(items.map((i) => i.id))}
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_audio_analysis.scene_ids_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_audio_analysis.max_scenes" />
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
            <FormattedMessage id="config.tasks.ai_audio_analysis.max_scenes_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-audio-analysis-overwrite"
          checked={overwrite}
          label={intl.formatMessage({
            id: "config.tasks.ai_audio_analysis.overwrite",
          })}
          onChange={() => setOverwrite(!overwrite)}
        />
        <Form.Text className="text-muted">
          <FormattedMessage id="config.tasks.ai_audio_analysis.overwrite_desc" />
        </Form.Text>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_audio_analysis.timeout" />
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
            <FormattedMessage id="config.tasks.ai_audio_analysis.timeout_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AIAudioAnalysisDialog;
