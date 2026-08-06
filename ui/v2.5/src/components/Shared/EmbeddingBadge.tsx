import React from "react";
import { Icon } from "./Icon";
import { faBrain } from "@fortawesome/free-solid-svg-icons";

export const EmbeddingBadge: React.FC = () => (
  <div className="embedding-badge" title="Has embedding">
    <Icon icon={faBrain} />
  </div>
);

export default EmbeddingBadge;
