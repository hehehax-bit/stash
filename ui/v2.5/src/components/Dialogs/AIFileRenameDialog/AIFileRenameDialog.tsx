import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAIFileRename } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faFileSignature } from "@fortawesome/free-solid-svg-icons";
import { SceneIDSelect, Scene } from "src/components/Scenes/SceneSelect";
import { ImageIDSelect, Image } from "src/components/Images/ImageSelect";

interface IAIFileRenameDialogProps {
  onClose: () => void;
}

export const AIFileRenameDialog: React.FC<IAIFileRenameDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [sceneIds, setSceneIds] = useState<string[]>([]);
  const [imageIds, setImageIds] = useState<string[]>([]);
  const [maxItems, setMaxItems] = useState<number>(0);
  const [overwrite, setOverwrite] = useState<boolean>(false);
  const [timeout, setTimeout] = useState<number>(0);

  async function onGenerate() {
    try {
      const input: GQL.AiFileRenameInput = {
        scene_ids: sceneIds.length > 0 ? sceneIds.map(Number) : undefined,
        image_ids: imageIds.length > 0 ? imageIds.map(Number) : undefined,
        max_items: maxItems > 0 ? maxItems : undefined,
        overwrite,
        timeout: timeout > 0 ? timeout : undefined,
      };

      await mutateMetadataAIFileRename(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "config.tasks.ai_file_rename.heading",
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
      icon={faFileSignature}
      header={intl.formatMessage({ id: "config.tasks.ai_file_rename.heading" })}
      accept={{
        onClick: onGenerate,
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
            <FormattedMessage id="config.tasks.ai_file_rename.scene_ids" />
          </Form.Label>
          <SceneIDSelect
            isMulti
            ids={sceneIds}
            onSelect={(items: Scene[]) => setSceneIds(items.map((i) => i.id))}
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_file_rename.scene_ids_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_file_rename.image_ids" />
          </Form.Label>
          <ImageIDSelect
            isMulti
            ids={imageIds}
            onSelect={(items: Image[]) => setImageIds(items.map((i) => i.id))}
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_file_rename.image_ids_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_file_rename.max_items" />
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
            <FormattedMessage id="config.tasks.ai_file_rename.max_items_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-file-rename-overwrite"
          checked={overwrite}
          label={intl.formatMessage({
            id: "config.tasks.ai_file_rename.overwrite",
          })}
          onChange={() => setOverwrite(!overwrite)}
        />
        <Form.Text className="text-muted">
          <FormattedMessage id="config.tasks.ai_file_rename.overwrite_desc" />
        </Form.Text>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_file_rename.timeout" />
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
            <FormattedMessage id="config.tasks.ai_file_rename.timeout_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AIFileRenameDialog;
