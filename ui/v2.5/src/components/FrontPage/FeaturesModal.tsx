import React, { useState } from "react";
import { Button } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import { ModalComponent } from "src/components/Shared/Modal";
import { faRocket } from "@fortawesome/free-solid-svg-icons";

const FEATURE_GROUPS: { heading: string; items: string[] }[] = [
  {
    heading: "AI Chat & Search",
    items: [
      "AI chat with sessions, image input, and tools",
      "Ask AI from scene, image, and performer pages",
      "Library context: the chat can answer questions about your collection",
      "recommend_scene tool: tell it a vibe, get scene links",
      "Semantic search and similar-items panels with batch actions",
      "Transcript search across scenes",
    ],
  },
  {
    heading: "Tagging & Analysis",
    items: [
      "AI scene/image tagging with performers-only and fill-missing modes",
      "Scene segmentation into markers with performers and intensity",
      "Climax map, Skip to the good part, highlight clips",
      "AI mood tagging with card badges, filter, and mood groups",
      "Audio analysis: timestamps, summaries, moans — moan leaderboard",
      "Steam score per scene (fire badges, filter, sort)",
      "Loop detection, smart collections, duplicate detection with merge",
    ],
  },
  {
    heading: "Review Flows",
    items: [
      "Performer merge suggestions with review",
      "AI audit of previously tagged scenes",
      "AI translation with review before applying",
      "Performer discovery with create/merge/reject",
      "Apply all / Reject all in every review dialog",
    ],
  },
  {
    heading: "The Gooner Update",
    items: [
      "Goon mode (G key): steam-filtered browsing",
      "Fap Roulette and Afterglow reel mode with session recap",
      "Session Builder and goon reel generation",
      "Daily Goon widget and the For You hub",
      "O board on the stats page",
      "Quick-hide (H key) for shared screens",
    ],
  },
  {
    heading: "Under the hood",
    items: [
      "Text + visual embeddings with stale-only refresh",
      "Scheduled AI maintenance",
      "Job queue AI badges and filter",
    ],
  },
];

export const FeaturesModal: React.FC<{ onClose: () => void }> = ({
  onClose,
}) => {
  const intl = useIntl();

  return (
    <ModalComponent
      show
      icon={faRocket}
      header={intl.formatMessage({ id: "features.heading" })}
      onHide={onClose}
      cancel={{
        onClick: onClose,
        text: intl.formatMessage({ id: "actions.close" }),
        variant: "secondary",
      }}
    >
      {FEATURE_GROUPS.map((group) => (
        <div key={group.heading} className="mb-3">
          <h6>{group.heading}</h6>
          <ul className="mb-0">
            {group.items.map((item) => (
              <li key={item}>{item}</li>
            ))}
          </ul>
        </div>
      ))}
    </ModalComponent>
  );
};

export const FeaturesButton: React.FC = () => {
  const [open, setOpen] = useState(false);

  return (
    <>
      <Button variant="secondary" onClick={() => setOpen(true)}>
        <FormattedMessage id="features.button" />
      </Button>
      {open && <FeaturesModal onClose={() => setOpen(false)} />}
    </>
  );
};

export default FeaturesModal;
