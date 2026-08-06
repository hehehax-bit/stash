import React, { useState } from "react";
import { Form } from "react-bootstrap";
import { mutateMetadataDetectLooping } from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faInfinity } from "@fortawesome/free-solid-svg-icons";

interface IDetectLoopingDialogProps {
  onClose: () => void;
}

export const DetectLoopingDialog: React.FC<IDetectLoopingDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const [maxScenes, setMaxScenes] = useState<number>(0);

  async function onDetect() {
    try {
      const input: GQL.DetectLoopingInput = {
        maxScenes: maxScenes > 0 ? maxScenes : undefined,
      };

      await mutateMetadataDetectLooping(input);

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: "Detect Looping Videos" }
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
      icon={faInfinity}
      header={intl.formatMessage({
        id: "config.tasks.detect_looping.heading",
      })}
      accept={{
        onClick: onDetect,
        text: intl.formatMessage({ id: "actions.detect" }),
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
            <FormattedMessage id="config.tasks.detect_looping.max_scenes" />
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
            <FormattedMessage id="config.tasks.detect_looping.max_scenes_desc" />
          </Form.Text>
        </Form.Group>
      </Form>
    </ModalComponent>
  );
};

export default DetectLoopingDialog;
