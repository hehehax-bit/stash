package models

import "context"

type AIPerformerSuggestion struct {
	ID                int64   `json:"id"`
	SourcePerformerID int     `json:"source_performer_id"`
	TargetPerformerID int     `json:"target_performer_id"`
	Confidence        float64 `json:"confidence"`
	Status            string  `json:"status"`
	CreatedAt         int64   `json:"created_at"`
	UpdatedAt         int64   `json:"updated_at"`
}

type AIPerformerSuggestionReader interface {
	FindByID(ctx context.Context, id int64) (*AIPerformerSuggestion, error)
	FindByStatus(ctx context.Context, status string) ([]*AIPerformerSuggestion, error)
	FindPair(ctx context.Context, sourceID, targetID int) (*AIPerformerSuggestion, error)
}

type AIPerformerSuggestionWriter interface {
	Create(ctx context.Context, suggestion *AIPerformerSuggestion) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateStatusByStatus(ctx context.Context, fromStatus, toStatus string) (int64, error)
	DeleteByPerformerID(ctx context.Context, performerID int) error
}

type AIPerformerSuggestionReaderWriter interface {
	AIPerformerSuggestionReader
	AIPerformerSuggestionWriter
}
