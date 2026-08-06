package models

import "context"

type AIMediaQuality struct {
	ID            int64  `json:"id"`
	EntityType    string `json:"entity_type"`
	EntityID      int    `json:"entity_id"`
	QualityScore  int    `json:"quality_score"`
	VisualClarity int    `json:"visual_clarity"`
	Lighting      int    `json:"lighting"`
	Composition   int    `json:"composition"`
	CameraWork    int    `json:"camera_work"`
	Notes         string `json:"notes"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type AIMediaQualityReader interface {
	FindByID(ctx context.Context, id int64) (*AIMediaQuality, error)
	FindByEntity(ctx context.Context, entityType string, entityID int) (*AIMediaQuality, error)
	FindAssessedEntities(ctx context.Context, entityType string) ([]int, error)
}

type AIMediaQualityWriter interface {
	Upsert(ctx context.Context, quality *AIMediaQuality) error
	Delete(ctx context.Context, id int64) error
}

type AIMediaQualityReaderWriter interface {
	AIMediaQualityReader
	AIMediaQualityWriter
}
