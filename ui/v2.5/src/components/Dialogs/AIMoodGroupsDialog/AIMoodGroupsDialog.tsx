import React from "react";
import { Button, Table } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { useToast } from "src/hooks/Toast";
import { FormattedMessage, useIntl } from "react-intl";
import { faSmileWink } from "@fortawesome/free-solid-svg-icons";

interface IAIMoodGroupsDialogProps {
  onClose: () => void;
}

export const AIMoodGroupsDialog: React.FC<IAIMoodGroupsDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();

  const { data, loading, refetch } = GQL.useAiMoodGroupsQuery();
  const [createGroup] = GQL.useAiMoodGroupCreateMutation();

  async function onCreate(mood: string) {
    try {
      await createGroup({ variables: { mood } });
      Toast.success(
        intl.formatMessage({ id: "toast.group_created" }) + `: ${mood}`
      );
      refetch();
    } catch (e) {
      Toast.error(e);
    }
  }

  const groups = data?.aiMoodGroups ?? [];

  return (
    <ModalComponent
      show
      icon={faSmileWink}
      header={intl.formatMessage({ id: "config.tasks.ai_mood_groups.heading" })}
      cancel={{
        onClick: () => onClose(),
        text: intl.formatMessage({ id: "actions.close" }),
        variant: "secondary",
      }}
    >
      {loading && <LoadingIndicator inline />}

      {!loading && groups.length === 0 && (
        <div className="text-muted">
          <FormattedMessage id="config.tasks.ai_mood_groups.empty" />
        </div>
      )}

      <Table striped size="sm">
        <tbody>
          {groups.map((g) => (
            <tr key={g.mood}>
              <td>
                <strong>{g.mood}</strong> ({g.scene_count})
              </td>
              <td className="text-right">
                <Button
                  size="sm"
                  variant="secondary"
                  onClick={() => onCreate(g.mood)}
                >
                  <FormattedMessage id="config.tasks.ai_mood_groups.create" />
                </Button>
              </td>
            </tr>
          ))}
        </tbody>
      </Table>
    </ModalComponent>
  );
};

export default AIMoodGroupsDialog;
