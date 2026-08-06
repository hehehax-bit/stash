import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAISuggestionGenerate } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faWandMagicSparkles } from "@fortawesome/free-solid-svg-icons";

interface IAISuggestionDialogProps {
  onClose: () => void;
}

export const AISuggestionDialog: React.FC<IAISuggestionDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [entityTypes, setEntityTypes] = useState<string[]>(["scene", "image"]);
  const [maxItems, setMaxItems] = useState<number>(0);

  async function onGenerate() {
    try {
      const input: GQL.AiSuggestionInput = {
        entity_types: entityTypes.length > 0 ? entityTypes : undefined,
        max_items: maxItems > 0 ? maxItems : undefined,
      };

      await mutateMetadataAISuggestionGenerate(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "config.tasks.ai_suggestion.heading",
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
      icon={faWandMagicSparkles}
      header={intl.formatMessage({ id: "config.tasks.ai_suggestion.heading" })}
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
            <FormattedMessage id="config.tasks.ai_suggestion.entity_types" />
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
            <FormattedMessage id="config.tasks.ai_suggestion.entity_types_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_suggestion.max_items" />
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
            <FormattedMessage id="config.tasks.ai_suggestion.max_items_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AISuggestionDialog;
