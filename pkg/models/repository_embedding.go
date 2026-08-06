package models

import "context"

type Embedding struct {
	ID         int64  `json:"id"`
	EntityType string `json:"entity_type"`
	EntityID   int    `json:"entity_id"`
	Model      string `json:"model"`
	CreatedAt  int64  `json:"created_at"`
}

type SimilarityResult struct {
	EntityID int
	Score    float64
}

type DuplicateGroup struct {
	EntityIDs []int
}

type EmbeddingReader interface {
	FindByEntity(ctx context.Context, entityType string, entityID int, model string) ([]float32, error)
	FindByEntityType(ctx context.Context, entityType string, model string) (map[int][]float32, error)
	SearchSimilar(ctx context.Context, entityType string, model string, query []float32, limit int) ([]SimilarityResult, error)
	FindNearDuplicates(ctx context.Context, entityType string, model string, threshold float64, minGroupSize int, maxEntities int) ([]DuplicateGroup, error)
	HasEmbedding(ctx context.Context, entityType string, entityID int) (bool, error)
	CountByEntityType(ctx context.Context) (map[string]int, error)
	FindStale(ctx context.Context, entityType, model string) ([]int, error)
}

type EmbeddingWriter interface {
	Set(ctx context.Context, entityType string, entityID int, model string, embedding []float32) error
	DeleteByEntity(ctx context.Context, entityType string, entityID int) error
	DeleteByEntityType(ctx context.Context, entityType string) error
	DeleteByModel(ctx context.Context, model string) error
}

type EmbeddingReaderWriter interface {
	EmbeddingReader
	EmbeddingWriter
}
