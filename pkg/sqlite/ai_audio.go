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

// StatsByPerformer aggregates audio analysis stats across a performer's scenes.
func (s *aiSceneAudioStore) StatsByPerformer(ctx context.Context, performerID int) (*models.AIPerformerAudioStats, error) {
	rows, err := dbWrapper.Queryx(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(CASE WHEN a.moans = 1 THEN 1 ELSE 0 END), 0),
		       COALESCE(AVG(a.silence_ratio), 0),
		       COALESCE(AVG(vf.duration), 0)
		FROM ai_scene_audio a
		JOIN performers_scenes ps ON ps.scene_id = a.scene_id
		LEFT JOIN scenes_files sf ON sf.scene_id = a.scene_id
		LEFT JOIN video_files vf ON vf.file_id = sf.file_id
		WHERE ps.performer_id = ?`, performerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	if !rows.Next() {
		return nil, rows.Err()
	}

	var out models.AIPerformerAudioStats
	if err := rows.Scan(&out.ScenesWithAudio, &out.MoanScenes, &out.AvgSilence, &out.AvgDuration); err != nil {
		return nil, err
	}
	if out.ScenesWithAudio > 0 {
		out.MoanRate = float64(out.MoanScenes) / float64(out.ScenesWithAudio)
	}
	return &out, rows.Err()
}

// MoanLeaderboard returns performers ordered by number of moaning scenes.
func (s *aiSceneAudioStore) MoanLeaderboard(ctx context.Context, limit int) ([]*models.AIMoanLeaderboardEntry, error) {
	rows, err := dbWrapper.Queryx(ctx, `
		SELECT ps.performer_id,
		       COUNT(*) AS scenes,
		       COALESCE(SUM(CASE WHEN a.moans = 1 THEN 1 ELSE 0 END), 0) AS moan_scenes,
		       COALESCE(AVG(a.silence_ratio), 0) AS avg_silence
		FROM ai_scene_audio a
		JOIN performers_scenes ps ON ps.scene_id = a.scene_id
		GROUP BY ps.performer_id
		ORDER BY moan_scenes DESC, scenes DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.AIMoanLeaderboardEntry
	for rows.Next() {
		var e models.AIMoanLeaderboardEntry
		if err := rows.Scan(&e.PerformerID, &e.Scenes, &e.MoanScenes, &e.AvgSilence); err != nil {
			return nil, err
		}
		if e.Scenes > 0 {
			e.MoanRate = float64(e.MoanScenes) / float64(e.Scenes)
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
