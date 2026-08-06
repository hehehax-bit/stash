import React, { useCallback, useEffect, useRef, useState } from "react";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { Icon } from "src/components/Shared/Icon";
import {
  faRobot,
  faPaperPlane,
  faTrashAlt,
  faCog,
  faFilm,
  faUser,
  faImage,
  faBuilding,
  faTag,
  faImages,
  faLayerGroup,
  faPlusCircle,
  faCamera,
  faTimes,
  faRedo,
  faExclamationTriangle,
  faArrowLeft,
  faEye,
} from "@fortawesome/free-solid-svg-icons";
import { useHistory, useLocation } from "react-router-dom";
import { useRemarkSync } from "react-remark";
import remarkGfm from "remark-gfm";
import { useApolloClient } from "@apollo/client";
import { useToast } from "src/hooks/Toast";
import { FormattedMessage, useIntl } from "react-intl";

const MAX_ATTACHMENT_SIZE = 20 * 1024 * 1024;

function playNotificationSound() {
  try {
    const audioCtx = new (
      window.AudioContext ||
      (window as unknown as { webkitAudioContext: typeof AudioContext })
        .webkitAudioContext
    )();
    const oscillator = audioCtx.createOscillator();
    const gainNode = audioCtx.createGain();
    oscillator.connect(gainNode);
    gainNode.connect(audioCtx.destination);
    oscillator.type = "sine";
    oscillator.frequency.setValueAtTime(880, audioCtx.currentTime); // A5
    gainNode.gain.setValueAtTime(0.1, audioCtx.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(
      0.01,
      audioCtx.currentTime + 0.2
    );
    oscillator.start(audioCtx.currentTime);
    oscillator.stop(audioCtx.currentTime + 0.2);
  } catch {
    // Ignore audio errors (e.g., autoplay policy, no audio context)
  }
}

interface DisplayMessage extends GQL.AiChatMessage {
  isOptimistic?: boolean;
  error?: string;
  pendingImage?: string | null;
}

function linkifyEntities(text: string): string {
  const entityRoutes: Record<string, string> = {
    scene: "scenes",
    Scene: "scenes",
    scenes: "scenes",
    Scenes: "scenes",
    performer: "performers",
    Performer: "performers",
    performers: "performers",
    Performers: "performers",
    image: "images",
    Image: "images",
    images: "images",
    Images: "images",
    studio: "studios",
    Studio: "studios",
    studios: "studios",
    Studios: "studios",
    tag: "tags",
    Tag: "tags",
    tags: "tags",
    Tags: "tags",
    gallery: "galleries",
    Gallery: "galleries",
    galleries: "galleries",
    Galleries: "galleries",
    group: "groups",
    Group: "groups",
    groups: "groups",
    Groups: "groups",
  };

  const entityPattern = Object.keys(entityRoutes)
    .map((k) => k.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"))
    .join("|");

  let result = text;

  // Expand bare #id refs in comma/and/or-separated lists
  // "Images #1, #2, #3" -> "Images #1, Images #2, Images #3"
  const listPattern = new RegExp(
    `\\b(${entityPattern})\\s+#(\\d+)([\\s,;]*(?:and|or)?\\s*)#(\\d+)\\b`,
    "gi"
  );
  {
    let prev = "";
    while (prev !== result) {
      prev = result;
      result = result.replace(
        listPattern,
        (_match, entity, id1, _sep, id2) =>
          `${entity} #${id1}, ${entity} #${id2}`
      );
    }
  }

  const entityRefRegex = new RegExp(`\\b(${entityPattern})\\s+#(\\d+)\\b`, "g");
  result = result.replace(entityRefRegex, (_match, entity, id) => {
    const route = entityRoutes[entity as keyof typeof entityRoutes];
    return `[${entity} #${id}](${route}/${id})`;
  });

  const pathRegex =
    /\/(scenes|performers|images|studios|tags|galleries|groups)\/(\d+)\b/g;
  result = result.replace(pathRegex, (match, type, id) => {
    const label =
      type.charAt(0).toUpperCase() + type.slice(1).replace(/s$/, "");
    return `[${label} #${id}](${match})`;
  });

  const fileUrlRegex = /\[([^\]]*)\]\(file:\/\/[^)]+\)/g;
  result = result.replace(fileUrlRegex, "$1");

  return result;
}

type EntityType =
  | "scenes"
  | "performers"
  | "images"
  | "studios"
  | "tags"
  | "galleries"
  | "groups";

interface EntityRef {
  type: EntityType;
  id: string;
}

function extractEntityRefs(text: string): EntityRef[] {
  const pathPattern =
    /\/(scenes|performers|images|studios|tags|galleries|groups)\/(\d+)/g;
  const namePattern =
    /\b(scene|scenes|performer|performers|image|images|studio|studios|tag|tags|gallery|galleries|group|groups)\s+#(\d+)/gi;
  const barePattern = /(?:[,;]\s*|\b(?:and|or)\s+)#(\d+)\b/gi;
  const seen = new Set<string>();
  const refs: EntityRef[] = [];
  const addRef = (type: string, id: string) => {
    const key = `${type}:${id}`;
    if (!seen.has(key)) {
      seen.add(key);
      refs.push({ type: type as EntityType, id });
    }
  };
  let match: RegExpExecArray | null;
  for (
    match = pathPattern.exec(text);
    match !== null;
    match = pathPattern.exec(text)
  ) {
    addRef(match[1], match[2]);
  }

  const namedPositions: Array<{ type: string; index: number }> = [];
  for (
    match = namePattern.exec(text);
    match !== null;
    match = namePattern.exec(text)
  ) {
    const singular = match[1].replace(/s$/, "").toLowerCase();
    const pluralMap: Record<string, string> = {
      scene: "scenes",
      performer: "performers",
      image: "images",
      studio: "studios",
      tag: "tags",
      gallery: "galleries",
      group: "groups",
    };
    const type = pluralMap[singular] ?? singular + "s";
    addRef(type, match[2]);
    namedPositions.push({ type, index: match.index });
  }

  for (
    match = barePattern.exec(text);
    match !== null;
    match = barePattern.exec(text)
  ) {
    const pos = match.index;
    let closestType: string | null = null;
    let closestDist = 200;
    for (const np of namedPositions) {
      const dist = pos - np.index;
      if (dist > 0 && dist < closestDist) {
        closestDist = dist;
        closestType = np.type;
      }
    }
    if (closestType) {
      addRef(closestType, match[1]);
    }
  }

  // Parse markdown table format from semantic search results:
  // ## Scenes (10 results...)
  // | # | Title | Similarity |
  // |---|-------|------------|
  // | 19919 | — | 72.7% |
  // Match header + all subsequent table rows (lines starting with |)
  const mdTablePattern =
    /(?:^|\n)##\s+(Scenes|Images)\s*\([^)]*\)\s*\n((?:\|[^\n]*\n)+)/gi;
  for (
    match = mdTablePattern.exec(text);
    match !== null;
    match = mdTablePattern.exec(text)
  ) {
    const sectionType = match[1].toLowerCase();
    const type = sectionType === "scenes" ? "scenes" : "images";
    const tableText = match[2]; // All table lines including header, separator, data

    const lines = tableText.split("\n");
    // Find header row index (contains #)
    let headerRowIndex = -1;
    for (let i = 0; i < lines.length; i++) {
      const cols = lines[i].split("|");
      if (cols.length >= 2) {
        const col1 = cols[1]?.trim() || "";
        if (col1 === "#") {
          headerRowIndex = i;
          break;
        }
      }
    }
    if (headerRowIndex === -1) continue;

    // Parse data rows after header and separator
    for (let i = headerRowIndex + 1; i < lines.length; i++) {
      const line = lines[i].trim();
      if (!line.startsWith("|")) break;
      if (/^\|[\s\-:|]+\|$/.test(line)) continue; // separator row

      const cols = line.split("|");
      if (cols.length >= 2) {
        const id = cols[1].trim();
        if (/^\d+$/.test(id)) {
          addRef(type, id);
        } else {
          break;
        }
      }
    }
  }

  // Parse HTML table format from semantic search results:
  // <h2>Scenes (10 results...)</h2><table>...<th>#</th>...<tbody>...<td>19919</td>...
  const htmlTablePattern =
    /<h2[^>]*>(Scenes|Images)\s*\([^)]*\)<\/h2>\s*<table[^>]*>[\s\S]*?<th[^>]*>#<\/th>[\s\S]*?<tbody[^>]*>([\s\S]*?)<\/tbody>/gi;
  for (
    match = htmlTablePattern.exec(text);
    match !== null;
    match = htmlTablePattern.exec(text)
  ) {
    const sectionType = match[1].toLowerCase();
    const type = sectionType === "scenes" ? "scenes" : "images";
    const tbodyContent = match[2];

    // Extract IDs from <td> cells in the first column
    const tdPattern = /<tr[^>]*>\s*<td[^>]*>(\d+)<\/td>/gi;
    let tdMatch: RegExpExecArray | null;
    for (
      tdMatch = tdPattern.exec(tbodyContent);
      tdMatch !== null;
      tdMatch = tdPattern.exec(tbodyContent)
    ) {
      addRef(type, tdMatch[1]);
    }
  }

  // Fallback: parse any HTML table with # header and numeric first column
  // (handles cases where <h2> or <thead> structure varies)
  const anyHtmlTablePattern =
    /<table[^>]*>[\s\S]*?<th[^>]*>#<\/th>[\s\S]*?<tbody[^>]*>([\s\S]*?)<\/tbody>/gi;
  for (
    match = anyHtmlTablePattern.exec(text);
    match !== null;
    match = anyHtmlTablePattern.exec(text)
  ) {
    // Try to infer type from preceding heading
    let type = "images"; // default
    const tableStart = match.index;
    const beforeTable = text.slice(Math.max(0, tableStart - 200), tableStart);
    if (/Scenes/i.test(beforeTable)) type = "scenes";
    else if (/Images/i.test(beforeTable)) type = "images";

    const tbodyContent = match[1];
    const tdPattern = /<tr[^>]*>\s*<td[^>]*>(\d+)<\/td>/gi;
    let tdMatch: RegExpExecArray | null;
    for (
      tdMatch = tdPattern.exec(tbodyContent);
      tdMatch !== null;
      tdMatch = tdPattern.exec(tbodyContent)
    ) {
      addRef(type, tdMatch[1]);
    }
  }

  // Parse bullet list format: <strong>#20603</strong> or <li>#20603</li>
  const bulletIdPattern = /<[^>]*>#(\d+)<\/[^>]*>/gi;
  for (
    match = bulletIdPattern.exec(text);
    match !== null;
    match = bulletIdPattern.exec(text)
  ) {
    // Infer type from context
    let type = "images"; // default
    const beforeId = text.slice(Math.max(0, match.index - 200), match.index);
    if (/Scenes/i.test(beforeId)) type = "scenes";
    else if (/Images/i.test(beforeId)) type = "images";
    addRef(type, match[1]);
  }

  // Parse plain text table format: "Scenes (N total)" followed by table with # column
  const tableSectionPattern = /(Scenes|Images)\s*\(\d+\s*total\)/gi;
  const tableHeaderPattern = /^#\t/;
  for (
    match = tableSectionPattern.exec(text);
    match !== null;
    match = tableSectionPattern.exec(text)
  ) {
    const sectionType = match[1].toLowerCase();
    const type = sectionType === "scenes" ? "scenes" : "images";
    const sectionStart = match.index + match[0].length;
    const sectionText = text.slice(sectionStart);

    const lines = sectionText.split("\n");
    let headerFound = false;
    for (const line of lines) {
      if (!headerFound) {
        if (tableHeaderPattern.test(line)) {
          headerFound = true;
        }
        continue;
      }
      const cols = line.split("\t");
      if (cols.length > 0) {
        const id = cols[0].trim();
        if (/^\d+$/.test(id)) {
          addRef(type, id);
        } else {
          break;
        }
      }
    }
  }

  return refs;
}

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  const s = Math.floor(seconds % 60);
  if (h > 0)
    return `${h}:${m.toString().padStart(2, "0")}:${s.toString().padStart(2, "0")}`;
  return `${m}:${s.toString().padStart(2, "0")}`;
}

const entityQueries = {
  scenes: {
    query: GQL.ScenePreviewDocument,
    map: (data: GQL.ScenePreviewQuery) => ({
      title: data.findScene?.title ?? `Scene #${data.findScene?.id}`,
      image: data.findScene?.paths?.screenshot ?? null,
      video:
        data.findScene?.paths?.stream ?? data.findScene?.paths?.preview ?? null,
      subtitle: data.findScene?.date
        ? `${data.findScene.date}${data.findScene.files?.[0]?.duration ? ` · ${formatDuration(data.findScene.files[0].duration)}` : ""}${data.findScene.o_counter != null ? ` · ${data.findScene.o_counter} orgasms` : ""}${data.findScene.play_count != null ? ` · ${data.findScene.play_count} plays` : ""}`
        : data.findScene?.files?.[0]?.duration
          ? `${formatDuration(data.findScene.files[0].duration)}${data.findScene.o_counter != null ? ` · ${data.findScene.o_counter} orgasms` : ""}${data.findScene.play_count != null ? ` · ${data.findScene.play_count} plays` : ""}`
          : [
              data.findScene?.o_counter != null
                ? `${data.findScene.o_counter} orgasms`
                : null,
              data.findScene?.play_count != null
                ? `${data.findScene.play_count} plays`
                : null,
            ]
              .filter(Boolean)
              .join(" · ") || null,
    }),
  },
  performers: {
    query: GQL.PerformerPreviewDocument,
    map: (data: GQL.PerformerPreviewQuery) => ({
      title: data.findPerformer?.name ?? `Performer #${data.findPerformer?.id}`,
      image: data.findPerformer?.image_path ?? null,
      video: null,
      subtitle: null,
    }),
  },
  images: {
    query: GQL.ImagePreviewDocument,
    map: (data: GQL.ImagePreviewQuery) => ({
      title: data.findImage?.title ?? `Image #${data.findImage?.id}`,
      image:
        data.findImage?.paths?.image ??
        data.findImage?.paths?.thumbnail ??
        null,
      video: null,
      subtitle: null,
    }),
  },
  studios: {
    query: GQL.StudioPreviewDocument,
    map: (data: GQL.StudioPreviewQuery) => ({
      title: data.findStudio?.name ?? `Studio #${data.findStudio?.id}`,
      image: data.findStudio?.image_path ?? null,
      video: null,
      subtitle: null,
    }),
  },
  tags: {
    query: GQL.TagPreviewDocument,
    map: (data: GQL.TagPreviewQuery) => ({
      title: data.findTag?.name ?? `Tag #${data.findTag?.id}`,
      image: data.findTag?.image_path ?? null,
      video: null,
      subtitle: null,
    }),
  },
  galleries: {
    query: GQL.GalleryPreviewDocument,
    map: (data: GQL.GalleryPreviewQuery) => ({
      title: data.findGallery?.title ?? `Gallery #${data.findGallery?.id}`,
      image: data.findGallery?.cover?.paths?.thumbnail ?? null,
      video: null,
      subtitle: null,
    }),
  },
  groups: {
    query: GQL.GroupPreviewDocument,
    map: (data: GQL.GroupPreviewQuery) => ({
      title: data.findGroup?.name ?? `Group #${data.findGroup?.id}`,
      image: data.findGroup?.front_image_path ?? null,
      video: null,
      subtitle: null,
    }),
  },
};

function entityIcon(type: string) {
  switch (type) {
    case "scenes":
      return faFilm;
    case "performers":
      return faUser;
    case "images":
      return faImage;
    case "studios":
      return faBuilding;
    case "tags":
      return faTag;
    case "galleries":
      return faImages;
    case "groups":
      return faLayerGroup;
    default:
      return faFilm;
  }
}

const SceneVideoPreview: React.FC<{ image: string; video: string }> = ({
  image,
  video,
}) => (
  <div className="entity-preview-image-container entity-preview-video-container">
    <img src={image} alt="" className="entity-preview-image" />
    <video
      src={video}
      className="entity-preview-video"
      loop
      muted
      playsInline
      controls
      disableRemotePlayback
    />
  </div>
);

const EntityPreviewCard: React.FC<{ entityRef: EntityRef }> = ({
  entityRef,
}) => {
  const client = useApolloClient();
  const [data, setData] = useState<{
    title: string;
    image: string | null;
    video: string | null;
    subtitle: string | null;
  } | null>(null);
  const [loading, setLoading] = useState(true);
  const [expanded, setExpanded] = useState(false);

  const toggleExpand = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      e.stopPropagation();
      setExpanded(!expanded);
    },
    [expanded]
  );

  useEffect(() => {
    const entry = entityQueries[entityRef.type];
    if (!entry) {
      setLoading(false);
      return;
    }
    let cancelled = false;
    client
      .query({ query: entry.query, variables: { id: entityRef.id } })
      .then((result) => {
        if (!cancelled) setData(entry.map(result.data));
      })
      .catch(() => {
        if (!cancelled) setData(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [entityRef.type, entityRef.id, client]);

  if (loading) {
    return (
      <a
        href={`/${entityRef.type}/${entityRef.id}`}
        className="entity-preview-card entity-preview-loading"
      >
        <div className="entity-preview-icon">
          <Icon icon={entityIcon(entityRef.type)} />
        </div>
        <div className="entity-preview-info">
          <div className="entity-preview-type">
            {entityRef.type.slice(0, -1)}
          </div>
          <div className="entity-preview-title">#{entityRef.id}</div>
        </div>
      </a>
    );
  }

  if (!data) return null;

  return (
    <div className="entity-preview-card" onClick={toggleExpand}>
      {data.video && data.image ? (
        <SceneVideoPreview image={data.image} video={data.video} />
      ) : data.image ? (
        <div className="entity-preview-image-container">
          <img
            src={data.image}
            alt=""
            className="entity-preview-image"
            loading="lazy"
          />
        </div>
      ) : (
        <div className="entity-preview-icon">
          <Icon icon={entityIcon(entityRef.type)} />
        </div>
      )}
      <a
        href={`/${entityRef.type}/${entityRef.id}`}
        className="entity-preview-info"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="entity-preview-type">{entityRef.type.slice(0, -1)}</div>
        <div className="entity-preview-title">{data.title}</div>
        {data.subtitle && (
          <div className="entity-preview-subtitle">{data.subtitle}</div>
        )}
      </a>
      {expanded && (
        <div className="entity-preview-expanded" onClick={toggleExpand}>
          <div className="entity-preview-expanded-content">
            <button
              className="entity-preview-close"
              onClick={(e) => {
                e.stopPropagation();
                setExpanded(false);
              }}
            >
              ×
            </button>
            {data.video && data.image ? (
              <video
                src={data.video}
                className="entity-preview-expanded-video"
                loop
                muted
                playsInline
                controls
                disableRemotePlayback
                autoPlay
              />
            ) : data.image ? (
              <img
                src={data.image}
                alt={data.title}
                className="entity-preview-expanded-image"
              />
            ) : (
              <div className="entity-preview-icon">
                <Icon icon={entityIcon(entityRef.type)} size="4x" />
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

const AiMarkdownContent: React.FC<{ text: string }> = ({ text }) =>
  useRemarkSync(linkifyEntities(text), {
    remarkPlugins: [remarkGfm],
  });

const AiMessage: React.FC<{
  message: DisplayMessage;
  onDelete: (id: string) => void;
  onRetry?: (id: string) => void;
}> = ({ message, onDelete, onRetry }) => {
  const entityRefs = extractEntityRefs(message.content);
  const isFailed = message.role === "user" && message.error;
  const [showAllRefs, setShowAllRefs] = useState(false);
  const visibleRefs = showAllRefs
    ? entityRefs
    : entityRefs.slice(0, ENTITY_PREVIEW_LIMIT);

  return (
    <div
      className={`ai-chat-message ${message.role === "user" ? "ai-chat-message-user" : "ai-chat-message-assistant"} ${isFailed ? "ai-chat-message-failed" : ""}`}
    >
      <div className="ai-chat-message-heading">
        <div className="ai-chat-message-role">
          {message.role === "user" ? "You" : "AI"}
        </div>
        <div className="ai-chat-message-actions">
          {isFailed && onRetry && (
            <button
              className="btn btn-minimal ai-chat-message-retry"
              title="Retry"
              onClick={(e) => {
                e.stopPropagation();
                onRetry(message.id);
              }}
            >
              <Icon icon={faRedo} />
            </button>
          )}
          <button
            className="btn btn-minimal ai-chat-message-delete"
            title="Delete message"
            onClick={(e) => {
              e.stopPropagation();
              onDelete(message.id);
            }}
          >
            <Icon icon={faTrashAlt} />
          </button>
        </div>
      </div>
      <div className="ai-chat-message-content">
        {message.role === "assistant" ? (
          <AiMarkdownContent text={message.content} />
        ) : (
          message.content
        )}
      </div>
      {isFailed && message.error && (
        <div className="ai-chat-message-error">
          <Icon icon={faExclamationTriangle} /> {message.error}
        </div>
      )}
      {message.role === "assistant" && entityRefs.length > 0 && (
        <div className="ai-chat-entity-previews">
          {visibleRefs.map((er) => (
            <EntityPreviewCard key={`${er.type}:${er.id}`} entityRef={er} />
          ))}
          {entityRefs.length > ENTITY_PREVIEW_LIMIT && (
            <button
              className="btn btn-sm btn-link ai-chat-preview-more"
              onClick={() => setShowAllRefs(!showAllRefs)}
            >
              {showAllRefs ? (
                <FormattedMessage id="ai_chat.show_fewer" />
              ) : (
                <FormattedMessage
                  id="ai_chat.show_all"
                  values={{ count: entityRefs.length }}
                />
              )}
            </button>
          )}
        </div>
      )}
    </div>
  );
};

// cap how many entity preview cards render per message
const ENTITY_PREVIEW_LIMIT = 4;

const AIChatPanel: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const intl = useIntl();

  const { data: configData } = GQL.useAiConfigQuery();
  const { data: sessionsData, loading: sessionsLoading } =
    GQL.useAiChatSessionsQuery();
  const [sendMessage, { loading: sending }] = GQL.useAiChatSendMutation();
  const [clearSession] = GQL.useAiChatClearMutation();
  const [deleteMessage] = GQL.useAiChatDeleteMessageMutation();

  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [watchingId, setWatchingId] = useState<string | null>(null);
  const [useLibraryContext, setUseLibraryContext] = useState(true);
  const [input, setInput] = useState("");

  // watch-along: opened from a scene player with ?watching=<sceneId>
  useEffect(() => {
    const params = new URLSearchParams(location.search);
    const watching = params.get("watching");
    setWatchingId(watching && watching !== "" ? watching : null);
  }, [location.search]);

  const { data: watchingData } = GQL.useFindSceneQuery({
    variables: { id: watchingId ?? "" },
    skip: !watchingId,
  });
  const watchingTitle = watchingData?.findScene?.title;
  const [imageBase64, setImageBase64] = useState<string | null>(null);
  const [imagePreview, setImagePreview] = useState<string | null>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const Toast = useToast();

  useEffect(() => {
    const params = new URLSearchParams(location.search);
    const sessionId = params.get("session");
    if (sessionId) {
      setActiveSessionId(sessionId);
    }
  }, [location.search]);

  // prefill the input from a ?message= param (e.g. "chat about this scene")
  const prefilledMessageRef = useRef(false);
  useEffect(() => {
    if (prefilledMessageRef.current) return;
    prefilledMessageRef.current = true;

    const params = new URLSearchParams(location.search);
    const msg = params.get("message");
    if (msg) {
      setInput(msg);
      params.delete("message");
      history.replace({ search: params.toString() });
    }
  }, [location.search, history]);

  useEffect(() => {
    const params = new URLSearchParams(location.search);
    if (activeSessionId) {
      params.set("session", activeSessionId);
      history.replace({ search: params.toString() });
    } else {
      params.delete("session");
      history.replace({ search: params.toString() });
    }
  }, [activeSessionId, history, location.search]);

  const { data: historyData } = GQL.useAiChatHistoryQuery({
    variables: { session_id: activeSessionId ?? "" },
    skip: !activeSessionId,
    pollInterval: 0,
  });

  const [displayMessages, setDisplayMessages] = useState<DisplayMessage[]>([]);
  const prevMessageCountRef = useRef(0);

  useEffect(() => {
    if (historyData?.aiChatHistory) {
      const newMessages = historyData.aiChatHistory;
      // Play sound when new AI response arrives
      if (newMessages.length > prevMessageCountRef.current) {
        const lastMessage = newMessages[newMessages.length - 1];
        if (
          lastMessage.role === "assistant" &&
          !("isOptimistic" in lastMessage && lastMessage.isOptimistic)
        ) {
          playNotificationSound();
        }
      }
      prevMessageCountRef.current = newMessages.length;
      setDisplayMessages(newMessages);
    }
  }, [historyData]);

  const chatMessages = displayMessages;
  const scrollToBottom = useCallback(() => {
    setTimeout(
      () => messagesEndRef.current?.scrollIntoView({ behavior: "smooth" }),
      50
    );
  }, []);

  useEffect(() => {
    scrollToBottom();
  });

  async function onSend(messageData?: { text: string; image: string | null }) {
    if (sending) return;
    const text = messageData?.text ?? input.trim();
    const image = messageData?.image ?? imageBase64;
    if (!text && !image) return;

    // watch-along: keep the chat aware of what is playing
    const watchingNote =
      watchingId && watchingTitle
        ? `\n\n[I am currently watching this scene: "${watchingTitle}" (id ${watchingId}) — answer as if you are watching along with me.]`
        : "";
    const finalText = watchingNote !== "" ? text + watchingNote : text;
    if (!messageData) {
      setInput("");
      inputRef.current?.focus();
    }

    const optimisticMsg: DisplayMessage = {
      __typename: "AIChatMessage",
      id: `opt-${Date.now()}`,
      session_id: activeSessionId ?? "",
      role: "user",
      content: text || "[image]",
      created_at: new Date().toISOString(),
      isOptimistic: true,
      pendingImage: image,
    };
    setDisplayMessages((prev) => [...prev, optimisticMsg]);
    scrollToBottom();

    try {
      await sendMessage({
        variables: {
          input: {
            session_id: activeSessionId,
            message: finalText,
            image: image,
            use_library_context: useLibraryContext,
          },
        },
        update: (cache, result) => {
          if (!result.data?.aiChatSend) return;
          const newSessionId = result.data.aiChatSend.session_id;
          if (!activeSessionId) {
            setActiveSessionId(newSessionId);
          }
          cache.evict({ fieldName: "aiChatSessions" });
          cache.evict({ fieldName: "aiChatHistory" });
          cache.gc();
        },
      });
    } catch (e) {
      setDisplayMessages((prev) =>
        prev.map((m) =>
          m.id === optimisticMsg.id
            ? { ...m, error: String(e), isOptimistic: false }
            : m
        )
      );
      Toast.error(e);
    } finally {
      if (!messageData) {
        setImageBase64(null);
        setImagePreview(null);
      }
    }
  }

  async function onRetry(messageId: string) {
    const msg = displayMessages.find((m) => m.id === messageId);
    if (msg?.role !== "user" || !msg.error) return;
    setDisplayMessages((prev) =>
      prev.map((m) => (m.id === messageId ? { ...m, error: undefined } : m))
    );
    await onSend({ text: msg.content, image: msg.pendingImage ?? null });
  }

  function onSelectImage(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    if (file.size > MAX_ATTACHMENT_SIZE) {
      Toast.error(
        `Image too large (${(file.size / 1024 / 1024).toFixed(1)} MB). Maximum attachment size is ${MAX_ATTACHMENT_SIZE / 1024 / 1024} MB.`
      );
      e.target.value = "";
      return;
    }
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      setImageBase64(result);
      setImagePreview(result);
    };
    reader.readAsDataURL(file);
    e.target.value = "";
  }

  function onClearImage() {
    setImageBase64(null);
    setImagePreview(null);
  }

  async function onClear() {
    if (!activeSessionId) return;
    await clearSession({
      variables: { session_id: activeSessionId },
      update: (cache) => {
        cache.evict({ fieldName: "aiChatSessions" });
        cache.gc();
      },
    });
    setActiveSessionId(null);
    setDisplayMessages([]);
  }

  async function onDeleteMessage(messageId: string) {
    await deleteMessage({
      variables: { message_id: messageId },
      update: (cache) => {
        cache.evict({ fieldName: "aiChatHistory" });
        cache.gc();
      },
    });
  }

  async function onDeleteSession(sessionId: string) {
    await clearSession({
      variables: { session_id: sessionId },
      update: (cache) => {
        cache.evict({ fieldName: "aiChatSessions" });
        cache.gc();
      },
    });
    if (activeSessionId === sessionId) {
      setActiveSessionId(null);
      setDisplayMessages([]);
    }
  }

  if (!configData?.aiConfig.enabled) {
    return (
      <div className="ai-chat-panel">
        <div className="ai-chat-empty">
          <Icon icon={faRobot} size="4x" />
          <h2>AI Chat</h2>
          <p>AI is not enabled. Configure it in Settings.</p>
          <button
            className="btn btn-primary"
            onClick={() => history.push("/settings?tab=ai")}
          >
            <Icon icon={faCog} /> AI Settings
          </button>
        </div>
      </div>
    );
  }

  const sessions = sessionsData?.aiChatSessions ?? [];

  return (
    <div className="ai-chat-panel">
      <div className="ai-chat-sidebar">
        <div className="ai-chat-sidebar-header">
          <h5>
            <Icon icon={faRobot} /> AI Chat
          </h5>
          <div className="ai-chat-header-actions">
            <button
              className="btn btn-minimal"
              onClick={() => {
                setActiveSessionId(null);
                setDisplayMessages([]);
                inputRef.current?.focus();
              }}
              title="New chat"
            >
              <Icon icon={faPlusCircle} />
            </button>
            <button
              className="btn btn-minimal"
              onClick={() => history.push("/settings?tab=ai")}
              title="AI Settings"
            >
              <Icon icon={faCog} />
            </button>
          </div>
        </div>
        <div className="ai-chat-sessions">
          {sessions.map((s) => (
            <div
              key={s.id}
              className={`ai-chat-session-item ${s.id === activeSessionId ? "active" : ""}`}
              onClick={() => setActiveSessionId(s.id)}
            >
              <span className="ai-chat-session-label">
                {s.title
                  ? `${s.title} — ${new Date(s.updated_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`
                  : `Session ${new Date(s.updated_at).toLocaleDateString()} ${new Date(s.updated_at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}`}
              </span>
              <button
                className="btn btn-minimal btn-danger ai-chat-session-delete"
                title="Delete session"
                onClick={(e) => {
                  e.stopPropagation();
                  onDeleteSession(s.id);
                }}
              >
                <Icon icon={faTrashAlt} />
              </button>
            </div>
          ))}
          {sessions.length === 0 && !sessionsLoading && (
            <div className="ai-chat-empty-sessions">No sessions yet</div>
          )}
        </div>
        {activeSessionId && (
          <div className="ai-chat-sidebar-footer">
            <button className="btn btn-minimal btn-danger" onClick={onClear}>
              <Icon icon={faTrashAlt} /> Clear
            </button>
          </div>
        )}
      </div>
      <div className="ai-chat-main">
        <div className="ai-chat-topbar">
          <button
            className="btn btn-minimal ai-chat-back"
            onClick={() => history.push("/")}
          >
            <Icon icon={faArrowLeft} /> <FormattedMessage id="ai_chat.back" />
          </button>
          {watchingId && watchingTitle && (
            <div className="ai-chat-watching">
              <Icon icon={faEye} />
              <FormattedMessage id="ai_chat.watching" />{" "}
              <strong>{watchingTitle}</strong>
              <button
                className="btn btn-sm btn-outline-secondary ml-2"
                onClick={() => {
                  setWatchingId(null);
                  const params = new URLSearchParams(location.search);
                  params.delete("watching");
                  history.replace({ search: params.toString() });
                }}
              >
                <FormattedMessage id="ai_chat.stop_watching" />
              </button>
            </div>
          )}
        </div>
        {!activeSessionId && chatMessages.length === 0 ? (
          <div className="ai-chat-empty">
            <Icon icon={faRobot} size="4x" />
            <h3>
              <FormattedMessage id="ai_chat.empty_heading" />
            </h3>
            <p>
              <FormattedMessage id="ai_chat.empty_subheading" />
            </p>
            <div className="ai-chat-suggestions">
              {[
                "ai_chat.suggestion_1",
                "ai_chat.suggestion_2",
                "ai_chat.suggestion_3",
                "ai_chat.suggestion_4",
              ].map((id) => (
                <button
                  key={id}
                  className="btn btn-outline-secondary btn-sm"
                  onClick={() =>
                    onSend({ text: intl.formatMessage({ id }), image: null })
                  }
                >
                  {intl.formatMessage({ id })}
                </button>
              ))}
            </div>
          </div>
        ) : (
          <div className="ai-chat-messages">
            {chatMessages.map((msg) => (
              <AiMessage
                key={msg.id}
                message={msg}
                onDelete={onDeleteMessage}
                onRetry={onRetry}
              />
            ))}
            <div ref={messagesEndRef} />
          </div>
        )}
        <div className="ai-chat-input">
          {imagePreview && (
            <div className="ai-chat-image-preview">
              <img src={imagePreview} alt="Upload preview" />
              <button
                className="btn btn-minimal ai-chat-image-remove"
                onClick={onClearImage}
              >
                <Icon icon={faTimes} />
              </button>
            </div>
          )}
          <label
            className="ai-chat-context-toggle"
            title={intl.formatMessage({
              id: "ai_chat.use_library_context_hint",
            })}
          >
            <input
              type="checkbox"
              checked={useLibraryContext}
              onChange={() => setUseLibraryContext(!useLibraryContext)}
            />
            <span className="ml-1">
              <FormattedMessage id="ai_chat.use_library_context" />
            </span>
          </label>
          <div className="ai-chat-input-row">
            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,image/webp,image/gif"
              className="d-none"
              onChange={onSelectImage}
            />
            <button
              className="btn btn-minimal ai-chat-image-btn"
              onClick={() => fileInputRef.current?.click()}
              title="Attach image"
              disabled={sending}
            >
              <Icon icon={faCamera} />
            </button>
            <textarea
              ref={inputRef}
              className="form-control ai-chat-textarea"
              placeholder={intl.formatMessage({
                id: "ai_chat.input_placeholder",
              })}
              value={input}
              rows={1}
              onChange={(e) => setInput(e.target.value)}
              onInput={(e) => {
                const el = e.currentTarget;
                el.style.height = "auto";
                el.style.height = `${Math.min(el.scrollHeight, 140)}px`;
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  onSend();
                }
              }}
              disabled={sending}
            />
            <button
              className="btn btn-primary"
              onClick={() => onSend()}
              disabled={(!input.trim() && !imageBase64) || sending}
            >
              {sending ? (
                <LoadingIndicator message="" />
              ) : (
                <Icon icon={faPaperPlane} />
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

export default AIChatPanel;
