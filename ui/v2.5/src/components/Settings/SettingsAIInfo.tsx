import React from "react";
import { Card } from "react-bootstrap";
import { Icon } from "src/components/Shared/Icon";
import {
  faRobot,
  faSearch,
  faClone,
  faUser,
  faFilm,
  faTags,
  faStar,
  faMusic,
  faChartLine,
  faFolderTree,
  faPenToSquare,
} from "@fortawesome/free-solid-svg-icons";

interface AICapability {
  icon: React.ComponentProps<typeof Icon>["icon"];
  title: string;
  description: string;
}

const capabilities: AICapability[] = [
  {
    icon: faRobot,
    title: "AI Chat Assistant",
    description:
      "Query your library using natural language through the AI chat panel.",
  },
  {
    icon: faSearch,
    title: "Semantic Search",
    description:
      "Find scenes, images, performers, studios, and tags by meaning rather than exact keywords.",
  },
  {
    icon: faClone,
    title: "Duplicate Detection",
    description:
      "Detect near-duplicate scenes using semantic embeddings of their content.",
  },
  {
    icon: faUser,
    title: "Performer Recognition & Clustering",
    description:
      "Identify performers and cluster their appearances across scenes and images.",
  },
  {
    icon: faFilm,
    title: "Scene Segmentation",
    description:
      "Automatically split scenes into markers based on detected scene changes.",
  },
  {
    icon: faTags,
    title: "Tagging Suggestions",
    description:
      "Generate titles, tags, and performer suggestions for scenes and images. Review them and apply or reject each one.",
  },
  {
    icon: faStar,
    title: "Media Quality Assessment",
    description:
      "Score scenes and images on visual clarity, lighting, composition, and camera work.",
  },
  {
    icon: faMusic,
    title: "Scene Audio Analysis",
    description:
      "Measure silence ratio, transcribe dialogue, and classify music, speech, moans, and ambient audio.",
  },
  {
    icon: faChartLine,
    title: "Performer Career Analytics",
    description:
      "Build career timelines, primary niches, notable studios, and summaries for performers.",
  },
  {
    icon: faFolderTree,
    title: "Smart Collections",
    description:
      "Automatically create saved filters ([AI] ...) from library statistics to organize your content.",
  },
  {
    icon: faPenToSquare,
    title: "File Renaming",
    description:
      "Suggest clean, organized filenames for scenes and images, then rename files on disk with one click.",
  },
];

export const SettingsAIInfo: React.FC = () => {
  return (
    <Card className="ai-info-card">
      <h5>
        <Icon icon={faRobot} /> AI Capabilities
      </h5>
      <div className="sub-heading">
        This build includes the following AI-powered features. Most run as
        background jobs and require AI to be enabled.
      </div>
      <ul className="ai-capability-list">
        {capabilities.map((cap) => (
          <li key={cap.title}>
            <span className="ai-capability-icon">
              <Icon icon={cap.icon} />
            </span>
            <div>
              <strong>{cap.title}</strong>
              <div className="sub-heading">{cap.description}</div>
            </div>
          </li>
        ))}
      </ul>
    </Card>
  );
};
