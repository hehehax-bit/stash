import React, { useState } from "react";
import { Button, Form } from "react-bootstrap";
import { useHistory } from "react-router-dom";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { Icon } from "src/components/Shared/Icon";
import { PerformerIDSelect } from "src/components/Performers/PerformerSelect";
import { useToast } from "src/hooks/Toast";
import {
  mutateMetadataGenerateGoonReel,
  mutateAiSavePlan,
  mutateAiDeletePlan,
} from "src/core/StashService";
import { FormattedMessage, useIntl } from "react-intl";
import {
  faCalendarAlt,
  faWandMagicSparkles,
} from "@fortawesome/free-solid-svg-icons";

interface IAISessionBuildDialogProps {
  onClose: () => void;
  initial?: {
    durationMinutes?: number;
    minSteam?: number;
    ordering?: string;
  };
  onStart?: () => void;
}

export const AISessionBuildDialog: React.FC<IAISessionBuildDialogProps> = ({
  onClose,
  initial,
  onStart,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const history = useHistory();

  const [duration, setDuration] = useState(initial?.durationMinutes ?? 30);
  const [minSteam, setMinSteam] = useState(initial?.minSteam ?? 6);
  const [vibe, setVibe] = useState("");
  const [ordering, setOrdering] = useState(initial?.ordering ?? "");
  const [ritualMode, setRitualMode] = useState(false);
  const [ritualMoods, setRitualMoods] = useState<string[]>([]);
  const [performerIds, setPerformerIds] = useState<string[]>([]);
  const [moods, setMoods] = useState<string[]>([]);
  const [plan, setPlan] = useState<
    GQL.AiSessionBuildQuery["aiSessionBuild"] | null
  >(null);

  const { data: moodGroupsData } = GQL.useAiMoodGroupsQuery();
  const [buildSession, { loading }] = GQL.useAiSessionBuildLazyQuery();
  const [describeSession, { loading: describing }] =
    GQL.useAiSessionDescribeLazyQuery();

  const [describeText, setDescribeText] = useState("");
  const [oracleSummary, setOracleSummary] = useState<string | null>(null);

  async function onDescribe() {
    if (describeText.trim() === "") return;
    try {
      const before = {
        duration,
        minSteam,
        moods: [...moods],
        ordering,
        vibe,
      };
      const result = await describeSession({
        variables: { text: describeText },
      });
      const d = result.data?.aiSessionDescribe;
      if (d) {
        // summarize what the oracle changed so nothing is silently overwritten
        const changes: string[] = [];
        if (d.duration_minutes && d.duration_minutes !== before.duration) {
          changes.push(
            `${intl.formatMessage({ id: "config.tasks.ai_session.duration" })} ${before.duration} → ${d.duration_minutes}`
          );
        }
        if (d.min_steam && d.min_steam !== before.minSteam) {
          changes.push(
            `${intl.formatMessage({ id: "config.tasks.ai_session.min_steam" })} ${before.minSteam} → ${d.min_steam}`
          );
        }
        const newMoods = d.moods ?? [];
        if (newMoods.join(",") !== before.moods.join(",")) {
          changes.push(
            `${intl.formatMessage({ id: "config.tasks.ai_session.moods" })}: ${newMoods.join(", ")}`
          );
        }
        if (d.ordering && d.ordering !== before.ordering) {
          changes.push(
            `${intl.formatMessage({ id: "config.tasks.ai_session.ordering" })}: ${d.ordering}`
          );
        }
        if (d.vibe && d.vibe !== before.vibe) {
          changes.push(`"${d.vibe}"`);
        }
        setOracleSummary(
          changes.length > 0
            ? `${intl.formatMessage({ id: "config.tasks.ai_session.oracle_changed" })} ${changes.join(" · ")}`
            : intl.formatMessage({
                id: "config.tasks.ai_session.oracle_no_change",
              })
        );

        setDuration(d.duration_minutes || 30);
        setMinSteam(d.min_steam || 6);
        setMoods(newMoods);
        setRitualMoods(newMoods);
        setOrdering(d.ordering ?? "");
        setVibe(d.vibe ?? "");
      }
    } catch (e) {
      Toast.error(e);
    }
  }

  function moveRitual(index: number, delta: number) {
    setRitualMoods((prev) => {
      const target = index + delta;
      if (target < 0 || target >= prev.length) return prev;
      const next = [...prev];
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
  }

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
            ordering: ordering !== "" ? ordering : undefined,
            ritual:
              ritualMode && ritualMoods.length > 0 ? ritualMoods : undefined,
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

  const [planName, setPlanName] = useState("");
  const { data: savedPlansData, refetch: refetchPlans } =
    GQL.useAiSavedPlansQuery();

  async function onSavePlan() {
    if (!plan || plan.scenes.length === 0 || planName.trim() === "") return;
    try {
      await mutateAiSavePlan(
        planName.trim(),
        plan.scenes.map((s) => s.scene_id)
      );
      Toast.success(intl.formatMessage({ id: "toast.plan_saved" }));
      setPlanName("");
      refetchPlans();
    } catch (e) {
      Toast.error(e);
    }
  }

  function playPlan(sceneIds: string[]) {
    if (sceneIds.length === 0) return;
    const params = sceneIds
      .map((id) => `qs=${id}`)
      .concat("afterglow=1", "autoplay=true")
      .join("&");
    history.push(`/scenes/${sceneIds[0]}?${params}`);
    onClose();
  }

  async function onDeletePlan(planId: string) {
    if (
      !window.confirm(
        intl.formatMessage({
          id: "config.tasks.ai_session.delete_plan_confirm",
        })
      )
    ) {
      return;
    }
    try {
      await mutateAiDeletePlan(planId);
      refetchPlans();
    } catch (e) {
      Toast.error(e);
    }
  }

  function startSession() {
    if (!plan || plan.scenes.length === 0) return;

    onStart?.();

    const params = plan.scenes
      .map((s) => `qs=${s.scene_id}`)
      .concat("afterglow=1")
      .join("&");
    const first = plan.scenes[0];
    history.push(`/scenes/${first.scene_id}?${params}`);
    onClose();
  }

  const savedPlans = savedPlansData?.aiSavedPlans ?? [];

  return (
    <ModalComponent
      show
      onHide={onClose}
      icon={faCalendarAlt}
      header={intl.formatMessage({ id: "config.tasks.ai_session.heading" })}
      dialogClassName="modal-lg"
    >
      <Form>
        <h6 className="ai-session-section-title">
          <FormattedMessage id="config.tasks.ai_session.section_configure" />
        </h6>
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
          <Form.Check
            type="checkbox"
            label={intl.formatMessage({
              id: "config.tasks.ai_session.ritual_mode",
            })}
            checked={ritualMode}
            onChange={() => setRitualMode(!ritualMode)}
          />
          <div className="mt-2">
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
                  setRitualMoods((prev) =>
                    prev.includes(g.mood)
                      ? prev.filter((m) => m !== g.mood)
                      : [...prev, g.mood]
                  );
                }}
              />
            ))}
          </div>
          {ritualMode && ritualMoods.length > 0 && (
            <div className="ritual-order-list mt-2">
              <Form.Text className="text-muted">
                <FormattedMessage id="config.tasks.ai_session.ritual_order" />
              </Form.Text>
              {ritualMoods.map((m, i) => (
                <div key={m} className="ritual-order-item">
                  <span>
                    {i + 1}. {m}
                  </span>
                  <span className="ritual-order-actions">
                    <button
                      type="button"
                      className="btn btn-sm btn-outline-secondary"
                      disabled={i === 0}
                      onClick={() => moveRitual(i, -1)}
                    >
                      ↑
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm btn-outline-secondary ml-1"
                      disabled={i === ritualMoods.length - 1}
                      onClick={() => moveRitual(i, 1)}
                    >
                      ↓
                    </button>
                  </span>
                </div>
              ))}
            </div>
          )}
        </Form.Group>

        {!ritualMode && (
          <Form.Group>
            <Form.Label>
              <FormattedMessage id="config.tasks.ai_session.ordering" />
            </Form.Label>
            <Form.Control
              as="select"
              value={ordering}
              onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
                setOrdering(e.currentTarget.value)
              }
            >
              <option value="">
                {intl.formatMessage({
                  id: "config.tasks.ai_session.ordering_default",
                })}
              </option>
              <option value="build_up">
                {intl.formatMessage({
                  id: "config.tasks.ai_session.ordering_build_up",
                })}
              </option>
              <option value="peak_first">
                {intl.formatMessage({
                  id: "config.tasks.ai_session.ordering_peak_first",
                })}
              </option>
            </Form.Control>
          </Form.Group>
        )}

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

        <h6 className="ai-session-section-title">
          <Icon icon={faWandMagicSparkles} />{" "}
          <FormattedMessage id="config.tasks.ai_session.section_oracle" />
        </h6>
        <Form.Group>
          <Form.Control
            type="text"
            value={describeText}
            placeholder={intl.formatMessage({
              id: "config.tasks.ai_session.describe_placeholder",
            })}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setDescribeText(e.currentTarget.value)
            }
          />
          <Button
            variant="outline-info"
            size="sm"
            className="mt-2"
            onClick={() => onDescribe()}
            disabled={describing}
          >
            {describing ? "…" : "✨ Oracle"}
          </Button>
          {oracleSummary && (
            <Form.Text className="text-muted mt-2 ai-session-oracle-summary">
              {oracleSummary}
            </Form.Text>
          )}
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
          <h6 className="ai-session-section-title">
            <FormattedMessage id="config.tasks.ai_session.section_plan" />:{" "}
            {Math.round(plan.total_minutes)} min · {plan.scenes.length} scenes
          </h6>
          <ul className="ai-session-preview">
            {plan.scenes.map((s) => (
              <li key={s.scene_id}>
                {s.title || s.scene_id} · 🔥 {s.steam} ·{" "}
                {intl.formatMessage({
                  id: "config.tasks.ai_session.duration_label",
                })}{" "}
                {Math.round(s.duration)}s
                {s.best_moment > 0 && ` · best @ ${Math.round(s.best_moment)}s`}
              </li>
            ))}
          </ul>
          <div className="d-flex flex-wrap gap-2 align-items-center">
            <Button variant="danger" onClick={() => startSession()}>
              <FormattedMessage id="config.tasks.ai_session.start" />
            </Button>
            <Button variant="outline-danger" onClick={() => onGenerateReel()}>
              <FormattedMessage id="config.tasks.ai_session.reel" />
            </Button>
            <Form.Control
              type="text"
              className="ai-session-plan-name"
              placeholder={intl.formatMessage({
                id: "config.tasks.ai_session.plan_name",
              })}
              value={planName}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                setPlanName(e.currentTarget.value)
              }
            />
            <Button
              variant="outline-secondary"
              onClick={() => onSavePlan()}
              disabled={planName.trim() === ""}
            >
              <FormattedMessage id="config.tasks.ai_session.save_plan" />
            </Button>
          </div>
        </div>
      )}

      <div className="mt-3">
        <h6 className="ai-session-section-title">
          <FormattedMessage id="config.tasks.ai_session.saved_plans" />
        </h6>
        {savedPlans.length === 0 ? (
          <div className="text-muted small">
            <FormattedMessage id="config.tasks.ai_session.no_saved_plans" />
          </div>
        ) : (
          <ul className="mb-0">
            {savedPlans.map((p) => (
              <li key={p.id} className="ai-session-saved-plan">
                <Button
                  size="sm"
                  variant="link"
                  onClick={() => playPlan(p.scene_ids)}
                >
                  ▶ {p.name}
                </Button>
                <Button
                  size="sm"
                  variant="link"
                  className="text-danger"
                  onClick={() => onDeletePlan(p.id)}
                >
                  ✕
                </Button>
              </li>
            ))}
          </ul>
        )}
      </div>
    </ModalComponent>
  );
};

export default AISessionBuildDialog;
