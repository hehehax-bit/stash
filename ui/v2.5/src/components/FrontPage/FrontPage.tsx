import React, { useEffect, useState } from "react";
import { FormattedMessage, useIntl } from "react-intl";
import { useHistory } from "react-router-dom";
import { useConfigureUI } from "src/core/StashService";
import { LoadingIndicator } from "../Shared/LoadingIndicator";
import { Button } from "react-bootstrap";
import { FrontPageConfig } from "./FrontPageConfig";
import { useToast } from "src/hooks/Toast";
import { Control } from "./Control";
import { DailyGoonWidget } from "./DailyGoonWidget";
import { FeaturesButton } from "./FeaturesModal";
import { ForYouHub } from "./ForYouHub";
import { useConfigurationContext } from "src/hooks/Config";
import {
  FrontPageContent,
  generateDefaultFrontPageContent,
  getFrontPageContent,
} from "src/core/config";
import { useScrollToTopOnMount } from "src/hooks/scrollToTop";
import { PatchComponent } from "src/patch";
import { Icon } from "src/components/Shared/Icon";
import { faBolt } from "@fortawesome/free-solid-svg-icons";

interface IResumeSession {
  count: number;
  minutes: number;
  lastSceneId: string;
  ids: string[];
}

function readResumeSession(): IResumeSession | null {
  try {
    if (localStorage.getItem("stash.afterglow") === "1") return null;
    const raw = localStorage.getItem("stash.goonSession");
    if (!raw) return null;
    const session = JSON.parse(raw);
    const entries = session?.entries ?? [];
    if (entries.length === 0) return null;
    const minutes = session.startedAt
      ? Math.max(1, Math.round((Date.now() - session.startedAt) / 60000))
      : 0;
    return {
      count: entries.length,
      minutes,
      lastSceneId: entries[entries.length - 1].sceneId,
      ids: entries.map((e: { sceneId: string }) => e.sceneId),
    };
  } catch {
    return null;
  }
}

const FrontPage: React.FC = PatchComponent("FrontPage", () => {
  const intl = useIntl();
  const Toast = useToast();
  const history = useHistory();

  const [isEditing, setIsEditing] = useState(false);
  const [saving, setSaving] = useState(false);
  const [resumeDismissed, setResumeDismissed] = useState(false);

  const [saveUI] = useConfigureUI();

  const { configuration } = useConfigurationContext();

  useScrollToTopOnMount();

  // persist the default front page content once, instead of during render
  // biome-ignore lint/correctness/useExhaustiveDependencies: one-time init
  useEffect(() => {
    const ui = configuration?.ui ?? {};
    if (!ui.frontPageContent) {
      onUpdateConfig(generateDefaultFrontPageContent(intl));
    }
  }, []);

  async function onUpdateConfig(content?: FrontPageContent[]) {
    setIsEditing(false);

    if (!content) {
      return;
    }

    setSaving(true);
    try {
      await saveUI({
        variables: {
          input: {
            ...configuration?.ui,
            frontPageContent: content,
          },
        },
      });
    } catch (e) {
      Toast.error(e);
    }
    setSaving(false);
  }

  if (saving) {
    return <LoadingIndicator />;
  }

  if (isEditing) {
    return <FrontPageConfig onClose={(content) => onUpdateConfig(content)} />;
  }

  const ui = configuration?.ui ?? {};

  const frontPageContent = getFrontPageContent(ui);

  const resume = !resumeDismissed ? readResumeSession() : null;

  return (
    <div className="recommendations-container">
      {resume && (
        <div className="resume-session-banner">
          <Icon icon={faBolt} />
          <span className="resume-session-info">
            <FormattedMessage id="front_page.resume_session" />:{" "}
            <FormattedMessage
              id="front_page.resume_session_desc"
              values={{ count: resume.count, minutes: resume.minutes }}
            />
          </span>
          <Button
            size="sm"
            variant="primary"
            onClick={() => {
              const params = resume.ids
                .map((id) => `qs=${id}`)
                .concat("afterglow=1", "autoplay=true")
                .join("&");
              history.push(`/scenes/${resume.lastSceneId}?${params}`);
            }}
          >
            <FormattedMessage id="front_page.resume_session" />
          </Button>
          <Button
            size="sm"
            variant="outline-secondary"
            onClick={() => {
              localStorage.removeItem("stash.goonSession");
              setResumeDismissed(true);
            }}
          >
            <FormattedMessage id="front_page.dismiss" />
          </Button>
        </div>
      )}
      <DailyGoonWidget />
      <ForYouHub />
      <div>
        {frontPageContent?.map((content, i) => (
          <Control key={i} content={content} />
        ))}
      </div>
      <div className="recommendations-footer">
        <FeaturesButton />
        <Button className="ml-2" onClick={() => setIsEditing(true)}>
          <FormattedMessage id={"actions.customise"} />
        </Button>
      </div>
    </div>
  );
});

export default FrontPage;
