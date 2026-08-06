import React, { useState, useCallback } from "react";
import { Form, Button } from "react-bootstrap";
import { useApolloClient } from "@apollo/client";
import { ModalComponent } from "src/components/Shared/Modal";
import { useToast } from "src/hooks/Toast";
import * as GQL from "src/core/generated-graphql";
import { FormattedMessage, useIntl } from "react-intl";
import { faSearch } from "@fortawesome/free-solid-svg-icons";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { ImageCard } from "src/components/Images/ImageCard";
import { PerformerCard } from "src/components/Performers/PerformerCard";

import { TagLink } from "src/components/Shared/TagLink";

interface ISemanticSearchDialogProps {
  onClose: () => void;
}

export const SemanticSearchDialog: React.FC<ISemanticSearchDialogProps> = ({
  onClose,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const client = useApolloClient();

  const [query, setQuery] = useState<string>("");
  const [imageData, setImageData] = useState<{
    dataUrl: string;
    base64: string;
    mediaType: string;
  } | null>(null);
  const [entityTypes, setEntityTypes] = useState<string[]>([
    "scene",
    "performer",
    "image",
    "gallery",
    "studio",
    "tag",
  ]);
  const [limit, setLimit] = useState<number>(20);
  const [model, setModel] = useState<string>("");
  const [results, setResults] = useState<GQL.SemanticSearchResult[] | null>(
    null
  );
  const [loading, setLoading] = useState(false);

  const handleSearch = useCallback(async () => {
    if (!query.trim() && !imageData) {
      Toast.toast({
        content: intl.formatMessage({
          id: "validation.required",
          defaultMessage: "Query is required",
        }),
        variant: "warning",
      });
      return;
    }

    setLoading(true);
    try {
      const input: GQL.SemanticSearchInput = {
        query: imageData ? "" : query.trim(),
        entity_types: entityTypes.length > 0 ? entityTypes : undefined,
        limit: limit > 0 ? limit : undefined,
        model: model || undefined,
        image: imageData ? imageData.base64 : undefined,
        image_media_type: imageData ? imageData.mediaType : undefined,
      };

      const result = await client.query({
        query: GQL.SemanticSearchDocument,
        variables: { input },
        fetchPolicy: "network-only",
      });

      setResults(result.data?.semanticSearch ?? []);
    } catch (e) {
      Toast.error(e);
    } finally {
      setLoading(false);
    }
  }, [client, intl, query, imageData, entityTypes, limit, model, Toast]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSearch();
    }
  };

  const renderResult = (result: GQL.SemanticSearchResult, index: number) => {
    const scorePercent = Math.round(result.score * 100);

    switch (result.entity_type) {
      case "scene":
        return (
          result.scene && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <SceneCard scene={result.scene} />
              <div className="mt-1 text-center">
                <span className="badge badge-info">{scorePercent}%</span>
              </div>
            </div>
          )
        );
      case "image":
        return (
          result.image && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <ImageCard image={result.image} zoomIndex={0} />
              <div className="mt-1 text-center">
                <span className="badge badge-info">{scorePercent}%</span>
              </div>
            </div>
          )
        );
      case "performer":
        return (
          result.performer && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <PerformerCard performer={result.performer} />
              <div className="mt-1 text-center">
                <span className="badge badge-info">{scorePercent}%</span>
              </div>
            </div>
          )
        );
      case "studio":
        return (
          result.studio && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <div className="card studio-card h-100">
                <div className="card-body text-center">
                  <h6 className="card-title">{result.studio.name}</h6>
                  <small className="text-muted">
                    {intl.formatMessage({
                      id: "studio",
                      defaultMessage: "Studio",
                    })}
                    : {result.studio.id}
                    <span className="ml-2 badge badge-info">
                      {scorePercent}%
                    </span>
                  </small>
                </div>
              </div>
            </div>
          )
        );
      case "tag":
        return (
          result.tag && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <TagLink tag={result.tag} />
              <div className="mt-1 text-center">
                <span className="badge badge-info">{scorePercent}%</span>
              </div>
            </div>
          )
        );
      case "gallery":
        return (
          result.gallery && (
            <div key={index} className="col-12 col-md-6 col-lg-4">
              <div className="card gallery-card h-100">
                <div className="card-body">
                  <h6 className="card-title">{result.gallery.title}</h6>
                  <small className="text-muted">
                    {intl.formatMessage({
                      id: "gallery",
                      defaultMessage: "Gallery",
                    })}
                    : {result.gallery.id}
                    <span className="ml-2 badge badge-info">
                      {scorePercent}%
                    </span>
                  </small>
                </div>
              </div>
            </div>
          )
        );
      default:
        return (
          <div key={index} className="col-12">
            <div className="card">
              <div className="card-body">
                <strong>
                  {result.entity_type}: {result.entity_id}
                </strong>
                <span className="ml-2 badge badge-info">{scorePercent}%</span>
              </div>
            </div>
          </div>
        );
    }
  };

  return (
    <ModalComponent
      show
      icon={faSearch}
      header={intl.formatMessage({
        id: "ai.semantic_search.title",
        defaultMessage: "Semantic Search",
      })}
      modalProps={{ size: "xl" }}
      accept={{
        onClick: handleSearch,
        text: loading
          ? intl.formatMessage({
              id: "actions.searching",
              defaultMessage: "Searching...",
            })
          : intl.formatMessage({
              id: "actions.search",
              defaultMessage: "Search",
            }),
        disabled: (!query.trim() && !imageData) || loading,
      }}
      isRunning={loading}
      cancel={{
        onClick: () => onClose(),
        text: intl.formatMessage({
          id: "actions.cancel",
          defaultMessage: "Cancel",
        }),
        variant: "secondary",
      }}
    >
      <Form>
        <Form.Group>
          <Form.Label>
            <FormattedMessage
              id="ai.semantic_search.query_label"
              defaultMessage="Search Query"
            />
          </Form.Label>
          <Form.Control
            as="textarea"
            rows={3}
            value={query}
            disabled={imageData !== null}
            onChange={(e: React.ChangeEvent<HTMLTextAreaElement>) =>
              setQuery(e.currentTarget.value)
            }
            onKeyDown={handleKeyDown}
            placeholder={intl.formatMessage({
              id: "ai.semantic_search.query_placeholder",
              defaultMessage: "Enter a natural language description...",
            })}
          />
          <Form.Text className="text-muted">
            <FormattedMessage
              id="ai.semantic_search.query_help"
              defaultMessage="Describe what you're looking for in natural language. The AI will find semantically similar content."
            />
          </Form.Text>
        </Form.Group>

        <Form.Group>
          <Form.Label>
            <FormattedMessage
              id="ai.semantic_search.image_label"
              defaultMessage="Search by Image (Optional)"
            />
          </Form.Label>
          <Form.Control
            type="file"
            accept="image/*"
            onChange={(e: React.ChangeEvent<HTMLInputElement>) => {
              const file = e.currentTarget.files?.[0];
              if (!file) return;
              const reader = new FileReader();
              reader.onload = () => {
                const dataUrl = reader.result as string;
                const match = /^data:([^;]+);base64,(.+)$/.exec(dataUrl);
                if (!match) return;
                setImageData({
                  dataUrl,
                  base64: match[2],
                  mediaType: match[1],
                });
              };
              reader.readAsDataURL(file);
            }}
          />
          {imageData && (
            <div className="mt-2 d-flex align-items-center">
              <img
                src={imageData.dataUrl}
                alt=""
                className="mr-2"
                style={{ maxHeight: "120px", maxWidth: "120px" }}
              />
              <Button
                variant="secondary"
                size="sm"
                onClick={() => setImageData(null)}
              >
                <FormattedMessage id="actions.clear" defaultMessage="Clear" />
              </Button>
            </div>
          )}
          <Form.Text className="text-muted">
            <FormattedMessage
              id="ai.semantic_search.image_help"
              defaultMessage="Optionally upload an image to find visually similar content (uses the image embedding model). When an image is provided, the text query is ignored."
            />
          </Form.Text>
        </Form.Group>

        <Form.Row>
          <Form.Group
            as={Form.Group}
            controlId="entityTypes"
            className="col-md-6"
          >
            <Form.Label>
              <FormattedMessage
                id="ai.semantic_search.entity_types"
                defaultMessage="Entity Types"
              />
            </Form.Label>
            {[
              {
                value: "scene",
                label: intl.formatMessage({
                  id: "scenes",
                  defaultMessage: "Scenes",
                }),
              },
              {
                value: "image",
                label: intl.formatMessage({
                  id: "images",
                  defaultMessage: "Images",
                }),
              },
              {
                value: "performer",
                label: intl.formatMessage({
                  id: "performers",
                  defaultMessage: "Performers",
                }),
              },
              {
                value: "gallery",
                label: intl.formatMessage({
                  id: "galleries",
                  defaultMessage: "Galleries",
                }),
              },
              {
                value: "studio",
                label: intl.formatMessage({
                  id: "studios",
                  defaultMessage: "Studios",
                }),
              },
              {
                value: "tag",
                label: intl.formatMessage({
                  id: "tags",
                  defaultMessage: "Tags",
                }),
              },
            ].map(({ value, label }) => (
              <Form.Check
                key={value}
                type="checkbox"
                checked={entityTypes.includes(value)}
                label={label}
                onChange={(e) => {
                  if (e.currentTarget.checked) {
                    setEntityTypes([...entityTypes, value]);
                  } else {
                    setEntityTypes(entityTypes.filter((t) => t !== value));
                  }
                }}
              />
            ))}
          </Form.Group>

          <Form.Group as={Form.Group} controlId="limit" className="col-md-6">
            <Form.Label>
              <FormattedMessage
                id="ai.semantic_search.limit"
                defaultMessage="Max Results per Type"
              />
            </Form.Label>
            <Form.Control
              type="number"
              min={1}
              max={100}
              value={limit}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
                setLimit(Number(e.currentTarget.value))
              }
            />
          </Form.Group>
        </Form.Row>

        <Form.Group>
          <Form.Label>
            <FormattedMessage
              id="ai.semantic_search.model"
              defaultMessage="Embedding Model"
            />
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
            <FormattedMessage
              id="ai.semantic_search.model_desc"
              defaultMessage="Optional: specify embedding model. Leave empty to use configured default."
            />
          </Form.Text>
        </Form.Group>

        {results !== null && (
          <div className="mt-4">
            <div className="d-flex justify-content-between align-items-center mb-3">
              <h6>
                {loading
                  ? intl.formatMessage({
                      id: "actions.searching",
                      defaultMessage: "Searching...",
                    })
                  : intl.formatMessage(
                      {
                        id: "ai.semantic_search.results",
                        defaultMessage: "Results",
                      },
                      { count: results.length }
                    )}
              </h6>
              {results.length === 0 && !loading && (
                <span className="text-muted">
                  {intl.formatMessage({
                    id: "ai.semantic_search.no_results",
                    defaultMessage: "No results found",
                  })}
                </span>
              )}
            </div>

            {results.length > 0 && (
              <div className="row">{results.map(renderResult)}</div>
            )}
          </div>
        )}
      </Form>
    </ModalComponent>
  );
};

export default SemanticSearchDialog;
