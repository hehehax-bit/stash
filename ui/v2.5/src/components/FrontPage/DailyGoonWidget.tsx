import React, { useEffect, useState } from "react";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { queryFindScenes } from "src/core/StashService";
import { ListFilterModel } from "src/models/list-filter/filter";
import { SteamScoreCriterion } from "src/models/list-filter/criteria/steam-score";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { Icon } from "src/components/Shared/Icon";
import { faFire } from "@fortawesome/free-solid-svg-icons";

const STREAK_KEY = "stash.dailyGoon.streak";
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

export const DailyGoonWidget: React.FC = () => {
  const [scenes, setScenes] = useState<GQL.SlimSceneDataFragment[]>([]);
  const [loading, setLoading] = useState(true);
  const [streak, setStreak] = useState(0);

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
    </div>
  );
};

export default DailyGoonWidget;
