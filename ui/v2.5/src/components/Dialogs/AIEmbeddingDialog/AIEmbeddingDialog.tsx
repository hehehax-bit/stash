import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAIEmbedding } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faDatabase } from "@fortawesome/free-solid-svg-icons";

interface IAIEmbeddingDialogProps {
  onClose: () => void;
}

const visualEntityTypes = ["scene", "performer", "image", "gallery"];

export const AIEmbeddingDialog: React.FC<IAIEmbeddingDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [entityTypes, setEntityTypes] = useState<string[]>([
    "scene",
    "performer",
    "image",
    "gallery",
    "studio",
    "tag",
  ]);
  const [overwrite, setOverwrite] = useState(false);
  const [visual, setVisual] = useState(false);
  const [staleOnly, setStaleOnly] = useState(false);

  const entityTypeOptions = [
    { value: "scene", label: intl.formatMessage({ id: "entities.scene" }) },
    {
      value: "performer",
      label: intl.formatMessage({ id: "entities.performer" }),
    },
    { value: "image", label: intl.formatMessage({ id: "entities.image" }) },
    { value: "gallery", label: intl.formatMessage({ id: "entities.gallery" }) },
    { value: "studio", label: intl.formatMessage({ id: "entities.studio" }) },
    { value: "tag", label: intl.formatMessage({ id: "entities.tag" }) },
  ];

  async function onGenerate() {
    try {
      const input: GQL.AiEmbeddingInput = {
        entityTypes: entityTypes.length > 0 ? entityTypes : undefined,
        overwrite: overwrite || undefined,
        visual: visual || undefined,
        stale_only: staleOnly || undefined,
      };

      await mutateMetadataAIEmbedding(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "actions.generate_embeddings",
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

  function handleEntityTypeChange(value: string) {
    if (visual && !visualEntityTypes.includes(value)) {
      return;
    }
    if (entityTypes.includes(value)) {
      setEntityTypes(entityTypes.filter((v) => v !== value));
    } else {
      setEntityTypes([...entityTypes, value]);
    }
  }

  function handleVisualChange(v: boolean) {
    setVisual(v);
    if (v) {
      setEntityTypes(
        entityTypes.filter((et) => visualEntityTypes.includes(et))
      );
    }
  }

  return (
    <ModalComponent
      show
      icon={faDatabase}
      header={intl.formatMessage({
        id: "config.tasks.ai_embedding.heading",
      })}
      accept={{
        onClick: onGenerate,
        text: intl.formatMessage({ id: "actions.generate_embeddings" }),
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
            <FormattedMessage id="config.tasks.ai_embedding.mode" />
          </Form.Label>
          <Form.Check
            id="ai-embedding-mode-text"
            type="radio"
            name="ai-embedding-mode"
            label={intl.formatMessage({
              id: "config.tasks.ai_embedding.mode_text",
            })}
            checked={!visual}
            onChange={() => handleVisualChange(false)}
          />
          <Form.Check
            id="ai-embedding-mode-visual"
            type="radio"
            name="ai-embedding-mode"
            label={intl.formatMessage({
              id: "config.tasks.ai_embedding.mode_visual",
            })}
            checked={visual}
            onChange={() => handleVisualChange(true)}
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_embedding.mode_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-embedding-stale-only"
          checked={staleOnly}
          label={intl.formatMessage({
            id: "config.tasks.ai_embedding.stale_only",
          })}
          onChange={() => setStaleOnly(!staleOnly)}
        />
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_embedding.entity_types" />
          </Form.Label>
          {entityTypeOptions.map((option) => {
            const disabled =
              visual && !visualEntityTypes.includes(option.value);
            return (
              <Form.Check
                key={option.value}
                id={`ai-embedding-${option.value}`}
                type="checkbox"
                label={option.label}
                checked={entityTypes.includes(option.value)}
                disabled={disabled}
                onChange={() => handleEntityTypeChange(option.value)}
              />
            );
          })}
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_embedding.entity_types_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-embedding-overwrite"
          checked={overwrite}
          label={intl.formatMessage({
            id: "config.tasks.ai_embedding.overwrite",
          })}
          onChange={() => setOverwrite(!overwrite)}
        />
        <Form.Text className="text-muted">
          <FormattedMessage id="config.tasks.ai_embedding.overwrite_desc" />
        </Form.Text>
        <hr />
        <p className="text-muted">
          <FormattedMessage id="config.tasks.ai_embedding.warning" />
        </p>
      </Form>
    </ModalComponent>
  );
};

export default AIEmbeddingDialog;
