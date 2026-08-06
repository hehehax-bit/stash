package models

import "context"

const (
	FileRenameStatusPending  = "pending"
	FileRenameStatusApplied  = "applied"
	FileRenameStatusRejected = "rejected"
)

type AIFileRename struct {
	ID            int64  `json:"id"`
	EntityType    string `json:"entity_type"`
	EntityID      int    `json:"entity_id"`
	CurrentName   string `json:"current_name"`
	SuggestedName string `json:"suggested_name"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

type AIFileRenameReader interface {
	FindByID(ctx context.Context, id int64) (*AIFileRename, error)
	FindByStatus(ctx context.Context, status string) ([]*AIFileRename, error)
	FindPendingByEntityType(ctx context.Context, entityType string) ([]*AIFileRename, error)
	FindByEntity(ctx context.Context, entityType string, entityID int) (*AIFileRename, error)
}

type AIFileRenameWriter interface {
	Upsert(ctx context.Context, rename *AIFileRename) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	Delete(ctx context.Context, id int64) error
}

type AIFileRenameReaderWriter interface {
	AIFileRenameReader
	AIFileRenameWriter
}
