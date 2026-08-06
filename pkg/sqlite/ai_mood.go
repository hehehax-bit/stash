package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiMoodStore struct{}

func NewAIMoodStore() *aiMoodStore {
	return &aiMoodStore{}
}

func (s *aiMoodStore) FindBySceneID(ctx context.Context, sceneID int) ([]*models.AISceneMood, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT id, scene_id, mood, confidence, created_at FROM ai_scene_moods WHERE scene_id = ? ORDER BY confidence DESC`, sceneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMoods(rows)
}

func (s *aiMoodStore) FindMoods(ctx context.Context, mood string, limit int) ([]int, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT scene_id FROM ai_scene_moods WHERE mood = ? LIMIT ?`, mood, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *aiMoodStore) Create(ctx context.Context, mood *models.AISceneMood) error {
	mood.CreatedAt = time.Now().Unix()
	_, err := dbWrapper.Exec(ctx, `INSERT INTO ai_scene_moods (scene_id, mood, confidence, created_at) VALUES (?, ?, ?, ?)`,
		mood.SceneID, mood.Mood, mood.Confidence, mood.CreatedAt)
	return err
}

func (s *aiMoodStore) DeleteBySceneID(ctx context.Context, sceneID int) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_scene_moods WHERE scene_id = ?`, sceneID)
	return err
}

func scanMoods(rows *sqlx.Rows) ([]*models.AISceneMood, error) {
	var out []*models.AISceneMood
	for rows.Next() {
		var m models.AISceneMood
		if err := rows.Scan(&m.ID, &m.SceneID, &m.Mood, &m.Confidence, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// MoodCounts returns the number of scenes per mood, ordered by count.
func (s *aiMoodStore) MoodCounts(ctx context.Context) ([]*models.AIMoodCount, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT mood, COUNT(*) FROM ai_scene_moods GROUP BY mood ORDER BY COUNT(*) DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.AIMoodCount
	for rows.Next() {
		var e models.AIMoodCount
		if err := rows.Scan(&e.Mood, &e.Count); err != nil {
			return nil, err
		}
		out = append(out, &e)
	}
	return out, rows.Err()
}
