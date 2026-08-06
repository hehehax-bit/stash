package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiTranslationStore struct{}

func NewAITranslationStore() *aiTranslationStore {
	return &aiTranslationStore{}
}

const aiTranslationColumns = "id, entity_type, entity_id, language, field, translated_text, status, created_at, updated_at"

func (s *aiTranslationStore) FindByID(ctx context.Context, id int64) (*models.AITranslation, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiTranslationColumns+` FROM ai_translations WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out[0], nil
}

func (s *aiTranslationStore) FindByEntity(ctx context.Context, entityType string, entityID int, language string) (*models.AITranslation, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiTranslationColumns+` FROM ai_translations WHERE entity_type = ? AND entity_id = ? AND language = ? AND status != ? LIMIT 1`,
		entityType, entityID, language, models.SuggestionStatusRejected)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out[0], nil
}

func (s *aiTranslationStore) FindByStatus(ctx context.Context, status string) ([]*models.AITranslation, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiTranslationColumns+` FROM ai_translations WHERE status = ? ORDER BY created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRows(rows)
}

func (s *aiTranslationStore) Create(ctx context.Context, translation *models.AITranslation) error {
	now := time.Now().Unix()
	translation.CreatedAt = now
	translation.UpdatedAt = now
	if translation.Status == "" {
		translation.Status = models.SuggestionStatusPending
	}

	_, err := dbWrapper.Exec(ctx, `
		INSERT INTO ai_translations (entity_type, entity_id, language, field, translated_text, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_id, language, field) DO UPDATE SET
			translated_text = excluded.translated_text,
			status = excluded.status,
			updated_at = excluded.updated_at
	`, translation.EntityType, translation.EntityID, translation.Language, translation.Field,
		translation.TranslatedText, translation.Status, translation.CreatedAt, translation.UpdatedAt)
	if err != nil {
		return err
	}

	return dbWrapper.Get(ctx, &translation.ID, `SELECT id FROM ai_translations WHERE entity_type = ? AND entity_id = ? AND language = ? AND field = ?`,
		translation.EntityType, translation.EntityID, translation.Language, translation.Field)
}

func (s *aiTranslationStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().Unix()
	_, err := dbWrapper.Exec(ctx, `UPDATE ai_translations SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	return err
}

func (s *aiTranslationStore) UpdateStatusByStatus(ctx context.Context, fromStatus, toStatus string) (int64, error) {
	now := time.Now().Unix()
	res, err := dbWrapper.Exec(ctx, `UPDATE ai_translations SET status = ?, updated_at = ? WHERE status = ?`, toStatus, now, fromStatus)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *aiTranslationStore) scanRows(rows *sqlx.Rows) ([]*models.AITranslation, error) {
	var out []*models.AITranslation
	for rows.Next() {
		var t models.AITranslation
		if err := rows.Scan(&t.ID, &t.EntityType, &t.EntityID, &t.Language, &t.Field, &t.TranslatedText, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}
