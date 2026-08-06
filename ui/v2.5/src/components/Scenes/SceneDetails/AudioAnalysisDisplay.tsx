import React, { useState } from "react";
import { FormattedMessage, useIntl } from "react-intl";
import * as GQL from "src/core/generated-graphql";
import { PatchComponent } from "src/patch";

function formatTime(seconds: number): string {
  const s = Math.floor(seconds);
  const m = Math.floor(s / 60);
  const rem = s % 60;
  return `${String(m).padStart(2, "0")}:${String(rem).padStart(2, "0")}`;
}

interface IAudioAnalysisDisplayProps {
  sceneID: string;
}

const AudioAnalysisDisplay: React.FC<IAudioAnalysisDisplayProps> =
  PatchComponent("AudioAnalysisDisplay", (props) => {
    const intl = useIntl();
    const { data, loading, error } = GQL.useAiSceneAudioQuery({
      variables: {
        scene_id: props.sceneID,
      },
      skip: !props.sceneID,
    });
    const [showTranscript, setShowTranscript] = useState(false);

    if (loading || !data?.aiSceneAudio) {
      return null;
    }

    if (error) {
      return null;
    }

    const audio = data.aiSceneAudio;

    if (!audio.has_audio) {
      return (
        <div className="ai-audio-analysis-display mb-3 p-3 border rounded">
          <h6 className="mb-3">
            <FormattedMessage
              id="ai.audio_analysis.title"
              defaultMessage="AI Audio Analysis"
            />
          </h6>
          <div className="text-muted">
            <FormattedMessage
              id="ai.audio_analysis.no_audio"
              defaultMessage="No audio stream detected in this scene."
            />
          </div>
        </div>
      );
    }

    const audioTags = [];
    if (audio.music)
      audioTags.push(
        intl.formatMessage({
          id: "ai.audio_analysis.music",
          defaultMessage: "Music",
        })
      );
    if (audio.speech)
      audioTags.push(
        intl.formatMessage({
          id: "ai.audio_analysis.speech",
          defaultMessage: "Speech",
        })
      );
    if (audio.moans)
      audioTags.push(
        intl.formatMessage({
          id: "ai.audio_analysis.moans",
          defaultMessage: "Moans",
        })
      );
    if (audio.ambient)
      audioTags.push(
        intl.formatMessage({
          id: "ai.audio_analysis.ambient",
          defaultMessage: "Ambient",
        })
      );

    return (
      <div className="ai-audio-analysis-display mb-3 p-3 border rounded">
        <h6 className="mb-3">
          <FormattedMessage
            id="ai.audio_analysis.title"
            defaultMessage="AI Audio Analysis"
          />
        </h6>

        <div className="row mb-3">
          <div className="col-md-6">
            <strong>
              <FormattedMessage
                id="ai.audio_analysis.silence_ratio"
                defaultMessage="Silence Ratio"
              />
              :
            </strong>{" "}
            <span className="ml-2">{audio.silence_ratio}%</span>
          </div>
          <div className="col-md-6">
            <strong>
              <FormattedMessage
                id="ai.audio_analysis.audio_codec"
                defaultMessage="Audio Codec"
              />
              :
            </strong>{" "}
            <span className="ml-2">{audio.audio_codec}</span>
          </div>
        </div>

        {audioTags.length > 0 && (
          <div className="mb-3">
            <strong>
              <FormattedMessage
                id="ai.audio_analysis.content_types"
                defaultMessage="Content Types"
              />
              :
            </strong>
            <div className="mt-1">
              {audioTags.map((tag) => (
                <span key={tag} className="badge badge-secondary mr-1 mb-1">
                  {tag}
                </span>
              ))}
            </div>
          </div>
        )}

        {audio.summary && (
          <div className="mb-3">
            <strong>
              <FormattedMessage
                id="ai.audio_analysis.summary"
                defaultMessage="Summary"
              />
              :
            </strong>
            <p className="mt-1 mb-0">{audio.summary}</p>
          </div>
        )}

        {audio.transcript && (
          <div className="mb-3">
            <button
              className="btn btn-sm btn-outline-secondary"
              onClick={() => setShowTranscript(!showTranscript)}
            >
              {showTranscript
                ? intl.formatMessage({
                    id: "ai.audio_analysis.hide_transcript",
                    defaultMessage: "Hide Transcript",
                  })
                : intl.formatMessage({
                    id: "ai.audio_analysis.show_transcript",
                    defaultMessage: "Show Transcript",
                  })}
            </button>
            {showTranscript && (
              <div className="mt-2 p-2 ai-audio-transcript border rounded small">
                {audio.transcript_segments &&
                audio.transcript_segments.length > 0 ? (
                  <div>
                    {audio.transcript_segments.map((seg, i) => (
                      <div key={i} className="mb-1">
                        <span className="text-muted mr-2">
                          {formatTime(seg.start)}
                        </span>
                        {seg.text}
                      </div>
                    ))}
                  </div>
                ) : (
                  <pre className="mb-0 whitespace-pre-wrap">
                    {audio.transcript}
                  </pre>
                )}
              </div>
            )}
          </div>
        )}
      </div>
    );
  });

export default AudioAnalysisDisplay;
