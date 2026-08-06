import React, { useState, useEffect } from "react";
import { FormattedMessage, useIntl } from "react-intl";
import { Button, Form } from "react-bootstrap";
import {
  mutateMetadataScan,
  mutateMetadataAutoTag,
  mutateMetadataGenerate,
} from "src/core/StashService";
import { withoutTypename } from "src/utils/data";
import { useConfigurationContext } from "src/hooks/Config";
import { useAutoTagTrigger } from "src/hooks/useAutoTagTrigger";
import { IdentifyDialog } from "../../Dialogs/IdentifyDialog/IdentifyDialog";
import { AIImageTagDialog } from "../../Dialogs/AIImageTagDialog/AIImageTagDialog";
import { AISceneTagDialog } from "../../Dialogs/AISceneTagDialog/AISceneTagDialog";
import { AIPerformerClusterDialog } from "../../Dialogs/AIPerformerClusterDialog/AIPerformerClusterDialog";
import { AISceneSegmentDialog } from "../../Dialogs/AISceneSegmentDialog/AISceneSegmentDialog";
import { AISuggestionDialog } from "../../Dialogs/AISuggestionDialog/AISuggestionDialog";
import { AIMediaQualityDialog } from "../../Dialogs/AIMediaQualityDialog/AIMediaQualityDialog";
import { AIAudioAnalysisDialog } from "../../Dialogs/AIAudioAnalysisDialog/AIAudioAnalysisDialog";
import { DetectLoopingDialog } from "../../Dialogs/DetectLoopingDialog/DetectLoopingDialog";
import { AIPerformerCareerDialog } from "../../Dialogs/AIPerformerCareerDialog/AIPerformerCareerDialog";
import { AISmartCollectionsDialog } from "../../Dialogs/AISmartCollectionsDialog/AISmartCollectionsDialog";
import { AIFileRenameDialog } from "../../Dialogs/AIFileRenameDialog/AIFileRenameDialog";
import { AISuggestionReviewDialog } from "../../Dialogs/AISuggestionReviewDialog/AISuggestionReviewDialog";
import { AIFileRenameReviewDialog } from "../../Dialogs/AIFileRenameReviewDialog/AIFileRenameReviewDialog";
import { AIDuplicateDetectionDialog } from "../../Dialogs/AIDuplicateDetectionDialog/AIDuplicateDetectionDialog";
import { SemanticSearchDialog } from "../../Dialogs/SemanticSearchDialog/SemanticSearchDialog";
import { AudioTranscriptSearchDialog } from "../../Dialogs/AudioTranscriptSearchDialog/AudioTranscriptSearchDialog";
import { AIPerformerMergeSuggestDialog } from "../../Dialogs/AIPerformerMergeSuggestDialog/AIPerformerMergeSuggestDialog";
import { AIAuditReviewDialog } from "../../Dialogs/AIAuditReviewDialog/AIAuditReviewDialog";
import { AITranslationReviewDialog } from "../../Dialogs/AITranslationReviewDialog/AITranslationReviewDialog";
import { AIPerformerDiscoveryReviewDialog } from "../../Dialogs/AIPerformerDiscoveryReviewDialog/AIPerformerDiscoveryReviewDialog";
import { AIMoodGroupsDialog } from "../../Dialogs/AIMoodGroupsDialog/AIMoodGroupsDialog";
import { AISessionBuildDialog } from "../../Dialogs/AISessionBuildDialog/AISessionBuildDialog";
import {
  mutateMetadataAIPerformerMergeSuggest,
  mutateMetadataAIAudit,
  mutateMetadataAITranslate,
  mutateMetadataAIPerformerDiscovery,
  mutateMetadataAIMoodTag,
} from "src/core/StashService";
import * as GQL from "src/core/generated-graphql";
import { DirectorySelectionDialog } from "./DirectorySelectionDialog";
import { ScanOptions } from "./ScanOptions";
import { useToast } from "src/hooks/Toast";
import { GenerateOptions } from "./GenerateOptions";
import { SettingSection } from "../SettingSection";
import { BooleanSetting, Setting, SettingGroup } from "../Inputs";
import { ManualLink } from "src/components/Help/context";
import { Icon } from "src/components/Shared/Icon";
import { faQuestionCircle } from "@fortawesome/free-solid-svg-icons";
import {
  AutoTagConfirmDialog,
  AutoTagWarning,
} from "src/components/Shared/AutoTagConfirmDialog";
import { useSettings } from "../context";

interface IAutoTagOptions {
  options: GQL.AutoTagMetadataInput;
  setOptions: (s: GQL.AutoTagMetadataInput) => void;
}

const AutoTagOptions: React.FC<IAutoTagOptions> = ({
  options,
  setOptions: setOptionsState,
}) => {
  const { performers, studios, tags } = options;
  const wildcard = ["*"];

  function set(v?: boolean) {
    if (v) {
      return wildcard;
    }
    return [];
  }

  function setOptions(input: Partial<GQL.AutoTagMetadataInput>) {
    setOptionsState({ ...options, ...input });
  }

  return (
    <>
      <BooleanSetting
        id="autotag-performers"
        checked={!!performers?.length}
        headingID="performers"
        onChange={(v) => setOptions({ performers: set(v) })}
      />
      <BooleanSetting
        id="autotag-studios"
        checked={!!studios?.length}
        headingID="studios"
        onChange={(v) => setOptions({ studios: set(v) })}
      />
      <BooleanSetting
        id="autotag-tags"
        checked={!!tags?.length}
        headingID="tags"
        onChange={(v) => setOptions({ tags: set(v) })}
      />
    </>
  );
};

export const LibraryTasks: React.FC = () => {
  const intl = useIntl();
  const Toast = useToast();
  const { ui, saveUI, loading } = useSettings();

  const { taskDefaults } = ui;

  const [dialogOpen, setDialogOpenState] = useState({
    scan: false,
    autoTag: false,
    autoTagAlert: false,
    identify: false,
    generate: false,
    aiImageTag: false,
    aiSceneTag: false,
    aiPerformerCluster: false,
    aiSceneSegment: false,
    aiSuggestion: false,
    aiMediaQuality: false,
    aiAudioAnalysis: false,
    detectLooping: false,
    aiPerformerCareer: false,
    aiSmartCollections: false,
    aiFileRename: false,
    aiSuggestionReview: false,
    aiFileRenameReview: false,
    aiDuplicateDetection: false,
    aiSemanticSearch: false,
    aiAudioSearch: false,
    aiPerformerMergeSuggest: false,
    aiAuditReview: false,
    aiTranslationReview: false,
    aiPerformerDiscoveryReview: false,
    aiMoodGroups: false,
    aiSessionBuild: false,
  });

  function getDefaultScanOptions(): GQL.ScanMetadataInput {
    return {
      scanGenerateCovers: true,
      scanGeneratePreviews: false,
      scanGenerateImagePreviews: false,
      scanGenerateSprites: false,
      scanGeneratePhashes: false,
      scanGenerateThumbnails: false,
      scanGenerateClipPreviews: false,
    };
  }

  const [scanOptions, setScanOptions] = useState<GQL.ScanMetadataInput>(
    getDefaultScanOptions()
  );
  const [autoTagOptions, setAutoTagOptions] =
    useState<GQL.AutoTagMetadataInput>({
      performers: ["*"],
      studios: ["*"],
      tags: ["*"],
    });

  function getDefaultGenerateOptions(): GQL.GenerateMetadataInput {
    return {
      covers: true,
      sprites: true,
      phashes: true,
      previews: true,
      markers: true,
      previewOptions: {
        previewSegments: 0,
        previewSegmentDuration: 0,
        previewPreset: GQL.PreviewPreset.Slow,
      },
    };
  }

  const [generateOptions, setGenerateOptions] =
    useState<GQL.GenerateMetadataInput>(getDefaultGenerateOptions());

  type DialogOpenState = typeof dialogOpen;

  const { configuration } = useConfigurationContext();
  const [configRead, setConfigRead] = useState(false);

  useEffect(() => {
    if (!configuration?.defaults || loading) {
      return;
    }

    const { scan, autoTag } = configuration.defaults;

    // prefer UI defaults over system defaults
    // other defaults should be deprecated
    if (taskDefaults?.scan) {
      setScanOptions(taskDefaults.scan);
    } else if (scan) {
      setScanOptions(withoutTypename(scan));
    }

    if (taskDefaults?.autoTag) {
      setAutoTagOptions(taskDefaults.autoTag);
    } else if (autoTag) {
      setAutoTagOptions(withoutTypename(autoTag));
    }

    if (taskDefaults?.generate) {
      setGenerateOptions(taskDefaults.generate);
    }

    // combine the defaults with the system preview generation settings
    // only do this once
    // don't do this if UI had a default
    if (!configRead && !taskDefaults?.generate) {
      if (configuration?.defaults.generate) {
        const { generate } = configuration.defaults;
        setGenerateOptions(withoutTypename(generate));
      }

      setConfigRead(true);
    }
  }, [configuration, configRead, taskDefaults, loading]);

  function configureDefaults(partial: Record<string, object>) {
    saveUI({ taskDefaults: { ...partial } });
  }

  function onSetScanOptions(s: GQL.ScanMetadataInput) {
    configureDefaults({ scan: s });
    setScanOptions(s);
  }

  function onSetGenerateOptions(s: GQL.GenerateMetadataInput) {
    configureDefaults({ generate: s });
    setGenerateOptions(s);
  }

  function onSetAutoTagOptions(s: GQL.AutoTagMetadataInput) {
    configureDefaults({ autoTag: s });
    setAutoTagOptions(s);
  }

  function setDialogOpen(s: Partial<DialogOpenState>) {
    setDialogOpenState((v) => {
      return { ...v, ...s };
    });
  }

  const onAutoTagClick = useAutoTagTrigger(
    () => runAutoTag(),
    () => setDialogOpen({ autoTagAlert: true }),
    ui.disableAutoTagWarning
  );

  function renderScanDialog() {
    if (!dialogOpen.scan) {
      return;
    }

    return <DirectorySelectionDialog onClose={onScanDialogClosed} />;
  }

  function onScanDialogClosed(paths?: string[]) {
    if (paths) {
      runScan(paths);
    }

    setDialogOpen({ scan: false });
  }

  async function runScan(paths?: string[]) {
    try {
      await mutateMetadataScan({
        ...scanOptions,
        paths,
      });

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: intl.formatMessage({ id: "actions.scan" }) }
        )
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  function renderAutoTagAlert() {
    return (
      <AutoTagConfirmDialog
        show={dialogOpen.autoTagAlert}
        onConfirm={() => {
          setDialogOpen({ autoTagAlert: false });
          runAutoTag();
        }}
        onCancel={() => setDialogOpen({ autoTagAlert: false })}
      />
    );
  }

  function renderAutoTagDialog() {
    if (!dialogOpen.autoTag) {
      return;
    }

    return (
      <DirectorySelectionDialog onClose={onAutoTagDialogClosed}>
        <AutoTagWarning />
      </DirectorySelectionDialog>
    );
  }

  function onAutoTagDialogClosed(paths?: string[]) {
    if (paths) {
      runAutoTag(paths);
    }

    setDialogOpen({ autoTag: false });
  }

  async function runAutoTag(paths?: string[]) {
    try {
      await mutateMetadataAutoTag({
        ...autoTagOptions,
        paths,
      });

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: intl.formatMessage({ id: "actions.auto_tag" }) }
        )
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  function maybeRenderIdentifyDialog() {
    if (!dialogOpen.identify) return;

    return (
      <IdentifyDialog onClose={() => setDialogOpen({ identify: false })} />
    );
  }

  function maybeRenderAIImageTagDialog() {
    if (!dialogOpen.aiImageTag) return;

    return (
      <AIImageTagDialog onClose={() => setDialogOpen({ aiImageTag: false })} />
    );
  }

  function maybeRenderAISceneTagDialog() {
    if (!dialogOpen.aiSceneTag) return;

    return (
      <AISceneTagDialog onClose={() => setDialogOpen({ aiSceneTag: false })} />
    );
  }

  function maybeRenderAIPerformerClusterDialog() {
    if (!dialogOpen.aiPerformerCluster) return;

    return (
      <AIPerformerClusterDialog
        onClose={() => setDialogOpen({ aiPerformerCluster: false })}
      />
    );
  }

  function maybeRenderAISceneSegmentDialog() {
    if (!dialogOpen.aiSceneSegment) return;

    return (
      <AISceneSegmentDialog
        onClose={() => setDialogOpen({ aiSceneSegment: false })}
      />
    );
  }

  function maybeRenderAISuggestionDialog() {
    if (!dialogOpen.aiSuggestion) return;

    return (
      <AISuggestionDialog
        onClose={() => setDialogOpen({ aiSuggestion: false })}
      />
    );
  }

  function maybeRenderAIMediaQualityDialog() {
    if (!dialogOpen.aiMediaQuality) return;

    return (
      <AIMediaQualityDialog
        onClose={() => setDialogOpen({ aiMediaQuality: false })}
      />
    );
  }

  function maybeRenderAIAudioAnalysisDialog() {
    if (!dialogOpen.aiAudioAnalysis) return;

    return (
      <AIAudioAnalysisDialog
        onClose={() => setDialogOpen({ aiAudioAnalysis: false })}
      />
    );
  }

  function maybeRenderDetectLoopingDialog() {
    if (!dialogOpen.detectLooping) return;

    return (
      <DetectLoopingDialog
        onClose={() => setDialogOpen({ detectLooping: false })}
      />
    );
  }

  function maybeRenderAIPerformerCareerDialog() {
    if (!dialogOpen.aiPerformerCareer) return;

    return (
      <AIPerformerCareerDialog
        onClose={() => setDialogOpen({ aiPerformerCareer: false })}
      />
    );
  }

  function maybeRenderAISmartCollectionsDialog() {
    if (!dialogOpen.aiSmartCollections) return;

    return (
      <AISmartCollectionsDialog
        onClose={() => setDialogOpen({ aiSmartCollections: false })}
      />
    );
  }

  function maybeRenderAIFileRenameDialog() {
    if (!dialogOpen.aiFileRename) return;

    return (
      <AIFileRenameDialog
        onClose={() => setDialogOpen({ aiFileRename: false })}
      />
    );
  }

  function maybeRenderAISuggestionReviewDialog() {
    if (!dialogOpen.aiSuggestionReview) return;

    return (
      <AISuggestionReviewDialog
        onClose={() => setDialogOpen({ aiSuggestionReview: false })}
      />
    );
  }

  function maybeRenderAIFileRenameReviewDialog() {
    if (!dialogOpen.aiFileRenameReview) return;

    return (
      <AIFileRenameReviewDialog
        onClose={() => setDialogOpen({ aiFileRenameReview: false })}
      />
    );
  }

  function maybeRenderAIDuplicateDetectionDialog() {
    if (!dialogOpen.aiDuplicateDetection) return;

    return (
      <AIDuplicateDetectionDialog
        onClose={() => setDialogOpen({ aiDuplicateDetection: false })}
      />
    );
  }

  function maybeRenderAISemanticSearchDialog() {
    if (!dialogOpen.aiSemanticSearch) return;

    return (
      <SemanticSearchDialog
        onClose={() => setDialogOpen({ aiSemanticSearch: false })}
      />
    );
  }

  function maybeRenderAIAudioSearchDialog() {
    if (!dialogOpen.aiAudioSearch) return;

    return (
      <AudioTranscriptSearchDialog
        onClose={() => setDialogOpen({ aiAudioSearch: false })}
      />
    );
  }

  function maybeRenderAIPerformerMergeSuggestDialog() {
    if (!dialogOpen.aiPerformerMergeSuggest) return;

    return (
      <AIPerformerMergeSuggestDialog
        onClose={() => setDialogOpen({ aiPerformerMergeSuggest: false })}
      />
    );
  }

  function maybeRenderAIAuditReviewDialog() {
    if (!dialogOpen.aiAuditReview) return;

    return (
      <AIAuditReviewDialog
        onClose={() => setDialogOpen({ aiAuditReview: false })}
      />
    );
  }

  function maybeRenderAITranslationReviewDialog() {
    if (!dialogOpen.aiTranslationReview) return;

    return (
      <AITranslationReviewDialog
        onClose={() => setDialogOpen({ aiTranslationReview: false })}
      />
    );
  }

  function maybeRenderAIPerformerDiscoveryReviewDialog() {
    if (!dialogOpen.aiPerformerDiscoveryReview) return;

    return (
      <AIPerformerDiscoveryReviewDialog
        onClose={() => setDialogOpen({ aiPerformerDiscoveryReview: false })}
      />
    );
  }

  function maybeRenderAIMoodGroupsDialog() {
    if (!dialogOpen.aiMoodGroups) return;

    return (
      <AIMoodGroupsDialog
        onClose={() => setDialogOpen({ aiMoodGroups: false })}
      />
    );
  }

  function maybeRenderAISessionBuildDialog() {
    if (!dialogOpen.aiSessionBuild) return;

    return (
      <AISessionBuildDialog
        onClose={() => setDialogOpen({ aiSessionBuild: false })}
      />
    );
  }

  function renderGenerateDialog() {
    if (!dialogOpen.generate) {
      return;
    }

    return <DirectorySelectionDialog onClose={onGenerateDialogClosed} />;
  }

  function onGenerateDialogClosed(paths?: string[]) {
    if (paths) {
      runGenerate(paths);
    }

    setDialogOpen({ generate: false });
  }

  async function runGenerate(paths?: string[]) {
    const general = configuration?.general;

    try {
      await mutateMetadataGenerate({
        ...generateOptions,
        paths,
        previewOptions: {
          ...generateOptions.previewOptions,
          previewSegments:
            general?.previewSegments ??
            generateOptions.previewOptions?.previewSegments,
          previewSegmentDuration:
            general?.previewSegmentDuration ??
            generateOptions.previewOptions?.previewSegmentDuration,
          previewExcludeStart:
            general?.previewExcludeStart ??
            generateOptions.previewOptions?.previewExcludeStart,
          previewExcludeEnd:
            general?.previewExcludeEnd ??
            generateOptions.previewOptions?.previewExcludeEnd,
          previewPreset:
            general?.previewPreset ??
            generateOptions.previewOptions?.previewPreset,
        },
      });

      Toast.success(
        intl.formatMessage(
          { id: "config.tasks.added_job_to_queue" },
          { operation_name: intl.formatMessage({ id: "actions.generate" }) }
        )
      );
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <Form.Group>
      {renderScanDialog()}
      {renderAutoTagAlert()}
      {renderAutoTagDialog()}
      {maybeRenderIdentifyDialog()}
      {maybeRenderAIImageTagDialog()}
      {maybeRenderAISceneTagDialog()}
      {maybeRenderAIPerformerClusterDialog()}
      {maybeRenderAISceneSegmentDialog()}
      {maybeRenderAISuggestionDialog()}
      {maybeRenderAIMediaQualityDialog()}
      {maybeRenderAIAudioAnalysisDialog()}
      {maybeRenderDetectLoopingDialog()}
      {maybeRenderAIPerformerCareerDialog()}
      {maybeRenderAISmartCollectionsDialog()}
      {maybeRenderAIFileRenameDialog()}
      {maybeRenderAISuggestionReviewDialog()}
      {maybeRenderAIFileRenameReviewDialog()}
      {maybeRenderAIDuplicateDetectionDialog()}
      {maybeRenderAISemanticSearchDialog()}
      {maybeRenderAIAudioSearchDialog()}
      {maybeRenderAIPerformerMergeSuggestDialog()}
      {maybeRenderAIAuditReviewDialog()}
      {maybeRenderAITranslationReviewDialog()}
      {maybeRenderAIPerformerDiscoveryReviewDialog()}
      {maybeRenderAIMoodGroupsDialog()}
      {maybeRenderAISessionBuildDialog()}
      {renderGenerateDialog()}

      <SettingSection headingID="library">
        <SettingGroup
          settingProps={{
            heading: (
              <>
                <FormattedMessage id="actions.scan" />
                <ManualLink tab="Tasks">
                  <Icon icon={faQuestionCircle} />
                </ManualLink>
              </>
            ),
            subHeadingID: "config.tasks.scan_for_content_desc",
          }}
          topLevel={
            <>
              <Button
                variant="secondary"
                type="button"
                className="mr-2"
                onClick={() => runScan()}
              >
                <FormattedMessage id="actions.scan" />
              </Button>

              <Button
                variant="secondary"
                type="button"
                className="mr-2"
                onClick={() => setDialogOpen({ scan: true })}
              >
                <FormattedMessage id="actions.selective_scan" />…
              </Button>
            </>
          }
          collapsible
        >
          <ScanOptions options={scanOptions} setOptions={onSetScanOptions} />
        </SettingGroup>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <>
              <FormattedMessage id="config.tasks.identify.heading" />
              <ManualLink tab="Identify">
                <Icon icon={faQuestionCircle} />
              </ManualLink>
            </>
          }
          subHeadingID="config.tasks.identify.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ identify: true })}
          >
            <FormattedMessage id="actions.identify" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection>
        <Setting
          heading={<FormattedMessage id="config.tasks.ai_image_tag.heading" />}
          subHeadingID="config.tasks.ai_image_tag.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiImageTag: true })}
          >
            <FormattedMessage id="actions.tag" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={<FormattedMessage id="config.tasks.ai_scene_tag.heading" />}
          subHeadingID="config.tasks.ai_scene_tag.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiSceneTag: true })}
          >
            <FormattedMessage id="actions.tag" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_performer_cluster.heading" />
          }
          subHeadingID="config.tasks.ai_performer_cluster.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiPerformerCluster: true })}
          >
            <FormattedMessage id="actions.generate" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_scene_segment.heading" />
          }
          subHeadingID="config.tasks.ai_scene_segment.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiSceneSegment: true })}
          >
            <FormattedMessage id="actions.generate" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection>
        <Setting
          heading={<FormattedMessage id="config.tasks.ai_suggestion.heading" />}
          subHeadingID="config.tasks.ai_suggestion.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiSuggestion: true })}
          >
            <FormattedMessage id="actions.generate" />…
          </Button>
          <Button
            variant="secondary"
            type="button"
            className="ml-2"
            onClick={() => setDialogOpen({ aiSuggestionReview: true })}
          >
            <FormattedMessage id="actions.manage" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_media_quality.heading" />
          }
          subHeadingID="config.tasks.ai_media_quality.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiMediaQuality: true })}
          >
            <FormattedMessage id="actions.generate" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_audio_analysis.heading" />
          }
          subHeadingID="config.tasks.ai_audio_analysis.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiAudioAnalysis: true })}
          >
            <FormattedMessage id="actions.generate" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.detect_looping.heading" />
          }
          subHeadingID="config.tasks.detect_looping.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ detectLooping: true })}
          >
            <FormattedMessage id="actions.detect" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_performer_career.heading" />
          }
          subHeadingID="config.tasks.ai_performer_career.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiPerformerCareer: true })}
          >
            <FormattedMessage id="actions.generate" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_smart_collections.heading" />
          }
          subHeadingID="config.tasks.ai_smart_collections.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiSmartCollections: true })}
          >
            <FormattedMessage id="actions.generate" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_file_rename.heading" />
          }
          subHeadingID="config.tasks.ai_file_rename.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiFileRename: true })}
          >
            <FormattedMessage id="actions.generate" />…
          </Button>
          <Button
            variant="secondary"
            type="button"
            className="ml-2"
            onClick={() => setDialogOpen({ aiFileRenameReview: true })}
          >
            <FormattedMessage id="actions.manage" />…
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.duplicate_detection.heading" />
          }
          subHeadingID="config.tasks.duplicate_detection.description"
        >
          <Button
            variant="secondary"
            type="button"
            className="mr-2"
            onClick={() => setDialogOpen({ aiDuplicateDetection: true })}
          >
            <FormattedMessage
              id="config.tasks.duplicate_detection.button"
              defaultMessage="Duplicate Detection…"
            />
          </Button>
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiSemanticSearch: true })}
          >
            <FormattedMessage
              id="config.tasks.semantic_search.button"
              defaultMessage="Semantic Search…"
            />
          </Button>
          <Button
            variant="secondary"
            type="button"
            className="ml-2"
            onClick={() => setDialogOpen({ aiAudioSearch: true })}
          >
            <FormattedMessage
              id="config.tasks.ai_audio_search.button"
              defaultMessage="Transcript Search…"
            />
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_performer_merge_suggest.heading" />
          }
          subHeadingID="config.tasks.ai_performer_merge_suggest.description"
        >
          <Button
            variant="secondary"
            type="button"
            className="mr-2"
            onClick={async () => {
              try {
                await mutateMetadataAIPerformerMergeSuggest({});
                Toast.success(
                  intl.formatMessage(
                    { id: "config.tasks.added_job_to_queue" },
                    { operation_name: "AI Performer Merge Suggestions" }
                  )
                );
              } catch (e) {
                Toast.error(e);
              }
            }}
          >
            <FormattedMessage
              id="config.tasks.ai_performer_merge_suggest.generate"
              defaultMessage="Generate Suggestions…"
            />
          </Button>
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiPerformerMergeSuggest: true })}
          >
            <FormattedMessage
              id="config.tasks.ai_performer_merge_suggest.review"
              defaultMessage="Review Suggestions…"
            />
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={<FormattedMessage id="config.tasks.ai_audit.heading" />}
          subHeadingID="config.tasks.ai_audit.description"
        >
          <Button
            variant="secondary"
            type="button"
            className="mr-2"
            onClick={async () => {
              try {
                await mutateMetadataAIAudit({});
                Toast.success(
                  intl.formatMessage(
                    { id: "config.tasks.added_job_to_queue" },
                    { operation_name: "AI Audit" }
                  )
                );
              } catch (e) {
                Toast.error(e);
              }
            }}
          >
            <FormattedMessage
              id="config.tasks.ai_audit.run"
              defaultMessage="Run Audit…"
            />
          </Button>
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiAuditReview: true })}
          >
            <FormattedMessage
              id="config.tasks.ai_audit.review"
              defaultMessage="Review Findings…"
            />
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={<FormattedMessage id="config.tasks.ai_translate.heading" />}
          subHeadingID="config.tasks.ai_translate.description"
        >
          <Button
            variant="secondary"
            type="button"
            className="mr-2"
            onClick={async () => {
              try {
                await mutateMetadataAITranslate({});
                Toast.success(
                  intl.formatMessage(
                    { id: "config.tasks.added_job_to_queue" },
                    { operation_name: "AI Translation" }
                  )
                );
              } catch (e) {
                Toast.error(e);
              }
            }}
          >
            <FormattedMessage
              id="config.tasks.ai_translate.run"
              defaultMessage="Translate…"
            />
          </Button>
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiTranslationReview: true })}
          >
            <FormattedMessage
              id="config.tasks.ai_translate.review"
              defaultMessage="Review Translations…"
            />
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={
            <FormattedMessage id="config.tasks.ai_performer_discovery.heading" />
          }
          subHeadingID="config.tasks.ai_performer_discovery.description"
        >
          <Button
            variant="secondary"
            type="button"
            className="mr-2"
            onClick={async () => {
              try {
                await mutateMetadataAIPerformerDiscovery({});
                Toast.success(
                  intl.formatMessage(
                    { id: "config.tasks.added_job_to_queue" },
                    { operation_name: "AI Performer Discovery" }
                  )
                );
              } catch (e) {
                Toast.error(e);
              }
            }}
          >
            <FormattedMessage
              id="config.tasks.ai_performer_discovery.run"
              defaultMessage="Discover Performers…"
            />
          </Button>
          <Button
            variant="secondary"
            type="button"
            onClick={() => setDialogOpen({ aiPerformerDiscoveryReview: true })}
          >
            <FormattedMessage
              id="config.tasks.ai_performer_discovery.review"
              defaultMessage="Review Candidates…"
            />
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <Setting
          heading={<FormattedMessage id="config.tasks.ai_mood_tag.heading" />}
          subHeadingID="config.tasks.ai_mood_tag.description"
        >
          <Button
            variant="secondary"
            type="button"
            onClick={async () => {
              try {
                await mutateMetadataAIMoodTag({});
                Toast.success(
                  intl.formatMessage(
                    { id: "config.tasks.added_job_to_queue" },
                    { operation_name: "AI Mood Tagging" }
                  )
                );
              } catch (e) {
                Toast.error(e);
              }
            }}
          >
            <FormattedMessage
              id="config.tasks.ai_mood_tag.run"
              defaultMessage="Tag Moods…"
            />
          </Button>
          <Button
            variant="secondary"
            type="button"
            className="ml-2"
            onClick={() => setDialogOpen({ aiMoodGroups: true })}
          >
            <FormattedMessage
              id="config.tasks.ai_mood_groups.button"
              defaultMessage="Mood Groups…"
            />
          </Button>
          <Button
            variant="secondary"
            type="button"
            className="ml-2"
            onClick={() => setDialogOpen({ aiSessionBuild: true })}
          >
            <FormattedMessage
              id="config.tasks.ai_session.button"
              defaultMessage="Build a Session…"
            />
          </Button>
        </Setting>
      </SettingSection>

      <SettingSection advanced>
        <SettingGroup
          settingProps={{
            heading: (
              <>
                <FormattedMessage id="actions.auto_tag" />
                <ManualLink tab="AutoTagging">
                  <Icon icon={faQuestionCircle} />
                </ManualLink>
              </>
            ),
            subHeadingID: "config.tasks.auto_tag_based_on_filenames",
          }}
          topLevel={
            <>
              <Button
                variant="secondary"
                type="button"
                className="mr-2"
                onClick={onAutoTagClick}
              >
                <FormattedMessage id="actions.auto_tag" />…
              </Button>
              <Button
                variant="secondary"
                type="button"
                onClick={() => setDialogOpen({ autoTag: true })}
              >
                <FormattedMessage id="actions.selective_auto_tag" />…
              </Button>
            </>
          }
          collapsible
        >
          <AutoTagOptions
            options={autoTagOptions}
            setOptions={onSetAutoTagOptions}
          />
          <BooleanSetting
            id="disable_auto_tag_warning"
            headingID="config.tasks.auto_tag.disable_warning.heading"
            subHeadingID="config.tasks.auto_tag.disable_warning.description"
            checked={ui.disableAutoTagWarning ?? undefined}
            onChange={(v) => saveUI({ disableAutoTagWarning: v })}
          />
        </SettingGroup>
      </SettingSection>

      <SettingSection headingID="config.tasks.generated_content">
        <SettingGroup
          settingProps={{
            heading: (
              <>
                <FormattedMessage id="actions.generate" />
                <ManualLink tab="Tasks">
                  <Icon icon={faQuestionCircle} />
                </ManualLink>
              </>
            ),
            subHeadingID: "config.tasks.generate_desc",
          }}
          topLevel={
            <>
              <Button
                variant="secondary"
                type="button"
                onClick={() => runGenerate()}
              >
                <FormattedMessage id="actions.generate" />
              </Button>
              <Button
                variant="secondary"
                type="button"
                className="mr-2"
                onClick={() => setDialogOpen({ generate: true })}
              >
                <FormattedMessage id="actions.selective_generate" />…
              </Button>
            </>
          }
          collapsible
        >
          <GenerateOptions
            options={generateOptions}
            setOptions={onSetGenerateOptions}
          />
        </SettingGroup>
      </SettingSection>
    </Form.Group>
  );
};
