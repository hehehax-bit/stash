import React, { useEffect, useState } from "react";
import { Button } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { ModalComponent } from "src/components/Shared/Modal";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { ImageCard } from "src/components/Images/ImageCard";
import { PerformerCard } from "src/components/Performers/PerformerCard";
import { StudioCard } from "src/components/Studios/StudioCard";
import { TagCard } from "src/components/Tags/TagCard";
import { GalleryCard } from "src/components/Galleries/GalleryCard";
import { GroupSelect } from "src/components/Groups/GroupSelect";
import { Icon } from "src/components/Shared/Icon";
import { faChevronRight, faImages } from "@fortawesome/free-solid-svg-icons";
import { FormattedMessage, useIntl } from "react-intl";
import { useToast } from "src/hooks/Toast";
import { AISceneTagDialog } from "../Dialogs/AISceneTagDialog/AISceneTagDialog";
import { AIImageTagDialog } from "../Dialogs/AIImageTagDialog/AIImageTagDialog";
import { mutateMetadataDetectLooping } from "src/core/StashService";

const COLLAPSE_DELAY_MS = 5000;

interface ISimilarItemsPanelProps {
  entityType: "scene" | "image" | "performer";
  entityId: string;
  limit?: number;
}

type SimilarItem = NonNullable<
  GQL.SimilarToEntityQuery["similarToEntity"]
>[number];

function previewImage(result: SimilarItem): string | null {
  switch (result.entity_type) {
    case "scene":
      return result.scene?.paths?.screenshot ?? null;
    case "image":
      return result.image?.paths?.thumbnail ?? null;
    case "performer":
      return result.performer?.image_path ?? null;
  }
  return null;
}

function previewTitle(result: SimilarItem): string {
  switch (result.entity_type) {
    case "scene":
      return result.scene?.title ?? result.scene?.id ?? "";
    case "image":
      return result.image?.title ?? result.image?.id ?? "";
    case "performer":
      return result.performer?.name ?? "";
  }
  return "";
}

const EntityCard: React.FC<{ result: SimilarItem }> = ({ result }) => {
  switch (result.entity_type) {
    case "scene":
      return result.scene ? <SceneCard scene={result.scene} /> : null;
    case "image":
      return result.image ? (
        <ImageCard image={result.image} zoomIndex={1} />
      ) : null;
    case "performer":
      return result.performer ? (
        <PerformerCard performer={result.performer} />
      ) : null;
    case "studio":
      return result.studio ? <StudioCard studio={result.studio} /> : null;
    case "tag":
      return result.tag ? <TagCard tag={result.tag} zoomIndex={1} /> : null;
    case "gallery":
      return result.gallery ? <GalleryCard gallery={result.gallery} /> : null;
    default:
      return null;
  }
};

export const SimilarItemsPanel: React.FC<ISimilarItemsPanelProps> = ({
  entityType,
  entityId,
  limit = 20,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const { data, loading, error } = GQL.useSimilarToEntityQuery({
    variables: {
      entity_type: entityType,
      entity_id: entityId,
      limit,
    },
    skip: !entityId,
  });

  const [collapsed, setCollapsed] = useState(false);
  const [showModal, setShowModal] = useState(false);
  const [showTagDialog, setShowTagDialog] = useState(false);
  const [showGroupPicker, setShowGroupPicker] = useState(false);

  const [bulkSceneUpdate] = GQL.useBulkSceneUpdateMutation();
  const [mergePerformers] = GQL.usePerformerMergeMutation();

  const results = data?.similarToEntity ?? [];
  const previews = results.slice(0, 2);

  const sceneIds = results
    .filter((r) => r.scene)
    .map((r) => r.scene?.id ?? "")
    .filter((id) => id !== "");
  const imageIds = results
    .filter((r) => r.image)
    .map((r) => r.image?.id ?? "")
    .filter((id) => id !== "");
  const performerIds = results
    .filter((r) => r.performer)
    .map((r) => r.performer?.id ?? "")
    .filter((id) => id !== "");

  useEffect(() => {
    if (results.length === 0) return;
    const t = setTimeout(() => setCollapsed(true), COLLAPSE_DELAY_MS);
    return () => clearTimeout(t);
  }, [results.length]);

  if (loading) {
    return <LoadingIndicator inline />;
  }
  if (error) {
    return <span className="text-muted">{error.message}</span>;
  }

  if (results.length === 0) {
    return (
      <div className="text-muted mt-2">
        <FormattedMessage id="no_similar_items" />
      </div>
    );
  }

  async function onDetectLoops() {
    try {
      await mutateMetadataDetectLooping({ sceneIds });
      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: "Detect Looping Videos" }
        )
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onAddToGroup(groupId: string) {
    try {
      await bulkSceneUpdate({
        variables: {
          input: {
            ids: sceneIds,
            group_ids: {
              ids: [groupId],
              mode: GQL.BulkUpdateIdMode.Add,
            },
          },
        },
      });
      Toast.success(intl.formatMessage({ id: "toast.group_updated" }));
      setShowGroupPicker(false);
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onMergeAllPerformers() {
    if (
      !window.confirm(intl.formatMessage({ id: "similar_items.merge_confirm" }))
    ) {
      return;
    }
    try {
      await mergePerformers({
        variables: {
          input: {
            source: performerIds,
            destination: entityId,
          },
        },
      });
      Toast.success(intl.formatMessage({ id: "toast.merged_entities" }));
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <>
      <div
        className={`similar-items-preview ${collapsed ? "collapsed" : ""}`}
        onClick={() => setShowModal(true)}
        role="button"
      >
        <div className="similar-items-label">
          <Icon icon={faImages} />
          <span>
            <FormattedMessage id="similar_items" /> ({results.length})
          </span>
        </div>
        <div className="similar-preview-cards">
          {previews.map((result) => (
            <div
              key={`${result.entity_type}-${result.entity_id}`}
              className="similar-preview-card"
            >
              <img src={previewImage(result) ?? ""} alt="" />
              <div className="title">{previewTitle(result)}</div>
            </div>
          ))}
        </div>
        <Icon icon={faChevronRight} className="ml-auto" />
      </div>

      <ModalComponent
        show={showModal}
        icon={faImages}
        header={intl.formatMessage({ id: "similar_items" })}
        dialogClassName="modal-lg"
        onHide={() => setShowModal(false)}
        cancel={{
          onClick: () => setShowModal(false),
          text: intl.formatMessage({ id: "actions.close" }),
          variant: "secondary",
        }}
      >
        {entityType === "scene" && sceneIds.length > 0 && (
          <div className="similar-items-actions mb-3">
            <Button
              size="sm"
              variant="secondary"
              className="mr-2"
              onClick={() => {
                setShowModal(false);
                setShowTagDialog(true);
              }}
            >
              <FormattedMessage id="similar_items.tag_all" />
            </Button>
            <Button
              size="sm"
              variant="secondary"
              className="mr-2"
              onClick={() => onDetectLoops()}
            >
              <FormattedMessage id="similar_items.detect_loops" />
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => setShowGroupPicker(!showGroupPicker)}
            >
              <FormattedMessage id="similar_items.add_to_group" />
            </Button>
          </div>
        )}
        {entityType === "image" && imageIds.length > 0 && (
          <div className="similar-items-actions mb-3">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setShowModal(false);
                setShowTagDialog(true);
              }}
            >
              <FormattedMessage id="similar_items.tag_all" />
            </Button>
          </div>
        )}
        {entityType === "performer" && performerIds.length > 0 && (
          <div className="similar-items-actions mb-3">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => onMergeAllPerformers()}
            >
              <FormattedMessage id="similar_items.merge_all" />
            </Button>
          </div>
        )}
        {showGroupPicker && (
          <div className="mb-3">
            <GroupSelect
              onSelect={(items) => {
                if (items && items.length > 0) onAddToGroup(items[0].id);
              }}
            />
          </div>
        )}
        <div className="row">
          {results.map((result) => (
            <div
              key={`${result.entity_type}-${result.entity_id}`}
              className="col-6"
            >
              <EntityCard result={result} />
            </div>
          ))}
        </div>
      </ModalComponent>

      {showTagDialog && entityType === "scene" && (
        <AISceneTagDialog
          selectedIds={sceneIds}
          onClose={() => setShowTagDialog(false)}
        />
      )}
      {showTagDialog && entityType === "image" && (
        <AIImageTagDialog
          selectedIds={imageIds}
          onClose={() => setShowTagDialog(false)}
        />
      )}
    </>
  );
};

export default SimilarItemsPanel;
