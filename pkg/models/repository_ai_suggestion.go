package models

import "context"

const (
	SuggestionStatusPending  = "pending"
	SuggestionStatusAccepted = "accepted"
	SuggestionStatusRejected = "rejected"
)

type AISuggestion struct {
	ID         int64    `json:"id"`
	EntityType string   `json:"entity_type"`
	EntityID   int      `json:"entity_id"`
	Title      string   `json:"title"`
	Details    string   `json:"details"`
	Performers []string `json:"performers"`
	Tags       []string `json:"tags"`
	Status     string   `json:"status"`
	CreatedAt  int64    `json:"created_at"`
	UpdatedAt  int64    `json:"updated_at"`
}

type AISuggestionReader interface {
	FindByID(ctx context.Context, id int64) (*AISuggestion, error)
	FindByStatus(ctx context.Context, status string) ([]*AISuggestion, error)
	FindByEntity(ctx context.Context, entityType string, entityID int) ([]*AISuggestion, error)
	FindPendingByEntityType(ctx context.Context, entityType string) ([]*AISuggestion, error)
}

type AISuggestionWriter interface {
	Create(ctx context.Context, suggestion *AISuggestion) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
}

type AISuggestionReaderWriter interface {
	AISuggestionReader
	AISuggestionWriter
}
