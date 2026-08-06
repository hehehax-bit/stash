package models

import "context"

type AISceneAudio struct {
	ID                 int64  `json:"id"`
	SceneID            int    `json:"scene_id"`
	HasAudio           bool   `json:"has_audio"`
	SilenceRatio       int    `json:"silence_ratio"`
	Music              bool   `json:"music"`
	Speech             bool   `json:"speech"`
	Moans              bool   `json:"moans"`
	Ambient            bool   `json:"ambient"`
	Transcript         string `json:"transcript"`
	TranscriptSegments string `json:"transcript_segments"`
	Summary            string `json:"summary"`
	AudioCodec         string `json:"audio_codec"`
	CreatedAt          int64  `json:"created_at"`
	UpdatedAt          int64  `json:"updated_at"`
}

type AISceneAudioReader interface {
	FindBySceneID(ctx context.Context, sceneID int) (*AISceneAudio, error)
	FindAssessedScenes(ctx context.Context) ([]int, error)
	SearchByTranscript(ctx context.Context, query string, limit int) ([]*AISceneAudio, error)
}

type AISceneAudioWriter interface {
	Upsert(ctx context.Context, audio *AISceneAudio) error
	Delete(ctx context.Context, id int64) error
}

type AISceneAudioReaderWriter interface {
	AISceneAudioReader
	AISceneAudioWriter
}
