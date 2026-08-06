import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAITagOrganize } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faMagic } from "@fortawesome/free-solid-svg-icons";

interface IAITagOrganizeDialogProps {
  onClose: () => void;
}

export const AITagOrganizeDialog: React.FC<IAITagOrganizeDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [maxTags, setMaxTags] = useState<number>(0);
  const [allowMerge, setAllowMerge] = useState(true);
  const [timeout, setTimeout] = useState<number>(0);

  async function onOrganize() {
    try {
      const input: GQL.AiTagOrganizeInput = {
        maxTags: maxTags > 0 ? maxTags : undefined,
        allowMerge,
        timeout: timeout > 0 ? timeout : undefined,
      };

      await mutateMetadataAITagOrganize(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: intl.formatMessage({ id: "actions.organize" }) }
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
        id: "config.tasks.ai_tag_organize.heading",
      })}
      accept={{
        onClick: onOrganize,
        text: intl.formatMessage({ id: "actions.organize" }),
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
            <FormattedMessage id="config.tasks.ai_tag_organize.max_tags" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={maxTags}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMaxTags(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_tag_organize.max_tags_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-tag-organize-allow-merge"
          checked={allowMerge}
          label={intl.formatMessage({
            id: "config.tasks.ai_tag_organize.allow_merge",
          })}
          onChange={() => setAllowMerge(!allowMerge)}
        />
        <Form.Text className="text-muted">
          <FormattedMessage id="config.tasks.ai_tag_organize.allow_merge_desc" />
        </Form.Text>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_tag_organize.timeout" />
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
            <FormattedMessage id="config.tasks.ai_tag_organize.timeout_desc" />
          </Form.Text>
        </Form.Group>
        <hr />
        <p className="text-muted">
          <FormattedMessage id="config.tasks.ai_tag_organize.warning" />
        </p>
      </Form>
    </ModalComponent>
  );
};

export default AITagOrganizeDialog;
