package models

import "context"

type AIPerformerCandidate struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	MemberCount int    `json:"member_count"`
	EntityType  string `json:"entity_type"`
	EntityID    int    `json:"entity_id"`
	MemberIDs   []int  `json:"member_ids"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"created_at"`
	UpdatedAt   int64  `json:"updated_at"`
}

type AIPerformerCandidateReader interface {
	FindByID(ctx context.Context, id int64) (*AIPerformerCandidate, error)
	FindByStatus(ctx context.Context, status string) ([]*AIPerformerCandidate, error)
}

type AIPerformerCandidateWriter interface {
	Create(ctx context.Context, candidate *AIPerformerCandidate) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateStatusByStatus(ctx context.Context, fromStatus, toStatus string) (int64, error)
	DeleteByID(ctx context.Context, id int64) error
}

type AIPerformerCandidateReaderWriter interface {
	AIPerformerCandidateReader
	AIPerformerCandidateWriter
}
