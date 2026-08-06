import React, { useEffect, useState } from "react";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { queryFindScenes, useStats } from "src/core/StashService";
import { ListFilterModel } from "src/models/list-filter/filter";
import { SteamScoreCriterion } from "src/models/list-filter/criteria/steam-score";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { Icon } from "src/components/Shared/Icon";
import { faFire } from "@fortawesome/free-solid-svg-icons";

const STREAK_KEY = "stash.dailyGoon.streak";
const QUESTS_KEY = "stash.quests";
const PICK_POOL = 30;

function dayOfYear(): number {
  const now = new Date();
  const start = new Date(now.getFullYear(), 0, 0);
  return Math.floor((now.getTime() - start.getTime()) / 86400000);
}

function todayStr(): string {
  return new Date().toISOString().slice(0, 10);
}

function loadStreak(): { date: string; streak: number } | null {
  try {
    const raw = localStorage.getItem(STREAK_KEY);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

function updateStreak(): number {
  const today = todayStr();
  const saved = loadStreak();
  if (saved && saved.date === today) {
    return saved.streak;
  }

  const yesterday = new Date(Date.now() - 86400000).toISOString().slice(0, 10);
  const streak = saved && saved.date === yesterday ? saved.streak + 1 : 1;
  localStorage.setItem(STREAK_KEY, JSON.stringify({ date: today, streak }));
  return streak;
}

// --- weekly quests ---

interface IQuestDef {
  id: string;
  messageID: string;
  target: number;
  // lifetime value accessor; weekly progress = current - stored baseline
  current: () => number;
}

function loadInt(key: string): number {
  try {
    return Number(localStorage.getItem(key) ?? "0");
  } catch {
    return 0;
  }
}

function loadSessionStat(field: string): number {
  try {
    const raw = localStorage.getItem("stash.goonSessionStats");
    const stats = raw ? JSON.parse(raw) : {};
    return Number(stats[field] ?? 0);
  } catch {
    return 0;
  }
}

function isoWeek(): string {
  const now = new Date();
  const d = new Date(
    Date.UTC(now.getFullYear(), now.getMonth(), now.getDate())
  );
  const dayNum = d.getUTCDay() || 7;
  d.setUTCDate(d.getUTCDate() + 4 - dayNum);
  const yearStart = new Date(Date.UTC(d.getUTCFullYear(), 0, 1));
  const week = Math.ceil(
    (d.getTime() - yearStart.getTime()) / 86400000 / 7 + 1
  );
  return `${d.getUTCFullYear()}-W${week}`;
}

function hashString(s: string): number {
  let h = 0;
  for (let i = 0; i < s.length; i++) {
    h = (h << 5) - h + s.charCodeAt(i);
    h |= 0;
  }
  return Math.abs(h);
}

const QUEST_POOL: IQuestDef[] = [
  {
    id: "edge",
    messageID: "quests.edge",
    target: 3,
    current: () => loadInt("stash.edgePauses"),
  },
  {
    id: "minutes",
    messageID: "quests.minutes",
    target: 60,
    current: () => loadSessionStat("totalMinutes"),
  },
  // "o" and "watch" use server data (weekly O sum, scenes played)
  { id: "o", messageID: "quests.o", target: 3, current: () => 0 },
  {
    id: "blind",
    messageID: "quests.blind",
    target: 1,
    current: () => loadInt("stash.blindCount"),
  },
  { id: "watch", messageID: "quests.watch", target: 5, current: () => 0 },
  {
    id: "sessions",
    messageID: "quests.sessions",
    target: 3,
    current: () => loadSessionStat("sessions"),
  },
];

interface IQuestState {
  baselines: Record<string, number>;
  completed: Record<string, boolean>;
}

function loadQuestState(week: string): IQuestState {
  try {
    const raw = localStorage.getItem(`${QUESTS_KEY}.${week}`);
    return raw ? JSON.parse(raw) : { baselines: {}, completed: {} };
  } catch {
    return { baselines: {}, completed: {} };
  }
}

function pickQuests(week: string): IQuestDef[] {
  const start = hashString(week) % QUEST_POOL.length;
  return [0, 1, 2].map((i) => QUEST_POOL[(start + i * 2) % QUEST_POOL.length]);
}

export const DailyGoonWidget: React.FC = () => {
  const [scenes, setScenes] = useState<GQL.SlimSceneDataFragment[]>([]);
  const [loading, setLoading] = useState(true);
  const [streak, setStreak] = useState(0);

  const week = isoWeek();
  const quests = pickQuests(week);
  const [questState, setQuestState] = useState<IQuestState>(() =>
    loadQuestState(week)
  );

  const { data: statsData } = useStats();
  const { data: oTimeline } = GQL.useAiOHistoryTimelineQuery({
    variables: { days: 7 },
  });
  const weeklyO = (oTimeline?.aiOHistoryTimeline ?? []).reduce(
    (sum, e) => sum + e.count,
    0
  );
  const scenesPlayed = statsData?.stats.scenes_played ?? 0;

  function questProgress(q: IQuestDef): number {
    let current: number;
    if (q.id === "o") {
      current = weeklyO;
    } else if (q.id === "watch") {
      current = scenesPlayed;
    } else {
      current = q.current();
    }
    const base =
      q.id === "o" || q.id === "watch" ? 0 : (questState.baselines[q.id] ?? 0);
    return Math.max(0, current - base);
  }

  // snapshot baselines for this week's quests on first render of the week
  useEffect(() => {
    setQuestState((prev) => {
      const next = { ...prev, baselines: { ...prev.baselines } };
      let changed = false;
      for (const q of quests) {
        if (q.id === "o" || q.id === "watch") continue;
        if (next.baselines[q.id] === undefined) {
          next.baselines[q.id] = q.current();
          changed = true;
        }
      }
      if (!changed) return prev;
      localStorage.setItem(`${QUESTS_KEY}.${week}`, JSON.stringify(next));
      return next;
    });
  }, [week, quests]);

  // grant XP when a quest completes
  useEffect(() => {
    setQuestState((prev) => {
      const next = { ...prev, completed: { ...prev.completed } };
      let changed = false;
      for (const q of quests) {
        if (next.completed[q.id]) continue;
        if (questProgress(q) >= q.target) {
          next.completed[q.id] = true;
          changed = true;
          localStorage.setItem(
            "stash.questXP",
            String(loadInt("stash.questXP") + 25)
          );
        }
      }
      if (!changed) return prev;
      localStorage.setItem(`${QUESTS_KEY}.${week}`, JSON.stringify(next));
      return next;
    });
    // questProgress depends on weeklyO/scenesPlayed/localStorage counters
  });

  useEffect(() => {
    setStreak(updateStreak());

    const filter = new ListFilterModel(GQL.FilterMode.Scenes);
    filter.sortBy = "random";
    filter.itemsPerPage = PICK_POOL;
    const steam = new SteamScoreCriterion();
    steam.value = 6;
    steam.modifier = GQL.CriterionModifier.GreaterThan;
    filter.criteria.push(steam);

    queryFindScenes(filter)
      .then((result) => {
        setScenes(result.data.findScenes.scenes);
      })
      .catch(() => {
        setScenes([]);
      })
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="daily-goon-widget">
        <FormattedMessage id="front_page.daily_goon.loading" />
      </div>
    );
  }

  if (scenes.length === 0) {
    return null;
  }

  const pick = scenes[dayOfYear() % scenes.length];
  if (!pick) {
    return null;
  }

  return (
    <div className="daily-goon-widget">
      <div className="daily-goon-header">
        <h4>
          <Icon icon={faFire} />{" "}
          <FormattedMessage id="front_page.daily_goon.heading" />
        </h4>
        <span className="daily-goon-streak">
          <FormattedMessage id="front_page.daily_goon.streak" />:{" "}
          <strong>{streak}</strong> 🔥
        </span>
      </div>
      <div className="row">
        <div className="col-6 col-sm-4 col-md-3 col-xl-2">
          <SceneCard scene={pick} />
        </div>
      </div>
      <div className="daily-goon-quests">
        <div className="daily-goon-quests-heading">
          <FormattedMessage id="quests.heading" />{" "}
          <span className="text-muted">({week})</span>
        </div>
        {quests.map((q) => {
          const progress = Math.min(q.target, questProgress(q));
          const done = questState.completed[q.id];
          return (
            <div
              key={q.id}
              className={`daily-goon-quest ${done ? "completed" : ""}`}
            >
              <div className="daily-goon-quest-label">
                <span>
                  <FormattedMessage id={q.messageID} />
                </span>
                <span className="text-muted">
                  {done ? "✓" : `${progress}/${q.target}`}
                </span>
              </div>
              <div className="progress daily-goon-quest-bar">
                <div
                  className="progress-bar"
                  style={{ width: `${(progress / q.target) * 100}%` }}
                />
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default DailyGoonWidget;
