import React, { useState } from "react";
import { Button, Form } from "react-bootstrap";
import { useHistory } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { PerformerIDSelect } from "src/components/Performers/PerformerSelect";
import { useToast } from "src/hooks/Toast";
import { mutateMetadataGenerateGoonReel } from "src/core/StashService";
import { FormattedMessage, useIntl } from "react-intl";
import { faCalendarAlt } from "@fortawesome/free-solid-svg-icons";

interface IAISessionBuildDialogProps {
  onClose: () => void;
}

export const AISessionBuildDialog: React.FC<IAISessionBuildDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const history = useHistory();

  const [duration, setDuration] = useState(30);
  const [minSteam, setMinSteam] = useState(6);
  const [vibe, setVibe] = useState("");
  const [performerIds, setPerformerIds] = useState<string[]>([]);
  const [moods, setMoods] = useState<string[]>([]);
  const [plan, setPlan] = useState<
    GQL.AiSessionBuildQuery["aiSessionBuild"] | null
  >(null);

  const { data: moodGroupsData } = GQL.useAiMoodGroupsQuery();
  const [buildSession, { loading }] = GQL.useAiSessionBuildLazyQuery();

  async function onBuild() {
    try {
      const result = await buildSession({
        variables: {
          input: {
            duration_minutes: duration,
            performer_ids: performerIds.length > 0 ? performerIds : undefined,
            moods: moods.length > 0 ? moods : undefined,
            min_steam: minSteam,
            vibe: vibe.trim() !== "" ? vibe : undefined,
          },
        },
      });
      setPlan(result.data?.aiSessionBuild ?? null);
      if (!result.data?.aiSessionBuild?.scenes?.length) {
        Toast.toast({
          content: intl.formatMessage({ id: "config.tasks.ai_session.empty" }),
          variant: "warning",
        });
      }
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onGenerateReel() {
    if (!plan || plan.scenes.length === 0) return;
    try {
      const result = await mutateMetadataGenerateGoonReel(
        plan.scenes.map((s) => s.scene_id)
      );
      Toast.success(
        intl.formatMessage({ id: "toast.reel_generated" }) +
          `: ${result.data?.metadataGenerateGoonReel ?? ""}`
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  function startSession() {
    if (!plan || plan.scenes.length === 0) return;

    const params = plan.scenes
      .map((s) => `qs=${s.scene_id}`)
      .concat("afterglow=1")
      .join("&");
    const first = plan.scenes[0];
    history.push(`/scenes/${first.scene_id}?${params}`);
    onClose();
  }

  return (
    <ModalComponent
      show
      icon={faCalendarAlt}
      header={intl.formatMessage({ id: "config.tasks.ai_session.heading" })}
      cancel={{
        onClick: () => onClose(),
        text: intl.formatMessage({ id: "actions.close" }),
        variant: "secondary",
      }}
    >
      <Form>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_session.duration" />:{" "}
            {duration} min
          </Form.Label>
          <Form.Control
            type="range"
            min={10}
            max={120}
            step={5}
            value={duration}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setDuration(Number(e.currentTarget.value))
            }
          />
        </Form.Group>

        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_session.min_steam" />:{" "}
            {minSteam}
          </Form.Label>
          <Form.Control
            type="range"
            min={1}
            max={10}
            step={1}
            value={minSteam}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMinSteam(Number(e.currentTarget.value))
            }
          />
        </Form.Group>

        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_session.performers" />
          </Form.Label>
          <PerformerIDSelect
            isMulti
            ids={performerIds}
            onSelect={(items) => setPerformerIds(items.map((p) => p.id))}
          />
        </Form.Group>

        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_session.moods" />
          </Form.Label>
          <div>
            {(moodGroupsData?.aiMoodGroups ?? []).map((g) => (
              <Form.Check
                key={g.mood}
                inline
                type="checkbox"
                label={g.mood}
                checked={moods.includes(g.mood)}
                onChange={() => {
                  setMoods((prev) =>
                    prev.includes(g.mood)
                      ? prev.filter((m) => m !== g.mood)
                      : [...prev, g.mood]
                  );
                }}
              />
            ))}
          </div>
        </Form.Group>

        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.ai_session.vibe" />
          </Form.Label>
          <Form.Control
            type="text"
            value={vibe}
            placeholder={intl.formatMessage({
              id: "config.tasks.ai_session.vibe_placeholder",
            })}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setVibe(e.currentTarget.value)
            }
          />
        </Form.Group>

        <Button variant="primary" onClick={() => onBuild()} disabled={loading}>
          {loading ? (
            <LoadingIndicator inline />
          ) : (
            <FormattedMessage id="config.tasks.ai_session.build" />
          )}
        </Button>
      </Form>

      {plan && plan.scenes.length > 0 && (
        <div className="mt-3">
          <h6>
            <FormattedMessage id="config.tasks.ai_session.plan" />:{" "}
            {Math.round(plan.total_minutes)} min · {plan.scenes.length} scenes
          </h6>
          <ul className="ai-session-preview">
            {plan.scenes.map((s) => (
              <li key={s.scene_id}>
                {s.title || s.scene_id} · 🔥 {s.steam} ·{" "}
                {Math.round(s.duration)}s
                {s.best_moment > 0 && ` · best @ ${Math.round(s.best_moment)}s`}
              </li>
            ))}
          </ul>
          <Button variant="danger" onClick={() => startSession()}>
            <FormattedMessage id="config.tasks.ai_session.start" />
          </Button>{" "}
          <Button variant="outline-danger" onClick={() => onGenerateReel()}>
            <FormattedMessage id="config.tasks.ai_session.reel" />
          </Button>
        </div>
      )}
    </ModalComponent>
  );
};

export default AISessionBuildDialog;
