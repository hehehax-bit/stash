package models

import "context"

type AISceneMood struct {
	ID         int64   `json:"id"`
	SceneID    int     `json:"scene_id"`
	Mood       string  `json:"mood"`
	Confidence float64 `json:"confidence"`
	CreatedAt  int64   `json:"created_at"`
}

type AIMoodCount struct {
	Mood  string `json:"mood"`
	Count int    `json:"count"`
}

type AIMoodReader interface {
	FindBySceneID(ctx context.Context, sceneID int) ([]*AISceneMood, error)
	FindMoods(ctx context.Context, mood string, limit int) ([]int, error)
	MoodCounts(ctx context.Context) ([]*AIMoodCount, error)
}

type AIMoodWriter interface {
	Create(ctx context.Context, mood *AISceneMood) error
	DeleteBySceneID(ctx context.Context, sceneID int) error
}

type AIMoodReaderWriter interface {
	AIMoodReader
	AIMoodWriter
}
