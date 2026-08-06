package models

import "context"

type AIAudit struct {
	ID           int64  `json:"id"`
	EntityType   string `json:"entity_type"`
	EntityID     int    `json:"entity_id"`
	Field        string `json:"field"`
	CurrentValue string `json:"current_value"`
	AIValue      string `json:"ai_value"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

type AIAuditReader interface {
	FindByID(ctx context.Context, id int64) (*AIAudit, error)
	FindByStatus(ctx context.Context, status string) ([]*AIAudit, error)
}

type AIAuditWriter interface {
	Create(ctx context.Context, audit *AIAudit) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateStatusByStatus(ctx context.Context, fromStatus, toStatus string) (int64, error)
}

type AIAuditReaderWriter interface {
	AIAuditReader
	AIAuditWriter
}
