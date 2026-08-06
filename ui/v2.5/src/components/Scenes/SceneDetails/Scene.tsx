import { Tab, Nav, Dropdown, Button, Form } from "react-bootstrap";
import React, {
  useCallback,
  useEffect,
  useState,
  useMemo,
  useRef,
  useLayoutEffect,
} from "react";
import { FormattedMessage, useIntl } from "react-intl";
import { useHistory, RouteComponentProps } from "react-router-dom";
import { Helmet } from "react-helmet";
import * as GQL from "src/core/generated-graphql";
import {
  mutateMetadataScan,
  mutateMetadataAISceneTag,
  useFindScene,
  useSceneIncrementO,
  useSceneGenerateScreenshot,
  useSceneUpdate,
  queryFindScenes,
  queryFindScenesByID,
  useSceneIncrementPlayCount,
} from "src/core/StashService";
import { ModalComponent } from "src/components/Shared/Modal";
import { AISessionBuildDialog } from "src/components/Dialogs/AISessionBuildDialog/AISessionBuildDialog";

import { SceneEditPanel } from "./SceneEditPanel";
import { ErrorMessage } from "src/components/Shared/ErrorMessage";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { Icon } from "src/components/Shared/Icon";
import { Counter } from "src/components/Shared/Counter";
import { useToast } from "src/hooks/Toast";
import SceneQueue, { QueuedScene } from "src/models/sceneQueue";
import { ListFilterModel } from "src/models/list-filter/filter";
import { SteamScoreCriterion } from "src/models/list-filter/criteria/steam-score";
import {
  MoodsCriterion,
  MoodsCriterionOption,
} from "src/models/list-filter/criteria/moods";
import { IOptionType } from "src/models/list-filter/types";
import Mousetrap from "mousetrap";
import { OrganizedButton } from "./OrganizedButton";
import { useConfigurationContext } from "src/hooks/Config";
import {
  getAbLoopPlugin,
  getPlayerPosition,
} from "src/components/ScenePlayer/util";
import {
  faEllipsisV,
  faChevronRight,
  faChevronLeft,
  faDice,
  faBolt,
  faHourglassHalf,
  faEyeSlash,
  faRadio,
  faComments,
  faDove,
  faClock,
  faHeart,
  faRocket,
} from "@fortawesome/free-solid-svg-icons";
import { objectPath, objectTitle } from "src/core/files";
import { RatingSystem } from "src/components/Shared/Rating/RatingSystem";
import TextUtils from "src/utils/text";
import {
  OCounterButton,
  ViewCountButton,
} from "src/components/Shared/CountButton";
import { useRatingKeybinds } from "src/hooks/keybinds";
import { lazyComponent } from "src/utils/lazyComponent";
import cx from "classnames";
import { TruncatedText } from "src/components/Shared/TruncatedText";
import { PatchComponent, PatchContainerComponent } from "src/patch";
import { SceneMergeModal } from "../SceneMergeDialog";
import { goBackOrReplace } from "src/utils/history";
import { FormattedDate } from "src/components/Shared/Date";
import { StudioLogo } from "src/components/Shared/StudioLogo";
import { JobFragment, useMonitorJob } from "src/utils/job";

const SubmitStashBoxDraft = lazyComponent(
  () => import("src/components/Dialogs/SubmitDraft")
);
const ScenePlayer = lazyComponent(
  () => import("src/components/ScenePlayer/ScenePlayer")
);

const GalleryViewer = lazyComponent(
  () => import("src/components/Galleries/GalleryViewer")
);
const ExternalPlayerButton = lazyComponent(
  () => import("./ExternalPlayerButton")
);

const QueueViewer = lazyComponent(() => import("./QueueViewer"));
const SceneMarkersPanel = lazyComponent(() => import("./SceneMarkersPanel"));
const SceneFileInfoPanel = lazyComponent(() => import("./SceneFileInfoPanel"));
const SceneDetailPanel = lazyComponent(() => import("./SceneDetailPanel"));
const SceneHistoryPanel = lazyComponent(() => import("./SceneHistoryPanel"));
const SceneGroupPanel = lazyComponent(() => import("./SceneGroupPanel"));
const SceneGalleriesPanel = lazyComponent(
  () => import("./SceneGalleriesPanel")
);
const DeleteScenesDialog = lazyComponent(() => import("../DeleteScenesDialog"));
const GenerateDialog = lazyComponent(
  () => import("../../Dialogs/GenerateDialog")
);
const SceneVideoFilterPanel = lazyComponent(
  () => import("./SceneVideoFilterPanel")
);

const VideoFrameRateResolution: React.FC<{
  width?: number;
  height?: number;
  frameRate?: number;
}> = ({ width, height, frameRate }) => {
  const intl = useIntl();

  const resolution = useMemo(() => {
    if (width && height) {
      const r = TextUtils.resolution(width, height);
      return (
        <span className="resolution" data-value={r}>
          {r}
        </span>
      );
    }
    return undefined;
  }, [width, height]);

  const frameRateDisplay = useMemo(() => {
    if (frameRate) {
      return (
        <span className="frame-rate" data-value={frameRate}>
          <FormattedMessage
            id="frames_per_second"
            values={{ value: intl.formatNumber(frameRate ?? 0) }}
          />
        </span>
      );
    }
    return undefined;
  }, [intl, frameRate]);

  const divider = useMemo(() => {
    return resolution && frameRateDisplay ? (
      <span className="divider"> | </span>
    ) : undefined;
  }, [resolution, frameRateDisplay]);

  return (
    <span>
      {frameRateDisplay}
      {divider}
      {resolution}
    </span>
  );
};

interface IProps {
  scene: GQL.SceneDataFragment;
  setTimestamp: (num: number) => void;
  queueScenes: QueuedScene[];
  onQueueNext: () => void;
  onQueuePrevious: () => void;
  onQueueRandom: () => void;
  onQueueSceneClicked: (sceneID: string) => void;
  onDelete: () => void;
  continuePlaylist: boolean;
  queueHasMoreScenes: boolean;
  onQueueMoreScenes: () => void;
  onQueueLessScenes: () => void;
  queueStart: number;
  collapsed: boolean;
  setCollapsed: (state: boolean) => void;
  setContinuePlaylist: (value: boolean) => void;
  onRefreshScene: () => Promise<void>;
  onSessionO?: () => void;
}

interface ISceneParams {
  id: string;
}

const ScenePageTabs = PatchContainerComponent<IProps>("ScenePage.Tabs");
const ScenePageTabContent = PatchContainerComponent<IProps>(
  "ScenePage.TabContent"
);

const ScenePage: React.FC<IProps> = PatchComponent("ScenePage", (props) => {
  const {
    scene,
    setTimestamp,
    queueScenes,
    onQueueNext,
    onQueuePrevious,
    onQueueRandom,
    onQueueSceneClicked,
    onDelete,
    continuePlaylist,
    queueHasMoreScenes,
    onQueueMoreScenes,
    onQueueLessScenes,
    queueStart,
    collapsed,
    setCollapsed,
    setContinuePlaylist,
    onRefreshScene,
  } = props;

  const Toast = useToast();
  const intl = useIntl();
  const history = useHistory();
  const [updateScene] = useSceneUpdate();
  const [generateScreenshot] = useSceneGenerateScreenshot();
  const [screenshotJobID, setScreenshotJobID] = useState<string>();
  const { configuration } = useConfigurationContext();
  const { data: aiConfig } = GQL.useAiConfigQuery();
  const { showStudioText } = configuration?.ui ?? {};

  const [showDraftModal, setShowDraftModal] = useState(false);
  const boxes = configuration?.general?.stashBoxes ?? [];

  const [incrementO] = useSceneIncrementO(scene.id);

  const [incrementPlay] = useSceneIncrementPlayCount();

  function incrementPlayCount() {
    incrementPlay({
      variables: {
        id: scene.id,
      },
    });
  }

  const [organizedLoading, setOrganizedLoading] = useState(false);

  const [activeTabKey, setActiveTabKey] = useState("scene-details-panel");

  const [isMerging, setIsMerging] = useState(false);
  const [isDeleteAlertOpen, setIsDeleteAlertOpen] = useState<boolean>(false);
  const [isGenerateDialogOpen, setIsGenerateDialogOpen] = useState(false);

  const onScreenshotJobComplete = useCallback(
    async (job?: JobFragment) => {
      setScreenshotJobID(undefined);

      if (job?.status === GQL.JobStatus.Failed) {
        Toast.error(job.error);
        return;
      }

      if (job?.status === GQL.JobStatus.Cancelled) {
        return;
      }

      await onRefreshScene();
      Toast.success(intl.formatMessage({ id: "toast.screenshot_generated" }));
    },
    [Toast, intl, onRefreshScene]
  );

  useMonitorJob(screenshotJobID, onScreenshotJobComplete);

  const onIncrementOClick = async () => {
    try {
      await incrementO();
      props.onSessionO?.();
    } catch (e) {
      Toast.error(e);
    }
  };

  async function onAIDetectPerformers() {
    try {
      await mutateMetadataAISceneTag({
        sceneIds: [scene.id],
        createMissingPerformers: true,
        createMissingTags: true,
        performersOnly: true,
      });
      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: "AI Scene Performers" }
        )
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  function onAskAI() {
    const title = scene.title ?? objectTitle(scene);
    history.push(
      `/aiChat?message=${encodeURIComponent(
        `Analyze this scene: ${title} (id ${scene.id})`
      )}`
    );
  }

  function setRating(v: number | null) {
    updateScene({
      variables: {
        input: {
          id: scene.id,
          rating100: v,
        },
      },
    });
  }

  useRatingKeybinds(
    true,
    configuration?.ui.ratingSystemOptions?.type,
    setRating
  );

  // set up hotkeys
  useEffect(() => {
    Mousetrap.bind("a", () => setActiveTabKey("scene-details-panel"));
    Mousetrap.bind("q", () => setActiveTabKey("scene-queue-panel"));
    Mousetrap.bind("e", () => setActiveTabKey("scene-edit-panel"));
    Mousetrap.bind("k", () => setActiveTabKey("scene-markers-panel"));
    Mousetrap.bind("i", () => setActiveTabKey("scene-file-info-panel"));
    // note: "h" is used globally for quick-hide; the history tab has no keybind
    Mousetrap.bind("o", () => {
      onIncrementOClick();
    });
    Mousetrap.bind("p n", () => onQueueNext());
    Mousetrap.bind("p p", () => onQueuePrevious());
    Mousetrap.bind("p r", () => onQueueRandom());
    Mousetrap.bind(",", () => setCollapsed(!collapsed));
    Mousetrap.bind("d d", () => setIsDeleteAlertOpen(true));
    Mousetrap.bind("c c", () => {
      onGenerateScreenshot(getPlayerPosition());
    });
    Mousetrap.bind("c d", () => {
      onGenerateScreenshot();
    });

    return () => {
      Mousetrap.unbind("a");
      Mousetrap.unbind("q");
      Mousetrap.unbind("e");
      Mousetrap.unbind("k");
      Mousetrap.unbind("i");
      Mousetrap.unbind("h");
      Mousetrap.unbind("o");
      Mousetrap.unbind("d d");
      Mousetrap.unbind("p n");
      Mousetrap.unbind("p p");
      Mousetrap.unbind("p r");
      Mousetrap.unbind(",");
      Mousetrap.unbind("c c");
      Mousetrap.unbind("c d");
    };
  });

  async function onSave(input: GQL.SceneCreateInput) {
    await updateScene({
      variables: {
        input: {
          id: scene.id,
          ...input,
        },
      },
    });
    Toast.success(
      intl.formatMessage(
        { id: "toast.updated_entity" },
        { entity: intl.formatMessage({ id: "scene" }).toLocaleLowerCase() }
      )
    );
  }

  const onOrganizedClick = async () => {
    try {
      setOrganizedLoading(true);
      await updateScene({
        variables: {
          input: {
            id: scene.id,
            organized: !scene.organized,
          },
        },
      });
    } catch (e) {
      Toast.error(e);
    } finally {
      setOrganizedLoading(false);
    }
  };

  function onClickMarker(marker: GQL.SceneMarkerDataFragment) {
    const abLoopPlugin = getAbLoopPlugin();
    const opts = abLoopPlugin?.getOptions();
    const start = opts?.start;
    const end = opts?.end;

    const hasLoopRange =
      opts?.enabled &&
      typeof start === "number" &&
      typeof end === "number" &&
      Number.isFinite(start) &&
      Number.isFinite(end);

    if (
      abLoopPlugin &&
      opts &&
      hasLoopRange &&
      (marker.seconds < Math.min(start as number, end as number) ||
        marker.seconds > Math.max(start as number, end as number))
    ) {
      abLoopPlugin.setOptions({
        ...opts,
        enabled: false,
      });
    }

    setTimestamp(marker.seconds);
  }

  function onLoopMarker(marker: GQL.SceneMarkerDataFragment) {
    if (marker.end_seconds == null) return;

    setTimestamp(marker.seconds);
    const start = Math.min(marker.seconds, marker.end_seconds);
    const end = Math.max(marker.seconds, marker.end_seconds);
    const abLoopPlugin = getAbLoopPlugin();
    const opts = abLoopPlugin?.getOptions();

    if (opts && abLoopPlugin) {
      abLoopPlugin.setOptions({
        ...opts,
        start,
        end,
        enabled: true,
      });
    }
  }

  async function onRescan() {
    await mutateMetadataScan({
      paths: [objectPath(scene)],
      rescan: true,
    });

    Toast.success(
      intl.formatMessage(
        { id: "toast.rescanning_entity" },
        {
          count: 1,
          singularEntity: intl
            .formatMessage({ id: "scene" })
            .toLocaleLowerCase(),
        }
      )
    );
  }

  async function onGenerateScreenshot(at?: number) {
    try {
      const result = await generateScreenshot({
        variables: {
          id: scene.id,
          at,
        },
      });
      const jobID = result.data?.sceneGenerateScreenshot;
      if (jobID) {
        setScreenshotJobID(jobID);
      }
      Toast.success(intl.formatMessage({ id: "toast.generating_screenshot" }));
    } catch (e) {
      Toast.error(e);
    }
  }

  function onDeleteDialogClosed(deleted: boolean) {
    setIsDeleteAlertOpen(false);
    if (deleted) {
      onDelete();
    }
  }

  function maybeRenderMergeDialog() {
    if (!scene.id) return;
    return (
      <SceneMergeModal
        show={isMerging}
        onClose={(mergedId) => {
          setIsMerging(false);
          if (mergedId !== undefined && mergedId !== scene.id) {
            // By default, the merge destination is the current scene, but
            // the user can change it, in which case we need to redirect.
            history.replace(`/scenes/${mergedId}`);
          }
        }}
        scenes={[{ id: scene.id, title: objectTitle(scene) }]}
      />
    );
  }

  function maybeRenderDeleteDialog() {
    if (isDeleteAlertOpen) {
      return (
        <DeleteScenesDialog selected={[scene]} onClose={onDeleteDialogClosed} />
      );
    }
  }

  function maybeRenderSceneGenerateDialog() {
    if (isGenerateDialogOpen) {
      return (
        <GenerateDialog
          selectedIds={[scene.id]}
          onClose={() => {
            setIsGenerateDialogOpen(false);
          }}
          type="scene"
        />
      );
    }
  }

  const renderOperations = () => (
    <Dropdown>
      <Dropdown.Toggle
        variant="secondary"
        id="operation-menu"
        className="minimal"
        title={intl.formatMessage({ id: "operations" })}
      >
        <Icon icon={faEllipsisV} />
      </Dropdown.Toggle>
      <Dropdown.Menu className="bg-secondary text-white">
        {!!scene.files.length && (
          <Dropdown.Item
            key="rescan"
            className="bg-secondary text-white"
            onClick={() => onRescan()}
          >
            <FormattedMessage id="actions.rescan" />
          </Dropdown.Item>
        )}
        <Dropdown.Item
          key="generate"
          className="bg-secondary text-white"
          onClick={() => setIsGenerateDialogOpen(true)}
        >
          <FormattedMessage id="actions.generate" />…
        </Dropdown.Item>
        {aiConfig?.aiConfig?.enabled && (
          <Dropdown.Item
            key="ai-detect-performers"
            className="bg-secondary text-white"
            onClick={() => onAIDetectPerformers()}
          >
            <FormattedMessage id="actions.ai_detect_performers" />
          </Dropdown.Item>
        )}
        <Dropdown.Item
          key="ask-ai"
          className="bg-secondary text-white"
          onClick={() => onAskAI()}
        >
          <FormattedMessage id="actions.ask_ai" />
        </Dropdown.Item>
        {boxes.length > 0 && (
          <Dropdown.Item
            key="submit"
            className="bg-secondary text-white"
            onClick={() => setShowDraftModal(true)}
          >
            <FormattedMessage id="actions.submit_stash_box" />
          </Dropdown.Item>
        )}
        <Dropdown.Item
          key="merge-scene"
          className="bg-secondary text-white"
          onClick={() => setIsMerging(true)}
        >
          <FormattedMessage id="actions.merge" />
          ...
        </Dropdown.Item>
        <Dropdown.Item
          key="delete-scene"
          className="bg-secondary text-white"
          onClick={() => setIsDeleteAlertOpen(true)}
        >
          <FormattedMessage
            id="actions.delete"
            values={{ entityType: intl.formatMessage({ id: "scene" }) }}
          />
        </Dropdown.Item>
      </Dropdown.Menu>
    </Dropdown>
  );

  const renderTabs = () => (
    <Tab.Container
      activeKey={activeTabKey}
      onSelect={(k) => k && setActiveTabKey(k)}
    >
      <div>
        <Nav variant="tabs" className="mr-auto">
          <ScenePageTabs {...props}>
            <Nav.Item>
              <Nav.Link eventKey="scene-details-panel">
                <FormattedMessage id="details" />
              </Nav.Link>
            </Nav.Item>
            {queueScenes.length > 0 ? (
              <Nav.Item>
                <Nav.Link eventKey="scene-queue-panel">
                  <FormattedMessage id="queue" />
                </Nav.Link>
              </Nav.Item>
            ) : (
              ""
            )}
            <Nav.Item>
              <Nav.Link eventKey="scene-markers-panel">
                <FormattedMessage id="markers" />
              </Nav.Link>
            </Nav.Item>
            {scene.groups.length > 0 ? (
              <Nav.Item>
                <Nav.Link eventKey="scene-group-panel">
                  <FormattedMessage
                    id="countables.groups"
                    values={{ count: scene.groups.length }}
                  />
                </Nav.Link>
              </Nav.Item>
            ) : (
              ""
            )}
            {scene.galleries.length >= 1 ? (
              <Nav.Item>
                <Nav.Link eventKey="scene-galleries-panel">
                  <FormattedMessage
                    id="countables.galleries"
                    values={{ count: scene.galleries.length }}
                  />
                </Nav.Link>
              </Nav.Item>
            ) : undefined}
            <Nav.Item>
              <Nav.Link eventKey="scene-video-filter-panel">
                <FormattedMessage id="effect_filters.name" />
              </Nav.Link>
            </Nav.Item>
            <Nav.Item>
              <Nav.Link eventKey="scene-file-info-panel">
                <FormattedMessage id="file_info" />
                <Counter count={scene.files.length} hideZero hideOne />
              </Nav.Link>
            </Nav.Item>
            <Nav.Item>
              <Nav.Link eventKey="scene-history-panel">
                <FormattedMessage id="history" />
              </Nav.Link>
            </Nav.Item>
            <Nav.Item>
              <Nav.Link eventKey="scene-edit-panel">
                <FormattedMessage id="actions.edit" />
              </Nav.Link>
            </Nav.Item>
          </ScenePageTabs>
        </Nav>
      </div>

      <Tab.Content>
        <ScenePageTabContent {...props}>
          <Tab.Pane eventKey="scene-details-panel">
            <SceneDetailPanel scene={scene} />
          </Tab.Pane>
          <Tab.Pane eventKey="scene-queue-panel">
            <QueueViewer
              scenes={queueScenes}
              currentID={scene.id}
              continue={continuePlaylist}
              setContinue={setContinuePlaylist}
              onSceneClicked={onQueueSceneClicked}
              onNext={onQueueNext}
              onPrevious={onQueuePrevious}
              onRandom={onQueueRandom}
              start={queueStart}
              hasMoreScenes={queueHasMoreScenes}
              onLessScenes={onQueueLessScenes}
              onMoreScenes={onQueueMoreScenes}
            />
          </Tab.Pane>
          <Tab.Pane eventKey="scene-markers-panel">
            <SceneMarkersPanel
              sceneId={scene.id}
              onClickMarker={onClickMarker}
              onLoopMarker={onLoopMarker}
              isVisible={activeTabKey === "scene-markers-panel"}
            />
          </Tab.Pane>
          <Tab.Pane eventKey="scene-group-panel">
            <SceneGroupPanel scene={scene} />
          </Tab.Pane>
          {scene.galleries.length >= 1 && (
            <Tab.Pane eventKey="scene-galleries-panel">
              <SceneGalleriesPanel galleries={scene.galleries} />
              {scene.galleries.length === 1 && (
                <GalleryViewer galleryId={scene.galleries[0].id} />
              )}
            </Tab.Pane>
          )}
          <Tab.Pane eventKey="scene-video-filter-panel">
            <SceneVideoFilterPanel scene={scene} />
          </Tab.Pane>
          <Tab.Pane
            className="file-info-panel"
            eventKey="scene-file-info-panel"
          >
            <SceneFileInfoPanel scene={scene} />
          </Tab.Pane>
          <Tab.Pane eventKey="scene-edit-panel" mountOnEnter>
            <SceneEditPanel
              isVisible={activeTabKey === "scene-edit-panel"}
              scene={scene}
              onGenerateThumbFromCurrent={() =>
                onGenerateScreenshot(getPlayerPosition())
              }
              onGenerateThumbDefault={() => onGenerateScreenshot()}
              onSubmit={onSave}
              onDelete={() => setIsDeleteAlertOpen(true)}
            />
          </Tab.Pane>
          <Tab.Pane eventKey="scene-history-panel">
            <SceneHistoryPanel scene={scene} />
          </Tab.Pane>
        </ScenePageTabContent>
      </Tab.Content>
    </Tab.Container>
  );

  function getCollapseButtonIcon() {
    return collapsed ? faChevronRight : faChevronLeft;
  }

  const title = objectTitle(scene);

  const file = useMemo(
    () => (scene.files.length > 0 ? scene.files[0] : undefined),
    [scene]
  );

  return (
    <>
      <Helmet>
        <title>{title}</title>
      </Helmet>
      {maybeRenderSceneGenerateDialog()}
      {maybeRenderMergeDialog()}
      {maybeRenderDeleteDialog()}
      <div
        className={`scene-tabs order-xl-first order-last ${
          collapsed ? "collapsed" : ""
        }`}
      >
        <div>
          <div className="scene-header-container">
            <StudioLogo studio={scene.studio} showText={showStudioText} />
            <h3 className={cx("scene-header", { "no-studio": !scene.studio })}>
              <TruncatedText lineCount={2} text={title} />
            </h3>
          </div>

          <div className="scene-subheader">
            <span className="date" data-value={scene.date}>
              {!!scene.date && <FormattedDate value={scene.date} />}
            </span>
            <VideoFrameRateResolution
              width={file?.width}
              height={file?.height}
              frameRate={file?.frame_rate}
            />
          </div>

          <div className="scene-toolbar">
            <span className="scene-toolbar-group">
              <RatingSystem
                value={scene.rating100}
                onSetRating={setRating}
                clickToRate
                withoutContext
              />
            </span>
            <span className="scene-toolbar-group">
              <span>
                <ExternalPlayerButton scene={scene} />
              </span>
              <span>
                <ViewCountButton
                  value={scene.play_count ?? 0}
                  onIncrement={() => incrementPlayCount()}
                />
              </span>
              <span>
                <OCounterButton
                  value={scene.o_counter ?? 0}
                  onIncrement={() => onIncrementOClick()}
                />
              </span>
              <span>
                <OrganizedButton
                  loading={organizedLoading}
                  organized={scene.organized}
                  onClick={onOrganizedClick}
                />
              </span>
              <span>{renderOperations()}</span>
            </span>
          </div>
        </div>
        {renderTabs()}
      </div>
      <div className="scene-divider d-none d-xl-block">
        <Button onClick={() => setCollapsed(!collapsed)}>
          <Icon className="fa-fw" icon={getCollapseButtonIcon()} />
        </Button>
      </div>
      <SubmitStashBoxDraft
        type="scene"
        boxes={boxes}
        entity={scene}
        show={showDraftModal}
        onHide={() => setShowDraftModal(false)}
      />
    </>
  );
});

// atmosphere layers crossed by a session's cumulative altitude, in km
const ATMO_LAYERS = [10, 30, 50, 80, 100];

const WhoIsSheChip: React.FC<{ sceneId?: string }> = ({ sceneId }) => {
  const history = useHistory();
  const { data } = GQL.useFindSceneMarkerTagsQuery({
    variables: { id: sceneId ?? "" },
    skip: !sceneId,
  });
  const markers =
    data?.sceneMarkerTags.flatMap((tag) => tag.scene_markers) ?? [];
  const [time, setTime] = useState(0);

  useEffect(() => {
    const t = setInterval(() => setTime(getPlayerPosition() ?? 0), 1000);
    return () => clearInterval(t);
  }, []);

  const sorted = [...markers].sort((a, b) => a.seconds - b.seconds);
  const idx = sorted.findIndex((m) => m.seconds <= time);
  const current = idx >= 0 ? sorted[idx] : null;
  const performer = current?.performers?.[0];
  if (!performer) return null;

  return (
    <Button
      variant="outline-primary"
      className="who-is-she-chip"
      onClick={() => history.push(`/performers/${performer.id}`)}
    >
      {performer.name} ✨
    </Button>
  );
};

const ClimaxProjection: React.FC<{
  sceneId?: string;
  active: boolean;
}> = ({ sceneId, active }) => {
  const { data } = GQL.useFindSceneMarkerTagsQuery({
    variables: { id: sceneId ?? "" },
    skip: !sceneId || !active,
  });
  const markers =
    data?.sceneMarkerTags.flatMap((tag) => tag.scene_markers) ?? [];
  const [time, setTime] = useState(0);

  useEffect(() => {
    if (!active) return;
    const t = setInterval(() => setTime(getPlayerPosition() ?? 0), 250);
    return () => clearInterval(t);
  }, [active]);

  const intensityMarkers = markers.filter(
    (m) => m.intensity !== null && m.intensity !== undefined
  );
  if (!active || intensityMarkers.length < 2) return null;

  const total =
    Math.max(...intensityMarkers.map((m) => m.end_seconds ?? m.seconds)) || 1;
  const pos = Math.min(100, (time / total) * 100);

  return (
    <div className="climax-projection">
      {intensityMarkers.map((m) => (
        <div
          key={m.id}
          className="climax-proj-peak"
          style={{
            height: `${(m.intensity ?? 0) * 10}%`,
            left: `${(m.seconds / total) * 100}%`,
          }}
        />
      ))}
      <div className="climax-proj-indicator" style={{ left: `${pos}%` }} />
    </div>
  );
};

const SceneLoader: React.FC<RouteComponentProps<ISceneParams>> = ({
  location,
  history,
  match,
}) => {
  const intl = useIntl();
  const Toast = useToast();
  const { id } = match.params;
  const { configuration } = useConfigurationContext();
  const { data, loading, error, refetch } = useFindScene(id);

  const [scene, setScene] = useState<GQL.SceneDataFragment>();

  const onRefreshScene = useCallback(async () => {
    const result = await refetch();
    if (result.data?.findScene) {
      setScene(result.data.findScene);
    }
  }, [refetch]);

  // useLayoutEffect to update before paint
  useLayoutEffect(() => {
    // only update scene when loading is done
    if (!loading) {
      setScene(data?.findScene ?? undefined);
    }
  }, [data, loading]);

  const queryParams = useMemo(
    () => new URLSearchParams(location.search),
    [location.search]
  );
  const sceneQueue = useMemo(
    () => SceneQueue.fromQueryParameters(queryParams),
    [queryParams]
  );
  const queryContinue = useMemo(() => {
    const cont = queryParams.get("continue");
    if (cont) {
      return cont === "true";
    } else {
      return !!configuration?.interface.continuePlaylistDefault;
    }
  }, [configuration?.interface.continuePlaylistDefault, queryParams]);

  const [queueScenes, setQueueScenes] = useState<QueuedScene[]>([]);

  const [collapsed, setCollapsed] = useState(false);
  const [continuePlaylist, setContinuePlaylist] = useState(queryContinue);
  const [hideScrubber, setHideScrubber] = useState(
    !(configuration?.interface.showScrubber ?? true)
  );

  const _setTimestamp = useRef<(value: number) => void>();
  const initialTimestamp = useMemo(() => {
    const t = queryParams.get("t");
    if (!t) return 0;

    const n = Number(t);
    if (Number.isNaN(n)) return 0;
    return n;
  }, [queryParams]);

  const [queueTotal, setQueueTotal] = useState(0);
  const [queueStart, setQueueStart] = useState(1);

  const autoplay = queryParams.get("autoplay") === "true";
  const autoPlayOnSelected =
    configuration?.interface.autostartVideoOnPlaySelected ?? false;

  const currentQueueIndex = useMemo(
    () => queueScenes.findIndex((s) => s.id === id),
    [queueScenes, id]
  );

  function getSetTimestamp(fn: (value: number) => void) {
    _setTimestamp.current = fn;
  }

  const _pausePlayer = useRef<() => void>(() => {});
  function getPausePlayer(fn: () => void) {
    _pausePlayer.current = fn;
  }

  const _playPlayer = useRef<() => void>(() => {});
  function getPlayPlayer(fn: () => void) {
    _playPlayer.current = fn;
  }

  function setTimestamp(value: number) {
    if (_setTimestamp.current) {
      _setTimestamp.current(value);
    }
  }

  // set up hotkeys
  useEffect(() => {
    Mousetrap.bind(".", () => setHideScrubber((value) => !value));

    return () => {
      Mousetrap.unbind(".");
    };
  }, []);

  useEffect(() => {
    async function getQueueFilterScenes(filter: ListFilterModel) {
      const query = await queryFindScenes(filter);
      const { scenes, count } = query.data.findScenes;
      setQueueScenes(scenes);
      setQueueTotal(count);
      setQueueStart((filter.currentPage - 1) * filter.itemsPerPage + 1);
    }

    async function getQueueScenes(sceneIDs: number[]) {
      const query = await queryFindScenesByID(sceneIDs);
      const { scenes, count } = query.data.findScenes;
      setQueueScenes(scenes);
      setQueueTotal(count);
      setQueueStart(1);
    }

    if (sceneQueue.query) {
      getQueueFilterScenes(sceneQueue.query);
    } else if (sceneQueue.sceneIDs) {
      getQueueScenes(sceneQueue.sceneIDs);
    }
  }, [sceneQueue]);

  async function onQueueLessScenes() {
    if (!sceneQueue.query || queueStart <= 1) {
      return;
    }

    const filterCopy = sceneQueue.query.clone();
    const newStart = queueStart - filterCopy.itemsPerPage;
    filterCopy.currentPage = Math.ceil(newStart / filterCopy.itemsPerPage);
    const query = await queryFindScenes(filterCopy);
    const { scenes } = query.data.findScenes;

    // prepend scenes to scene list
    const newScenes = (scenes as QueuedScene[]).concat(queueScenes);
    setQueueScenes(newScenes);
    setQueueStart(newStart);

    return scenes;
  }

  const queueHasMoreScenes = useMemo(() => {
    return queueStart + queueScenes.length - 1 < queueTotal;
  }, [queueStart, queueScenes, queueTotal]);

  async function onQueueMoreScenes() {
    if (!sceneQueue.query || !queueHasMoreScenes) {
      return;
    }

    const filterCopy = sceneQueue.query.clone();
    const newStart = queueStart + queueScenes.length;
    filterCopy.currentPage = Math.ceil(newStart / filterCopy.itemsPerPage);
    const query = await queryFindScenes(filterCopy);
    const { scenes } = query.data.findScenes;

    // append scenes to scene list
    const newScenes = queueScenes.concat(scenes);
    setQueueScenes(newScenes);
    // don't change queue start
    return scenes;
  }

  function loadScene(sceneID: string, autoPlay?: boolean, newPage?: number) {
    const sceneLink = sceneQueue.makeLink(sceneID, {
      newPage,
      autoPlay,
      continue: continuePlaylist,
    });
    history.replace(sceneLink);
  }

  async function queueNext(autoPlay: boolean) {
    if (currentQueueIndex === -1) return;

    if (currentQueueIndex < queueScenes.length - 1) {
      loadScene(queueScenes[currentQueueIndex + 1].id, autoPlay);
    } else {
      // if we're at the end of the queue, load more scenes
      if (currentQueueIndex === queueScenes.length - 1 && queueHasMoreScenes) {
        const loadedScenes = await onQueueMoreScenes();
        if (loadedScenes && loadedScenes.length > 0) {
          // set the page to the next page
          const newPage = (sceneQueue.query?.currentPage ?? 0) + 1;
          loadScene(loadedScenes[0].id, autoPlay, newPage);
        }
      }
    }
  }

  async function queuePrevious(autoPlay: boolean) {
    if (currentQueueIndex === -1) return;

    if (currentQueueIndex > 0) {
      loadScene(queueScenes[currentQueueIndex - 1].id, autoPlay);
    } else {
      // if we're at the beginning of the queue, load the previous page
      if (queueStart > 1) {
        const loadedScenes = await onQueueLessScenes();
        if (loadedScenes && loadedScenes.length > 0) {
          const newPage = (sceneQueue.query?.currentPage ?? 0) - 1;
          loadScene(
            loadedScenes[loadedScenes.length - 1].id,
            autoPlay,
            newPage
          );
        }
      }
    }
  }

  async function queueRandom(autoPlay: boolean) {
    if (sceneQueue.query) {
      const { query } = sceneQueue;
      const pages = Math.ceil(queueTotal / query.itemsPerPage);
      const page = Math.floor(Math.random() * pages) + 1;
      const index = Math.floor(
        Math.random() * Math.min(query.itemsPerPage, queueTotal)
      );
      const filterCopy = sceneQueue.query.clone();
      filterCopy.currentPage = page;
      const queryResults = await queryFindScenes(filterCopy);
      if (queryResults.data.findScenes.scenes.length > index) {
        const { id: sceneID } = queryResults.data.findScenes.scenes[index];
        // navigate to the image player page
        loadScene(sceneID, autoPlay, page);
      }
    } else if (queueTotal !== 0) {
      const index = Math.floor(Math.random() * queueTotal);
      loadScene(queueScenes[index].id, autoPlay);
    } else {
      // no queue context: pick a random scene from the whole library
      const q = new ListFilterModel(GQL.FilterMode.Scenes);
      q.sortBy = "random";
      const queryResults = await queryFindScenes(q);
      const scenes = queryResults.data.findScenes.scenes;
      if (scenes.length > 0) {
        loadScene(scenes[0].id, autoPlay, 1);
      }
    }
  }

  const [afterglow, setAfterglow] = useState(() => {
    const fromURL =
      new URLSearchParams(location.search).get("afterglow") === "1";
    if (fromURL) {
      localStorage.setItem("stash.afterglow", "1");
      return true;
    }
    return localStorage.getItem("stash.afterglow") === "1";
  });

  // --- session recap tracking (afterglow mode) ---
  const SESSION_KEY = "stash.goonSession";

  type SessionLog = {
    entries: {
      sceneId: string;
      title: string;
      bestMoment: number;
      at: string;
      height?: number;
    }[];
    oCount: number;
    startedAt?: number;
  };

  const readSession = useCallback((): SessionLog | null => {
    try {
      const raw = localStorage.getItem(SESSION_KEY);
      return raw ? JSON.parse(raw) : null;
    } catch {
      return null;
    }
  }, []);

  const writeSession = useCallback((session: SessionLog) => {
    localStorage.setItem(SESSION_KEY, JSON.stringify(session));
  }, []);

  const [sessionRecap, setSessionRecap] = useState<SessionLog | null>(null);

  // --- stratosphere: session altitude & atmosphere milestones ---
  const [sessionPaused, setSessionPaused] = useState(false);
  const [milestoneOverlay, setMilestoneOverlay] = useState<number | null>(null);
  const [milestoneBlind, setMilestoneBlind] = useState(false);
  const prevAltitudeRef = useRef(0);

  // ticking clock for the session status strip while a session runs
  const [now, setNow] = useState(Date.now());
  useEffect(() => {
    if (!afterglow || sessionPaused) return;
    const t = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(t);
  }, [afterglow, sessionPaused]);

  const sessionLog = afterglow ? readSession() : null;
  const elapsedSecs = sessionLog?.startedAt
    ? Math.max(0, Math.floor((now - sessionLog.startedAt) / 1000))
    : 0;

  const altitude = (sessionLog?.entries ?? []).reduce(
    (sum, e) => sum + (e.height ?? 0),
    0
  );
  const nextLayer = ATMO_LAYERS.find((l) => l > altitude);

  useEffect(() => {
    if (!afterglow) {
      prevAltitudeRef.current = 0;
      return;
    }
    const prev = prevAltitudeRef.current;
    for (const layer of ATMO_LAYERS) {
      if (prev < layer && altitude >= layer) {
        _pausePlayer.current();
        if (layer === 30) {
          setMilestoneOverlay(30);
        } else if (layer === 50) {
          setMilestoneBlind(true);
          setMilestoneOverlay(50);
        } else {
          setMilestoneOverlay(layer);
        }
        break;
      }
    }
    prevAltitudeRef.current = altitude;

    try {
      const best = Number(localStorage.getItem("stash.bestAltitude") ?? "0");
      if (altitude > best) {
        localStorage.setItem("stash.bestAltitude", String(altitude));
      }
    } catch {
      // ignore altitude tracking failures
    }
  }, [afterglow, altitude]);

  // milestone blindness lasts one scene
  // biome-ignore lint/correctness/useExhaustiveDependencies: intentionally re-runs when the scene changes
  useEffect(() => {
    setMilestoneBlind(false);
  }, [scene?.id, afterglow]);

  const sceneRef = useRef(scene);
  sceneRef.current = scene;

  const onSessionO = useCallback(() => {
    if (!afterglow) return;
    const session = readSession() ?? { entries: [], oCount: 0 };
    session.oCount = (session.oCount ?? 0) + 1;
    writeSession(session);

    // ritual step complete: advance to the next scene shortly after
    const advance = setTimeout(() => {
      queueNextRef.current(true);
    }, 1000);
    return () => clearTimeout(advance);
  }, [afterglow, readSession, writeSession]);

  function toggleAfterglow() {
    if (afterglow) {
      const session = readSession();
      if (session && session.entries.length > 0) {
        setSessionRecap(session);
        localStorage.setItem("stash.achievement.firstSession", "1");

        // accumulate session stats for the Pantheon
        const started = session.startedAt ?? Date.now();
        const minutes = Math.max(1, Math.round((Date.now() - started) / 60000));
        try {
          const raw = localStorage.getItem("stash.goonSessionStats");
          const stats = raw
            ? JSON.parse(raw)
            : { totalMinutes: 0, maxMinutes: 0, sessions: 0 };
          stats.totalMinutes = (stats.totalMinutes ?? 0) + minutes;
          stats.maxMinutes = Math.max(stats.maxMinutes ?? 0, minutes);
          stats.sessions = (stats.sessions ?? 0) + 1;
          localStorage.setItem("stash.goonSessionStats", JSON.stringify(stats));
        } catch {
          // ignore stats failures
        }
      }
      localStorage.removeItem(SESSION_KEY);
      localStorage.setItem("stash.afterglow", "0");
      setAfterglow(false);
    } else {
      const session = readSession() ?? {
        entries: [],
        oCount: 0,
        startedAt: Date.now(),
      };
      session.startedAt = Date.now();
      writeSession(session);
      localStorage.setItem("stash.afterglow", "1");
      setAfterglow(true);
    }
  }
  const { data: bestMomentData } = GQL.useAiBestMomentQuery({
    variables: { scene_id: scene?.id ?? "" },
    skip: !afterglow || !scene?.id,
  });

  function abandonSession() {
    if (
      !window.confirm(
        intl.formatMessage({ id: "scene_roulette.abandon_confirm" })
      )
    ) {
      return;
    }
    localStorage.removeItem(SESSION_KEY);
    localStorage.setItem("stash.afterglow", "0");
    setAfterglow(false);
    setSessionPaused(false);
    setMilestoneOverlay(null);
    setMilestoneBlind(false);
  }

  // keep fresh references so the effect can depend only on the scene
  const queueNextRef = useRef(queueNext);
  queueNextRef.current = queueNext;
  const setTimestampRef = useRef(setTimestamp);
  setTimestampRef.current = setTimestamp;

  // afterglow (reel) mode: jump to each scene's best moment and advance
  useEffect(() => {
    if (!afterglow || sessionPaused) return;

    // record this scene in the session log
    const current = sceneRef.current;
    if (current?.id) {
      const session = readSession() ?? { entries: [], oCount: 0 };
      if (
        !session.entries.some(
          (e: { sceneId: string }) => e.sceneId === current.id
        )
      ) {
        session.entries.push({
          sceneId: current.id,
          title: current.title || current.id,
          bestMoment: bestMomentData?.aiBestMoment ?? 0,
          at: new Date().toISOString(),
          height: current.height,
        });
        writeSession(session);
      }
    }

    const best = bestMomentData?.aiBestMoment;
    if (best !== null && best !== undefined) {
      setTimestampRef.current(best);
    }

    const timer = setTimeout(() => {
      queueNextRef.current(true);
    }, 45000);
    return () => clearTimeout(timer);
  }, [afterglow, sessionPaused, bestMomentData, readSession, writeSession]);

  // --- edging mode ---
  // interval in minutes; -1 = random (0.5-10 min), never revealed
  const [edging, setEdging] = useState(false);
  const [edgeInterval, setEdgeInterval] = useState(-1);
  const [edgePaused, setEdgePaused] = useState(false);

  useEffect(() => {
    if (!edging || edgePaused || sessionPaused) return;
    const delay =
      edgeInterval === -1
        ? 30000 + Math.floor(Math.random() * 570000)
        : edgeInterval * 60 * 1000;
    const t = setTimeout(() => {
      _pausePlayer.current();
      setEdgePaused(true);
      try {
        const n = Number(localStorage.getItem("stash.edgePauses") ?? "0") + 1;
        localStorage.setItem("stash.edgePauses", String(n));
      } catch {
        // ignore counter failures
      }
    }, delay);
    return () => clearTimeout(t);
  }, [edging, edgePaused, edgeInterval, sessionPaused]);

  function toggleEdging() {
    if (edging) {
      setEdgePaused(false);
      setEdging(false);
    } else {
      setEdging(true);
    }
  }

  // --- blind goon mode ---
  const [blind, setBlind] = useState(false);

  async function startBlindGoon() {
    try {
      const n = Number(localStorage.getItem("stash.blindCount") ?? "0") + 1;
      localStorage.setItem("stash.blindCount", String(n));
    } catch {
      // ignore counter failures
    }
    const filter = new ListFilterModel(GQL.FilterMode.Scenes);
    filter.sortBy = "random";
    filter.itemsPerPage = 50;
    const steam = new SteamScoreCriterion();
    steam.value = 6;
    steam.modifier = GQL.CriterionModifier.GreaterThan;
    filter.criteria.push(steam);

    const result = await queryFindScenes(filter);
    const scenes = result.data.findScenes.scenes;
    if (scenes.length === 0) {
      Toast.error(intl.formatMessage({ id: "scene_roulette.blind_empty" }));
      return;
    }
    setBlind(true);
    loadScene(scenes[0].id, true, 1);
  }

  function toggleBlind() {
    if (blind) {
      setBlind(false);
    } else {
      startBlindGoon();
    }
  }

  // --- transcend mode ---
  const [transcend, setTranscend] = useState(false);
  const [showSessionBuild, setShowSessionBuild] = useState(false);
  const [transcendPending, setTranscendPending] = useState(false);
  const transcendCount = useRef(0);
  const [transcendScene, setTranscendScene] = useState(0);

  // blind every third scene while transcending
  useEffect(() => {
    if (!transcend || !scene?.id) return;
    transcendCount.current += 1;
    setTranscendScene(transcendCount.current);
    setBlind(transcendCount.current % 3 === 0);
  }, [transcend, scene?.id]);

  function onTranscend() {
    // open the session builder pre-filled for a transcend session; the
    // transcend mode itself activates when the built plan is started
    setTranscendPending(true);
    setShowSessionBuild(true);
  }

  // --- vibe radio ---
  async function startVibeRadio(mood: string) {
    const filter = new ListFilterModel(GQL.FilterMode.Scenes);
    filter.sortBy = "random";
    filter.itemsPerPage = 100;
    const moods = new MoodsCriterion();
    moods.value = { items: [{ id: mood, label: mood }], excluded: [] };
    filter.criteria.push(moods);

    const result = await queryFindScenes(filter);
    const scenes = result.data.findScenes.scenes;
    if (scenes.length === 0) {
      Toast.error(intl.formatMessage({ id: "scene_roulette.radio_empty" }));
      return;
    }
    const params = scenes
      .map((s) => `qs=${s.id}`)
      .concat("afterglow=1", "autoplay=true")
      .join("&");
    history.push(`/scenes/${scenes[0].id}?${params}`);
  }

  function onComplete() {
    setBlind(false);
    // load the next scene if we're continuing
    if (continuePlaylist) {
      queueNext(true);
    }
  }

  function onDelete() {
    if (
      continuePlaylist &&
      currentQueueIndex >= 0 &&
      currentQueueIndex < queueScenes.length - 1
    ) {
      loadScene(queueScenes[currentQueueIndex + 1].id);
    } else {
      goBackOrReplace(history, "/scenes");
    }
  }

  function getScenePage(sceneID: string) {
    if (!sceneQueue.query) return;

    // find the page that the scene is on
    const index = queueScenes.findIndex((s) => s.id === sceneID);

    if (index === -1) return;

    const perPage = sceneQueue.query.itemsPerPage;
    return Math.floor((index + queueStart - 1) / perPage) + 1;
  }

  function onQueueSceneClicked(sceneID: string) {
    loadScene(sceneID, autoPlayOnSelected, getScenePage(sceneID));
  }

  if (!scene) {
    if (loading) return <LoadingIndicator />;
    if (error) return <ErrorMessage error={error.message} />;
    return <ErrorMessage error={`No scene found with id ${id}.`} />;
  }

  return (
    <div className="row">
      <ScenePage
        scene={scene}
        setTimestamp={setTimestamp}
        queueScenes={queueScenes}
        queueStart={queueStart}
        onDelete={onDelete}
        onQueueNext={() => queueNext(autoPlayOnSelected)}
        onQueuePrevious={() => queuePrevious(autoPlayOnSelected)}
        onQueueRandom={() => queueRandom(autoPlayOnSelected)}
        onQueueSceneClicked={onQueueSceneClicked}
        onSessionO={onSessionO}
        continuePlaylist={continuePlaylist}
        queueHasMoreScenes={queueHasMoreScenes}
        onQueueLessScenes={onQueueLessScenes}
        onQueueMoreScenes={onQueueMoreScenes}
        collapsed={collapsed}
        setCollapsed={setCollapsed}
        setContinuePlaylist={setContinuePlaylist}
        onRefreshScene={onRefreshScene}
      />
      <div
        className={`scene-player-container ${collapsed ? "expanded" : ""} ${
          blind || milestoneBlind ? "blind-goon" : ""
        }`}
      >
        <div className="scene-roulette-bar">
          <div className="roulette-segment roulette-queue">
            <Button
              variant="danger"
              onClick={() => queueRandom(autoPlayOnSelected)}
            >
              <Icon icon={faDice} />{" "}
              <FormattedMessage id="scene_roulette.button" />
            </Button>
          </div>
          <div className="roulette-segment roulette-session">
            <Button
              variant={afterglow ? "primary" : "outline-primary"}
              onClick={() => toggleAfterglow()}
            >
              <Icon icon={faBolt} />{" "}
              <FormattedMessage id="scene_roulette.afterglow" />
            </Button>
            <Button
              variant={blind ? "dark" : "outline-dark"}
              onClick={() => toggleBlind()}
            >
              <Icon icon={faEyeSlash} />{" "}
              <FormattedMessage id="scene_roulette.blind" />
            </Button>
            <Button
              variant={edging ? "warning" : "outline-warning"}
              onClick={() => toggleEdging()}
            >
              <Icon icon={faHourglassHalf} />{" "}
              <FormattedMessage id="scene_roulette.edging" />
            </Button>
            <Form.Control
              as="select"
              className="edge-interval-select"
              value={edgeInterval}
              disabled={!edging}
              onChange={(e: React.ChangeEvent<HTMLSelectElement>) =>
                setEdgeInterval(Number(e.currentTarget.value))
              }
            >
              <option value={-1}>
                {intl.formatMessage({ id: "scene_roulette.edging_random" })}
              </option>
              {[1, 2, 3, 4, 5, 7, 10].map((m) => (
                <option key={m} value={m}>
                  {m} min
                </option>
              ))}
            </Form.Control>
            <Button
              variant={transcend ? "success" : "outline-success"}
              onClick={() => onTranscend()}
            >
              <Icon icon={faDove} />{" "}
              <FormattedMessage id="scene_roulette.transcend" />
            </Button>
            <Dropdown id="vibe-radio-dropdown">
              <Dropdown.Toggle variant="outline-danger" size="sm">
                <Icon icon={faRadio} />{" "}
                <FormattedMessage id="scene_roulette.radio" />
              </Dropdown.Toggle>
              <Dropdown.Menu>
                {((MoodsCriterionOption.options ?? []) as IOptionType[]).map(
                  (o) => (
                    <Dropdown.Item
                      key={String(o.id)}
                      onClick={() => startVibeRadio(String(o.id))}
                    >
                      {String(o.id)}
                    </Dropdown.Item>
                  )
                )}
              </Dropdown.Menu>
            </Dropdown>
          </div>
          <div className="roulette-segment roulette-meta">
            <Button
              variant="outline-info"
              onClick={() =>
                history.push(`/aiChat?watching=${scene?.id ?? ""}`)
              }
            >
              <Icon icon={faComments} />{" "}
              <FormattedMessage id="scene_roulette.chat" />
            </Button>
            <div className="who-is-she-slot">
              <WhoIsSheChip sceneId={scene?.id} />
            </div>
          </div>
        </div>
        {afterglow && (
          <div className="session-status-strip">
            <span className="session-status-elapsed">
              <Icon icon={faClock} /> {Math.floor(elapsedSecs / 60)}:
              {String(elapsedSecs % 60).padStart(2, "0")}
            </span>
            <span className="session-status-o">
              <Icon icon={faHeart} /> {sessionLog?.oCount ?? 0}
            </span>
            <span className="session-status-altitude" title="Session altitude">
              <Icon icon={faRocket} /> {altitude} km
            </span>
            <div className="session-altitude-progress">
              <div
                className="session-altitude-bar"
                style={{
                  width: nextLayer
                    ? `${Math.min(100, (altitude / nextLayer) * 100)}%`
                    : "100%",
                }}
              />
            </div>
            {transcend && (
              <span className="session-badge session-badge-transcend">
                Transcend · scene {transcendScene}
              </span>
            )}
            {blind && (
              <span className="session-badge session-badge-blind">Blind</span>
            )}
            {milestoneBlind && (
              <span className="session-badge session-badge-blind">
                Blind climb
              </span>
            )}
            {edging && (
              <span className="session-badge session-badge-edging">Edging</span>
            )}
            <span className="session-strip-controls">
              <Button
                size="sm"
                variant={sessionPaused ? "warning" : "outline-secondary"}
                onClick={() => setSessionPaused(!sessionPaused)}
              >
                {sessionPaused ? (
                  <FormattedMessage id="scene_roulette.resume" />
                ) : (
                  <FormattedMessage id="scene_roulette.pause" />
                )}
              </Button>
              <Button
                size="sm"
                variant="outline-secondary"
                onClick={() => queueNextRef.current(true)}
              >
                <FormattedMessage id="scene_roulette.next_scene" />
              </Button>
              <Button
                size="sm"
                variant="outline-danger"
                onClick={() => abandonSession()}
              >
                <FormattedMessage id="scene_roulette.abandon" />
              </Button>
            </span>
          </div>
        )}
        <ClimaxProjection sceneId={scene?.id} active={afterglow} />
        <ScenePlayer
          key="ScenePlayer"
          scene={scene}
          hideScrubberOverride={hideScrubber}
          autoplay={autoplay}
          permitLoop={!continuePlaylist}
          initialTimestamp={initialTimestamp}
          sendSetTimestamp={getSetTimestamp}
          sendPause={getPausePlayer}
          sendPlay={getPlayPlayer}
          onComplete={onComplete}
          onNext={() => queueNext(true)}
          onPrevious={() => queuePrevious(true)}
        />
      </div>

      {edgePaused && (
        <div className="edging-overlay">
          <h3>
            <Icon icon={faHourglassHalf} />{" "}
            <FormattedMessage id="scene_roulette.edging_overlay" />
          </h3>
          <p className="text-muted">
            <FormattedMessage id="scene_roulette.edging_overlay_hint" />
          </p>
          <div>
            <Button
              variant="danger"
              className="mr-2"
              onClick={() => {
                setEdgePaused(false);
                _playPlayer.current();
              }}
            >
              <FormattedMessage id="scene_roulette.edging_continue" />
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                setEdgePaused(false);
                setEdging(false);
              }}
            >
              <FormattedMessage id="scene_roulette.edging_stop" />
            </Button>
          </div>
        </div>
      )}

      {milestoneOverlay && (
        <div className="edging-overlay milestone-overlay">
          <h3>
            <Icon icon={faRocket} />{" "}
            <FormattedMessage
              id={`scene_roulette.milestone_${milestoneOverlay}`}
            />
          </h3>
          <div>
            <Button
              variant="danger"
              onClick={() => {
                setMilestoneOverlay(null);
                _playPlayer.current();
              }}
            >
              <FormattedMessage id="scene_roulette.edging_continue" />
            </Button>
          </div>
        </div>
      )}

      {showSessionBuild && (
        <AISessionBuildDialog
          initial={{ durationMinutes: 60, minSteam: 7, ordering: "build_up" }}
          onStart={() => {
            if (!transcendPending) return;
            setTranscendPending(false);
            setTranscend(true);
            transcendCount.current = 0;
            setTranscendScene(0);
            setEdging(true);
            localStorage.setItem("stash.achievement.transcend", "1");
          }}
          onClose={() => {
            setShowSessionBuild(false);
            setTranscendPending(false);
          }}
        />
      )}
      {sessionRecap && (
        <ModalComponent
          show
          icon={faBolt}
          header={intl.formatMessage({ id: "scene_roulette.recap" })}
          onHide={() => setSessionRecap(null)}
        >
          <div>
            <p>
              <FormattedMessage id="scene_roulette.recap_scenes" />:{" "}
              <strong>{sessionRecap.entries.length}</strong> ·{" "}
              <FormattedMessage id="scene_roulette.recap_o" />:{" "}
              <strong>{sessionRecap.oCount}</strong>{" "}
              {sessionRecap.oCount > 0 ? "💦" : ""}
            </p>
            <ul className="mb-0">
              {sessionRecap.entries.map((e) => (
                <li key={e.sceneId}>
                  {e.title}
                  {e.bestMoment > 0 && (
                    <span className="text-muted">
                      {" "}
                      · best @ {Math.round(e.bestMoment)}s
                    </span>
                  )}
                </li>
              ))}
            </ul>
          </div>
        </ModalComponent>
      )}
    </div>
  );
};

export default SceneLoader;
