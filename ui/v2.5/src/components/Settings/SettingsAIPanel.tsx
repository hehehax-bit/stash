import React, { useState } from "react";
import { Button, Card } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import {
  BooleanSetting,
  StringSetting,
  SelectSetting,
  NumberSetting,
} from "./Inputs";
import { useToast } from "src/hooks/Toast";
import { Icon } from "src/components/Shared/Icon";
import {
  faWandMagicSparkles,
  faDatabase,
  faRobot,
  faBrain,
} from "@fortawesome/free-solid-svg-icons";
import { AIEmbeddingDialog } from "src/components/Dialogs/AIEmbeddingDialog/AIEmbeddingDialog";
import { SettingsAIInfo } from "./SettingsAIInfo";
import { FormattedMessage } from "react-intl";

const SettingsAIPanel: React.FC = () => {
  const Toast = useToast();

  const { data, loading, error } = GQL.useAiConfigQuery();
  const { data: embeddingStatsData } = GQL.useAiEmbeddingStatsQuery();
  const [configureAI] = GQL.useConfigureAiMutation();

  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [baseURL, setBaseURL] = useState<string | null>(null);
  const [endpoint, setEndpoint] = useState<string | null>(null);
  const [model, setModel] = useState<string | null>(null);
  const [embeddingModel, setEmbeddingModel] = useState<string | null>(null);
  const [imageEmbeddingModel, setImageEmbeddingModel] = useState<string | null>(
    null
  );
  const [systemPrompt, setSystemPrompt] = useState<string | null>(null);
  const [a1111Enabled, setA1111Enabled] = useState<boolean | null>(null);
  const [a1111BaseURL, setA1111BaseURL] = useState<string | null>(null);
  const [transcriptionBaseURL, setTranscriptionBaseURL] = useState<
    string | null
  >(null);
  const [transcriptionModel, setTranscriptionModel] = useState<string | null>(
    null
  );
  const [transcriptionEndpoint, setTranscriptionEndpoint] = useState<
    string | null
  >(null);
  const [tag, setTag] = useState<string | null>(null);
  const [performerClusterMinConfidence, setPerformerClusterMinConfidence] =
    useState<number | null>(null);
  const [maxTokens, setMaxTokens] = useState<number | null>(null);
  const [silenceNoiseThreshold, setSilenceNoiseThreshold] = useState<
    string | null
  >(null);
  const [silenceDurationMin, setSilenceDurationMin] = useState<number | null>(
    null
  );
  const [framesToSample, setFramesToSample] = useState<number | null>(null);
  const [translationLanguage, setTranslationLanguage] = useState<string | null>(
    null
  );
  const [imageEmbeddingBaseUrl, setImageEmbeddingBaseUrl] = useState<
    string | null
  >(null);
  const [embeddingRefreshHours, setEmbeddingRefreshHours] = useState<
    number | null
  >(null);
  const [audioAnalysisHours, setAudioAnalysisHours] = useState<number | null>(
    null
  );
  const [showEmbeddingDialog, setShowEmbeddingDialog] = useState(false);

  if (error) return <h1>{error.message}</h1>;
  if (loading) return <LoadingIndicator />;

  const config = data?.aiConfig;
  const embeddingStats = embeddingStatsData?.aiEmbeddingStats;
  const currentEnabled = enabled ?? config?.enabled ?? false;
  const currentBaseURL = baseURL ?? config?.base_url ?? "";
  const currentEndpoint = endpoint ?? config?.endpoint ?? "lmstudio";
  const currentModel = model ?? config?.model ?? "";
  const currentEmbeddingModel = embeddingModel ?? config?.embedding_model ?? "";
  const currentImageEmbeddingModel =
    imageEmbeddingModel ?? config?.image_embedding_model ?? "";
  const currentSystemPrompt = systemPrompt ?? config?.system_prompt ?? "";
  const currentA1111Enabled =
    a1111Enabled ?? config?.automatic1111_enabled ?? false;
  const currentA1111BaseURL =
    a1111BaseURL ?? config?.automatic1111_base_url ?? "";
  const currentTranscriptionBaseURL =
    transcriptionBaseURL ?? config?.transcription_base_url ?? "";
  const currentTranscriptionModel =
    transcriptionModel ?? config?.transcription_model ?? "";
  const currentTranscriptionEndpoint =
    transcriptionEndpoint ?? config?.transcription_endpoint ?? "";
  const currentTag = tag ?? config?.tag ?? "AI Tagged";
  const currentPerformerClusterMinConfidence =
    performerClusterMinConfidence ??
    config?.performer_cluster_min_confidence ??
    0.7;
  const currentMaxTokens = maxTokens ?? config?.max_tokens ?? 2048;
  const currentSilenceNoiseThreshold =
    silenceNoiseThreshold ?? config?.silence_noise_threshold ?? "-35dB";
  const currentSilenceDurationMin =
    silenceDurationMin ?? config?.silence_duration_min ?? 0.5;
  const currentFramesToSample = framesToSample ?? config?.frames_to_sample ?? 0;
  const currentTranslationLanguage =
    translationLanguage ?? config?.translation_language ?? "";
  const currentImageEmbeddingBaseUrl =
    imageEmbeddingBaseUrl ?? config?.image_embedding_base_url ?? "";
  const currentEmbeddingRefreshHours =
    embeddingRefreshHours ??
    config?.scheduled_tasks?.embedding_refresh_hours ??
    0;
  const currentAudioAnalysisHours =
    audioAnalysisHours ?? config?.scheduled_tasks?.audio_analysis_hours ?? 0;

  const currentConfig = {
    enabled: currentEnabled,
    base_url: currentBaseURL,
    endpoint: currentEndpoint,
    model: currentModel,
    embedding_model: currentEmbeddingModel,
    image_embedding_model: currentImageEmbeddingModel,
    image_embedding_base_url: currentImageEmbeddingBaseUrl,
    system_prompt: currentSystemPrompt,
    transcription_base_url: currentTranscriptionBaseURL,
    transcription_model: currentTranscriptionModel,
    transcription_endpoint: currentTranscriptionEndpoint,
    automatic1111_enabled: currentA1111Enabled,
    automatic1111_base_url: currentA1111BaseURL,
    tag: currentTag,
    performer_cluster_min_confidence: currentPerformerClusterMinConfidence,
    max_tokens: currentMaxTokens,
    silence_noise_threshold: currentSilenceNoiseThreshold,
    silence_duration_min: currentSilenceDurationMin,
    frames_to_sample: currentFramesToSample,
    translation_language: currentTranslationLanguage,
    scheduled_tasks: {
      embedding_refresh_hours: currentEmbeddingRefreshHours,
      audio_analysis_hours: currentAudioAnalysisHours,
    },
  };

  async function save(input: Partial<GQL.AiConfigInput> = {}) {
    try {
      await configureAI({
        variables: { input: { ...currentConfig, ...input } },
        refetchQueries: [GQL.AiConfigDocument],
      });
      Toast.success("AI settings saved");
    } catch (e) {
      Toast.error(e);
    }
  }

  return (
    <div className="setting-section">
      <h1>AI</h1>
      <div className="sub-heading">
        Configure AI assistant settings for LM Studio or OpenAI-compatible APIs.
      </div>
      <SettingsAIInfo />
      <Card className="mt-3">
        <BooleanSetting
          id="ai-enabled"
          heading="Enable AI"
          subHeading="Allow users to query the library using natural language via an AI assistant."
          checked={currentEnabled}
          onChange={(v) => {
            setEnabled(v);
            save({ enabled: v });
          }}
        />
        <StringSetting
          id="ai-base-url"
          heading="Base URL"
          subHeading="URL of your LM Studio (or OpenAI-compatible) API endpoint (e.g. http://localhost:1234/v1). For Ollama use http://localhost:11434."
          value={currentBaseURL}
          onChange={(v) => {
            setBaseURL(v);
            save({ base_url: v });
          }}
        />
        <SelectSetting
          id="ai-endpoint"
          heading="Embedding API"
          subHeading="Server API used for image embeddings. OpenAI-compatible works with LM Studio and Ollama's /v1 endpoint; Ollama native uses /api/embed."
          value={currentEndpoint}
          onChange={(v) => {
            setEndpoint(v);
            save({ endpoint: v });
          }}
        >
          <option value="lmstudio">
            OpenAI-compatible (LM Studio, Ollama /v1)
          </option>
          <option value="ollama">Ollama native (/api/embed)</option>
        </SelectSetting>
        <StringSetting
          id="ai-model"
          heading="Model"
          subHeading="Model name to use (e.g. local-model). Leave empty for default."
          value={currentModel}
          onChange={(v) => {
            setModel(v);
            save({ model: v });
          }}
        />
        <StringSetting
          id="ai-embedding-model"
          heading="Embedding Model"
          subHeading="Model name to use for text embeddings (e.g. nomic-embed-text). Leave empty to use the chat model."
          value={currentEmbeddingModel}
          onChange={(v) => {
            setEmbeddingModel(v);
            save({ embedding_model: v });
          }}
        />
        <StringSetting
          id="ai-image-embedding-model"
          heading="Image Embedding Model"
          subHeading="Model name to use for visual (pixel) embeddings of images, galleries, scene frames, and performer images (e.g. a CLIP-based model). Leave empty to use the text embedding model."
          value={currentImageEmbeddingModel}
          onChange={(v) => {
            setImageEmbeddingModel(v);
            save({ image_embedding_model: v });
          }}
        />
        <StringSetting
          id="ai-image-embedding-base-url"
          heading="Image Embedding Base URL"
          subHeading="Base URL of the image embedding server (e.g. http://<host>:8000). Leave empty to use the AI base URL."
          value={currentImageEmbeddingBaseUrl}
          onChange={(v) => {
            setImageEmbeddingBaseUrl(v);
            save({ image_embedding_base_url: v });
          }}
        />
        <StringSetting
          id="ai-system-prompt"
          heading="System Prompt"
          subHeading="Instructions for the AI assistant on how to behave and respond."
          value={currentSystemPrompt}
          onChange={(v) => {
            setSystemPrompt(v);
            save({ system_prompt: v });
          }}
        />
        <StringSetting
          id="ai-tag"
          heading="AI Tag"
          subHeading="Tag applied to scenes/images after AI tagging. Items with this tag will be skipped in future runs."
          value={currentTag}
          onChange={(v) => {
            setTag(v);
            save({ tag: v });
          }}
        />
        <NumberSetting
          id="ai-performer-cluster-min-confidence"
          heading="Performer Cluster Min Confidence"
          subHeading="Minimum confidence (0.0-1.0) for a face match to be applied during performer clustering."
          value={currentPerformerClusterMinConfidence}
          onChange={(v) => {
            setPerformerClusterMinConfidence(v);
            save({ performer_cluster_min_confidence: v });
          }}
        />
        <NumberSetting
          id="ai-max-tokens"
          heading="Max Tokens"
          subHeading="Maximum number of tokens allowed in an AI completion response."
          value={currentMaxTokens}
          onChange={(v) => {
            setMaxTokens(v);
            save({ max_tokens: v });
          }}
        />
        <StringSetting
          id="ai-silence-noise-threshold"
          heading="Silence Noise Threshold"
          subHeading="Silence detection threshold used during scene audio analysis, e.g. -35dB."
          value={currentSilenceNoiseThreshold}
          onChange={(v) => {
            setSilenceNoiseThreshold(v);
            save({ silence_noise_threshold: v });
          }}
        />
        <NumberSetting
          id="ai-silence-duration-min"
          heading="Silence Duration Min"
          subHeading="Minimum silence duration in seconds before a silent region is treated as a scene break."
          value={currentSilenceDurationMin}
          onChange={(v) => {
            setSilenceDurationMin(v);
            save({ silence_duration_min: v });
          }}
        />
        <NumberSetting
          id="ai-embedding-refresh-hours"
          heading="Embedding refresh interval"
          subHeading="Automatically re-embed items whose metadata changed, every N hours. 0 disables."
          value={currentEmbeddingRefreshHours}
          onChange={(v) => {
            setEmbeddingRefreshHours(v);
            save({
              scheduled_tasks: {
                ...(config?.scheduled_tasks ?? {}),
                embedding_refresh_hours: v,
              },
            });
          }}
        />
        <NumberSetting
          id="ai-audio-analysis-hours"
          heading="Audio analysis interval"
          subHeading="Automatically analyse audio of scenes without a transcript, every N hours. 0 disables."
          value={currentAudioAnalysisHours}
          onChange={(v) => {
            setAudioAnalysisHours(v);
            save({
              scheduled_tasks: {
                ...(config?.scheduled_tasks ?? {}),
                audio_analysis_hours: v,
              },
            });
          }}
        />
        <StringSetting
          id="ai-translation-language"
          heading="Translation Language"
          subHeading="Language to translate scene, image, and performer text into (e.g. en, de). Leave empty to disable AI translation."
          value={currentTranslationLanguage}
          onChange={(v) => {
            setTranslationLanguage(v);
            save({ translation_language: v });
          }}
        />
        <NumberSetting
          id="ai-frames-to-sample"
          heading="Frames To Sample"
          subHeading="Number of frames sampled from each scene during AI analysis. 0 (default) uses automatic duration-based sampling."
          value={currentFramesToSample}
          onChange={(v) => {
            setFramesToSample(v);
            save({ frames_to_sample: v });
          }}
        />
      </Card>

      <Card className="mt-3">
        <h5>
          <Icon icon={faWandMagicSparkles} /> Automatic1111 (Image Generation)
        </h5>
        <div className="sub-heading">
          Configure connection to Automatic1111's Stable Diffusion Web UI for AI
          image generation.
        </div>
        <BooleanSetting
          id="ai-a1111-enabled"
          heading="Enable Image Generation"
          subHeading="Allow the AI assistant to generate images using Automatic1111."
          checked={currentA1111Enabled}
          onChange={(v) => {
            setA1111Enabled(v);
            save({ automatic1111_enabled: v });
          }}
        />
        <StringSetting
          id="ai-a1111-base-url"
          heading="Automatic1111 Base URL"
          subHeading="URL of your Automatic1111 Web UI (e.g. http://localhost:7860)."
          value={currentA1111BaseURL}
          onChange={(v) => {
            setA1111BaseURL(v);
            save({ automatic1111_base_url: v });
          }}
        />
      </Card>

      <Card className="mt-3">
        <h5>
          <Icon icon={faRobot} /> Transcription
        </h5>
        <div className="sub-heading">
          Configure an OpenAI-compatible (e.g. Whisper) or whisper.cpp (e.g.
          OpenWhispr, whisper-server) endpoint used for scene audio
          transcription and analysis.
        </div>
        <StringSetting
          id="ai-transcription-base-url"
          heading="Transcription Base URL"
          subHeading="URL of the transcription endpoint (e.g. http://localhost:9000/v1 or your OpenWhispr server). Leave empty to disable transcription."
          value={currentTranscriptionBaseURL}
          onChange={(v) => {
            setTranscriptionBaseURL(v);
            save({ transcription_base_url: v });
          }}
        />
        <StringSetting
          id="ai-transcription-model"
          heading="Transcription Model"
          subHeading="Model name to use for transcription (e.g. whisper-1). Leave empty for default."
          value={currentTranscriptionModel}
          onChange={(v) => {
            setTranscriptionModel(v);
            save({ transcription_model: v });
          }}
        />
        <StringSetting
          id="ai-transcription-endpoint"
          heading="Transcription Endpoint Path"
          subHeading="URL path for transcription requests. Default /audio/transcriptions (OpenAI-compatible); use /inference for older whisper.cpp servers."
          value={currentTranscriptionEndpoint}
          onChange={(v) => {
            setTranscriptionEndpoint(v);
            save({ transcription_endpoint: v });
          }}
        />
      </Card>

      <Card className="mt-3">
        <h5>
          <Icon icon={faDatabase} /> AI Embeddings
        </h5>
        <div className="sub-heading">
          <FormattedMessage id="config.tasks.ai_embedding.description" />
        </div>
        <div className="ai-embedding-stats">
          {embeddingStats ? (
            <table className="table table-sm">
              <tbody>
                <tr>
                  <td>Scenes</td>
                  <td className="text-right">{embeddingStats.scenes}</td>
                </tr>
                <tr>
                  <td>Images</td>
                  <td className="text-right">{embeddingStats.images}</td>
                </tr>
                <tr>
                  <td>Performers</td>
                  <td className="text-right">{embeddingStats.performers}</td>
                </tr>
                <tr>
                  <td>Studios</td>
                  <td className="text-right">{embeddingStats.studios}</td>
                </tr>
                <tr>
                  <td>Tags</td>
                  <td className="text-right">{embeddingStats.tags}</td>
                </tr>
                <tr>
                  <td>Galleries</td>
                  <td className="text-right">{embeddingStats.galleries}</td>
                </tr>
                <tr className="table-active">
                  <td>Total</td>
                  <td className="text-right">{embeddingStats.total}</td>
                </tr>
              </tbody>
            </table>
          ) : (
            <LoadingIndicator />
          )}
          <div className="text-muted">
            Items with an embedding are marked with a <Icon icon={faBrain} />{" "}
            badge in the lists and can be filtered with the "Has embedding"
            filter.
          </div>
        </div>
        <Button variant="primary" onClick={() => setShowEmbeddingDialog(true)}>
          <Icon icon={faDatabase} />{" "}
          <FormattedMessage id="actions.generate_embeddings" />
        </Button>
      </Card>

      {showEmbeddingDialog && (
        <AIEmbeddingDialog onClose={() => setShowEmbeddingDialog(false)} />
      )}
    </div>
  );
};

export default SettingsAIPanel;
