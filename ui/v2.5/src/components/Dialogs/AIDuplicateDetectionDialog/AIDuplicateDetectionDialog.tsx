import React, { useState } from "react";
import { Badge, Button, Form } from "react-bootstrap";
import { Link } from "react-router-dom";
import { useApolloClient } from "@apollo/client";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faClone } from "@fortawesome/free-solid-svg-icons";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { ImageCard } from "src/components/Images/ImageCard";
import { PerformerCard } from "src/components/Performers/PerformerCard";
import { GalleryLink, TagLink } from "src/components/Shared/TagLink";
import { mutateSceneMerge, mutatePerformerMerge } from "src/core/StashService";

interface IAIEquivalentsDialogProps {
  onClose: () => void;
}

export const AIDuplicateDetectionDialog: React.FC<
  IAIEquivalentsDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const Toast = useToast();
  const client = useApolloClient();

  const [entityTypes, setEntityTypes] = useState<string[]>(["scene"]);
  const [threshold, setThreshold] = useState<number>(0.9);
  const [minGroupSize, setMinGroupSize] = useState<number>(2);
  const [model, setModel] = useState<string>("");
  const [results, setResults] = useState<GQL.SemanticDuplicateGroup[] | null>(
    null
  );
  const [loading, setLoading] = useState(false);

  const entityTypeLabel = (entityType: string) => {
    switch (entityType) {
      case "scene":
        return intl.formatMessage({ id: "scenes", defaultMessage: "Scenes" });
      case "image":
        return intl.formatMessage({ id: "images", defaultMessage: "Images" });
      case "performer":
        return intl.formatMessage({
          id: "performers",
          defaultMessage: "Performers",
        });
      case "gallery":
        return intl.formatMessage({
          id: "galleries",
          defaultMessage: "Galleries",
        });
      case "studio":
        return intl.formatMessage({
          id: "studios",
          defaultMessage: "Studios",
        });
      case "tag":
        return intl.formatMessage({ id: "tags", defaultMessage: "Tags" });
      default:
        return entityType;
    }
  };

  const renderEntity = (
    entityType: string,
    entity: GQL.SemanticDuplicateEntity,
    index: number
  ) => {
    switch (entityType) {
      case "scene":
        return (
          entity.scene && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <SceneCard scene={entity.scene} />
            </div>
          )
        );
      case "image":
        return (
          entity.image && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <ImageCard image={entity.image} zoomIndex={0} />
            </div>
          )
        );
      case "performer":
        return (
          entity.performer && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <PerformerCard performer={entity.performer} />
            </div>
          )
        );
      case "gallery":
        return (
          entity.gallery && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <div className="card h-100">
                <div className="card-body text-center">
                  <GalleryLink gallery={entity.gallery} linkType="details" />
                </div>
              </div>
            </div>
          )
        );
      case "studio":
        return (
          entity.studio && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <Badge className="tag-item tag-link" variant="secondary">
                <Link to={`/studios/${entity.studio.id}`}>
                  {entity.studio.name}
                </Link>
              </Badge>
            </div>
          )
        );
      case "tag":
        return (
          entity.tag && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <TagLink tag={entity.tag} linkType="details" />
            </div>
          )
        );
      default:
        return (
          <div key={index} className="col-12">
            <div className="card">
              <div className="card-body">
                <strong>{entity.entity_id}</strong>
              </div>
            </div>
          </div>
        );
    }
  };

  async function onMergeGroup(group: GQL.SemanticDuplicateGroup) {
    const entities = group.entities;
    const first = entities[0];
    const others = entities
      .slice(1)
      .map((e) => e.entity_id)
      .filter((id) => id !== first.entity_id);
    if (others.length === 0) return;

    if (
      !window.confirm(
        intl.formatMessage({
          id: "config.tasks.duplicate_detection.merge_confirm",
        }) + ` (${first.entity_id})`
      )
    ) {
      return;
    }

    try {
      if (group.entity_type === "performer") {
        await mutatePerformerMerge(
          first.entity_id,
          others,
          {} as GQL.PerformerUpdateInput
        );
      } else if (group.entity_type === "scene") {
        await mutateSceneMerge(
          first.entity_id,
          others,
          {} as GQL.SceneUpdateInput,
          true,
          true
        );
      } else {
        return;
      }
      Toast.success(intl.formatMessage({ id: "toast.merged_entities" }));
      setResults((prev) => (prev ? prev.filter((g) => g !== group) : prev));
    } catch (e) {
      Toast.error(e);
    }
  }

  async function onDetect() {
    setLoading(true);
    try {
      const input: GQL.SemanticDuplicateInput = {
        entity_types: entityTypes.length > 0 ? entityTypes : undefined,
        threshold: threshold > 0 ? threshold : undefined,
        min_group_size: minGroupSize > 0 ? minGroupSize : undefined,
        model: model || undefined,
      };

      const result = await client.query({
        query: GQL.SemanticDuplicatesDocument,
        variables: { input },
        fetchPolicy: "network-only",
      });

      setResults(result.data?.semanticDuplicates ?? []);
    } catch (e) {
      Toast.error(e);
    } finally {
      setLoading(false);
    }
  }

  return (
    <ModalComponent
      show
      icon={faClone}
      header={intl.formatMessage({
        id: "config.tasks.duplicate_detection.heading",
      })}
      accept={{
        onClick: onDetect,
        text: loading
          ? "Detecting..."
          : intl.formatMessage({ id: "actions.semantic_search" }),
      }}
      isRunning={loading}
      cancel={{
        onClick: () => onClose(),
        text: intl.formatMessage({ id: "actions.cancel" }),
        variant: "secondary",
      }}
    >
      <Form>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.duplicate_detection.entity_types" />
          </Form.Label>
          <Form.Check
            type="checkbox"
            checked={entityTypes.includes("scene")}
            label="Scenes"
            onChange={(e) => {
              if (e.currentTarget.checked) {
                setEntityTypes([...entityTypes, "scene"]);
              } else {
                setEntityTypes(entityTypes.filter((t) => t !== "scene"));
              }
            }}
          />
          <Form.Check
            type="checkbox"
            checked={entityTypes.includes("image")}
            label="Images"
            onChange={(e) => {
              if (e.currentTarget.checked) {
                setEntityTypes([...entityTypes, "image"]);
              } else {
                setEntityTypes(entityTypes.filter((t) => t !== "image"));
              }
            }}
          />
          <Form.Check
            type="checkbox"
            checked={entityTypes.includes("performer")}
            label="Performers"
            onChange={(e) => {
              if (e.currentTarget.checked) {
                setEntityTypes([...entityTypes, "performer"]);
              } else {
                setEntityTypes(entityTypes.filter((t) => t !== "performer"));
              }
            }}
          />
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.duplicate_detection.threshold" />
          </Form.Label>
          <Form.Control
            type="number"
            min={0}
            max={1}
            step={0.01}
            value={threshold}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setThreshold(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.duplicate_detection.threshold_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.duplicate_detection.min_group_size" />
          </Form.Label>
          <Form.Control
            type="number"
            min={2}
            value={minGroupSize}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setMinGroupSize(Number(e.currentTarget.value))
            }
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.duplicate_detection.min_group_size_desc" />
          </Form.Text>
        </Form.Group>
        <Form.Group>
          <Form.Label>
            <FormattedMessage id="config.tasks.duplicate_detection.model" />
          </Form.Label>
          <Form.Control
            type="text"
            value={model}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setModel(e.currentTarget.value)
            }
            placeholder="Leave empty for default"
          />
          <Form.Text className="text-muted">
            <FormattedMessage id="config.tasks.duplicate_detection.model_desc" />
          </Form.Text>
        </Form.Group>

        {results && results.length > 0 && (
          <div className="mt-3">
            <h6>
              {intl.formatMessage({
                id: "config.tasks.duplicate_detection.results",
              })}
              ({results.length}{" "}
              {intl.formatMessage({
                id: "config.tasks.duplicate_detection.groups",
                defaultMessage: "groups",
              })}
              )
            </h6>
            {results.map((group, groupIndex) => (
              <div key={`${group.entity_type}-${groupIndex}`} className="mb-4">
                <div className="d-flex align-items-center">
                  <h6 className="text-capitalize mb-0">
                    {entityTypeLabel(group.entity_type)}
                  </h6>
                  {(group.entity_type === "scene" ||
                    group.entity_type === "performer") &&
                    group.entities.length > 1 && (
                      <Button
                        size="sm"
                        variant="secondary"
                        className="ml-2"
                        onClick={() => onMergeGroup(group)}
                      >
                        <FormattedMessage
                          id="config.tasks.duplicate_detection.merge_group"
                          defaultMessage="Merge into first"
                        />
                      </Button>
                    )}
                </div>
                <div className="row mt-2">
                  {group.entities.map((entity, idx) =>
                    renderEntity(group.entity_type, entity, idx)
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </Form>
    </ModalComponent>
  );
};

export default AIDuplicateDetectionDialog;
