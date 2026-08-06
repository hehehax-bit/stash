package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiSceneAudioStore struct{}

func NewAISceneAudioStore() *aiSceneAudioStore {
	return &aiSceneAudioStore{}
}

const aiSceneAudioColumns = "id, scene_id, has_audio, silence_ratio, music, speech, moans, ambient, transcript, transcript_segments, summary, audio_codec, created_at, updated_at"

func (s *aiSceneAudioStore) FindBySceneID(ctx context.Context, sceneID int) (*models.AISceneAudio, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiSceneAudioColumns+` FROM ai_scene_audio WHERE scene_id = ?`, sceneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	audio, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(audio) == 0 {
		return nil, nil
	}
	return audio[0], nil
}

func (s *aiSceneAudioStore) FindAssessedScenes(ctx context.Context) ([]int, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT scene_id FROM ai_scene_audio`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// SearchByTranscript returns scene audio rows whose transcript contains the
// given query text, most recently updated first.
func (s *aiSceneAudioStore) SearchByTranscript(ctx context.Context, query string, limit int) ([]*models.AISceneAudio, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiSceneAudioColumns+` FROM ai_scene_audio WHERE transcript LIKE ? ORDER BY updated_at DESC LIMIT ?`, like(query), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRows(rows)
}

func (s *aiSceneAudioStore) Upsert(ctx context.Context, audio *models.AISceneAudio) error {
	now := time.Now().Unix()
	audio.UpdatedAt = now
	if audio.CreatedAt == 0 {
		audio.CreatedAt = now
	}

	hasAudio, music, speech, moans, ambient := 0, 0, 0, 0, 0
	if audio.HasAudio {
		hasAudio = 1
	}
	if audio.Music {
		music = 1
	}
	if audio.Speech {
		speech = 1
	}
	if audio.Moans {
		moans = 1
	}
	if audio.Ambient {
		ambient = 1
	}

	_, err := dbWrapper.Exec(ctx, `
		INSERT INTO ai_scene_audio (scene_id, has_audio, silence_ratio, music, speech, moans, ambient, transcript, transcript_segments, summary, audio_codec, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(scene_id) DO UPDATE SET
			has_audio = excluded.has_audio,
			silence_ratio = excluded.silence_ratio,
			music = excluded.music,
			speech = excluded.speech,
			moans = excluded.moans,
			ambient = excluded.ambient,
			transcript = excluded.transcript,
			transcript_segments = excluded.transcript_segments,
			summary = excluded.summary,
			audio_codec = excluded.audio_codec,
			updated_at = excluded.updated_at
	`, audio.SceneID, hasAudio, audio.SilenceRatio, music, speech, moans, ambient,
		audio.Transcript, audio.TranscriptSegments, audio.Summary, audio.AudioCodec, audio.CreatedAt, audio.UpdatedAt)

	if err == nil {
		_ = dbWrapper.Get(ctx, &audio.ID, `SELECT id FROM ai_scene_audio WHERE scene_id = ?`, audio.SceneID)
	}
	return err
}

func (s *aiSceneAudioStore) Delete(ctx context.Context, id int64) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_scene_audio WHERE id = ?`, id)
	return err
}

func (s *aiSceneAudioStore) scanRows(rows *sqlx.Rows) ([]*models.AISceneAudio, error) {
	var out []*models.AISceneAudio
	for rows.Next() {
		var a models.AISceneAudio
		var hasAudio, music, speech, moans, ambient int
		if err := rows.Scan(&a.ID, &a.SceneID, &hasAudio, &a.SilenceRatio, &music, &speech, &moans, &ambient, &a.Transcript, &a.TranscriptSegments, &a.Summary, &a.AudioCodec, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		a.HasAudio = hasAudio != 0
		a.Music = music != 0
		a.Speech = speech != 0
		a.Moans = moans != 0
		a.Ambient = ambient != 0
		out = append(out, &a)
	}
	return out, nil
}
