package sqlite

import (
	"context"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/stashapp/stash/pkg/models"
)

type aiAuditStore struct{}

func NewAIAuditStore() *aiAuditStore {
	return &aiAuditStore{}
}

const aiAuditColumns = "id, entity_type, entity_id, field, current_value, ai_value, status, created_at, updated_at"

func (s *aiAuditStore) FindByID(ctx context.Context, id int64) (*models.AIAudit, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiAuditColumns+` FROM ai_audits WHERE id = ?`, id)
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

func (s *aiAuditStore) FindByStatus(ctx context.Context, status string) ([]*models.AIAudit, error) {
	rows, err := dbWrapper.Queryx(ctx, `SELECT `+aiAuditColumns+` FROM ai_audits WHERE status = ? ORDER BY created_at DESC`, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanRows(rows)
}

func (s *aiAuditStore) Create(ctx context.Context, audit *models.AIAudit) error {
	now := time.Now().Unix()
	audit.CreatedAt = now
	audit.UpdatedAt = now
	if audit.Status == "" {
		audit.Status = models.SuggestionStatusPending
	}

	// replace any pending finding for the same entity/field
	if _, err := dbWrapper.Exec(ctx, `DELETE FROM ai_audits WHERE entity_type = ? AND entity_id = ? AND field = ? AND status = ?`,
		audit.EntityType, audit.EntityID, audit.Field, models.SuggestionStatusPending); err != nil {
		return err
	}

	_, err := dbWrapper.Exec(ctx, `
		INSERT INTO ai_audits (entity_type, entity_id, field, current_value, ai_value, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, audit.EntityType, audit.EntityID, audit.Field, audit.CurrentValue, audit.AIValue,
		audit.Status, audit.CreatedAt, audit.UpdatedAt)
	if err != nil {
		return err
	}

	return dbWrapper.Get(ctx, &audit.ID, `SELECT id FROM ai_audits WHERE entity_type = ? AND entity_id = ? AND field = ? AND status = ?`,
		audit.EntityType, audit.EntityID, audit.Field, audit.Status)
}

func (s *aiAuditStore) UpdateStatus(ctx context.Context, id int64, status string) error {
	now := time.Now().Unix()
	_, err := dbWrapper.Exec(ctx, `UPDATE ai_audits SET status = ?, updated_at = ? WHERE id = ?`, status, now, id)
	return err
}

func (s *aiAuditStore) UpdateStatusByStatus(ctx context.Context, fromStatus, toStatus string) (int64, error) {
	now := time.Now().Unix()
	res, err := dbWrapper.Exec(ctx, `UPDATE ai_audits SET status = ?, updated_at = ? WHERE status = ?`, toStatus, now, fromStatus)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *aiAuditStore) scanRows(rows *sqlx.Rows) ([]*models.AIAudit, error) {
	var out []*models.AIAudit
	for rows.Next() {
		var a models.AIAudit
		if err := rows.Scan(&a.ID, &a.EntityType, &a.EntityID, &a.Field, &a.CurrentValue, &a.AIValue, &a.Status, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &a)
	}
	return out, rows.Err()
}
