import React, { useEffect, useState } from "react";
import { useHistory } from "react-router-dom";
import { Link } from "react-router-dom";
import { FormattedMessage } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { queryFindScenes } from "src/core/StashService";
import { ListFilterModel } from "src/models/list-filter/filter";
import { SteamScoreCriterion } from "src/models/list-filter/criteria/steam-score";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { Icon } from "src/components/Shared/Icon";
import { Button } from "react-bootstrap";
import { faCrown, faFire } from "@fortawesome/free-solid-svg-icons";
import { AISessionBuildDialog } from "src/components/Dialogs/AISessionBuildDialog/AISessionBuildDialog";
import { Achievements } from "./Achievements";
import { ThroneProgress } from "./ThroneProgress";

const PICK_COUNT = 6;

function buildSteamFilter(): ListFilterModel {
  const filter = new ListFilterModel(GQL.FilterMode.Scenes);
  filter.sortBy = "random";
  filter.itemsPerPage = PICK_COUNT * 3;
  const steam = new SteamScoreCriterion();
  steam.value = 7;
  steam.modifier = GQL.CriterionModifier.GreaterThan;
  filter.criteria.push(steam);
  return filter;
}

const TonightPicks: React.FC = () => {
  const [scenes, setScenes] = useState<GQL.SlimSceneDataFragment[]>([]);

  useEffect(() => {
    queryFindScenes(buildSteamFilter())
      .then((r) => setScenes(r.data.findScenes.scenes.slice(0, PICK_COUNT)))
      .catch(() => setScenes([]));
  }, []);

  if (scenes.length === 0) return null;

  return (
    <div className="for-you-row">
      <h5>
        <Icon icon={faFire} />{" "}
        <FormattedMessage id="front_page.tonight_picks" />
      </h5>
      <div className="row">
        {scenes.map((s) => (
          <div key={s.id} className="col-6 col-sm-4 col-md-3 col-xl-2 mb-3">
            <SceneCard scene={s} />
          </div>
        ))}
      </div>
    </div>
  );
};

const MoanerOfTheWeek: React.FC = () => {
  const { data } = GQL.useAiMoanLeaderboardQuery({ variables: { limit: 3 } });

  const entries = data?.aiMoanLeaderboard.filter((e) => e.performer) ?? [];
  if (entries.length === 0) return null;

  return (
    <div className="for-you-row">
      <h5>
        <Icon icon={faCrown} /> <FormattedMessage id="front_page.moaner_week" />
      </h5>
      <div className="row">
        {entries.map((e, i) => (
          <div
            key={e.performer?.id}
            className="col-6 col-sm-4 col-md-3 col-xl-2 mb-3"
          >
            <Link
              to={`/performers/${e.performer?.id}`}
              className="for-you-moaner"
            >
              {i === 0 && <span className="for-you-crown">👑</span>}
              <span>{e.performer?.name}</span>
              <span className="text-muted">
                {e.moan_scenes} moan scenes ·{" "}
                {Math.round((e.moan_rate ?? 0) * 100)}%
              </span>
            </Link>
          </div>
        ))}
      </div>
    </div>
  );
};

const RecentScenes: React.FC = () => {
  const [scenes, setScenes] = useState<GQL.SlimSceneDataFragment[]>([]);

  useEffect(() => {
    const filter = new ListFilterModel(GQL.FilterMode.Scenes);
    filter.sortBy = "last_played_at";
    filter.sortDirection = GQL.SortDirectionEnum.Desc;
    filter.itemsPerPage = 10;
    queryFindScenes(filter)
      .then((r) => setScenes(r.data.findScenes.scenes))
      .catch(() => setScenes([]));
  }, []);

  if (scenes.length === 0) return null;

  return (
    <div className="for-you-row">
      <h5>
        <FormattedMessage id="front_page.recent_scenes" />
      </h5>
      <div className="row">
        {scenes.map((s) => (
          <div key={s.id} className="col-6 col-sm-4 col-md-3 col-xl-2 mb-3">
            <SceneCard scene={s} />
          </div>
        ))}
      </div>
    </div>
  );
};

export const ForYouHub: React.FC = () => {
  const history = useHistory();
  const [showSession, setShowSession] = useState(false);
  return (
    <>
      <div className="for-you-row for-you-actions">
        <Button variant="outline-danger" onClick={() => setShowSession(true)}>
          <FormattedMessage id="config.tasks.ai_session.button" />
        </Button>
        <Button
          variant="outline-warning"
          onClick={() => history.push("/scoreboard")}
        >
          <FormattedMessage id="scoreboard.heading" />
        </Button>
        <Button
          variant="outline-primary"
          onClick={() => history.push("/aiChat")}
        >
          <FormattedMessage id="scene_roulette.chat" />
        </Button>
        <Button
          variant="outline-secondary"
          onClick={() => history.push("/saved")}
        >
          <FormattedMessage id="saved_moments.heading" />
        </Button>
      </div>
      <ThroneProgress compact />
      <Achievements />
      <TonightPicks />
      <MoanerOfTheWeek />
      <RecentScenes />
      {showSession && (
        <AISessionBuildDialog onClose={() => setShowSession(false)} />
      )}
    </>
  );
};

export default ForYouHub;
