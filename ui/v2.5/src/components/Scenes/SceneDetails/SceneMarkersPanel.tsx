import React, { useState, useEffect } from "react";
import { Button } from "react-bootstrap";
import { FormattedMessage, useIntl } from "react-intl";
import Mousetrap from "mousetrap";
import * as GQL from "src/core/generated-graphql";
import { MarkerWallPanel } from "src/components/Wall/WallPanel";
import { PrimaryTags } from "./PrimaryTags";
import { SceneMarkerForm } from "./SceneMarkerForm";
import {
  mutateMetadataGenerateHighlightClip,
  mutateAiSaveMoment,
  mutateAiUnsaveMoment,
} from "src/core/StashService";
import { useToast } from "src/hooks/Toast";

interface ISceneMarkersPanelProps {
  sceneId: string;
  isVisible: boolean;
  onClickMarker: (marker: GQL.SceneMarkerDataFragment) => void;
  onLoopMarker: (marker: GQL.SceneMarkerDataFragment) => void;
}

function ClimaxMap({
  markers,
  onClickMarker,
  savedSet,
  onToggleSaved,
}: {
  markers: GQL.SceneMarkerDataFragment[];
  onClickMarker: (marker: GQL.SceneMarkerDataFragment) => void;
  savedSet: Set<string>;
  onToggleSaved: (markerId: string) => void;
}) {
  const intensityMarkers = markers.filter(
    (m) => m.intensity !== null && m.intensity !== undefined
  );
  if (intensityMarkers.length < 2) return null;

  const total =
    Math.max(...intensityMarkers.map((m) => m.end_seconds ?? m.seconds)) || 1;

  return (
    <div className="climax-map">
      {intensityMarkers.map((m) => {
        const start = m.seconds;
        const end = m.end_seconds ?? start + total * 0.08;
        const left = (start / total) * 100;
        const width = Math.max(2, ((end - start) / total) * 100);
        const level = (m.intensity ?? 0) / 10;
        return (
          <div
            key={m.id}
            className={`climax-peak ${savedSet.has(m.id) ? "saved" : ""}`}
            style={{
              height: `${Math.max(15, level * 100)}%`,
              left: `${left}%`,
              width: `${width}%`,
            }}
            title={`${m.title} \u2014 ${Math.round(m.intensity ?? 0)}/10`}
            onClick={() => onClickMarker(m)}
          >
            <span
              className="climax-heart"
              role="button"
              onClick={(e) => {
                e.stopPropagation();
                onToggleSaved(m.id);
              }}
            >
              {savedSet.has(m.id) ? "\u2764\uFE0F" : "\u2661"}
            </span>
          </div>
        );
      })}
    </div>
  );
}

export const SceneMarkersPanel: React.FC<ISceneMarkersPanelProps> = ({
  sceneId,
  isVisible,
  onClickMarker,
  onLoopMarker,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const { data, loading } = GQL.useFindSceneMarkerTagsQuery({
    variables: { id: sceneId },
  });
  const { data: savedData, refetch: refetchSaved } =
    GQL.useAiSavedMomentsQuery();
  const savedSet = new Set(
    savedData?.aiSavedMoments?.map((s) => s.marker_id) ?? []
  );

  async function toggleSaved(markerId: string) {
    try {
      if (savedSet.has(markerId)) {
        await mutateAiUnsaveMoment(markerId);
      } else {
        await mutateAiSaveMoment(markerId);
      }
      refetchSaved();
    } catch (e) {
      Toast.error(e);
    }
  }
  const [isEditorOpen, setIsEditorOpen] = useState<boolean>(false);
  const [editingMarker, setEditingMarker] =
    useState<GQL.SceneMarkerDataFragment>();

  // set up hotkeys
  useEffect(() => {
    if (!isVisible) return;

    Mousetrap.bind("n", () => onOpenEditor());

    return () => {
      Mousetrap.unbind("n");
    };
  });

  if (loading) return null;

  function onOpenEditor(marker?: GQL.SceneMarkerDataFragment) {
    setIsEditorOpen(true);
    setEditingMarker(marker ?? undefined);
  }

  const closeEditor = () => {
    setEditingMarker(undefined);
    setIsEditorOpen(false);
  };

  if (isEditorOpen)
    return (
      <SceneMarkerForm
        sceneID={sceneId}
        marker={editingMarker}
        onClose={closeEditor}
      />
    );

  const sceneMarkers =
    data?.sceneMarkerTags.flatMap((tag) => tag.scene_markers) ?? [];

  const sceneMarkersAll =
    data?.sceneMarkerTags.flatMap((tag) => tag.scene_markers) ?? [];

  const bestMarker = sceneMarkersAll
    .filter((m) => m.intensity !== null && m.intensity !== undefined)
    .sort((a, b) => (b.intensity ?? 0) - (a.intensity ?? 0))[0];

  async function onGenerateClip() {
    if (!bestMarker) return;
    try {
      const result = await mutateMetadataGenerateHighlightClip(
        sceneId,
        bestMarker.id
      );
      Toast.success(
        intl.formatMessage({ id: "toast.clip_generated" }) +
          `: ${result.data?.metadataGenerateHighlightClip ?? ""}`
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <div className="scene-markers-panel">
      <ClimaxMap
        markers={sceneMarkersAll}
        onClickMarker={onClickMarker}
        savedSet={savedSet}
        onToggleSaved={toggleSaved}
      />
      <Button onClick={() => onOpenEditor()}>
        <FormattedMessage id="actions.create_marker" />
      </Button>
      {bestMarker && (
        <>
          <Button
            variant="warning"
            className="ml-2"
            onClick={() => onClickMarker(bestMarker)}
          >
            <FormattedMessage id="scene_markers.best_part" />
          </Button>
          <Button
            variant="outline-warning"
            className="ml-2"
            onClick={() => onGenerateClip()}
          >
            <FormattedMessage id="scene_markers.generate_clip" />
          </Button>
        </>
      )}
      <div className="container">
        <PrimaryTags
          sceneMarkers={sceneMarkers}
          onClickMarker={onClickMarker}
          onLoopMarker={onLoopMarker}
          onEdit={onOpenEditor}
        />
      </div>
      <MarkerWallPanel
        markers={sceneMarkers}
        clickHandler={(e, marker) => {
          e.preventDefault();
          window.scrollTo(0, 0);
          onClickMarker(marker);
        }}
      />
    </div>
  );
};

export default SceneMarkersPanel;
