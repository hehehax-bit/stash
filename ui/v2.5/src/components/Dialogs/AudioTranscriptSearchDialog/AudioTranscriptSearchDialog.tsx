import React, { useState } from "react";
import { Form } from "react-bootstrap";
import * as GQL from "src/core/generated-graphql";
import { ModalComponent } from "src/components/Shared/Modal";
import { LoadingIndicator } from "src/components/Shared/LoadingIndicator";
import { SceneCard } from "src/components/Scenes/SceneCard";
import { useIntl, FormattedMessage } from "react-intl";
import { faMicrophoneAlt } from "@fortawesome/free-solid-svg-icons";

interface IAudioTranscriptSearchDialogProps {
  onClose: () => void;
}

function snippet(transcript: string, query: string, maxLen = 160): string {
  const idx = transcript.toLowerCase().indexOf(query.toLowerCase());
  const start = Math.max(0, idx - 40);
  const slice = transcript.slice(start, start + maxLen);
  return (start > 0 ? "…" : "") + slice + (slice.length >= maxLen ? "…" : "");
}

export const AudioTranscriptSearchDialog: React.FC<
  IAudioTranscriptSearchDialogProps
> = ({ onClose }) => {
  const intl = useIntl();
  const [query, setQuery] = useState("");

  const { data, loading } = GQL.useAiSceneAudioSearchQuery({
    variables: { query: query.trim(), limit: 50 },
    skip: query.trim().length === 0,
  });

  const results = data?.aiSceneAudioSearch ?? [];

  return (
    <ModalComponent
      show
      icon={faMicrophoneAlt}
      header={intl.formatMessage({
        id: "config.tasks.ai_audio_search.heading",
      })}
      cancel={{
        onClick: () => onClose(),
        text: intl.formatMessage({ id: "actions.close" }),
        variant: "secondary",
      }}
    >
      <Form>
        <Form.Group>
          <Form.Control
            type="text"
            value={query}
            autoFocus
            placeholder={intl.formatMessage({
              id: "config.tasks.ai_audio_search.placeholder",
            })}
            onChange={(e: React.ChangeEvent<HTMLInputElement>) =>
              setQuery(e.currentTarget.value)
            }
          />
        </Form.Group>
      </Form>

      {loading && <LoadingIndicator inline />}

      {!loading && query.trim() !== "" && results.length === 0 && (
        <div className="text-muted">
          <FormattedMessage id="config.tasks.ai_audio_search.no_results" />
        </div>
      )}

      <div className="row">
        {results.map(
          (r) =>
            r.scene && (
              <div
                key={r.scene.id}
                className="col-6 col-sm-4 col-md-3 col-xl-2"
              >
                <SceneCard scene={r.scene} />
                <div className="small text-muted mb-3">
                  {snippet(r.transcript ?? "", query)}
                </div>
              </div>
            )
        )}
      </div>
    </ModalComponent>
  );
};

export default AudioTranscriptSearchDialog;
