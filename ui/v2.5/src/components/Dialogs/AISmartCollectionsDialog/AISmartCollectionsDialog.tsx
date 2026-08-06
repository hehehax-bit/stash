import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAISmartCollections } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faMagic } from "@fortawesome/free-solid-svg-icons";

interface IAISmartCollectionsDialogProps {
  onClose: () => void;
}

const outputTypeOptions = [
  { value: "GROUPS", label: "Groups (Scenes)" },
  { value: "GALLERIES", label: "Galleries (Images)" },
  { value: "GROUPS_AND_GALLERIES", label: "Groups & Galleries" },
];

export const AISmartCollectionsDialog: React.FC<
  IAISmartCollectionsDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();

  const [maxScenes, setMaxScenes] = useState<number>(0);
  const [maxImages, setMaxImages] = useState<number>(0);
  const [maxCollections, setMaxCollections] = useState<number>(0);
  const [overwrite, setOverwrite] = useState<boolean>(false);
  const [outputType, setOutputType] =
    useState<GQL.AiSmartCollectionsOutputType>(
      GQL.AiSmartCollectionsOutputType.GroupsAndGalleries
    );
  const [timeout, setTimeout] = useState<number>(0);

  async function onGenerate() {
    try {
      const input: GQL.AiSmartCollectionsInput = {
        max_scenes: maxScenes > 0 ? maxScenes : undefined,
        max_images: maxImages > 0 ? maxImages : undefined,
        max_collections: maxCollections > 0 ? maxCollections : undefined,
        overwrite,
        output_type: outputType,
        timeout: timeout > 0 ? timeout : undefined,
      };

      await mutateMetadataAISmartCollections(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "config.tasks.ai_smart_collections.heading",
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
      icon={faMagic}
      header={intl.formatMessage({
        id: "config.tasks.ai_smart_collections.heading",
      })}
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
            <FormattedMessage id="config.tasks.ai_smart_collections.max_scenes" />
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
            <FormattedMessage id="config.tasks.ai_smart_collections.max_scenes_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_smart_collections.max_images" />
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
            <FormattedMessage id="config.tasks.ai_smart_collections.max_images_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_smart_collections.max_collections" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={maxCollections}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMaxCollections(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_smart_collections.max_collections_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-smart-collections-overwrite"
          checked={overwrite}
          label={intl.formatMessage({
            id: "config.tasks.ai_smart_collections.overwrite",
          })}
          onChange={() => setOverwrite(!overwrite)}
        />
        <Form.Text className="text-muted">
          <FormattedMessage id="config.tasks.ai_smart_collections.overwrite_desc" />
        </Form.Text>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_smart_collections.output_type" />
          </Form.Label>
          <Form.Control
            as="select"
            value={outputType}
            onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
              setOutputType(
                e.currentTarget.value as GQL.AiSmartCollectionsOutputType
              )
            }
          >
            {outputTypeOptions.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </Form.Control>
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_smart_collections.output_type_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_smart_collections.timeout" />
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
            <FormattedMessage id="config.tasks.ai_smart_collections.timeout_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AISmartCollectionsDialog;
