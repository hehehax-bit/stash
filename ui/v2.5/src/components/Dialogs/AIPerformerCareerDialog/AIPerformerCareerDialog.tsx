import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataAIPerformerCareer } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faBriefcase } from "@fortawesome/free-solid-svg-icons";
import {
  PerformerIDSelect,
  Performer,
} from "src/components/Performers/PerformerSelect";

interface IAIPerformerCareerDialogProps {
  onClose: () => void;
}

export const AIPerformerCareerDialog: React.FC<
  IAIPerformerCareerDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();

  const [performerIds, setPerformerIds] = useState<string[]>([]);
  const [maxPerformers, setMaxPerformers] = useState<number>(0);
  const [overwrite, setOverwrite] = useState<boolean>(false);
  const [timeout, setTimeout] = useState<number>(0);

  async function onAnalyze() {
    try {
      const input: GQL.AiPerformerCareerInput = {
        performer_ids:
          performerIds.length > 0 ? performerIds.map(Number) : undefined,
        max_performers: maxPerformers > 0 ? maxPerformers : undefined,
        overwrite,
        timeout: timeout > 0 ? timeout : undefined,
      };

      await mutateMetadataAIPerformerCareer(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          {
            operation_name: intl.formatMessage({
              id: "config.tasks.ai_performer_career.heading",
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
      icon={faBriefcase}
      header={intl.formatMessage({
        id: "config.tasks.ai_performer_career.heading",
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
            <FormattedMessage id="config.tasks.ai_performer_career.performer_ids" />
          </Form.Label>
          <PerformerIDSelect
            isMulti
            ids={performerIds.map((id) => id.toString())}
            onSelect={(items: Performer[]) =>
              setPerformerIds(items.map((i) => i.id))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_performer_career.performer_ids_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_performer_career.max_performers" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            value={maxPerformers}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMaxPerformers(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.ai_performer_career.max_performers_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Check
          id="ai-performer-career-overwrite"
          checked={overwrite}
          label={intl.formatMessage({
            id: "config.tasks.ai_performer_career.overwrite",
          })}
          onChange={() => setOverwrite(!overwrite)}
        />
        <Form.Text className="text-muted">
          <FormattedMessage id="config.tasks.ai_performer_career.overwrite_desc" />
        </Form.Text>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_performer_career.timeout" />
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
            <FormattedMessage id="config.tasks.ai_performer_career.timeout_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default AIPerformerCareerDialog;
