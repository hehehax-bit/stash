package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiMediaQualityStore struct{}

func NewAIMediaQualityStore() *aiMediaQualityStore {
	return &aiMediaQualityStore{}
}

const aiMediaQualityColumns = "id, entity_type, entity_id, quality_score, visual_clarity, lighting, composition, camera_work, notes, created_at, updated_at"

func (s *aiMediaQualityStore) FindByID(ctx context.Context, id int64) (*models.AIMediaQuality, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiMediaQualityColumns+` FROM ai_media_quality WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	quality, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(quality) == 0 {
		return nil, nil
	}
	return quality[0], nil
}

func (s *aiMediaQualityStore) FindByEntity(ctx context.Context, entityType string, entityID int) (*models.AIMediaQuality, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiMediaQualityColumns+` FROM ai_media_quality WHERE entity_type = ? AND entity_id = ?`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	quality, err := s.scanRows(rows)
	if err != nil {
		return nil, err
	}
	if len(quality) == 0 {
		return nil, nil
	}
	return quality[0], nil
}

func (s *aiMediaQualityStore) FindAssessedEntities(ctx context.Context, entityType string) ([]int, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT entity_id FROM ai_media_quality WHERE entity_type = ?`, entityType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *aiMediaQualityStore) Upsert(ctx context.Context, quality *models.AIMediaQuality) error {
	now := time.Now().Unix()
	quality.UpdatedAt = now
	if quality.CreatedAt == 0 {
		quality.CreatedAt = now
	}

	_, err := dbWrapper.Exec(ctx, `
		INSERT INTO ai_media_quality (entity_type, entity_id, quality_score, visual_clarity, lighting, composition, camera_work, notes, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(entity_type, entity_id) DO UPDATE SET
			quality_score = excluded.quality_score,
			visual_clarity = excluded.visual_clarity,
			lighting = excluded.lighting,
			composition = excluded.composition,
			camera_work = excluded.camera_work,
			notes = excluded.notes,
			updated_at = excluded.updated_at
	`, quality.EntityType, quality.EntityID, quality.QualityScore, quality.VisualClarity,
		quality.Lighting, quality.Composition, quality.CameraWork, quality.Notes,
		quality.CreatedAt, quality.UpdatedAt)

	if err == nil {
		_ = dbWrapper.Get(ctx, &quality.ID, `SELECT id FROM ai_media_quality WHERE entity_type = ? AND entity_id = ?`, quality.EntityType, quality.EntityID)
	}
	return err
}

func (s *aiMediaQualityStore) Delete(ctx context.Context, id int64) error {
	_, err := dbWrapper.Exec(ctx, `DELETE FROM ai_media_quality WHERE id = ?`, id)
	return err
}

func (s *aiMediaQualityStore) scanRows(rows *sqlx.Rows) ([]*models.AIMediaQuality, error) {
	var out []*models.AIMediaQuality
	for rows.Next() {
		var q models.AIMediaQuality
		if err := rows.Scan(&q.ID, &q.EntityType, &q.EntityID, &q.QualityScore, &q.VisualClarity, &q.Lighting, &q.Composition, &q.CameraWork, &q.Notes, &q.CreatedAt, &q.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &q)
	}
	return out, nil
}
