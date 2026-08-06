package models

import "context"

type AITranslation struct {
	ID             int64  `json:"id"`
	EntityType     string `json:"entity_type"`
	EntityID       int    `json:"entity_id"`
	Language       string `json:"language"`
	Field          string `json:"field"`
	TranslatedText string `json:"translated_text"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

type AITranslationReader interface {
	FindByID(ctx context.Context, id int64) (*AITranslation, error)
	FindByStatus(ctx context.Context, status string) ([]*AITranslation, error)
	FindByEntity(ctx context.Context, entityType string, entityID int, language string) (*AITranslation, error)
}

type AITranslationWriter interface {
	Create(ctx context.Context, translation *AITranslation) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateStatusByStatus(ctx context.Context, fromStatus, toStatus string) (int64, error)
}

type AITranslationReaderWriter interface {
	AITranslationReader
	AITranslationWriter
}
